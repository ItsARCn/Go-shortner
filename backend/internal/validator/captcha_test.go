package validator

import (
	"testing"
)

func TestVerifyTurnstileToken(t *testing.T) {
	// 1. When disabled, empty token should pass
	err := VerifyTurnstileToken("dummy_secret", "127.0.0.1", "", false)
	if err != nil {
		t.Errorf("expected nil when disabled, got %v", err)
	}

	// 2. When enabled, empty token should fail
	err = VerifyTurnstileToken("dummy_secret", "127.0.0.1", "", true)
	if err != ErrInvalidCaptcha {
		t.Errorf("expected ErrInvalidCaptcha for empty token, got %v", err)
	}

	// 3. Test token 1x (Always pass)
	err = VerifyTurnstileToken("dummy_secret", "127.0.0.1", "1x0000000000000000000000000000000AA", true)
	if err != nil {
		t.Errorf("expected pass for 1x test token, got %v", err)
	}

	// 4. Test token 2x (Always fail)
	err = VerifyTurnstileToken("dummy_secret", "127.0.0.1", "2x0000000000000000000000000000000AA", true)
	if err != ErrInvalidCaptcha {
		t.Errorf("expected fail for 2x test token, got %v", err)
	}
}
