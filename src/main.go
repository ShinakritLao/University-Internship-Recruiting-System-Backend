package main

import "github.com/gin-gonic/gin"

func main() {
    LoadEnv()
    InitDB()

    r := gin.Default()

    r.POST("/register", Register)
    r.POST("/login", Login)
    r.POST("/reset-password", ResetPassword)

    port := GetEnv("PORT")
    r.Run(":" + port)
}
