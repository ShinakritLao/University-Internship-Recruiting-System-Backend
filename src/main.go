package main

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	LoadEnv()
	InitDB()

	r := gin.Default()

	r.Use(cors.Default())

	// Health check
	r.GET("/health", healthCheck)

	// Public routes
	r.POST("/register", Register)
	r.POST("/login", Login)

	// Protected routes
	r.POST("/reset-password", JWTAuthMiddleware(), ResetPassword)

	r.Run(":8080")
}

func healthCheck(c *gin.Context) {
	if err := DB.Ping(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "unhealthy",
			"error":  err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
