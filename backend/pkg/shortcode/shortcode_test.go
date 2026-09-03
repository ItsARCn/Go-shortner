package shortcode

import (
	"strings"
	"testing"
)

func TestGenerateDefault(t *testing.T) {
	code, err := GenerateDefault()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(code) != 6 {
		t.Errorf("expected length 6, got %d", len(code))
	}
	for _, char := range code {
		if !strings.ContainsRune(charset, char) {
			t.Errorf("invalid character in code: %c", char)
		}
	}
}

func TestGenerateLengths(t *testing.T) {
	lengths := []int{6, 7, 8}
	for _, l := range lengths {
		code, err := Generate(l)
		if err != nil {
			t.Fatalf("length %d failed: %v", l, err)
		}
		if len(code) != l {
			t.Errorf("expected length %d, got %d", l, len(code))
		}
	}

	_, err := Generate(0)
	if err == nil {
		t.Errorf("expected error for length 0")
	}
}

func TestUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	iterations := 10000
	for i := 0; i < iterations; i++ {
		code, err := GenerateDefault()
		if err != nil {
			t.Fatalf("error on iteration %d: %v", i, err)
		}
		if seen[code] {
			t.Fatalf("duplicate code generated: %s", code)
		}
		seen[code] = true
	}
}
