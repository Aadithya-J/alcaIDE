package model

import "github.com/google/uuid"

type UserResponse struct {
    Username string    `json:"username"`
    ID       uuid.UUID `json:"id"`
    Email    string    `json:"email"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthResponse struct {
	UserResponse
	Token string `json:"token"`
}