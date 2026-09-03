package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/arc/go-shortener/internal/config"
	"github.com/arc/go-shortener/internal/database"
	"github.com/arc/go-shortener/internal/middleware"
	"github.com/arc/go-shortener/internal/models"
	"github.com/arc/go-shortener/internal/repository"
	"github.com/arc/go-shortener/internal/service"
)

func setupAdminAPIServer(t *testing.T) (*httptest.Server, *service.AuthService, *repository.UserRepository, *repository.LinkRepository, func()) {
	tmpDB, err := os.CreateTemp("", "go-admin-api-*.sqlite")
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
	adminSvc := service.NewAdminService(userRepo, linkRepo)

	adminH := NewAdminHandler(adminSvc)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/reports", adminH.HandlePublicReport)
	mux.HandleFunc("GET /api/admin/overview", middleware.RequireRole(authSvc, models.RoleModerator, adminH.HandleAdminOverview))

	server := httptest.NewServer(mux)

	cleanup := func() {
		server.Close()
		db.Close()
		os.Remove(tmpDB.Name())
		os.Remove(tmpDB.Name() + "-wal")
		os.Remove(tmpDB.Name() + "-shm")
	}

	return server, authSvc, userRepo, linkRepo, cleanup
}

func TestAdminAPIAccessControl(t *testing.T) {
	ts, authSvc, userRepo, linkRepo, cleanup := setupAdminAPIServer(t)
	defer cleanup()

	client := ts.Client()

	// 1. Create a regular user
	regUser := &models.User{
		ID:         "regular-user",
		FirstName:  "Reg",
		LastName:   "User",
		Email:      "regular@example.com",
		Role:       models.RoleUser,
		Status:     models.UserStatusActive,
		QuotaLimit: 100,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	_ = userRepo.CreateUser(regUser)
	regToken, _ := authSvc.GenerateToken(regUser)

	// 2. Regular user accesses /api/admin/overview -> Expect 403 Forbidden
	req, _ := http.NewRequest("GET", ts.URL+"/api/admin/overview", nil)
	req.Header.Set("Authorization", "Bearer "+regToken)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for regular user, got %d", resp.StatusCode)
	}

	// 3. Verify UNAUTHORIZED entry in login_records
	records, _, _ := userRepo.GetLoginRecords("UNAUTHORIZED", "", 1, 10)
	if len(records) != 1 || records[0].AccountEmail != "regular@example.com" {
		t.Errorf("expected audit record for regular@example.com unauthorized attempt, got: %+v", records)
	}

	// 4. Create super_admin
	adminUser := &models.User{
		ID:         "super-admin",
		FirstName:  "Admin",
		LastName:   "Boss",
		Email:      "boss@example.com",
		Role:       models.RoleSuperAdmin,
		Status:     models.UserStatusActive,
		QuotaLimit: 999999,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	_ = userRepo.CreateUser(adminUser)
	adminToken, _ := authSvc.GenerateToken(adminUser)

	// 5. Admin accesses /api/admin/overview -> Expect 200 OK
	adminReq, _ := http.NewRequest("GET", ts.URL+"/api/admin/overview", nil)
	adminReq.Header.Set("Authorization", "Bearer "+adminToken)
	adminResp, err := client.Do(adminReq)
	if err != nil {
		t.Fatalf("admin request failed: %v", err)
	}
	defer adminResp.Body.Close()

	if adminResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK for super admin, got %d", adminResp.StatusCode)
	}

	// 6. Test Public Reporting
	// Create link to report
	link := &models.Link{
		ID:             "link-report-1",
		ShortCode:      "rep123",
		DestinationURL: "https://badsite.com",
		Status:         models.StatusActive,
		CreatedAt:      time.Now().UTC(),
		ExpiresAt:      time.Now().Add(24 * time.Hour),
	}
	_ = linkRepo.CreateLink(link)

	reportPayload := `{"short_code":"rep123","reason":"phishing","details":"Testing public report"}`
	repResp, err := client.Post(ts.URL+"/api/reports", "application/json", strings.NewReader(reportPayload))
	if err != nil {
		t.Fatalf("report post failed: %v", err)
	}
	defer repResp.Body.Close()

	if repResp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201 Created for public report, got %d", repResp.StatusCode)
	}
}
