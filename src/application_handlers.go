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

// POST /internships/:id/apply
func ApplyForInternship(c *gin.Context) {
	studentID, _ := c.Get("id")
	internshipID := c.Param("id")

	// Check internship exists and is approved
	var exists bool
	err := DB.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM internships
			WHERE id=$1 AND status='approved'
		)
	`, internshipID).Scan(&exists)

	if err != nil || !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Internship not found",
		})
		return
	}

	// Prevent duplicate applications
	var alreadyApplied bool
	err = DB.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM applications
			WHERE internship_id=$1 AND student_id=$2
		)
	`, internshipID, studentID).Scan(&alreadyApplied)

	if alreadyApplied {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "You already applied for this internship",
		})
		return
	}

	description := c.PostForm("description")

	var cvPath string
	var transcriptPath string

	cvFile, err := c.FormFile("cv")
	if err == nil {
		cvPath, err = SaveUploadedFile(cvFile, "uploads/cv")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to upload CV",
			})
			return
		}
	}

	transcriptFile, err := c.FormFile("transcript")
	if err == nil {
		transcriptPath, err = SaveUploadedFile(transcriptFile, "uploads/transcripts")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to upload transcript",
			})
			return
		}
	}

	var applicationID string

	err = DB.QueryRow(`
		INSERT INTO applications
		(student_id, internship_id, apply_date, status, cv_path, transcript_path, description)
		VALUES ($1,$2,NOW(),'submitted',$3,$4,$5)
		RETURNING id
	`,
		studentID,
		internshipID,
		cvPath,
		transcriptPath,
		description,
	).Scan(&applicationID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to submit application",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Application submitted successfully",
		"id":      applicationID,
	})
}

// GET /applications/my
func GetMyApplications(c *gin.Context) {
	studentID, _ := c.Get("id")

	rows, err := DB.Query(`
		SELECT a.id,
		       a.internship_id,
		       i.title,
		       a.apply_date::text,
		       a.status,
		       COALESCE(a.cv_path,''),
		       COALESCE(a.transcript_path,''),
		       COALESCE(a.description,'')
		FROM applications a
		JOIN internships i ON i.id = a.internship_id
		WHERE a.student_id=$1
		ORDER BY a.apply_date DESC
	`, studentID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch applications",
		})
		return
	}

	defer rows.Close()

	applications := []Application{}

	for rows.Next() {
		var item Application

		rows.Scan(
			&item.ID,
			&item.InternshipID,
			&item.InternshipTitle,
			&item.ApplyDate,
			&item.Status,
			&item.CVPath,
			&item.TranscriptPath,
			&item.Description,
		)

		applications = append(applications, item)
	}

	c.JSON(http.StatusOK, applications)
}

// GET /profile/my
func GetMyProfile(c *gin.Context) {
	userID, _ := c.Get("id")

	var user User

	err := DB.QueryRow(`
		SELECT id,
		       email,
		       first_name,
		       last_name,
		       user_id,
		       role
		FROM users
		WHERE id=$1
	`, userID).Scan(
		&user.ID,
		&user.Email,
		&user.FirstName,
		&user.LastName,
		&user.UserID,
		&user.Role,
	)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":        user.ID,
		"name":      user.FirstName + " " + user.LastName,
		"email":     user.Email,
		"studentId": user.UserID,
		"role":      user.Role,
	})
}