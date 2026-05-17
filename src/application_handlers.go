package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

var confirmDeadline = time.Date(
	2026,
	time.June,
	30,
	23,
	59,
	59,
	0,
	time.UTC,
)

// GET /internships/:id/applications
func GetApplicationsForInternship(c *gin.Context) {
	internshipID := c.Param("id")
	companyID, _ := c.Get("id")

	var ownerID string

	err := DB.QueryRow(`
		SELECT company_id
		FROM internships
		WHERE id=$1
	`, internshipID).Scan(&ownerID)

	if err != nil || ownerID != companyID.(string) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied",
		})
		return
	}

	rows, err := DB.Query(`
		SELECT a.id,
		       a.student_id,
		       u.first_name || ' ' || u.last_name AS student_name,
		       a.internship_id,
		       a.apply_date::text,
		       a.status,
		       COALESCE(a.documents_path,''),
		       COALESCE(a.cv_path,''),
		       COALESCE(a.transcript_path,''),
		       COALESCE(a.description,'')
		FROM applications a
		JOIN users u ON u.id = a.student_id
		WHERE a.internship_id=$1
		ORDER BY a.apply_date DESC
	`, internshipID)

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
			&item.StudentID,
			&item.StudentName,
			&item.InternshipID,
			&item.ApplyDate,
			&item.Status,
			&item.DocumentsPath,
			&item.CVPath,
			&item.TranscriptPath,
			&item.Description,
		)

		applications = append(applications, item)
	}

	c.JSON(http.StatusOK, applications)
}

// GET /applications
func GetAllApplications(c *gin.Context) {
	rows, err := DB.Query(`
		SELECT a.id,
		       a.student_id,
		       u.first_name || ' ' || u.last_name AS student_name,
		       a.internship_id,
		       i.title,
		       a.apply_date::text,
		       a.status,
		       COALESCE(a.documents_path,''),
		       COALESCE(a.cv_path,''),
		       COALESCE(a.transcript_path,''),
		       COALESCE(a.description,'')
		FROM applications a
		JOIN users u ON u.id = a.student_id
		JOIN internships i ON i.id = a.internship_id
		ORDER BY a.apply_date DESC
	`)

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
			&item.StudentID,
			&item.StudentName,
			&item.InternshipID,
			&item.InternshipTitle,
			&item.ApplyDate,
			&item.Status,
			&item.DocumentsPath,
			&item.CVPath,
			&item.TranscriptPath,
			&item.Description,
		)

		applications = append(applications, item)
	}

	c.JSON(http.StatusOK, applications)
}

// PUT /applications/:id/status
func UpdateApplicationStatus(c *gin.Context) {
	applicationID := c.Param("id")
	companyID, _ := c.Get("id")

	var input struct {
		Status string `json:"status"`
	}

	if err := c.BindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid input",
		})
		return
	}

	allowed := map[string]bool{
		"submitted":    true,
		"under-review": true,
		"accepted":     true,
		"rejected":     true,
		"confirmed":    true,
		"withdrawn":    true,
	}

	if !allowed[input.Status] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid status",
		})
		return
	}

	// check ownership + get data for notification
	var internshipTitle string
	var studentID string
	var ownerID string

	err := DB.QueryRow(`
		SELECT i.company_id, i.title, a.student_id
		FROM applications a
		JOIN internships i ON i.id = a.internship_id
		WHERE a.id=$1
	`, applicationID).Scan(&ownerID, &internshipTitle, &studentID)

	if err != nil || ownerID != companyID.(string) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Access denied",
		})
		return
	}

	// update application status
	_, err = DB.Exec(`
		UPDATE applications
		SET status=$1
		WHERE id=$2
	`, input.Status, applicationID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update application",
		})
		return
	}

	// insert notification (ONLY ONCE, CLEAN)
	message := "📢 Application " + internshipTitle + " updated to: " + input.Status

	_, _ = DB.Exec(`
		INSERT INTO notifications (user_id, message)
		VALUES ($1, $2)
	`, studentID, message)

	c.JSON(http.StatusOK, gin.H{
		"message": "Application updated",
	})
}

// POST /internships/:id/apply
func ApplyForInternship(c *gin.Context) {
	studentID, _ := c.Get("id")
	internshipID := c.Param("id")

	var deadline time.Time

	err := DB.QueryRow(`
		SELECT deadline
		FROM internships
		WHERE id=$1
	`, internshipID).Scan(&deadline)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Internship not found",
		})
		return
	}

	if time.Now().After(deadline) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Application deadline has passed",
		})
		return
	}

	var confirmedCount int

	DB.QueryRow(`
		SELECT COUNT(*)
		FROM applications
		WHERE student_id=$1
		AND status='confirmed'
	`, studentID).Scan(&confirmedCount)

	if confirmedCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "You already confirmed another internship",
		})
		return
	}

	var existingID string

	err = DB.QueryRow(`
		SELECT id
		FROM applications
		WHERE student_id=$1
		AND internship_id=$2
		LIMIT 1
	`,
		studentID,
		internshipID,
	).Scan(&existingID)

	if err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "You already applied for this internship",
		})
		return
	}

	description := c.PostForm("description")

	cvPath := ""
	transcriptPath := ""

	cvFile, err := c.FormFile("cv")

	if err == nil {
		cvPath, err = UploadToSupabase("application-documents", cvFile)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to upload CV",
			})
			return
		}
	}

	transcriptFile, err := c.FormFile("transcript")

	if err == nil {
		transcriptPath, err = UploadToSupabase("application-documents", transcriptFile)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to upload transcript",
			})
			return
		}
	}

	var id string

	err = DB.QueryRow(`
		INSERT INTO applications (
			student_id,
			internship_id,
			description,
			cv_path,
			transcript_path,
			status
		)
		VALUES ($1,$2,$3,$4,$5,'submitted')
		RETURNING id
	`,
		studentID,
		internshipID,
		description,
		cvPath,
		transcriptPath,
	).Scan(&id)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to submit application",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Application submitted successfully",
		"id":      id,
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

