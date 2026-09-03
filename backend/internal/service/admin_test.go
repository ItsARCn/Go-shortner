package service

import (
	"testing"

	"github.com/arc/go-shortener/internal/models"
)

func TestAdminOverviewAndBootstrap(t *testing.T) {
	authSvc, userRepo, cleanup := setupAuthTestDB(t)
	defer cleanup()

	// 1. Bootstrap super admin
	err := userRepo.EnsureSuperAdminBootstrap("admin@example.com", "$2a$12$dummyhashforadminbootstrap")
	if err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}

	admin, err := userRepo.GetUserByEmail("admin@example.com")
	if err != nil {
		t.Fatalf("admin not found: %v", err)
	}
	if admin.Role != models.RoleSuperAdmin {
		t.Errorf("expected RoleSuperAdmin, got %s", admin.Role)
	}

	// 2. Overview metrics
	adminSvc := NewAdminService(userRepo, nil)
	stats, err := adminSvc.GetOverviewStats()
	if err != nil {
		t.Fatalf("failed to get overview stats: %v", err)
	}
	if stats.TotalUsers < 1 {
		t.Errorf("expected at least 1 total user, got %d", stats.TotalUsers)
	}

	_ = authSvc
}

func TestAbuseReportingFlow(t *testing.T) {
	svc, repo, cleanup := setupUserLinkTestDB(t)
	defer cleanup()

	adminSvc := NewAdminService(nil, repo)

	// 1. Create link
	linkOwner := "report-user-1"
	shortenResp, err := svc.Shorten(models.ShortenRequest{
		URL:        "https://malicious-site.example.com",
		Expiration: "7d",
	}, &linkOwner, "127.0.0.1")
	if err != nil {
		t.Fatalf("shorten failed: %v", err)
	}

	// 2. Submit abuse report
	err = adminSvc.SubmitReport(shortenResp.ShortCode, "phishing", "Fake bank credential harvester", "192.0.2.1")
	if err != nil {
		t.Fatalf("submit report failed: %v", err)
	}

	// 3. List reports
	reports, total, err := adminSvc.ListReports("pending", 1, 10)
	if err != nil {
		t.Fatalf("list reports failed: %v", err)
	}
	if total != 1 || len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", total)
	}
	if reports[0].Reason != "phishing" {
		t.Errorf("expected reason phishing, got %s", reports[0].Reason)
	}

	// 4. Resolve report
	err = adminSvc.ResolveReport(reports[0].ID, "reviewed")
	if err != nil {
		t.Fatalf("resolve report failed: %v", err)
	}

	// 5. Verify pending reports now 0
	_, total, _ = adminSvc.ListReports("pending", 1, 10)
	if total != 0 {
		t.Errorf("expected 0 pending reports after resolve, got %d", total)
	}
}

func TestLinkModerationDisableAndEnable(t *testing.T) {
	svc, repo, cleanup := setupUserLinkTestDB(t)
	defer cleanup()

	adminSvc := NewAdminService(nil, repo)

	owner := "mod-user"
	shortenResp, _ := svc.Shorten(models.ShortenRequest{
		URL:        "https://test-link.com",
		Expiration: "7d",
	}, &owner, "127.0.0.1")

	// Disable link
	err := adminSvc.DisableLink(shortenResp.ShortCode)
	if err != nil {
		t.Fatalf("disable link failed: %v", err)
	}

	// Resolution should now report disabled
	_, err = svc.Resolve(shortenResp.ShortCode, "127.0.0.1", "curl", "")
	if err != ErrLinkDisabled {
		t.Errorf("expected ErrLinkDisabled, got: %v", err)
	}

	// Enable link
	err = adminSvc.EnableLink(shortenResp.ShortCode)
	if err != nil {
		t.Fatalf("enable link failed: %v", err)
	}

	// Resolution should now succeed
	resolvedLink, err := svc.Resolve(shortenResp.ShortCode, "127.0.0.1", "curl", "")
	if err != nil || resolvedLink.DestinationURL != "https://test-link.com" {
		t.Errorf("expected resolution to https://test-link.com, got: %v, err: %v", resolvedLink, err)
	}
}
