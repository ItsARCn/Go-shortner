package main

import (
	"bytes"
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
	"github.com/arc/go-shortener/internal/embedded"
	"github.com/arc/go-shortener/internal/handlers"
	"github.com/arc/go-shortener/internal/middleware"
	"github.com/arc/go-shortener/internal/models"
	"github.com/arc/go-shortener/internal/repository"
	"github.com/arc/go-shortener/internal/service"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	// 1. Load config
	cfg := config.Load(".env")

	// Ensure SQLite data directory exists
	dbDir := filepath.Dir(cfg.DBPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		log.Fatalf("[FATAL] Failed to create database directory: %v", err)
	}

	// 2. Initialize Database & Migrations
	db, err := database.InitDB(cfg.DBPath)
	if err != nil {
		log.Fatalf("[FATAL] Failed to initialize SQLite database at %s: %v", cfg.DBPath, err)
	}
	defer db.Close()
	log.Printf("[INFO] Connected to SQLite database: %s", cfg.DBPath)

	// 3. Initialize repositories, services, and handlers
	linkRepo := repository.NewLinkRepository(db)
	userRepo := repository.NewUserRepository(db)

	linkService := service.NewLinkService(linkRepo, cfg)
	authService := service.NewAuthService(userRepo, cfg)
	adminService := service.NewAdminService(userRepo, linkRepo)

	// Bootstrap initial Super Admin if configured
	if cfg.AdminBootstrapEmail != "" && cfg.AdminBootstrapPassword != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminBootstrapPassword), 12)
		if err == nil {
			_ = userRepo.EnsureSuperAdminBootstrap(cfg.AdminBootstrapEmail, string(hash))
		}
	}

	h := handlers.NewHandler(linkService, cfg)
	authHandler := handlers.NewAuthHandler(authService, cfg)
	userHandler := handlers.NewUserHandler(linkService, authService)
	adminHandler := handlers.NewAdminHandler(adminService)

	// 4. Rate Limiter (30 requests/minute per IP for shortening, 15/min for auth)
	shortenLimiter := middleware.NewRateLimiter(30, time.Minute)
	authLimiter := middleware.NewRateLimiter(15, time.Minute)

	// 5. Setup Router (Go 1.22+ pattern routing)
	mux := http.NewServeMux()

	// Public Core API Endpoints
	mux.HandleFunc("POST /api/links/shorten", middleware.OptionalAuth(authService, shortenLimiter.Limit(h.HandleShorten)))
	mux.HandleFunc("GET /api/links/{code}/info", h.HandleGetInfo)
	mux.HandleFunc("GET /api/health", h.HandleHealth)
	mux.HandleFunc("POST /api/reports", shortenLimiter.Limit(adminHandler.HandlePublicReport))

	// Auth API Endpoints
	mux.HandleFunc("POST /api/auth/register", authLimiter.Limit(authHandler.HandleRegister))
	mux.HandleFunc("POST /api/auth/login", authLimiter.Limit(authHandler.HandleLogin))
	mux.HandleFunc("POST /api/auth/google", authLimiter.Limit(authHandler.HandleGoogleLogin))
	mux.HandleFunc("POST /api/auth/logout", authHandler.HandleLogout)
	mux.HandleFunc("GET /api/auth/me", middleware.RequireAuth(authService, authHandler.HandleMe))
	mux.HandleFunc("GET /api/auth/firebase-config", authHandler.HandleFirebaseConfig)

	// User API Endpoints (Protected)
	mux.HandleFunc("GET /api/user/dashboard", middleware.RequireAuth(authService, userHandler.HandleDashboard))
	mux.HandleFunc("GET /api/user/links", middleware.RequireAuth(authService, userHandler.HandleLinks))
	mux.HandleFunc("GET /api/user/links/{code}/analytics", middleware.RequireAuth(authService, userHandler.HandleLinkAnalytics))
	mux.HandleFunc("POST /api/user/links/{code}/renew", middleware.RequireAuth(authService, userHandler.HandleRenewLink))
	mux.HandleFunc("POST /api/user/links/{code}/request-permanent", middleware.RequireAuth(authService, userHandler.HandleRequestPermanentLink))
	mux.HandleFunc("DELETE /api/user/links/{code}", middleware.RequireAuth(authService, userHandler.HandleDeleteLink))
	mux.HandleFunc("POST /api/user/links/restore", middleware.RequireAuth(authService, userHandler.HandleRestoreLink))
	mux.HandleFunc("POST /api/user/links/{code}/restore", middleware.RequireAuth(authService, userHandler.HandleRestoreLink))
	mux.HandleFunc("DELETE /api/user/links/{code}/permanent", middleware.RequireAuth(authService, userHandler.HandlePermanentDeleteLink))

	// Admin API Endpoints (Protected by RBAC)
	mux.HandleFunc("GET /api/admin/overview", middleware.RequireRole(authService, models.RoleModerator, adminHandler.HandleAdminOverview))
	mux.HandleFunc("GET /api/admin/users", middleware.RequireRole(authService, models.RoleModerator, adminHandler.HandleAdminUsers))
	mux.HandleFunc("POST /api/admin/users/{id}/timeout", middleware.RequireRole(authService, models.RoleModerator, adminHandler.HandleAdminTimeoutUser))
	mux.HandleFunc("POST /api/admin/users/{id}/ban", middleware.RequireRole(authService, models.RoleModerator, adminHandler.HandleAdminBanUser))
	mux.HandleFunc("POST /api/admin/users/{id}/unban", middleware.RequireRole(authService, models.RoleModerator, adminHandler.HandleAdminUnbanUser))
	mux.HandleFunc("POST /api/admin/users/{id}/role", middleware.RequireRole(authService, models.RoleSuperAdmin, adminHandler.HandleAdminUpdateUserRole))
	mux.HandleFunc("GET /api/admin/links", middleware.RequireRole(authService, models.RoleModerator, adminHandler.HandleAdminLinks))
	mux.HandleFunc("POST /api/admin/links/{code}/disable", middleware.RequireRole(authService, models.RoleModerator, adminHandler.HandleAdminDisableLink))
	mux.HandleFunc("POST /api/admin/links/{code}/enable", middleware.RequireRole(authService, models.RoleModerator, adminHandler.HandleAdminEnableLink))
	mux.HandleFunc("DELETE /api/admin/links/{code}", middleware.RequireRole(authService, models.RoleModerator, adminHandler.HandleAdminDeleteLink))
	mux.HandleFunc("GET /api/admin/reports", middleware.RequireRole(authService, models.RoleModerator, adminHandler.HandleAdminReports))
	mux.HandleFunc("POST /api/admin/reports/{id}/resolve", middleware.RequireRole(authService, models.RoleModerator, adminHandler.HandleAdminResolveReport))
	mux.HandleFunc("GET /api/admin/permanent-requests", middleware.RequireRole(authService, models.RoleModerator, adminHandler.HandleAdminPermanentRequests))
	mux.HandleFunc("POST /api/admin/permanent-requests/{id}/resolve", middleware.RequireRole(authService, models.RoleModerator, adminHandler.HandleAdminResolvePermanentRequest))
	mux.HandleFunc("GET /api/admin/login-records", middleware.RequireRole(authService, models.RoleSuperAdmin, adminHandler.HandleAdminLoginRecords))

	// Static Assets & Web Frontend (Transparent disk + embedded fallback for standalone binary)
	staticDir := "frontend/public"
	hasLocalStatic := false
	if info, err := os.Stat(staticDir); err == nil && info.IsDir() {
		hasLocalStatic = true
	} else if info, err := os.Stat("../../frontend/public"); err == nil && info.IsDir() {
		staticDir = "../../frontend/public"
		hasLocalStatic = true
	}

	if hasLocalStatic {
		fs := http.FileServer(http.Dir(staticDir))
		mux.Handle("GET /assets/", http.StripPrefix("/assets/", fs))
	} else {
		mux.Handle("GET /assets/", http.StripPrefix("/assets/", embedded.GetFileServer()))
	}

	// Specific Page Routes
	serveHTML := func(filename string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if hasLocalStatic {
				diskPath := filepath.Join(staticDir, filename)
				if _, err := os.Stat(diskPath); err == nil {
					http.ServeFile(w, r, diskPath)
					return
				}
			}
			content, err := embedded.ReadFile(filename)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(content)
		}
	}

	mux.HandleFunc("GET /login", serveHTML("login.html"))
	mux.HandleFunc("GET /register", serveHTML("register.html"))
	mux.HandleFunc("GET /dashboard", serveHTML("dashboard.html"))
	mux.HandleFunc("GET /admin", serveHTML("admin.html"))
	mux.HandleFunc("GET /report", serveHTML("report.html"))

	// Homepage and catch-all for short links
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			if hasLocalStatic {
				http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
				return
			}
			content, err := embedded.ReadFile("index.html")
			if err == nil {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write(content)
				return
			}
		}

		// If requesting a specific static file
		if hasLocalStatic {
			staticFilePath := filepath.Join(staticDir, path)
			if info, err := os.Stat(staticFilePath); err == nil && !info.IsDir() {
				http.ServeFile(w, r, staticFilePath)
				return
			}
		} else {
			if content, err := embedded.ReadFile(path); err == nil {
				http.ServeContent(w, r, path, time.Now(), bytes.NewReader(content))
				return
			}
		}

		// Otherwise, treat path as a short code (GET /{code})
		h.HandleRedirect(w, r)
	})

	// Wrap global middleware
	handler := middleware.Recoverer(mux)
	handler = middleware.SecurityHeaders(handler)
	handler = middleware.Logger(handler)

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 6. Background Cleanup Worker (purges bin links older than 7 days)
	go func() {
		if purged, err := linkRepo.PurgeExpiredBinLinks(); err == nil && purged > 0 {
			log.Printf("[CLEANUP] Initial startup purged %d expired bin links (> 7 days)", purged)
		}
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if purged, err := linkRepo.PurgeExpiredBinLinks(); err == nil && purged > 0 {
				log.Printf("[CLEANUP] Hourly purged %d expired bin links (> 7 days)", purged)
			}
		}
	}()

	// 7. Graceful Shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("[INFO] GO Shortener server listening on http://%s (Base URL: %s)", addr, cfg.BaseURL)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] Server failed: %v", err)
		}
	}()

	<-stop
	log.Println("[INFO] Shutting down server gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[ERROR] Server forced to shutdown: %v", err)
	}

	log.Println("[INFO] Server stopped.")
}
