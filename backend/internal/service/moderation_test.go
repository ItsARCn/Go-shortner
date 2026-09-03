package service

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/arc/go-shortener/internal/config"
	"github.com/arc/go-shortener/internal/database"
	"github.com/arc/go-shortener/internal/models"
	"github.com/arc/go-shortener/internal/repository"
)

func setupModerationTestDB(t *testing.T) (*LinkService, *repository.LinkRepository, *repository.UserRepository, func()) {
	tmpDB, err := os.CreateTemp("", "go-mod-test-*.sqlite")
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
		BaseURL:                     "https://go.arcn.online",
		AnonymousDailyQuota:         15,
		RegisteredMonthlyQuota:      100,
		AnonymousMaxExpirationDays:  7,
		RegisteredMaxExpirationDays: 365,
	}

	linkRepo := repository.NewLinkRepository(db)
	userRepo := repository.NewUserRepository(db)
	svc := NewLinkService(linkRepo, cfg)

	cleanup := func() {
		db.Close()
		os.Remove(tmpDB.Name())
		os.Remove(tmpDB.Name() + "-wal")
		os.Remove(tmpDB.Name() + "-shm")
	}

	return svc, linkRepo, userRepo, cleanup
}

func TestUserTimeoutRestriction(t *testing.T) {
	svc, _, userRepo, cleanup := setupModerationTestDB(t)
	defer cleanup()

	// 1. Create active user
	user := &models.User{
		ID:           "timeout-user-id",
		FirstName:    "Restricted",
		LastName:     "User",
		Email:        "restricted@example.com",
		AuthProvider: "email",
		Role:         models.RoleUser,
		Status:       models.UserStatusActive,
		QuotaLimit:   100,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	_ = userRepo.CreateUser(user)

	adminSvc := NewAdminService(userRepo, nil)

	// Can create link normally before timeout
	ownerID := user.ID
	_, err := svc.Shorten(models.ShortenRequest{URL: "https://example.com/ok", Expiration: "7d"}, &ownerID, "127.0.0.1")
	if err != nil {
		t.Fatalf("initial shorten failed: %v", err)
	}

	// Apply 5-second timeout
	until, err := adminSvc.TimeoutUser(user.ID, "5s", "Spam investigation")
	if err != nil {
		t.Fatalf("timeout failed: %v", err)
	}
	if until == nil || until.Before(time.Now().UTC()) {
		t.Fatalf("invalid until timestamp: %v", until)
	}

	// Shortening should now be blocked
	_, err = svc.Shorten(models.ShortenRequest{URL: "https://example.com/blocked", Expiration: "7d"}, &ownerID, "127.0.0.1")
	if err == nil || !strings.Contains(err.Error(), "temporarily restricted") {
		t.Fatalf("expected restriction error, got: %v", err)
	}

	// Unban/un-timeout user restores access immediately
	err = adminSvc.UnbanUser(user.ID)
	if err != nil {
		t.Fatalf("unban failed: %v", err)
	}

	// Shortening succeeds again
	_, err = svc.Shorten(models.ShortenRequest{URL: "https://example.com/unblocked", Expiration: "7d"}, &ownerID, "127.0.0.1")
	if err != nil {
		t.Fatalf("shorten after un-timeout failed: %v", err)
	}
}

func TestUserBanWithLinkDeactivation(t *testing.T) {
	svc, linkRepo, userRepo, cleanup := setupModerationTestDB(t)
	defer cleanup()

	user := &models.User{
		ID:           "ban-user-id",
		FirstName:    "Bad",
		LastName:     "Actor",
		Email:        "bad@example.com",
		AuthProvider: "email",
		Role:         models.RoleUser,
		Status:       models.UserStatusActive,
		QuotaLimit:   100,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	_ = userRepo.CreateUser(user)

	adminSvc := NewAdminService(userRepo, linkRepo)

	// Create 2 links
	ownerID := user.ID
	l1, _ := svc.Shorten(models.ShortenRequest{URL: "https://site1.example.com", Expiration: "7d"}, &ownerID, "127.0.0.1")
	l2, _ := svc.Shorten(models.ShortenRequest{URL: "https://site2.example.com", Expiration: "7d"}, &ownerID, "127.0.0.1")

	// Ban user with disableLinks = true
	err := adminSvc.BanUser(user.ID, "Phishing campaign", true)
	if err != nil {
		t.Fatalf("ban failed: %v", err)
	}

	// Both links should now be disabled
	link1, _ := linkRepo.GetLinkByCode(l1.ShortCode)
	link2, _ := linkRepo.GetLinkByCode(l2.ShortCode)

	if link1.Status != models.StatusDisabled {
		t.Errorf("expected link1 to be DISABLED, got %s", link1.Status)
	}
	if link2.Status != models.StatusDisabled {
		t.Errorf("expected link2 to be DISABLED, got %s", link2.Status)
	}
}

func TestSuperAdminCannotBeModerated(t *testing.T) {
	_, _, userRepo, cleanup := setupModerationTestDB(t)
	defer cleanup()

	superAdmin := &models.User{
		ID:           "super-admin-id",
		FirstName:    "Root",
		LastName:     "Admin",
		Email:        "root@example.com",
		AuthProvider: "email",
		Role:         models.RoleSuperAdmin,
		Status:       models.UserStatusActive,
		QuotaLimit:   999999,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	_ = userRepo.CreateUser(superAdmin)

	adminSvc := NewAdminService(userRepo, nil)

	// Attempting timeout should return ErrCannotModerateAdmin
	_, err := adminSvc.TimeoutUser(superAdmin.ID, "1h", "Test")
	if err != ErrCannotModerateAdmin {
		t.Errorf("expected ErrCannotModerateAdmin on timeout, got: %v", err)
	}

	// Attempting ban should return ErrCannotModerateAdmin
	err = adminSvc.BanUser(superAdmin.ID, "Test", false)
	if err != ErrCannotModerateAdmin {
		t.Errorf("expected ErrCannotModerateAdmin on ban, got: %v", err)
	}
}
