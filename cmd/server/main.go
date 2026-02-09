package main

import (
	"log"
	"net/http"
	"os"
	"strings"

	_ "github.com/joho/godotenv/autoload"
	"github.com/theLastOfCats/kotatsu-go-server/internal/api"
	"github.com/theLastOfCats/kotatsu-go-server/internal/auth"
	"github.com/theLastOfCats/kotatsu-go-server/internal/db"
	"github.com/theLastOfCats/kotatsu-go-server/internal/mail"
	"github.com/theLastOfCats/kotatsu-go-server/internal/templates"
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

	// Initialize Services
	mailer := mail.NewSenderFromEnv()
	templatesMgr := templates.NewManager("templates")
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Initialize Handlers
	authHandler := &api.AuthHandler{
		DB:        database,
		Mailer:    mailer,
		Templates: templatesMgr,
		BaseURL:   baseURL,
	}
	syncHandler := &api.SyncHandler{DB: database}
	userHandler := &api.UserHandler{DB: database}

	// Initialize Middleware
	middleware := &api.Middleware{DB: database}

	// Router
	mux := http.NewServeMux()

	// Auth Routes
	// Public Routes
	mux.HandleFunc("GET /", api.Health)

	mux.HandleFunc("POST /auth", authHandler.Login)
	mux.HandleFunc("POST /forgot-password", authHandler.ForgotPassword)
	mux.HandleFunc("POST /reset-password", authHandler.ResetPassword)
	mux.HandleFunc("GET /deeplink/reset-password", authHandler.ResetPasswordDeeplink)

	// Protected Routes
	mux.Handle("GET /me", middleware.AuthMiddleware(http.HandlerFunc(userHandler.GetMe)))

	// Sync Routes (Protected)
	mux.Handle("GET /resource/history", middleware.AuthMiddleware(http.HandlerFunc(syncHandler.GetHistory)))
	mux.Handle("POST /resource/history", middleware.AuthMiddleware(http.HandlerFunc(syncHandler.PostHistory)))
	mux.Handle("GET /resource/favourites", middleware.AuthMiddleware(http.HandlerFunc(syncHandler.GetFavourites)))
	mux.Handle("POST /resource/favourites", middleware.AuthMiddleware(http.HandlerFunc(syncHandler.PostFavourites)))

	// Start Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on port %s...", port)

	handler := http.Handler(mux)
	if isDebugEnabled() {
		log.Println("Debug logging middleware enabled")
		handler = api.LoggingMiddleware(handler)
	}

	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func isDebugEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DEBUG"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
