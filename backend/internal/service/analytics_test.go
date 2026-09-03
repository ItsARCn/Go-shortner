package service

import (
	"testing"

	"github.com/arc/go-shortener/internal/models"
	"github.com/arc/go-shortener/internal/repository"
)

func TestLinkAnalyticsMetrics(t *testing.T) {
	svc, repo, cleanup := setupUserLinkTestDB(t)
	defer cleanup()

	ownerID := "analytics-user-1"
	otherUser := "unauthorized-user"

	// 1. Create link
	req := models.ShortenRequest{
		URL:        "https://example.com/docs",
		Expiration: "30d",
	}
	resp, err := svc.Shorten(req, &ownerID, "127.0.0.1")
	if err != nil {
		t.Fatalf("shorten failed: %v", err)
	}

	link, _ := repo.GetLinkByCode(resp.ShortCode)

	// 2. Record simulated clicks
	// 3 clicks from Chrome Desktop (twitter referrer)
	for i := 0; i < 3; i++ {
		_ = repo.RecordClick(link.ID, "US", "https://twitter.com", "Desktop", "Chrome", "macOS")
	}
	// 2 clicks from Mobile Safari (google referrer)
	for i := 0; i < 2; i++ {
		_ = repo.RecordClick(link.ID, "UK", "https://google.com", "Mobile", "Safari", "iOS")
	}

	// 3. Unauthorized user cannot fetch analytics
	_, err = svc.GetLinkAnalytics(resp.ShortCode, otherUser)
	if err != repository.ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized for other user, got: %v", err)
	}

	// 4. Owner fetches analytics
	analytics, err := svc.GetLinkAnalytics(resp.ShortCode, ownerID)
	if err != nil {
		t.Fatalf("owner analytics fetch failed: %v", err)
	}

	if analytics.TotalClicks != 5 {
		t.Errorf("expected 5 total clicks, got %d", analytics.TotalClicks)
	}
	if analytics.ClicksToday != 5 {
		t.Errorf("expected 5 clicks today, got %d", analytics.ClicksToday)
	}

	// Verify device breakdown (3 Desktop = 60%, 2 Mobile = 40%)
	if len(analytics.Devices) < 2 {
		t.Fatalf("expected at least 2 device categories, got %d", len(analytics.Devices))
	}
	if analytics.Devices[0].Name != "Desktop" || analytics.Devices[0].Count != 3 || analytics.Devices[0].Percentage != 60.0 {
		t.Errorf("unexpected Desktop metrics: %+v", analytics.Devices[0])
	}
	if analytics.Devices[1].Name != "Mobile" || analytics.Devices[1].Count != 2 || analytics.Devices[1].Percentage != 40.0 {
		t.Errorf("unexpected Mobile metrics: %+v", analytics.Devices[1])
	}

	// Verify browsers
	if len(analytics.Browsers) < 2 {
		t.Fatalf("expected at least 2 browser categories, got %d", len(analytics.Browsers))
	}
	if analytics.Browsers[0].Name != "Chrome" || analytics.Browsers[0].Count != 3 {
		t.Errorf("unexpected Chrome count: %+v", analytics.Browsers[0])
	}
}
