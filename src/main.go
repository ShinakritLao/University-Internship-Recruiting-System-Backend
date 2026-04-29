package main

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	LoadEnv()
	InitDB()

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	// Public routes
	r.POST("/register", Register)
	r.POST("/login", Login)
	r.GET("/internships/approved", GetApprovedInternships)

	// Protected routes
	auth := r.Group("/", JWTAuthMiddleware())
	{
		auth.POST("/reset-password", ResetPassword)

		// Company routes
		auth.POST("/internships", CreateInternship)
		auth.PUT("/internships/:id", UpdateInternship)
		auth.DELETE("/internships/:id", DeleteInternship)
		auth.GET("/internships/my", GetMyInternships)
		auth.POST("/mou-requests", CreateMOURequest)
		auth.GET("/mou-requests/my", GetMyMOURequest)
		auth.GET("/internships/:id/applications", GetApplicationsForInternship)
		auth.PUT("/applications/:id/status", UpdateApplicationStatus)

		// Staff routes
		auth.GET("/internships/pending", GetPendingInternships)
		auth.PUT("/internships/:id/status", UpdateInternshipStatus)
		auth.GET("/mou-requests", GetAllMOURequests)
		auth.PUT("/mou-requests/:id/status", UpdateMOUStatus)
		auth.GET("/applications", GetAllApplications)
	}

	r.Run(":8080")
}
