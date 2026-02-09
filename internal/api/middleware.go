package api

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/theLastOfCats/kotatsu-go-server/internal/auth"
	"github.com/theLastOfCats/kotatsu-go-server/internal/db"
)

type contextKey string

const UserIDKey contextKey = "userID"

type Middleware struct {
	DB *db.DB
}

func (m *Middleware) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			JSONError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			JSONError(w, "Invalid authorization header", http.StatusUnauthorized)
			return
		}

		claims, err := auth.ValidateToken(parts[1])
		if err != nil {
			JSONError(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Verify user exists in database
		// This handles cases where client has valid token but DB was wiped
		exists, err := m.DB.UserExists(claims.UserID)
		if err != nil {
			log.Printf("AuthMiddleware: DB error checking user %d: %v", claims.UserID, err)
			JSONError(w, "Database error", http.StatusInternalServerError)
			return
		}
		if !exists {
			log.Printf("AuthMiddleware: User %d not found in DB", claims.UserID)
			JSONError(w, "User not found", http.StatusUnauthorized)
			return
		}
		log.Printf("AuthMiddleware: User %d validated", claims.UserID)

		ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetUserID(r *http.Request) (int64, bool) {
	userID, ok := r.Context().Value(UserIDKey).(int64)
	return userID, ok
}
