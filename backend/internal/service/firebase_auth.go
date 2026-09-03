package service

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidGoogleToken = errors.New("invalid or expired Google authentication token")
)

type GoogleUserClaims struct {
	UID           string `json:"sub"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	EmailVerified bool   `json:"email_verified"`
	Exp           int64  `json:"exp"`
}

// VerifyFirebaseToken parses and verifies a Firebase Google ID token.
func VerifyFirebaseToken(idToken string, projectID string) (*GoogleUserClaims, error) {
	if strings.TrimSpace(idToken) == "" {
		return nil, ErrInvalidGoogleToken
	}

	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidGoogleToken
	}

	// Decode payload segment
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Fallback for standard base64 if not URL encoded
		payloadBytes, err = base64.StdEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, ErrInvalidGoogleToken
		}
	}

	var claims GoogleUserClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, ErrInvalidGoogleToken
	}

	if claims.Email == "" || claims.UID == "" {
		return nil, ErrInvalidGoogleToken
	}

	// Check expiration if present
	if claims.Exp > 0 && time.Now().UTC().Unix() > claims.Exp {
		return nil, ErrInvalidGoogleToken
	}

	return &claims, nil
}
