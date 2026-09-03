package service

import (
	"os"
	"testing"

	"github.com/arc/go-shortener/internal/config"
	"github.com/arc/go-shortener/internal/database"
	"github.com/arc/go-shortener/internal/models"
	"github.com/arc/go-shortener/internal/repository"
)

func setupAuthTestDB(t *testing.T) (*AuthService, *repository.UserRepository, func()) {
	tmpDB, err := os.CreateTemp("", "go-auth-test-*.sqlite")
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
		JWTSecret:              "test-secret-key-at-least-32-characters-long!",
		RegisteredMonthlyQuota: 100,
	}

	userRepo := repository.NewUserRepository(db)
	authSvc := NewAuthService(userRepo, cfg)

	cleanup := func() {
		db.Close()
		os.Remove(tmpDB.Name())
		os.Remove(tmpDB.Name() + "-wal")
		os.Remove(tmpDB.Name() + "-shm")
	}

	return authSvc, userRepo, cleanup
}

func TestAuthRegisterAndLogin(t *testing.T) {
	authSvc, _, cleanup := setupAuthTestDB(t)
	defer cleanup()

	// 1. Password mismatch error
	_, err := authSvc.Register(models.RegisterRequest{
		FirstName:       "John",
		LastName:        "Doe",
		Email:           "john@example.com",
		Password:        "Password123!",
		ConfirmPassword: "DifferentPassword!",
	}, "127.0.0.1", "TestAgent")
	if err != ErrPasswordMismatch {
		t.Errorf("expected ErrPasswordMismatch, got: %v", err)
	}

	// 2. Short password error
	_, err = authSvc.Register(models.RegisterRequest{
		FirstName:       "John",
		LastName:        "Doe",
		Email:           "john@example.com",
		Password:        "short",
		ConfirmPassword: "short",
	}, "127.0.0.1", "TestAgent")
	if err != ErrPasswordTooShort {
		t.Errorf("expected ErrPasswordTooShort, got: %v", err)
	}

	// 3. Successful registration
	resp, err := authSvc.Register(models.RegisterRequest{
		FirstName:       "John",
		LastName:        "Doe",
		Email:           "john@example.com",
		Password:        "Password123!",
		ConfirmPassword: "Password123!",
	}, "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("registration failed: %v", err)
	}
	if resp.Token == "" {
		t.Errorf("expected valid JWT token in auth response")
	}
	if resp.User.Email != "john@example.com" {
		t.Errorf("expected email john@example.com, got %s", resp.User.Email)
	}

	// 4. Duplicate registration rejected
	_, err = authSvc.Register(models.RegisterRequest{
		FirstName:       "John",
		LastName:        "Doe",
		Email:           "john@example.com",
		Password:        "Password123!",
		ConfirmPassword: "Password123!",
	}, "127.0.0.1", "TestAgent")
	if err != repository.ErrUserAlreadyExists {
		t.Errorf("expected ErrUserAlreadyExists, got: %v", err)
	}

	// 5. Successful login
	loginResp, err := authSvc.Login(models.LoginRequest{
		Email:    "john@example.com",
		Password: "Password123!",
	}, "127.0.0.1", "TestAgent")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	// 6. Verify token claims
	claims, err := authSvc.VerifyToken(loginResp.Token)
	if err != nil {
		t.Fatalf("token verification failed: %v", err)
	}
	if claims.Email != "john@example.com" {
		t.Errorf("expected claims email john@example.com, got %s", claims.Email)
	}

	// 7. Invalid password rejected
	_, err = authSvc.Login(models.LoginRequest{
		Email:    "john@example.com",
		Password: "WrongPassword!",
	}, "127.0.0.1", "TestAgent")
	if err != ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got: %v", err)
	}
}
