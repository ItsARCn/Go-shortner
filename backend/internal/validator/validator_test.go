package validator

import (
	"testing"
)

func TestValidateURL(t *testing.T) {
	serviceDomain := "go.arcn.online"

	tests := []struct {
		name      string
		rawURL    string
		expectErr bool
	}{
		{"Valid HTTPS URL", "https://github.com/golang/go", false},
		{"Valid HTTP URL", "http://example.com/path?arg=1#hash", false},
		{"Empty URL", "", true},
		{"Invalid Scheme (ftp)", "ftp://example.com", true},
		{"Invalid Scheme (javascript)", "javascript:alert(1)", true},
		{"Localhost hostname", "http://localhost:8080/secret", true},
		{"127.0.0.1 loopback", "http://127.0.0.1/admin", true},
		{"Private 192.168.1.1", "http://192.168.1.1/", true},
		{"Private 10.0.0.1", "http://10.0.0.1/", true},
		{"AWS Metadata IP", "http://169.254.169.254/latest/meta-data/", true},
		{"Self domain recursion", "https://go.arcn.online/xyz123", true},
		{"Subdomain recursion", "https://sub.go.arcn.online/xyz123", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ValidateURL(tc.rawURL, serviceDomain)
			if (err != nil) != tc.expectErr {
				t.Errorf("ValidateURL(%q) err = %v, expectErr = %v", tc.rawURL, err, tc.expectErr)
			}
		})
	}
}
