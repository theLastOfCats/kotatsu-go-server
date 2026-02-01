package api

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/theLastOfCats/kotatsu-go-server/internal/auth"
	"github.com/theLastOfCats/kotatsu-go-server/internal/db"
	"github.com/theLastOfCats/kotatsu-go-server/internal/model"
)

type AuthHandler struct {
	DB *db.DB
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	_, err = h.DB.Exec("INSERT INTO users (email, password_hash) VALUES (?, ?)", req.Email, hash)
	if err != nil {
		// Check for duplicate email error (SQLite specific error check could be added here)
        // For simplicity, generic error
		http.Error(w, "Failed to register user (email might be taken)", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var user model.User
	row := h.DB.QueryRow("SELECT id, password_hash FROM users WHERE email = ?", req.Email)
	err := row.Scan(&user.ID, &user.PasswordHash)
    
    // User not found
	if err == sql.ErrNoRows {
        // Attempt registration (Auto-register behavior to match original server)
        // In a real env, check config "ALLOW_NEW_REGISTER". For now, assume true.
        hash, err := auth.HashPassword(req.Password)
        if err != nil {
            http.Error(w, "Internal server error", http.StatusInternalServerError)
            return
        }
        res, err := h.DB.Exec("INSERT INTO users (email, password_hash) VALUES (?, ?)", req.Email, hash)
        if err != nil {
             http.Error(w, "Failed to register user", http.StatusInternalServerError)
             return
        }
        userID, _ := res.LastInsertId()
        
        token, err := auth.GenerateToken(userID)
        if err != nil {
            http.Error(w, "Failed to generate token", http.StatusInternalServerError)
            return
        }
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(map[string]string{"token": token})
        return
	} else if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

    // User found, verify password
	match, err := auth.VerifyPassword(req.Password, user.PasswordHash)
    if err != nil {
        http.Error(w, "Error verifying password", http.StatusInternalServerError)
        return
    }

	if !match {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := auth.GenerateToken(user.ID)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}
