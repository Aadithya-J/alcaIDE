package model

import "github.com/google/uuid"

type User struct {
	Username string `json:"username"`
	ID       uuid.UUID `json:"id"`
	Email    string `json:"email"`
	Password string `json:"password"`
}