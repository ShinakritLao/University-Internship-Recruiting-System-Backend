package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GET /internships/:id/applications — company views applications for their internship
func GetApplicationsForInternship(c *gin.Context) {
	internshipID := c.Param("id")
	companyID, _ := c.Get("id")

	// Verify this internship belongs to the requesting company
	var ownerID string
	err := DB.QueryRow(`SELECT company_id FROM internships WHERE id=$1`, internshipID).Scan(&ownerID)
	if err != nil || ownerID != companyID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	rows, err := DB.Query(`
		SELECT a.id, a.student_id, u.first_name || ' ' || u.last_name AS student_name,
		       a.internship_id, a.apply_date::text, a.status,
		       COALESCE(a.documents_path,''), COALESCE(a.cv_path,''), COALESCE(a.transcript_path,''), COALESCE(a.description,'')
		FROM applications a
		JOIN users u ON u.id = a.student_id
		WHERE a.internship_id=$1
		ORDER BY a.apply_date DESC
	`, internshipID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch applications"})
		return
	}
	defer rows.Close()

	applications := []Application{}
	for rows.Next() {
		var item Application
		rows.Scan(&item.ID, &item.StudentID, &item.StudentName, &item.InternshipID, &item.ApplyDate, &item.Status,
			&item.DocumentsPath, &item.CVPath, &item.TranscriptPath, &item.Description)
		applications = append(applications, item)
	}

	c.JSON(http.StatusOK, applications)
}

// GET /applications/my — staff views all applications across all internships
func GetAllApplications(c *gin.Context) {
	rows, err := DB.Query(`
		SELECT a.id, a.student_id, u.first_name || ' ' || u.last_name AS student_name,
		       a.internship_id, i.title AS internship_title, a.apply_date::text, a.status,
		       COALESCE(a.documents_path,''), COALESCE(a.cv_path,''), COALESCE(a.transcript_path,''), COALESCE(a.description,'')
		FROM applications a
		JOIN users u ON u.id = a.student_id
		JOIN internships i ON i.id = a.internship_id
		ORDER BY a.apply_date DESC
	`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch applications"})
		return
	}
	defer rows.Close()

	applications := []Application{}
	for rows.Next() {
		var item Application
		rows.Scan(&item.ID, &item.StudentID, &item.StudentName, &item.InternshipID, &item.InternshipTitle, &item.ApplyDate, &item.Status,
			&item.DocumentsPath, &item.CVPath, &item.TranscriptPath, &item.Description)
		applications = append(applications, item)
	}

	c.JSON(http.StatusOK, applications)
}

// PUT /applications/:id/status — company updates application status (under-review, accepted, rejected)
func UpdateApplicationStatus(c *gin.Context) {
	applicationID := c.Param("id")
	companyID, _ := c.Get("id")

	var input struct {
		Status string `json:"status"`
	}
	if err := c.BindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	allowed := map[string]bool{"under-review": true, "accepted": true, "rejected": true}
	if !allowed[input.Status] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Status must be 'under-review', 'accepted', or 'rejected'"})
		return
	}

	// Verify the application belongs to an internship owned by this company
	var ownerID string
	err := DB.QueryRow(`
		SELECT i.company_id FROM applications a
		JOIN internships i ON i.id = a.internship_id
		WHERE a.id=$1
	`, applicationID).Scan(&ownerID)
	if err != nil || ownerID != companyID.(string) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	res, err := DB.Exec(`UPDATE applications SET status=$1 WHERE id=$2`, input.Status, applicationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update application status"})
		return
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Application not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Application status updated to " + input.Status})
}
