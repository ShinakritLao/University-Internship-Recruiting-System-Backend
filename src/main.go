package main

import (
    "github.com/gin-gonic/gin"
    "github.com/gin-contrib/cors"
)

func main() {
    LoadEnv()
    InitDB()

    r := gin.Default()

    r.Use(cors.Default())

    r.POST("/register", Register)
    r.POST("/login", Login)
    r.POST("/reset-password", ResetPassword)

    r.Run(":8080")
}