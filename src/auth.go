package main

import (
    "time"
    "github.com/golang-jwt/jwt/v5"
)

func GenerateJWT(user User) (string, error) {
    jwtKey := []byte(GetEnv("JWT_SECRET"))

    claims := jwt.MapClaims{
        "email": user.Email,
        "role":  user.Role,
        "exp":   time.Now().Add(time.Hour * 24).Unix(),
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(jwtKey)
}