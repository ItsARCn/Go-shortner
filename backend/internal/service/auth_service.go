package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/arc/go-shortener/internal/config"
	"github.com/arc/go-shortener/internal/models"
	"github.com/arc/go-shortener/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidEmail       = errors.New("please provide a valid email address")
	ErrNameRequired       = errors.New("first and last name are required")
	ErrPasswordTooShort   = errors.New("password must be at least 8 characters")
	ErrPasswordMismatch   = errors.New("passwords do not match")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrAccountBanned      = errors.New("this account has been banned due to terms of service violations")
	ErrAccountTimedOut    = errors.New("this account is temporarily restricted")
	ErrInvalidToken       = errors.New("invalid or expired session token")
)

type AuthService struct {
	userRepo *repository.UserRepository
	cfg      *config.Config
}

func NewAuthService(userRepo *repository.UserRepository, cfg *config.Config) *AuthService {
	return &AuthService{
		userRepo: userRepo,
		cfg:      cfg,
	}
}

// Register registers a new user with email and password.
func (s *AuthService) Register(req models.RegisterRequest, clientIP, userAgent string) (*models.AuthResponse, error) {
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if req.FirstName == "" || req.LastName == "" {
		return nil, ErrNameRequired
	}

	if _, err := mail.ParseAddress(req.Email); err != nil {
		return nil, ErrInvalidEmail
	}

	if len(req.Password) < 8 {
		return nil, ErrPasswordTooShort
	}

	if req.Password != req.ConfirmPassword {
		return nil, ErrPasswordMismatch
	}

	// Check for existing user
	_, err := s.userRepo.GetUserByEmail(req.Email)
	if err == nil {
		return nil, repository.ErrUserAlreadyExists
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	userID := generateID()
	now := time.Now().UTC()

	// Automatic Super Admin claim for the very first user
	role := models.RoleUser
	quotaLimit := s.cfg.RegisteredMonthlyQuota
	hasAdmin, err := s.userRepo.HasSuperAdmin()
	if err == nil && !hasAdmin {
		role = models.RoleSuperAdmin
		quotaLimit = 999999
	}

	user := &models.User{
		ID:           userID,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		AuthProvider: "email",
		Role:         role,
		Status:       models.UserStatusActive,
		QuotaLimit:   quotaLimit,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.userRepo.CreateUser(user); err != nil {
		return nil, err
	}

	// Record initial audit
	ipHash := hashIdentity(clientIP)
	_ = s.userRepo.RecordLoginAttempt(user.Email, "email", "SUCCESS", ipHash, userAgent)
	_ = s.userRepo.UpdateLastLogin(user.ID)

	token, err := s.GenerateToken(user)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		Token: token,
		User:  user,
	}, nil
}

// Login authenticates a user and returns a session token.
func (s *AuthService) Login(req models.LoginRequest, clientIP, userAgent string) (*models.AuthResponse, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	ipHash := hashIdentity(clientIP)

	user, err := s.userRepo.GetUserByEmail(email)
	if err != nil {
		_ = s.userRepo.RecordLoginAttempt(email, "email", "FAILED", ipHash, userAgent)
		return nil, ErrInvalidCredentials
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		_ = s.userRepo.RecordLoginAttempt(email, "email", "FAILED", ipHash, userAgent)
		return nil, ErrInvalidCredentials
	}

	// Check ban status
	if user.Status == models.UserStatusBanned {
		_ = s.userRepo.RecordLoginAttempt(email, "email", "UNAUTHORIZED", ipHash, userAgent)
		return nil, ErrAccountBanned
	}

	// Check timeout status
	if user.Status == models.UserStatusTimedOut && user.TimeoutUntil != nil {
		if time.Now().UTC().Before(*user.TimeoutUntil) {
			// Account is still timed out
			// In PRD: "user can still log in but cannot create or manage links according to restriction policy"
		}
	}

	// Login successful
	_ = s.userRepo.RecordLoginAttempt(email, "email", "SUCCESS", ipHash, userAgent)
	_ = s.userRepo.UpdateLastLogin(user.ID)

	token, err := s.GenerateToken(user)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		Token: token,
		User:  user,
	}, nil
}

// GetUserByID retrieves user profile by ID.
func (s *AuthService) GetUserByID(userID string) (*models.User, error) {
	return s.userRepo.GetUserByID(userID)
}

