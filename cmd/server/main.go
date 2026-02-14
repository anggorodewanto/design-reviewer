package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"

	"github.com/ab/design-reviewer/internal/api"
	"github.com/ab/design-reviewer/internal/auth"
	"github.com/ab/design-reviewer/internal/db"
	"github.com/ab/design-reviewer/internal/seed"
	"github.com/ab/design-reviewer/internal/storage"
)

func main() {
	_ = godotenv.Load()

	port := flag.Int("port", 8080, "server port")
	dbPath := flag.String("db", "./data/design-reviewer.db", "SQLite database path")
	uploads := flag.String("uploads", "./data/uploads", "upload directory")
	flag.Parse()

	os.MkdirAll(filepath.Dir(*dbPath), 0o755)

	database, err := db.New(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	store := storage.New(*uploads)

	seed.Run(database, *uploads)

	h := &api.Handler{DB: database, Storage: store, TemplatesDir: "web/templates", StaticDir: "web/static"}

	// Configure auth if env vars are set
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	sessionSecret := os.Getenv("SESSION_SECRET")
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%d", *port)
	}

	if clientID != "" && clientSecret != "" && sessionSecret != "" {
		cfg := &auth.Config{
			ClientID:       clientID,
			ClientSecret:   clientSecret,
			RedirectURL:    baseURL + "/auth/google/callback",
			CLIRedirectURL: baseURL + "/auth/google/cli-callback",
			SessionSecret:  sessionSecret,
			BaseURL:        baseURL,
		}
		h.Auth = cfg
		oauthCfg := auth.NewGoogleOAuthConfig(*cfg)
		h.OAuthConfig = &api.GoogleOAuth{Config: oauthCfg}
		fmt.Println("auth enabled (Google OAuth)")
	} else {
		fmt.Println("auth disabled (set GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, SESSION_SECRET to enable)")
	}

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	addr := fmt.Sprintf(":%d", *port)
	fmt.Printf("server running on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, securityHeaders(mux)))
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}