// DELETE /applications/:id
func DeleteMyApplication(c *gin.Context) {
	studentID, _ := c.Get("id")
	applicationID := c.Param("id")

	var status string

	err := DB.QueryRow(`
		SELECT status
		FROM applications
		WHERE id=$1
		AND student_id=$2
	`,
		applicationID,
		studentID,
	).Scan(&status)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Application not found",
		})
		return
	}

	if status != "submitted" && status != "under-review" {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Only submitted and under-reviewed applications can be deleted",
		})
		return
	}

	_, err = DB.Exec(`
		DELETE FROM applications
		WHERE id=$1
		AND student_id=$2
	`,
		applicationID,
		studentID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete application",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Application deleted",
	})
}

// POST /applications/:id/confirm
func ConfirmApplication(c *gin.Context) {
	studentID, _ := c.Get("id")
	applicationID := c.Param("id")

	if time.Now().After(confirmDeadline) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Confirmation deadline has passed",
		})
		return
	}

	var status string

	err := DB.QueryRow(`
		SELECT status
		FROM applications
		WHERE id=$1
		AND student_id=$2
	`,
		applicationID,
		studentID,
	).Scan(&status)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Application not found",
		})
		return
	}

	if status != "accepted" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Only accepted applications can be confirmed",
		})
		return
	}

	var confirmedCount int

	DB.QueryRow(`
		SELECT COUNT(*)
		FROM applications
		WHERE student_id=$1
		AND status='confirmed'
	`, studentID).Scan(&confirmedCount)

	if confirmedCount > 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "You already confirmed another internship",
		})
		return
	}

	tx, err := DB.Begin()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to start transaction",
		})
		return
	}

	_, err = tx.Exec(`
		UPDATE applications
		SET status='confirmed'
		WHERE id=$1
	`, applicationID)

	if err != nil {
		tx.Rollback()

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to confirm application",
		})
		return
	}

	_, err = tx.Exec(`
		UPDATE applications
		SET status='withdrawn'
		WHERE student_id=$1
		AND id != $2
		AND status IN ('accepted', 'under-review', 'submitted')
	`,
		studentID,
		applicationID,
	)

	if err != nil {
		tx.Rollback()

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to withdraw applications",
		})
		return
	}

	err = tx.Commit()

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to finalize confirmation",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Internship confirmed successfully",
	})
}

// GET /notifications/my
func GetMyNotifications(c *gin.Context) {
	userID, _ := c.Get("id")

	rows, err := DB.Query(`
		SELECT id, message, is_read, created_at
		FROM notifications
		WHERE user_id=$1
		ORDER BY created_at DESC
	`, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch notifications",
		})
		return
	}
	defer rows.Close()

	type Notification struct {
		ID        string `json:"id"`
		Message   string `json:"message"`
		IsRead    bool   `json:"is_read"`
		CreatedAt string `json:"created_at"`
	}

	var notifications []Notification

	for rows.Next() {
		var n Notification
		rows.Scan(&n.ID, &n.Message, &n.IsRead, &n.CreatedAt)
		notifications = append(notifications, n)
	}

	c.JSON(http.StatusOK, notifications)
}

// PUT /notifications/:id/read
func MarkNotificationRead(c *gin.Context) {
	notificationID := c.Param("id")
	userID, _ := c.Get("id")

	res, err := DB.Exec(`
		UPDATE notifications
		SET is_read = true
		WHERE id = $1 AND user_id = $2
	`, notificationID, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update notification",
		})
		return
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Notification not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Notification marked as read",
	})
}

// GET /profile/my
func GetMyProfile(c *gin.Context) {
	userIDRaw, _ := c.Get("id")

	// assume middleware already gives correct ID now
	userID, _ := userIDRaw.(string)

	// user info
	var name, email, studentID string
	err := DB.QueryRow(`
		SELECT first_name || ' ' || last_name, email, user_id
		FROM users
		WHERE id = $1
	`, userID).Scan(&name, &email, &studentID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "user not found"})
		return
	}

	// confirmed internship (STRICT + SAFE)
	var internshipTitle string
	err = DB.QueryRow(`
		SELECT i.title
		FROM applications a
		JOIN internships i ON i.id = a.internship_id
		WHERE a.student_id = $1
		AND a.status = 'confirmed'
		LIMIT 1
	`, userID).Scan(&internshipTitle)

	// IMPORTANT: if no confirmed internship → empty string (NOT error)
	if err != nil {
		internshipTitle = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"name":       name,
		"email":      email,
		"studentId":  studentID,
		"internship": internshipTitle,
	})
}