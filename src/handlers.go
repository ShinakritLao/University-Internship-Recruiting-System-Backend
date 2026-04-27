package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func Register(c *gin.Context) {
	var input User
	c.BindJSON(&input)

	if !isValidEmail(input.Email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email"})
		return
	}

	if !isValidPassword(input.Password) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Weak password"})
		return
	}

	role := detectRole(input.UserID)
	if role == "invalid" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID"})
		return
	}

	hashed, _ := bcrypt.GenerateFromPassword([]byte(input.Password), 10)

	_, err := DB.Exec(`
        insert into users (email, password, first_name, last_name, user_id, role)
        values ($1,$2,$3,$4,$5,$6)
    `,
		input.Email, string(hashed), input.FirstName, input.LastName, input.UserID, role,
	)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email already exists"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Registered"})
}

//-------------------------------------------------------------------------------------------------------//

func Login(c *gin.Context) {
	var input User
	c.BindJSON(&input)

	var user User

	err := DB.QueryRow(`
        select id, email, password, role, user_id, first_name, last_name
        from users where email=$1
    `, input.Email).Scan(&user.ID, &user.Email, &user.Password, &user.Role, &user.UserID, &user.FirstName, &user.LastName)

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password))
	if err != nil || user.UserID != input.UserID {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	token, _ := GenerateJWT(user)

	c.JSON(http.StatusOK, gin.H{
		"token":     token,
		"role":      user.Role,
		"id":        user.ID,
		"email":     user.Email,
		"firstName": user.FirstName,
		"lastName":  user.LastName,
	})
}

//-------------------------------------------------------------------------------------------------------//

func ResetPassword(c *gin.Context) {
	var input struct {
		NewPassword string `json:"newPassword"`
	}

	c.BindJSON(&input)

	// Get user info from JWT token
	email, exists := c.Get("email")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	if !isValidPassword(input.NewPassword) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Weak password"})
		return
	}

	hashed, _ := bcrypt.GenerateFromPassword([]byte(input.NewPassword), 10)

	res, _ := DB.Exec(`
        update users set password=$1 where email=$2 and user_id=$3
    `, string(hashed), email, userID)

	rows, _ := res.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to update password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password updated"})
}
