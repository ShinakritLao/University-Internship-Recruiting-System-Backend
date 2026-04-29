package main

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
)

// POST /mou-requests — company submits a MOU request (multipart/form-data: message + document)
func CreateMOURequest(c *gin.Context) {
	companyID, _ := c.Get("id")

	// Block if a pending or approved MOU already exists
	var existingStatus string
	err := DB.QueryRow(`SELECT status FROM mou_requests WHERE company_id=$1 ORDER BY created_at DESC LIMIT 1`, companyID).Scan(&existingStatus)
	if err == nil && (existingStatus == "pending" || existingStatus == "approved") {
		c.JSON(http.StatusConflict, gin.H{"error": "MOU request already " + existingStatus})
		return
	}

	message := c.PostForm("message")

	file, err := c.FormFile("document")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "MOU document (PDF) is required"})
		return
	}

	if file.Header.Get("Content-Type") != "application/pdf" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only PDF files are accepted"})
		return
	}

	publicURL, err := UploadToSupabase("mou-documents", file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload document: " + err.Error()})
		return
	}

	var id string
	err = DB.QueryRow(`
		INSERT INTO mou_requests (company_id, message, status, document_path)
		VALUES ($1,$2,'pending',$3)
		RETURNING id
	`, companyID, message, publicURL).Scan(&id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit MOU request"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "MOU request submitted", "id": id, "documentPath": publicURL})
}

// GET /mou-requests/my — company views their own MOU status
func GetMyMOURequest(c *gin.Context) {
	companyID, _ := c.Get("id")

	var mou MOURequest
	err := DB.QueryRow(`
		SELECT id, company_id, COALESCE(message,''), status, COALESCE(reviewed_at::text,''), created_at,
		       COALESCE(document_path,''), COALESCE(rejection_reason,''), COALESCE(expires_at::text,'')
		FROM mou_requests
		WHERE company_id=$1
		ORDER BY created_at DESC
		LIMIT 1
	`, companyID).Scan(&mou.ID, &mou.CompanyID, &mou.Message, &mou.Status, &mou.ReviewedAt, &mou.CreatedAt,
		&mou.DocumentPath, &mou.RejectionReason, &mou.ExpiresAt)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No MOU request found"})
		return
	}

	c.JSON(http.StatusOK, mou)
}

// GET /mou-requests — staff views all MOU requests
func GetAllMOURequests(c *gin.Context) {
	rows, err := DB.Query(`
		SELECT m.id, m.company_id, u.first_name || ' ' || u.last_name AS company_name,
		       COALESCE(m.message,''), m.status, COALESCE(m.reviewed_at::text,''), m.created_at,
		       COALESCE(m.document_path,''), COALESCE(m.rejection_reason,''), COALESCE(m.expires_at::text,'')
		FROM mou_requests m
		JOIN users u ON u.id = m.company_id
		ORDER BY m.created_at DESC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch MOU requests"})
		return
	}
	defer rows.Close()

	requests := []MOURequest{}
	for rows.Next() {
		var item MOURequest
		rows.Scan(&item.ID, &item.CompanyID, &item.CompanyName, &item.Message, &item.Status, &item.ReviewedAt, &item.CreatedAt,
			&item.DocumentPath, &item.RejectionReason, &item.ExpiresAt)
		requests = append(requests, item)
	}

	c.JSON(http.StatusOK, requests)
}

// PUT /mou-requests/:id/status — staff approves or rejects MOU
func UpdateMOUStatus(c *gin.Context) {
	mouID := c.Param("id")
	staffID, _ := c.Get("id")

	var input struct {
		Status          string `json:"status"`
		RejectionReason string `json:"rejectionReason"`
	}
	if err := c.BindJSON(&input); err != nil || (input.Status != "approved" && input.Status != "rejected") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Status must be 'approved' or 'rejected'"})
		return
	}

	if input.Status == "rejected" && input.RejectionReason == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Rejection reason is required when rejecting"})
		return
	}

	var res sql.Result
	var err error
	if input.Status == "approved" {
		// Set expiration to 1 year from now on approval
		res, err = DB.Exec(`
			UPDATE mou_requests
			SET status=$1, reviewed_by=$2, reviewed_at=NOW(), expires_at=NOW() + INTERVAL '1 year', rejection_reason=NULL
			WHERE id=$3
		`, input.Status, staffID, mouID)
	} else {
		res, err = DB.Exec(`
			UPDATE mou_requests
			SET status=$1, reviewed_by=$2, reviewed_at=NOW(), rejection_reason=$3, expires_at=NULL
			WHERE id=$4
		`, input.Status, staffID, input.RejectionReason, mouID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update MOU status"})
		return
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "MOU request not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "MOU status updated to " + input.Status})
}
