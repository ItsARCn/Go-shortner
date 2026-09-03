package service

import (
	"testing"
	"time"

	"github.com/arc/go-shortener/internal/models"
)

func TestUpdateUserRole(t *testing.T) {
	_, _, userRepo, cleanup := setupModerationTestDB(t)
	defer cleanup()

	adminSvc := NewAdminService(userRepo, nil)

	superAdminID := "super-admin-root"
	user := &models.User{
		ID:           "target-user-1",
		FirstName:    "New",
		LastName:     "Mod",
		Email:        "newmod@example.com",
		AuthProvider: "email",
		Role:         models.RoleUser,
		Status:       models.UserStatusActive,
		QuotaLimit:   100,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	_ = userRepo.CreateUser(user)

	// Promote user to moderator
	err := adminSvc.UpdateUserRole(superAdminID, user.ID, models.RoleModerator)
	if err != nil {
		t.Fatalf("promote to moderator failed: %v", err)
	}

	u, err := userRepo.GetUserByID(user.ID)
	if err != nil || u.Role != models.RoleModerator {
		t.Errorf("expected role moderator, got: %s", u.Role)
	}

	// Cannot update own role
	err = adminSvc.UpdateUserRole(superAdminID, superAdminID, models.RoleUser)
	if err == nil {
		t.Errorf("expected error when trying to change own role")
	}
}
