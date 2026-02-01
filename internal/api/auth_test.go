package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/theLastOfCats/kotatsu-go-server/internal/auth"
	"github.com/theLastOfCats/kotatsu-go-server/internal/testutil"
)

func TestLogin(t *testing.T) {
	database := testutil.SetupTestDB(t)
	defer database.Close()

	// 1. Test Login with Non-Existent User (Should Auto-Register)
	handler := &AuthHandler{DB: database}

	creds := map[string]string{
		"email":    "newuser@example.com",
		"password": "securepassword",
	}
	body, _ := json.Marshal(creds)
	req, _ := http.NewRequest("POST", "/auth", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.Login(rr, req)

	// Should create user and return token
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Auto-register failed, got status %v", status)
	}

	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["token"] == "" {
		t.Error("Expected token in response")
	}

	// Verify User Created
	_, err := database.GetUserByEmail("newuser@example.com")
	if err != nil {
		t.Fatalf("User was not created: %v", err)
	}

	// 2. Test Login with Correct Password
	req2, _ := http.NewRequest("POST", "/auth", bytes.NewBuffer(body))
	rr2 := httptest.NewRecorder()
	handler.Login(rr2, req2)

	if status := rr2.Code; status != http.StatusOK {
		t.Errorf("Login with correct password failed, got status %v", status)
	}

	// 3. Test Login with Wrong Password
	badCreds := map[string]string{
		"email":    "newuser@example.com",
		"password": "wrongpassword",
	}
	bodyBad, _ := json.Marshal(badCreds)
	req3, _ := http.NewRequest("POST", "/auth", bytes.NewBuffer(bodyBad))
	rr3 := httptest.NewRecorder()
	handler.Login(rr3, req3)

	if status := rr3.Code; status != http.StatusUnauthorized {
		t.Errorf("Login with wrong password should be Unauthorized, got %v", status)
	}

	// 4. Test Manual Registration (if Register handler is used, but Login covers auto-reg)
	// We'll skip specific Register handler test since it's not currently wired in main.go
	// but the auto-reg logic in Login covers the core requirement.
}

func TestVerifyPasswordInternal(t *testing.T) {
	// Quick check on auth package helpers
	password := "testpass"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	match, err := auth.VerifyPassword(password, hash)
	if !match || err != nil {
		t.Error("Password verification failed")
	}
}
