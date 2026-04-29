package main

type User struct {
    ID        string
    Email     string
    Password  string
    FirstName string
    LastName  string
    UserID    string
    Role      string
}

type Internship struct {
	ID               string  `json:"id"`
	CompanyID        string  `json:"companyId"`
	CompanyName      string  `json:"companyName"`
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	Duration         string  `json:"duration"`
	Location         string  `json:"location"`
	Deadline         string  `json:"deadline"`
	Qualifications   string  `json:"qualifications"`
	PaymentPerDay    float64 `json:"paymentPerDay"`
	Status           string  `json:"status"`
	CreatedAt        string  `json:"createdAt"`
	RejectionReason  string  `json:"rejectionReason"`
	ApplicationCount int     `json:"applicationCount"`
}

type MOURequest struct {
	ID              string `json:"id"`
	CompanyID       string `json:"companyId"`
	CompanyName     string `json:"companyName"`
	Message         string `json:"message"`
	Status          string `json:"status"`
	ReviewedAt      string `json:"reviewedAt"`
	CreatedAt       string `json:"createdAt"`
	DocumentPath    string `json:"documentPath"`
	RejectionReason string `json:"rejectionReason"`
	ExpiresAt       string `json:"expiresAt"`
}

type Application struct {
	ID              string `json:"id"`
	StudentID       string `json:"studentId"`
	StudentName     string `json:"studentName"`
	InternshipID    string `json:"internshipId"`
	InternshipTitle string `json:"internshipTitle"`
	ApplyDate       string `json:"applyDate"`
	Status          string `json:"status"`
	DocumentsPath   string `json:"documentsPath"`
	CVPath          string `json:"cvPath"`
	TranscriptPath  string `json:"transcriptPath"`
	Description     string `json:"description"`
}