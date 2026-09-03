package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/arc/go-shortener/internal/config"
	"github.com/arc/go-shortener/internal/database"
	"github.com/arc/go-shortener/internal/middleware"
	"github.com/arc/go-shortener/internal/models"
	"github.com/arc/go-shortener/internal/repository"
	"github.com/arc/go-shortener/internal/service"
)

func setupAuthUserServer(t *testing.T) (*httptest.Server, *service.AuthService, func()) {
	tmpDB, err := os.CreateTemp("", "go-auth-api-*.sqlite")
	if err != nil {
		t.Fatalf("failed to create temp db: %v", err)
	}
	tmpDB.Close()

	db, err := database.InitDB(tmpDB.Name())
	if err != nil {
		os.Remove(tmpDB.Name())
		t.Fatalf("failed to init db: %v", err)
	}

	cfg := &config.Config{
		BaseURL:                     "http://test.local",
		JWTSecret:                   "test-secret-at-least-32-chars-long-here!",
		AnonymousDailyQuota:         15,
		RegisteredMonthlyQuota:      100,
		AnonymousMaxExpirationDays:  7,
		RegisteredMaxExpirationDays: 365,
	}

	userRepo := repository.NewUserRepository(db)
	linkRepo := repository.NewLinkRepository(db)

	authSvc := service.NewAuthService(userRepo, cfg)
	linkSvc := service.NewLinkService(linkRepo, cfg)

	h := NewHandler(linkSvc, cfg)
	authH := NewAuthHandler(authSvc, cfg)
	userH := NewUserHandler(linkSvc, authSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/register", authH.HandleRegister)
	mux.HandleFunc("POST /api/auth/login", authH.HandleLogin)
	mux.HandleFunc("POST /api/auth/logout", authH.HandleLogout)
	mux.HandleFunc("GET /api/auth/me", middleware.RequireAuth(authSvc, authH.HandleMe))
	mux.HandleFunc("GET /api/auth/firebase-config", authH.HandleFirebaseConfig)

	mux.HandleFunc("POST /api/links/shorten", middleware.OptionalAuth(authSvc, h.HandleShorten))
	mux.HandleFunc("GET /api/user/dashboard", middleware.RequireAuth(authSvc, userH.HandleDashboard))
	mux.HandleFunc("POST /api/user/links/{code}/renew", middleware.RequireAuth(authSvc, userH.HandleRenewLink))
	mux.HandleFunc("DELETE /api/user/links/{code}", middleware.RequireAuth(authSvc, userH.HandleDeleteLink))

	server := httptest.NewServer(mux)

	cleanup := func() {
		server.Close()
		db.Close()
		os.Remove(tmpDB.Name())
		os.Remove(tmpDB.Name() + "-wal")
		os.Remove(tmpDB.Name() + "-shm")
	}

	return server, authSvc, cleanup
}

func TestAuthAndUserLifecycle(t *testing.T) {
	ts, _, cleanup := setupAuthUserServer(t)
	defer cleanup()

	client := ts.Client()

	// 1. Register User
	regPayload := `{"first_name":"Alice","last_name":"Smith","email":"alice@example.com","password":"StrongPassword123!","confirm_password":"StrongPassword123!"}`
	regResp, err := client.Post(ts.URL+"/api/auth/register", "application/json", strings.NewReader(regPayload))
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	defer regResp.Body.Close()

	if regResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", regResp.StatusCode)
	}

	var authData models.AuthResponse
	_ = json.NewDecoder(regResp.Body).Decode(&authData)
	token := authData.Token
	if token == "" {
		t.Fatalf("expected JWT token")
	}

	// 2. Verify GET /api/auth/me with Bearer token
	reqMe, _ := http.NewRequest("GET", ts.URL+"/api/auth/me", nil)
	reqMe.Header.Set("Authorization", "Bearer "+token)
	respMe, err := client.Do(reqMe)
	if err != nil {
		t.Fatalf("GET /api/auth/me failed: %v", err)
	}
	defer respMe.Body.Close()
	if respMe.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK from /api/auth/me, got %d", respMe.StatusCode)
	}

	// 3. Shorten a link as authenticated user
	shortenReq, _ := http.NewRequest("POST", ts.URL+"/api/links/shorten", strings.NewReader(`{"url":"https://golang.org","expiration":"365d"}`))
	shortenReq.Header.Set("Authorization", "Bearer "+token)
	shortenReq.Header.Set("Content-Type", "application/json")
	shortenResp, err := client.Do(shortenReq)
	if err != nil {
		t.Fatalf("shorten as user failed: %v", err)
	}
	defer shortenResp.Body.Close()
	if shortenResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", shortenResp.StatusCode)
	}

	var sResp models.ShortenResponse
	_ = json.NewDecoder(shortenResp.Body).Decode(&sResp)

	// 4. Verify user dashboard
	dashReq, _ := http.NewRequest("GET", ts.URL+"/api/user/dashboard", nil)
	dashReq.Header.Set("Authorization", "Bearer "+token)
	dashResp, err := client.Do(dashReq)
	if err != nil {
		t.Fatalf("get dashboard failed: %v", err)
	}
	defer dashResp.Body.Close()

	var dash models.DashboardResponse
	_ = json.NewDecoder(dashResp.Body).Decode(&dash)
	if dash.Stats.TotalLinks != 1 {
		t.Errorf("expected 1 total link in dashboard, got %d", dash.Stats.TotalLinks)
	}
	if dash.Stats.QuotaUsed != 1 {
		t.Errorf("expected 1 quota used, got %d", dash.Stats.QuotaUsed)
	}

	// 5. Renew link
	renewReq, _ := http.NewRequest("POST", ts.URL+"/api/user/links/"+sResp.ShortCode+"/renew", strings.NewReader(`{"expiration":"90d"}`))
	renewReq.Header.Set("Authorization", "Bearer "+token)
	renewReq.Header.Set("Content-Type", "application/json")
	renewResp, err := client.Do(renewReq)
	if err != nil {
		t.Fatalf("renew request failed: %v", err)
	}
	defer renewResp.Body.Close()
	if renewResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK from renew, got %d", renewResp.StatusCode)
	}

	// 6. Delete link
	delReq, _ := http.NewRequest("DELETE", ts.URL+"/api/user/links/"+sResp.ShortCode, nil)
	delReq.Header.Set("Authorization", "Bearer "+token)
	delResp, err := client.Do(delReq)
	if err != nil {
		t.Fatalf("delete request failed: %v", err)
	}
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK from delete, got %d", delResp.StatusCode)
	}
}

func TestFirebaseConfigEndpoint(t *testing.T) {
	ts, _, cleanup := setupAuthUserServer(t)
	defer cleanup()

	// 1. Check disabled config
	resp, err := http.Get(ts.URL + "/api/auth/firebase-config")
	if err != nil {
		t.Fatalf("failed to fetch firebase config: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if enabled, ok := data["enabled"].(bool); !ok || enabled {
		t.Errorf("expected enabled to be false when no API key configured, got %v", data["enabled"])
	}
}
