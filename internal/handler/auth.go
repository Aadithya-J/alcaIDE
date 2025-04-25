package handler

import (
	"encoding/json"
	"net/http"
	"fmt"
	"github.com/google/uuid"
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

func toUserResponse(user User) UserResponse {
    return UserResponse{
        Username: user.Username,
        ID:       user.ID,
        Email:    user.Email,
    }
}

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
	fmt.Println("Registered User :", user.Username, user.ID, user.Email, user.Password)
	
	json.NewEncoder(w).Encode(toUserResponse(user))
}