// GenerateToken generates an HMAC-SHA256 signed JWT session token.
func (s *AuthService) GenerateToken(user *models.User) (string, error) {
	headerJSON := `{"alg":"HS256","typ":"JWT"}`
	headerB64 := base64.RawURLEncoding.EncodeToString([]byte(headerJSON))

	exp := time.Now().UTC().Add(72 * time.Hour).Unix()
	claims := models.TokenClaims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
		Exp:    exp,
	}

	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsBytes)

	message := headerB64 + "." + claimsB64
	sig := signHMAC(message, s.cfg.JWTSecret)

	return message + "." + sig, nil
}

// VerifyToken validates the JWT signature and returns the token claims.
func (s *AuthService) VerifyToken(tokenString string) (*models.TokenClaims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	message := parts[0] + "." + parts[1]
	expectedSig := signHMAC(message, s.cfg.JWTSecret)
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return nil, ErrInvalidToken
	}

	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims models.TokenClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	if time.Now().UTC().Unix() > claims.Exp {
		return nil, ErrInvalidToken
	}

	return &claims, nil
}

func signHMAC(message, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func generateID() string {
	bytes := make([]byte, 12)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// LoginWithGoogle authenticates or registers a user via a Google/Firebase ID token with safe account linking.
func (s *AuthService) LoginWithGoogle(idToken string, clientIP, userAgent string) (*models.AuthResponse, error) {
	claims, err := VerifyFirebaseToken(idToken, s.cfg.FirebaseProjectID)
	if err != nil {
		return nil, err
	}

	email := strings.ToLower(strings.TrimSpace(claims.Email))
	ipHash := hashIdentity(clientIP)

	user, err := s.userRepo.GetUserByEmail(email)
	if err == nil {
		// Existing account found: perform safe account linking
		if user.Status == models.UserStatusBanned {
			_ = s.userRepo.RecordLoginAttempt(email, "google", "UNAUTHORIZED", ipHash, userAgent)
			return nil, ErrAccountBanned
		}

		// Link Firebase UID if not already linked
		if user.FirebaseUID == nil || *user.FirebaseUID != claims.UID {
			_ = s.userRepo.LinkGoogleAccount(user.ID, claims.UID)
			user.FirebaseUID = &claims.UID
			user.AuthProvider = "google"
		}
	} else if errors.Is(err, repository.ErrUserNotFound) {
		// New user: create user account with Google provider
		firstName := "Google"
		lastName := "User"
		if claims.Name != "" {
			nameParts := strings.SplitN(strings.TrimSpace(claims.Name), " ", 2)
			firstName = nameParts[0]
			if len(nameParts) > 1 {
				lastName = nameParts[1]
			}
		}

		userID := generateID()
		now := time.Now().UTC()

		// Automatic Super Admin claim for the very first user
		role := models.RoleUser
		quotaLimit := s.cfg.RegisteredMonthlyQuota
		hasAdmin, err := s.userRepo.HasSuperAdmin()
		if err == nil && !hasAdmin {
			role = models.RoleSuperAdmin
			quotaLimit = 999999
		}

		user = &models.User{
			ID:           userID,
			FirstName:    firstName,
			LastName:     lastName,
			Email:        email,
			AuthProvider: "google",
			FirebaseUID:  &claims.UID,
			Role:         role,
			Status:       models.UserStatusActive,
			QuotaLimit:   quotaLimit,
			CreatedAt:    now,
			UpdatedAt:    now,
		}

		if err := s.userRepo.CreateUser(user); err != nil {
			return nil, fmt.Errorf("failed to create Google user: %w", err)
		}
	} else {
		return nil, err
	}

	// Record audit
	_ = s.userRepo.RecordLoginAttempt(email, "google", "SUCCESS", ipHash, userAgent)
	_ = s.userRepo.UpdateLastLogin(user.ID)

	token, err := s.GenerateToken(user)
	if err != nil {
		return nil, err
	}

	return &models.AuthResponse{
		Token: token,
		User:  user,
	}, nil
}

// RecordUnauthorizedAudit writes an unauthorized access attempt to the login_records security log.
func (s *AuthService) RecordUnauthorizedAudit(email, method, clientIP, userAgent string) {
	ipHash := hashIdentity(clientIP)
	_ = s.userRepo.RecordLoginAttempt(email, method, "UNAUTHORIZED", ipHash, userAgent)
}
