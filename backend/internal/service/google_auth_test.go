package service

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/arc/go-shortener/internal/models"
)

func createDummyGoogleToken(uid, email, name string, expired bool) string {
	header := `{"alg":"none","typ":"JWT"}`
	exp := time.Now().Add(1 * time.Hour).Unix()
	if expired {
		exp = time.Now().Add(-1 * time.Hour).Unix()
	}

	payload := fmt.Sprintf(`{"sub":"%s","email":"%s","name":"%s","email_verified":true,"exp":%d}`, uid, email, name, exp)

	hB64 := base64.RawURLEncoding.EncodeToString([]byte(header))
	pB64 := base64.RawURLEncoding.EncodeToString([]byte(payload))
	sigB64 := base64.RawURLEncoding.EncodeToString([]byte("sig"))

	return fmt.Sprintf("%s.%s.%s", hB64, pB64, sigB64)
}

func TestGoogleLoginNewUser(t *testing.T) {
	authSvc, userRepo, cleanup := setupAuthTestDB(t)
	defer cleanup()

	token := createDummyGoogleToken("firebase-uid-100", "sam@gmail.com", "Sam Wilson", false)

	resp, err := authSvc.LoginWithGoogle(token, "127.0.0.1", "Chrome")
	if err != nil {
		t.Fatalf("Google login failed: %v", err)
	}

	if resp.User.Email != "sam@gmail.com" {
		t.Errorf("expected email sam@gmail.com, got %s", resp.User.Email)
	}
	if resp.User.FirstName != "Sam" || resp.User.LastName != "Wilson" {
		t.Errorf("expected Sam Wilson, got %s %s", resp.User.FirstName, resp.User.LastName)
	}
	if resp.User.AuthProvider != "google" {
		t.Errorf("expected auth_provider google, got %s", resp.User.AuthProvider)
	}
	if resp.User.FirebaseUID == nil || *resp.User.FirebaseUID != "firebase-uid-100" {
		t.Errorf("expected firebase_uid firebase-uid-100")
	}

	// Verify user persisted in DB
	u, err := userRepo.GetUserByEmail("sam@gmail.com")
	if err != nil {
		t.Fatalf("user not found in db: %v", err)
	}
	if u.ID != resp.User.ID {
		t.Errorf("mismatched user ID")
	}
}

func TestGoogleLoginAccountLinking(t *testing.T) {
	authSvc, userRepo, cleanup := setupAuthTestDB(t)
	defer cleanup()

	// 1. User registers first with email & password
	regResp, err := authSvc.Register(models.RegisterRequest{
		FirstName:       "Alex",
		LastName:        "Morgan",
		Email:           "alex@gmail.com",
		Password:        "MyPassword123!",
		ConfirmPassword: "MyPassword123!",
	}, "127.0.0.1", "Agent")
	if err != nil {
		t.Fatalf("email register failed: %v", err)
	}

	originalUserID := regResp.User.ID

	// 2. User logs in with Google using the same email (alex@gmail.com)
	token := createDummyGoogleToken("firebase-uid-alex-555", "alex@gmail.com", "Alex Morgan", false)

	googleResp, err := authSvc.LoginWithGoogle(token, "127.0.0.1", "Chrome")
	if err != nil {
		t.Fatalf("Google login failed: %v", err)
	}

	// Must link to the SAME user ID without duplicate error!
	if googleResp.User.ID != originalUserID {
		t.Errorf("account linking failed: expected user ID %s, got %s", originalUserID, googleResp.User.ID)
	}

	// Verify in database that firebase_uid was updated
	u, _ := userRepo.GetUserByID(originalUserID)
	if u.FirebaseUID == nil || *u.FirebaseUID != "firebase-uid-alex-555" {
		t.Errorf("firebase_uid was not linked in database")
	}
	if u.AuthProvider != "google" {
		t.Errorf("auth_provider was not updated to google")
	}
}
