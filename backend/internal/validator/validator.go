package validator

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

var (
	ErrEmptyURL         = errors.New("URL cannot be empty")
	ErrURLTooLong       = errors.New("URL cannot exceed 2048 characters")
	ErrInvalidScheme    = errors.New("only http and https protocols are supported")
	ErrInvalidHost      = errors.New("URL host is invalid or missing")
	ErrPrivateIPBlocked = errors.New("destination URL points to a prohibited local or private address")
	ErrRecursiveShortener = errors.New("cannot shorten links to this URL shortener domain")
)

// ValidateURL validates and sanitizes a destination URL.
func ValidateURL(rawURL string, serviceDomain string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", ErrEmptyURL
	}
	if len(rawURL) > 2048 {
		return "", ErrURLTooLong
	}

	// Parse URL
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return "", errors.New("malformed URL")
	}

	// Check scheme
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", ErrInvalidScheme
	}

	// Check host
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return "", ErrInvalidHost
	}

	// Block loopback / recursive links to own service
	if serviceDomain != "" {
		serviceDomain = strings.ToLower(strings.TrimSpace(serviceDomain))
		if host == serviceDomain || strings.HasSuffix(host, "."+serviceDomain) {
			return "", ErrRecursiveShortener
		}
	}

	// Block common localhost names
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || host == "0.0.0.0" {
		return "", ErrPrivateIPBlocked
	}

	// Block private and link-local IP addresses (SSRF protection)
	ip := net.ParseIP(host)
	if ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
			return "", ErrPrivateIPBlocked
		}
		// Block cloud metadata IP 169.254.169.254 explicitly
		if ip.String() == "169.254.169.254" {
			return "", ErrPrivateIPBlocked
		}
	}

	return parsed.String(), nil
}
