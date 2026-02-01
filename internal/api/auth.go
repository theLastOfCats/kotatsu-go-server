package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/theLastOfCats/kotatsu-go-server/internal/auth"
	"github.com/theLastOfCats/kotatsu-go-server/internal/db"
	"github.com/theLastOfCats/kotatsu-go-server/internal/mail"
	"github.com/theLastOfCats/kotatsu-go-server/internal/model"
	"github.com/theLastOfCats/kotatsu-go-server/internal/templates"
)

type AuthHandler struct {
	DB        *db.DB
	Mailer    mail.MailSender
	Templates *templates.Manager
	BaseURL   string
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

func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := h.DB.GetUserByEmail(req.Email)
	if err != nil {
		// User not found: return OK safely
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode("A password reset email was sent")
		return
	}

	// Generate reset token
	token, hash, err := auth.GenerateResetToken()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	expiresAt := time.Now().Add(1 * time.Hour).Unix()

	if err := h.DB.SetPasswordResetToken(user.ID, hash, expiresAt); err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// Send Email
	link := fmt.Sprintf("%s/deeplink/reset-password?token=%s", h.BaseURL, token)

	htmlBody, err := h.Templates.Render("mail/forgot-password.html", map[string]string{"ResetPasswordLink": link})
	if err != nil {
		// Just log error, email might fail or send plain text if we had fallback
		fmt.Printf("Template render error: %v\n", err)
	}

	err = h.Mailer.Send(user.Email, "Password reset", "Reset link: "+link, htmlBody)
	if err != nil {
		fmt.Printf("Mail send error: %v\n", err)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode("A password reset email was sent")
}

func (h *AuthHandler) ResetPasswordDeeplink(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Missing token", http.StatusBadRequest)
		return
	}

	// DeepLink scheme: kotatsu://reset-password?base_url=...&token=...
	deepLink := fmt.Sprintf("kotatsu://reset-password?base_url=%s&token=%s", h.BaseURL, token)

	html, err := h.Templates.Render("pages/reset-password.html", map[string]string{"DeepLink": deepLink})
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ResetToken string `json:"reset_token"`
		Password   string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	tokenHash := auth.HashToken(req.ResetToken)
	user, err := h.DB.GetUserByResetToken(tokenHash)
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusBadRequest)
		return
	}

	if user.PasswordResetTokenExpires == nil || *user.PasswordResetTokenExpires < time.Now().Unix() {
		http.Error(w, "Invalid or expired token", http.StatusBadRequest)
		return
	}

	if len(req.Password) < 2 || len(req.Password) > 24 {
		http.Error(w, "Password should be from 2 to 24 characters long", http.StatusBadRequest)
		return
	}

	newHash, err := auth.HashPassword(req.Password)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	if err := h.DB.UpdatePassword(user.ID, newHash); err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	h.DB.ClearResetToken(user.ID)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode("Password has been reset successfully")
}
