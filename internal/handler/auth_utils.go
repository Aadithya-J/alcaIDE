package handler

import (
	"time"
	"os"

	"github.com/Aadithya-J/alcaIDE/model"
	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

func toUserResponse(user model.User) model.UserResponse {
	return model.UserResponse{
		Username: user.Username,
		ID:       user.ID,
		Email:    user.Email,
	}
}

func generateJWT(user model.User) (string, error) {
	claims := jwt.MapClaims{
		"user_id":       user.ID,
		"user_email":    user.Email,
		"user_username": user.Username,
		"exp":           time.Now().Add(time.Hour * 72).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}
