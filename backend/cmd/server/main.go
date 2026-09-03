package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/arc/go-shortener/internal/config"
	"github.com/arc/go-shortener/internal/database"
	"github.com/arc/go-shortener/internal/handlers"
	"github.com/arc/go-shortener/internal/middleware"
	"github.com/arc/go-shortener/internal/repository"
	"github.com/arc/go-shortener/internal/service"
)

func main() {
	// 1. Load config
	cfg := config.Load(".env")
	log.Printf("[INFO] Initializing GO Shortener in %s mode...", cfg.AppEnv)

	// 2. Initialize database
	db, err := database.InitDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize SQLite database at %s: %v", cfg.DBPath, err)
	}
	defer db.Close()
	log.Printf("[INFO] Connected to SQLite database: %s", cfg.DBPath)

	// 3. Initialize repository, service, and handlers
	linkRepo := repository.NewLinkRepository(db)
	linkService := service.NewLinkService(linkRepo, cfg)
	h := handlers.NewHandler(linkService)

	// 4. In-memory Rate Limiter (30 requests/minute per IP for shortening)
	shortenLimiter := middleware.NewRateLimiter(30, time.Minute)

	// 5. Setup Router (using enhanced Go 1.22+ pattern routing)
	mux := http.NewServeMux()

	// API Endpoints
	mux.HandleFunc("POST /api/links/shorten", shortenLimiter.Limit(h.HandleShorten))
	mux.HandleFunc("GET /api/links/{code}/info", h.HandleGetInfo)
	mux.HandleFunc("GET /api/health", h.HandleHealth)

	// Static Assets & Web Frontend
	staticDir := "frontend/public"
	if _, err := os.Stat(staticDir); os.IsNotExist(err) {
		// Fallback for when running from backend/ directory
		staticDir = "../../frontend/public"
	}

	fs := http.FileServer(http.Dir(staticDir))
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", fs))

	// Homepage and catch-all
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
			return
		}
		// If requesting a specific file that exists in staticDir
		staticFilePath := filepath.Join(staticDir, path)
		if info, err := os.Stat(staticFilePath); err == nil && !info.IsDir() {
			http.ServeFile(w, r, staticFilePath)
			return
		}

		// Otherwise, treat path as a short code (GET /{code})
		h.HandleRedirect(w, r)
	})

	// Wrap router with security headers and request logger
	handler := middleware.LoggingMiddleware(middleware.SecurityHeaders(mux))

	// 6. Server configuration
	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 7. Start server in goroutine
	go func() {
		log.Printf("[INFO] Server listening on http://%s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] Listen error: %v", err)
		}
	}()

	// 8. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("[INFO] Shutting down server gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("[ERROR] Server shutdown forced: %v", err)
	}

	log.Println("[INFO] Server stopped.")
}
