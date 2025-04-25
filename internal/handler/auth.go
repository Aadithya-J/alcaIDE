package handler

import (
	"fmt"
	"log"
	"context"
	"time"
	"os"
	"encoding/json"
	"net/http"

	"github.com/Aadithya-J/alcaIDE/internal/db"

	"github.com/google/uuid"
	"github.com/golang-jwt/jwt/v5"
)

type User struct {
	Username string `json:"username"`
	ID       uuid.UUID `json:"id"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserResponse struct {
    Username string    `json:"username"`
    ID       uuid.UUID `json:"id"`
    Email    string    `json:"email"`
}

var jwtSecret = []byte(os.Getenv("JWT_SECRET"))

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var user User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	user.ID = uuid.New();

	res, err := db.Conn.Exec(context.Background(),
		"INSERT INTO USERS (id, username, email, password) VALUES ($1, $2, $3, $4)",
		user.ID,user.Username,user.Email,user.Password,
	)
	if err != nil {
		log.Println("Error inserting user into database:", err)
		http.Error(w, "Failed to register user",http.StatusInternalServerError)
		return
	}
	log.Println("User inserted into database:", res)
	fmt.Println("Registered User :", user.Username, user.ID, user.Email, user.Password)


	claims := jwt.MapClaims{
        "user_id": user.ID,
		"user_email" : user.Email,
		"user_username" : user.Username,
        "exp":     time.Now().Add(time.Hour * 72).Unix(),
    }

	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err := t.SignedString(jwtSecret)

	if err != nil {
		log.Fatal("Error Signing jwt.")
		return
	}
	respWithToken := struct {
		UserResponse
		Token string `json:"token"`
	}{
		UserResponse: toUserResponse(user),
		Token : token,
	}

	json.NewEncoder(w).Encode(respWithToken)
}

func toUserResponse(user User) UserResponse {
    return UserResponse{
        Username: user.Username,
        ID:       user.ID,
        Email:    user.Email,
    }
}