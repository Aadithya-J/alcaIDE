package handler

import (
	"log"
	"encoding/json"
	"net/http"

	"github.com/Aadithya-J/alcaIDE/internal/db"
	"github.com/Aadithya-J/alcaIDE/model"

	"github.com/google/uuid"
)


func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var err error
	var user model.User
	
	if err = json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	user.ID = uuid.New()
	user.Password, err = HashPassword(user.Password)
	if err != nil {
		log.Println("Error hashing password:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	_, err = db.Conn.Exec(r.Context(),
		"INSERT INTO USERS (id, username, email, password) VALUES ($1, $2, $3, $4)",
		user.ID, user.Username, user.Email, user.Password,
	)
	if err != nil {
		log.Println("Error inserting user:", err)
		http.Error(w, "Failed to register user", http.StatusInternalServerError)
		return
	}

	token, err := generateJWT(user)
	if err != nil {
		log.Println("Error Signing jwt.")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, model.AuthResponse{
		UserResponse: toUserResponse(user),
		Token:        token,
	})
}


func LoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req model.LoginRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var user model.User

	err = db.Conn.QueryRow(r.Context(),
		"SELECT id, username, email, password FROM users WHERE username = $1",
		req.Username,
	).Scan(&user.ID,&user.Username,&user.Email,&user.Password)

	if err != nil{
		http.Error(w, "Invalid Credentials", http.StatusUnauthorized)
		log.Println("Incorrect Credentials",err)
		return
	}

	if !ComparePasswordHash(req.Password, user.Password){
		http.Error(w, "Invalid Credentials", http.StatusUnauthorized)
		log.Println("Incorrect Credentials")
		return
	}

	token, err := generateJWT(user)

	if err != nil {
		log.Println("Error Signing jwt.")
		return
	}

	respondJSON(w, model.AuthResponse{
		UserResponse: toUserResponse(user),
		Token:        token,
	})

}