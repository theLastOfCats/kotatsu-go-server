package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/theLastOfCats/kotatsu-go-server/internal/auth"
	"github.com/theLastOfCats/kotatsu-go-server/internal/templates"
	"github.com/theLastOfCats/kotatsu-go-server/internal/testutil"
)

func TestHealth(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(Health)

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := "Alive"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v", rr.Body.String(), expected)
	}
}

func TestForgotPassword(t *testing.T) {
	database := testutil.SetupTestDB(t)
	defer database.Close()

	// Seed user
	email := "test@example.com"
	_, err := database.Exec("INSERT INTO users (email, password_hash) VALUES (?, ?)", email, "dummyhash")
	if err != nil {
		t.Fatalf("Failed to seed user: %v", err)
	}

	mailer := &testutil.MockMailSender{}
	// Use relative path to templates from internal/api
	tmplMgr := templates.NewManager("../../templates")

	handler := &AuthHandler{
		DB:        database,
		Mailer:    mailer,
		Templates: tmplMgr,
		BaseURL:   "http://test.local",
	}

	payload := map[string]string{"email": email}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/forgot-password", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.ForgotPassword(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	if len(mailer.SentEmails) != 1 {
		t.Errorf("Expected 1 email sent, got %d", len(mailer.SentEmails))
	} else {
		sent := mailer.SentEmails[0]
		if sent.To != email {
			t.Errorf("Expected email to %s, got %s", email, sent.To)
		}
	}

	// Verify token in DB
	user, err := database.GetUserByEmail(email)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if user.PasswordResetTokenHash == nil {
		t.Error("Expected password reset token to be set in DB")
	}
}

func TestResetPassword(t *testing.T) {
	database := testutil.SetupTestDB(t)
	defer database.Close()

	// Generate a token
	token, hash, _ := auth.GenerateResetToken()
	expires := time.Now().Add(1 * time.Hour).Unix()

	// Seed user with token
	res, err := database.Exec("INSERT INTO users (email, password_hash, password_reset_token_hash, password_reset_token_expires_at) VALUES (?, ?, ?, ?)",
		"reset@example.com", "oldhash", hash, expires)
	if err != nil {
		t.Fatalf("Failed to seed user: %v", err)
	}
	userID, _ := res.LastInsertId()

	handler := &AuthHandler{DB: database}

	newPass := "newsecurepassword"
	payload := map[string]string{
		"reset_token": token,
		"password":    newPass,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/reset-password", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.ResetPassword(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v body: %s", status, http.StatusOK, rr.Body.String())
	}

	// Verify password changed
	user, _ := database.GetUserByID(userID)
	match, err := auth.VerifyPassword(newPass, user.PasswordHash)
	if err != nil {
		t.Fatalf("Error verifying password: %v", err)
	}
	if !match {
		t.Error("Password should match new password")
	}

	// Verify token cleared
	if user.PasswordResetTokenHash != nil {
		t.Error("Token should be cleared after reset")
	}
}

func TestGetMe(t *testing.T) {
	database := testutil.SetupTestDB(t)
	defer database.Close()

	email := "me@example.com"
	res, _ := database.Exec("INSERT INTO users (email, password_hash) VALUES (?, ?)", email, "hash")
	userID, _ := res.LastInsertId()

	handler := &UserHandler{DB: database}

	req, _ := http.NewRequest("GET", "/me", nil)
	// Inject user_id into context (simulating AuthMiddleware)
	ctx := context.WithValue(req.Context(), UserIDKey, userID)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handler.GetMe(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var resp UserResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if resp.Email != email {
		t.Errorf("Expected email %s, got %s", email, resp.Email)
	}
	if resp.ID != userID {
		t.Errorf("Expected ID %d, got %d", userID, resp.ID)
	}
}
