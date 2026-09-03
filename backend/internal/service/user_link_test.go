package service

import (
	"os"
	"testing"
	"time"

	"github.com/arc/go-shortener/internal/config"
	"github.com/arc/go-shortener/internal/database"
	"github.com/arc/go-shortener/internal/models"
	"github.com/arc/go-shortener/internal/repository"
)

func setupUserLinkTestDB(t *testing.T) (*LinkService, *repository.LinkRepository, func()) {
	tmpDB, err := os.CreateTemp("", "go-userlink-test-*.sqlite")
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

	repo := repository.NewLinkRepository(db)
	svc := NewLinkService(repo, cfg)

	cleanup := func() {
		db.Close()
		os.Remove(tmpDB.Name())
		os.Remove(tmpDB.Name() + "-wal")
		os.Remove(tmpDB.Name() + "-shm")
	}

	return svc, repo, cleanup
}

func TestRegisteredUserLinkCreationAndQuota(t *testing.T) {
	svc, repo, cleanup := setupUserLinkTestDB(t)
	defer cleanup()

	userID := "user-12345"
	monthWindow := time.Now().UTC().Format("2006-01")

	// 1. Create a 1-year link as registered user
	req := models.ShortenRequest{
		URL:        "https://example.com/portfolio",
		Expiration: "365d",
	}

	resp, err := svc.Shorten(req, &userID, "192.0.2.1")
	if err != nil {
		t.Fatalf("failed to shorten as registered user: %v", err)
	}

	// Registered user expiration can be 1 year (~365 days)
	duration := time.Until(resp.ExpiresAt)
	if duration < 300*24*time.Hour {
		t.Errorf("expected ~1 year expiration, got %v", duration)
	}

	// 2. Check quota consumed is 1
	stats, err := repo.GetUserStats(userID, 100, monthWindow)
	if err != nil {
		t.Fatalf("failed to get user stats: %v", err)
	}
	if stats.QuotaUsed != 1 {
		t.Errorf("expected QuotaUsed = 1, got %d", stats.QuotaUsed)
	}
	if stats.TotalLinks != 1 {
		t.Errorf("expected TotalLinks = 1, got %d", stats.TotalLinks)
	}
	if stats.ActiveLinks != 1 {
		t.Errorf("expected ActiveLinks = 1, got %d", stats.ActiveLinks)
	}
}

func TestRenewExpiredLinkKeepsCodeAndZeroQuotaCost(t *testing.T) {
	svc, repo, cleanup := setupUserLinkTestDB(t)
	defer cleanup()

	ownerID := "user-999"
	otherUser := "user-attacker"
	monthWindow := time.Now().UTC().Format("2006-01")

	// 1. Create a link
	req := models.ShortenRequest{
		URL:        "https://example.com/project",
		Expiration: "1d",
	}
	resp, err := svc.Shorten(req, &ownerID, "192.0.2.5")
	if err != nil {
		t.Fatalf("shorten failed: %v", err)
	}
	shortCode := resp.ShortCode

	// Quota used should be 1
	statsBefore, _ := repo.GetUserStats(ownerID, 100, monthWindow)
	if statsBefore.QuotaUsed != 1 {
		t.Fatalf("expected 1 quota used, got %d", statsBefore.QuotaUsed)
	}

	// 2. Simulate expiration
	_ = repo.UpdateLinkStatus(resp.ShortCode, models.StatusExpired)
	link, _ := repo.GetLinkByCode(shortCode)
	_ = repo.UpdateLinkStatus(link.ID, models.StatusExpired)

	// Verify it is expired
	_, err = svc.Resolve(shortCode, "192.0.2.5", "", "")
	if err != ErrLinkExpired {
		t.Errorf("expected ErrLinkExpired, got: %v", err)
	}

	// 3. Unauthorized user cannot renew it
	_, err = svc.RenewLink(shortCode, otherUser, "30d")
	if err != repository.ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized for non-owner, got: %v", err)
	}

	// 4. Owner renews it for 30 days
	renewedLink, err := svc.RenewLink(shortCode, ownerID, "30d")
	if err != nil {
		t.Fatalf("owner renewal failed: %v", err)
	}

	// Must keep the exact same short code!
	if renewedLink.ShortCode != shortCode {
		t.Errorf("expected same short code %s, got %s", shortCode, renewedLink.ShortCode)
	}
	if renewedLink.Status != models.StatusActive {
		t.Errorf("expected status ACTIVE, got %s", renewedLink.Status)
	}

	// 5. Renewal MUST NOT consume another creation quota!
	statsAfter, _ := repo.GetUserStats(ownerID, 100, monthWindow)
	if statsAfter.QuotaUsed != 1 {
		t.Errorf("renewal consumed a quota slot! expected QuotaUsed = 1, got %d", statsAfter.QuotaUsed)
	}

	// 6. Link can now be resolved successfully again!
	resolved, err := svc.Resolve(shortCode, "192.0.2.5", "", "")
	if err != nil {
		t.Fatalf("failed to resolve renewed link: %v", err)
	}
	if resolved.DestinationURL != "https://example.com/project" {
		t.Errorf("unexpected destination url: %s", resolved.DestinationURL)
	}
}

func TestSoftDeleteLink(t *testing.T) {
	svc, repo, cleanup := setupUserLinkTestDB(t)
	defer cleanup()

	ownerID := "user-delete-test"
	req := models.ShortenRequest{
		URL:        "https://example.com/to-delete",
		Expiration: "7d",
	}
	resp, _ := svc.Shorten(req, &ownerID, "192.0.2.10")

	// Delete link
	err := svc.DeleteLink(resp.ShortCode, ownerID)
	if err != nil {
		t.Fatalf("failed to delete link: %v", err)
	}

	// Deleted link cannot be resolved
	_, err = svc.Resolve(resp.ShortCode, "192.0.2.10", "", "")
	if err != ErrLinkNotFound {
		t.Errorf("expected ErrLinkNotFound for deleted link, got: %v", err)
	}

	// Deleted link does not appear in active user links
	links, total, err := repo.GetUserLinks(ownerID, "", "active", 1, 10)
	if err != nil {
		t.Fatalf("failed to get user links: %v", err)
	}
	if total != 0 || len(links) != 0 {
		t.Errorf("deleted link should not appear in user links")
	}
}
