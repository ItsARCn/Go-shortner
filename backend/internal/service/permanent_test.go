package service

import (
	"testing"
	"time"

	"github.com/arc/go-shortener/internal/models"
)

func TestPermanentLinkRequestAndApproval(t *testing.T) {
	svc, linkRepo, userRepo, cleanup := setupModerationTestDB(t)
	defer cleanup()

	// 1. Create registered user & admin user
	user := &models.User{
		ID:           "perm-user-1",
		FirstName:    "Permanent",
		LastName:     "Requester",
		Email:        "perm@example.com",
		AuthProvider: "email",
		Role:         models.RoleUser,
		Status:       models.UserStatusActive,
		QuotaLimit:   100,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	_ = userRepo.CreateUser(user)

	admin := &models.User{
		ID:           "admin-approver-1",
		FirstName:    "Admin",
		LastName:     "Approver",
		Email:        "approver@example.com",
		AuthProvider: "email",
		Role:         models.RoleSuperAdmin,
		Status:       models.UserStatusActive,
		QuotaLimit:   999999,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	_ = userRepo.CreateUser(admin)

	// 2. Create link with short expiration
	ownerID := user.ID
	shortenResp, err := svc.Shorten(models.ShortenRequest{
		URL:        "https://docs.myproduct.org",
		Expiration: "7d",
	}, &ownerID, "127.0.0.1")
	if err != nil {
		t.Fatalf("shorten failed: %v", err)
	}

	adminSvc := NewAdminService(userRepo, linkRepo)

	// 3. User requests permanent status
	err = svc.RequestPermanentLink(shortenResp.ShortCode, ownerID, "Important docs URL on printed flyers")
	if err != nil {
		t.Fatalf("request permanent failed: %v", err)
	}

	// 4. Admin lists permanent requests
	requests, total, err := adminSvc.ListPermanentRequests("pending", 1, 10)
	if err != nil {
		t.Fatalf("list permanent requests failed: %v", err)
	}
	if total != 1 || len(requests) != 1 {
		t.Fatalf("expected 1 permanent request, got %d", total)
	}
	if requests[0].ShortCode != shortenResp.ShortCode {
		t.Errorf("expected short code %s, got %s", shortenResp.ShortCode, requests[0].ShortCode)
	}

	// 5. Admin approves permanent request
	err = adminSvc.ResolvePermanentRequest(requests[0].ID, true, admin.ID)
	if err != nil {
		t.Fatalf("resolve permanent request failed: %v", err)
	}

	// 6. Verify link in DB now has auto_renew = 1
	link, err := linkRepo.GetLinkByCode(shortenResp.ShortCode)
	if err != nil {
		t.Fatalf("link not found: %v", err)
	}
	if !link.AutoRenew {
		t.Errorf("expected AutoRenew to be true")
	}

	// 7. Test that AutoRenew links NEVER report expired even if expired_at is in past
	link.ExpiresAt = time.Now().Add(-48 * time.Hour)
	if link.IsExpired() {
		t.Errorf("expected AutoRenew link to NEVER expire, but IsExpired returned true")
	}
}

func TestPermanentLinkRequestRejection(t *testing.T) {
	svc, linkRepo, userRepo, cleanup := setupModerationTestDB(t)
	defer cleanup()

	user := &models.User{
		ID:           "perm-user-2",
		FirstName:    "John",
		LastName:     "Doe",
		Email:        "john@example.com",
		AuthProvider: "email",
		Role:         models.RoleUser,
		Status:       models.UserStatusActive,
		QuotaLimit:   100,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	_ = userRepo.CreateUser(user)

	admin := &models.User{
		ID:           "admin-rejecter-1",
		FirstName:    "Admin",
		LastName:     "Rejecter",
		Email:        "rejecter@example.com",
		AuthProvider: "email",
		Role:         models.RoleModerator,
		Status:       models.UserStatusActive,
		QuotaLimit:   999999,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	_ = userRepo.CreateUser(admin)

	ownerID := user.ID
	shortenResp, _ := svc.Shorten(models.ShortenRequest{
		URL:        "https://temporary.example.com",
		Expiration: "7d",
	}, &ownerID, "127.0.0.1")

	adminSvc := NewAdminService(userRepo, linkRepo)

	_ = svc.RequestPermanentLink(shortenResp.ShortCode, ownerID, "Test reason")
	requests, _, _ := adminSvc.ListPermanentRequests("pending", 1, 10)

	// Admin rejects request
	err := adminSvc.ResolvePermanentRequest(requests[0].ID, false, admin.ID)
	if err != nil {
		t.Fatalf("reject failed: %v", err)
	}

	// Link should NOT be auto_renew
	link, _ := linkRepo.GetLinkByCode(shortenResp.ShortCode)
	if link.AutoRenew {
		t.Errorf("rejected link should not have AutoRenew")
	}
}
