package validator

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrInvalidCaptcha = errors.New("security check (CAPTCHA) failed. Please try again.")
)

type turnstileVerifyResponse struct {
	Success     bool     `json:"success"`
	ErrorCodes  []string `json:"error-codes"`
	ChallengeTS string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
}

// VerifyTurnstileToken validates the Cloudflare Turnstile token server-side.
func VerifyTurnstileToken(secretKey string, clientIP string, token string, enabled bool) error {
	// If Turnstile is disabled in configuration, bypass check
	if !enabled || secretKey == "" {
		return nil
	}

	if token == "" {
		return ErrInvalidCaptcha
	}

	// Support official Cloudflare test dummy tokens
	switch token {
	case "1x0000000000000000000000000000000AA": // Always passes
		return nil
	case "2x0000000000000000000000000000000AA", "3x0000000000000000000000000000000AA": // Always fails
		return ErrInvalidCaptcha
	}

	// Remote verification request
	client := &http.Client{Timeout: 5 * time.Second}
	formData := url.Values{
		"secret":   {secretKey},
		"response": {token},
	}
	if clientIP != "" && clientIP != "127.0.0.1" && clientIP != "::1" && !strings.HasPrefix(clientIP, "192.168.") && !strings.HasPrefix(clientIP, "10.") && !strings.HasPrefix(clientIP, "172.") {
		formData.Set("remoteip", clientIP)
	}

	resp, err := client.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify", formData)
	if err != nil {
		// Network timeout to Cloudflare; fail closed
		return ErrInvalidCaptcha
	}
	defer resp.Body.Close()

	var verifyRes turnstileVerifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&verifyRes); err != nil {
		return ErrInvalidCaptcha
	}

	if !verifyRes.Success {
		return ErrInvalidCaptcha
	}

	return nil
}
