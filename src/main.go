package main

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	LoadEnv()
	InitDB()

	r := gin.Default()

	r.Use(cors.Default())

	// Public routes
	r.POST("/register", Register)
	r.POST("/login", Login)

	// Protected routes
	r.POST("/reset-password", JWTAuthMiddleware(), ResetPassword)

	r.Run(":8080")
}
