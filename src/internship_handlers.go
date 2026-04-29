package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// POST /internships — company creates internship (requires approved MOU)
func CreateInternship(c *gin.Context) {
	companyID, _ := c.Get("id")

	var mouStatus string
	err := DB.QueryRow(`SELECT status FROM mou_requests WHERE company_id=$1 ORDER BY created_at DESC LIMIT 1`, companyID).Scan(&mouStatus)
	if err != nil || mouStatus != "approved" {
		c.JSON(http.StatusForbidden, gin.H{"error": "MOU must be approved before posting internships"})
		return
	}

	var input Internship
	if err := c.BindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	if input.Title == "" || input.Description == "" || input.Duration == "" || input.Location == "" || input.Deadline == "" || input.Qualifications == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "All fields are required"})
		return
	}

	if input.PaymentPerDay <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payment per day must be greater than 0"})
		return
	}

	var id string
	err = DB.QueryRow(`
		INSERT INTO internships (company_id, title, description, duration, location, deadline, qualifications, payment_per_day, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'pending')
		RETURNING id
	`, companyID, input.Title, input.Description, input.Duration, input.Location, input.Deadline, input.Qualifications, input.PaymentPerDay).Scan(&id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create internship"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Internship submitted for approval", "id": id})
}

// PUT /internships/:id — company updates their own pending internship
func UpdateInternship(c *gin.Context) {
	internshipID := c.Param("id")
	companyID, _ := c.Get("id")

	// Verify ownership and that status is pending or rejected (allow editing rejected ones for resubmission)
	var ownerID, status string
	err := DB.QueryRow(`SELECT company_id, status FROM internships WHERE id=$1`, internshipID).Scan(&ownerID, &status)
	if err != nil || ownerID != companyID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if status == "approved" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot edit an approved internship"})
		return
	}

	var input Internship
	if err := c.BindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	if input.Title == "" || input.Description == "" || input.Duration == "" || input.Location == "" || input.Deadline == "" || input.Qualifications == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "All fields are required"})
		return
	}

	if input.PaymentPerDay <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Payment per day must be greater than 0"})
		return
	}

	// Editing a rejected internship resets it to pending so it gets re-reviewed
	_, err = DB.Exec(`
		UPDATE internships
		SET title=$1, description=$2, duration=$3, location=$4, deadline=$5, qualifications=$6, payment_per_day=$7,
		    status='pending', rejection_reason=NULL
		WHERE id=$8
	`, input.Title, input.Description, input.Duration, input.Location, input.Deadline, input.Qualifications, input.PaymentPerDay, internshipID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update internship"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Internship updated"})
}

// DELETE /internships/:id — company deletes their own internship (cascades to applications)
func DeleteInternship(c *gin.Context) {
	internshipID := c.Param("id")
	companyID, _ := c.Get("id")

	var ownerID string
	err := DB.QueryRow(`SELECT company_id FROM internships WHERE id=$1`, internshipID).Scan(&ownerID)
	if err != nil || ownerID != companyID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	_, err = DB.Exec(`DELETE FROM internships WHERE id=$1`, internshipID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete internship"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Internship deleted"})
}

// GET /internships/my — company views their own postings (with application counts)
func GetMyInternships(c *gin.Context) {
	companyID, _ := c.Get("id")

	rows, err := DB.Query(`
		SELECT i.id, i.title, i.description, i.duration, i.location, i.deadline, i.qualifications,
		       COALESCE(i.payment_per_day,0), i.status, i.created_at, COALESCE(i.rejection_reason,''),
		       (SELECT COUNT(*) FROM applications a WHERE a.internship_id = i.id) AS application_count
		FROM internships i
		WHERE i.company_id=$1
		ORDER BY i.created_at DESC
	`, companyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch internships"})
		return
	}
	defer rows.Close()

	internships := []Internship{}
	for rows.Next() {
		var item Internship
		rows.Scan(&item.ID, &item.Title, &item.Description, &item.Duration, &item.Location, &item.Deadline, &item.Qualifications,
			&item.PaymentPerDay, &item.Status, &item.CreatedAt, &item.RejectionReason, &item.ApplicationCount)
		internships = append(internships, item)
	}

	c.JSON(http.StatusOK, internships)
}

// GET /internships/pending — staff views internships awaiting approval
func GetPendingInternships(c *gin.Context) {
	rows, err := DB.Query(`
		SELECT i.id, i.company_id, u.first_name || ' ' || u.last_name AS company_name,
		       i.title, i.description, i.duration, i.location, i.deadline, i.qualifications,
		       COALESCE(i.payment_per_day,0), i.status, i.created_at, COALESCE(i.rejection_reason,'')
		FROM internships i
		JOIN users u ON u.id = i.company_id
		WHERE i.status='pending'
		ORDER BY i.created_at ASC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch internships"})
		return
	}
	defer rows.Close()

	internships := []Internship{}
	for rows.Next() {
		var item Internship
		rows.Scan(&item.ID, &item.CompanyID, &item.CompanyName, &item.Title, &item.Description, &item.Duration, &item.Location, &item.Deadline, &item.Qualifications,
			&item.PaymentPerDay, &item.Status, &item.CreatedAt, &item.RejectionReason)
		internships = append(internships, item)
	}

	c.JSON(http.StatusOK, internships)
}

// GET /internships/approved — approved internships (students will use this)
func GetApprovedInternships(c *gin.Context) {
	rows, err := DB.Query(`
		SELECT i.id, i.company_id, u.first_name || ' ' || u.last_name AS company_name,
		       i.title, i.description, i.duration, i.location, i.deadline, i.qualifications,
		       COALESCE(i.payment_per_day,0), i.status, i.created_at
		FROM internships i
		JOIN users u ON u.id = i.company_id
		WHERE i.status='approved'
		ORDER BY i.created_at DESC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch internships"})
		return
	}
	defer rows.Close()

	internships := []Internship{}
	for rows.Next() {
		var item Internship
		rows.Scan(&item.ID, &item.CompanyID, &item.CompanyName, &item.Title, &item.Description, &item.Duration, &item.Location, &item.Deadline, &item.Qualifications,
			&item.PaymentPerDay, &item.Status, &item.CreatedAt)
		internships = append(internships, item)
	}

	c.JSON(http.StatusOK, internships)
}

// PUT /internships/:id/status — staff approves or rejects internship
func UpdateInternshipStatus(c *gin.Context) {
	internshipID := c.Param("id")

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

	var err error
	if input.Status == "approved" {
		_, err = DB.Exec(`UPDATE internships SET status=$1, rejection_reason=NULL WHERE id=$2`, input.Status, internshipID)
	} else {
		_, err = DB.Exec(`UPDATE internships SET status=$1, rejection_reason=$2 WHERE id=$3`, input.Status, input.RejectionReason, internshipID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Internship status updated to " + input.Status})
}
