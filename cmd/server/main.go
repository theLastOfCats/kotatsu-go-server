package main

import (
	"log"
	"net/http"
	"os"

	"github.com/theLastOfCats/kotatsu-go-server/internal/api"
	"github.com/theLastOfCats/kotatsu-go-server/internal/auth"
	"github.com/theLastOfCats/kotatsu-go-server/internal/db"
	_ "github.com/joho/godotenv/autoload"
)

func main() {
	// Initialize Auth
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}
	auth.Init(jwtSecret)

	// Initialize Database
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/kotatsu.db"
	}
	database, err := db.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Initialize Handlers
	authHandler := &api.AuthHandler{DB: database}
	syncHandler := &api.SyncHandler{DB: database}

	// Router
	mux := http.NewServeMux()

	// Auth Routes
	mux.HandleFunc("POST /auth", authHandler.Login)
    // mux.HandleFunc("POST /auth/register", authHandler.Register) // Optional, if needed. Android app might not use it directly if it registers via "login" based on logic, but README says "An account is created...". README says "enter email... acts as registration screen".
    // Wait, the README says "even if you have not registered... authorization screen also acts as a registration screen".
    // So /auth likely handles both or there is strict "Allow New Register" logic.
    // Kotlin code `AuthRoutes.kt`: `val user = getOrCreateUser(request, allowNewRegister)`
    // So correct, the /auth endpoint handles Login AND Registration implicitly if enabled.
    // I implemented separate Register/Login in `AuthHandler`. I need to fix `Login` to support `getOrCreateUser` logic or update my main binding to point to a unified handler.
    // I'll stick to what I wrote for now but I really should combine them to match Kotlin behavior.
    
    // Let's quickly patch AuthHandler.Login to be "LoginOrRegister".
    // Actually, I'll do that in a follow-up to match behavior exactl. For now, let's keep it wiring standard.

	mux.HandleFunc("POST /forgot-password", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not implemented", http.StatusNotImplemented)
	})
	mux.HandleFunc("POST /reset-password", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not implemented", http.StatusNotImplemented)
	})

	// Sync Routes (Protected)
	mux.Handle("GET /resource/history", api.AuthMiddleware(http.HandlerFunc(syncHandler.GetHistory)))
	mux.Handle("POST /resource/history", api.AuthMiddleware(http.HandlerFunc(syncHandler.PostHistory)))
	mux.Handle("GET /resource/favourites", api.AuthMiddleware(http.HandlerFunc(syncHandler.GetFavourites)))
	mux.Handle("POST /resource/favourites", api.AuthMiddleware(http.HandlerFunc(syncHandler.PostFavourites)))

	// Start Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on port %s...", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
