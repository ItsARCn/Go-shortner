package service

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/arc/go-shortener/internal/config"
	"github.com/arc/go-shortener/internal/database"
	"github.com/arc/go-shortener/internal/models"
	"github.com/arc/go-shortener/internal/repository"
)

func setupTestDB(t *testing.T) (*LinkService, func()) {
	tmpDB, err := os.CreateTemp("", "go-short-test-*.sqlite")
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

	return svc, cleanup
}

func TestShortenAndResolve(t *testing.T) {
	svc, cleanup := setupTestDB(t)
	defer cleanup()

	req := models.ShortenRequest{
		URL:        "https://example.com/blog/article",
		Expiration: "7d",
	}

	resp, err := svc.Shorten(req, nil, "198.51.100.1")
	if err != nil {
		t.Fatalf("failed to shorten: %v", err)
	}

	if len(resp.ShortCode) != 6 {
		t.Errorf("expected 6-char code, got %q", resp.ShortCode)
	}
	if resp.TargetURL != "https://example.com/blog/article" {
		t.Errorf("unexpected target URL: %s", resp.TargetURL)
	}

	// Resolve link
	link, err := svc.Resolve(resp.ShortCode, "198.51.100.2", "https://google.com", "Mozilla/5.0")
	if err != nil {
		t.Fatalf("failed to resolve link: %v", err)
	}
	if link.DestinationURL != "https://example.com/blog/article" {
		t.Errorf("expected destination url, got %s", link.DestinationURL)
	}

	// Small pause for asynchronous analytics recording
	time.Sleep(50 * time.Millisecond)

	// Fetch public info
	info, err := svc.GetInfo(resp.ShortCode)
	if err != nil {
		t.Fatalf("failed to get info: %v", err)
	}
	if info.IsExpired {
		t.Errorf("link should not be expired")
	}
}

func TestAnonymousQuotaLimit(t *testing.T) {
	svc, cleanup := setupTestDB(t)
	defer cleanup()

	clientIP := "203.0.113.42"

	// Create 15 links (should all succeed)
	for i := 1; i <= 15; i++ {
		req := models.ShortenRequest{
			URL:        fmt.Sprintf("https://example.com/page/%d", i),
			Expiration: "1d",
		}
		_, err := svc.Shorten(req, nil, clientIP)
		if err != nil {
			t.Fatalf("expected link %d to succeed, got: %v", i, err)
		}
	}

	// 16th link must fail with quota error
	req16 := models.ShortenRequest{
		URL:        "https://example.com/page/16",
		Expiration: "1d",
	}
	_, err := svc.Shorten(req16, nil, clientIP)
	if err != ErrAnonymousQuotaMet {
		t.Fatalf("expected ErrAnonymousQuotaMet, got: %v", err)
	}

	// Different IP should still have its own quota
	_, err = svc.Shorten(req16, nil, "203.0.113.43")
	if err != nil {
		t.Fatalf("expected different IP to succeed, got: %v", err)
	}
}

func TestAnonymousExpirationClamping(t *testing.T) {
	svc, cleanup := setupTestDB(t)
	defer cleanup()

	// Anonymous request tries to set 30-day or 1-year expiration
	req := models.ShortenRequest{
		URL:        "https://example.com/clamp-test",
		Expiration: "365d",
	}

	resp, err := svc.Shorten(req, nil, "198.51.100.99")
	if err != nil {
		t.Fatalf("failed to shorten: %v", err)
	}

	// Max anonymous expiration is 7 days (~168 hours)
	maxAllowedDuration := 7*24*time.Hour + time.Minute
	actualDuration := time.Until(resp.ExpiresAt)

	if actualDuration > maxAllowedDuration {
		t.Errorf("anonymous expiration was not clamped to 7 days: duration is %v", actualDuration)
	}
}

func TestBinAndRestoreWithin7Days(t *testing.T) {
	svc, cleanup := setupTestDB(t)
	defer cleanup()

	ownerID := "user-test-123"
	req := models.ShortenRequest{
		URL:        "https://example.com/bin-test",
		Expiration: "30d",
	}

	resp, err := svc.Shorten(req, &ownerID, "127.0.0.1")
	if err != nil {
		t.Fatalf("failed to shorten: %v", err)
	}

	// 1. Delete link -> moves to bin
	err = svc.DeleteLink(resp.ShortCode, ownerID)
	if err != nil {
		t.Fatalf("failed to delete link: %v", err)
	}

	// 2. Resolve link should now fail because it's deleted
	_, err = svc.Resolve(resp.ShortCode, "127.0.0.1", "", "test-agent")
	if err != ErrLinkNotFound {
		t.Fatalf("expected ErrLinkNotFound for deleted link, got: %v", err)
	}

	// 3. Check dashboard with status=bin
	dash, err := svc.GetUserDashboard(ownerID, 100, "", "bin", 1, 20)
	if err != nil {
		t.Fatalf("failed to get bin links: %v", err)
	}
	if len(dash.Links) != 1 {
		t.Fatalf("expected 1 link in bin, got: %d", len(dash.Links))
	}
	if dash.Links[0].ShortCode != resp.ShortCode {
		t.Fatalf("expected short code %s in bin, got %s", resp.ShortCode, dash.Links[0].ShortCode)
	}
	if dash.Links[0].DaysRemainingInBin == nil || *dash.Links[0].DaysRemainingInBin < 1 {
		t.Fatalf("expected valid days remaining in bin, got: %v", dash.Links[0].DaysRemainingInBin)
	}

	// 4. Restore link
	err = svc.RestoreLink(resp.ShortCode, ownerID)
	if err != nil {
		t.Fatalf("failed to restore link: %v", err)
	}

	// 5. Resolve link should now succeed again
	resTarget, err := svc.Resolve(resp.ShortCode, "127.0.0.1", "", "test-agent")
	if err != nil {
		t.Fatalf("expected resolved link to succeed after restore, got: %v", err)
	}
	if resTarget.DestinationURL != "https://example.com/bin-test" {
		t.Fatalf("expected destination url https://example.com/bin-test, got: %s", resTarget.DestinationURL)
	}

	// 6. Delete again, then permanently delete
	err = svc.DeleteLink(resp.ShortCode, ownerID)
	if err != nil {
		t.Fatalf("failed to delete link second time: %v", err)
	}
	err = svc.PermanentDeleteLink(resp.ShortCode, ownerID)
	if err != nil {
		t.Fatalf("failed to permanent delete link: %v", err)
	}
	dashAfterPurge, err := svc.GetUserDashboard(ownerID, 100, "", "bin", 1, 20)
	if err != nil {
		t.Fatalf("failed to get bin links after purge: %v", err)
	}
	if len(dashAfterPurge.Links) != 0 {
		t.Fatalf("expected 0 links in bin after permanent deletion, got: %d", len(dashAfterPurge.Links))
	}
}
