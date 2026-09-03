package service

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/arc/go-shortener/internal/config"
	"github.com/arc/go-shortener/internal/models"
	"github.com/arc/go-shortener/internal/repository"
	"github.com/arc/go-shortener/internal/validator"
	"github.com/arc/go-shortener/pkg/shortcode"
)

var (
	ErrLinkNotFound       = errors.New("short link not found")
	ErrLinkExpired        = errors.New("this link has expired")
	ErrLinkDisabled       = errors.New("this link has been disabled due to a policy violation")
	ErrAnonymousQuotaMet  = errors.New("anonymous creation quota exceeded (maximum 15 links per 24 hours)")
	ErrRegisteredQuotaMet = errors.New("monthly creation quota exceeded")
	ErrMaxRetriesExceeded = errors.New("failed to generate unique short code, please try again")
)

type LinkService struct {
	repo *repository.LinkRepository
	cfg  *config.Config
}

func NewLinkService(repo *repository.LinkRepository, cfg *config.Config) *LinkService {
	return &LinkService{
		repo: repo,
		cfg:  cfg,
	}
}

// Shorten creates a new shortened link respecting anonymous & user quotas and expiration limits.
func (s *LinkService) Shorten(req models.ShortenRequest, ownerID *string, clientIP string) (*models.ShortenResponse, error) {
	// 1. Validate destination URL
	cleanURL, err := validator.ValidateURL(req.URL, s.cfg.BaseURL)
	if err != nil {
		return nil, err
	}

	// 2. Parse and enforce expiration
	expiresAt, err := s.resolveExpiration(req.Expiration, ownerID != nil)
	if err != nil {
		return nil, err
	}

	// 3. Enforce Quotas
	isAnonymous := (ownerID == nil)
	if isAnonymous {
		// Daily quota: 15 per 24h
		today := time.Now().UTC().Format("2006-01-02")
		ipHash := hashIdentity(clientIP)
		identityKey := fmt.Sprintf("anon:%s", ipHash)

		allowed, _, err := s.repo.CheckAndIncrementQuota(identityKey, true, today, s.cfg.AnonymousDailyQuota)
		if err != nil {
			if errors.Is(err, repository.ErrQuotaExceeded) {
				return nil, ErrAnonymousQuotaMet
			}
			return nil, fmt.Errorf("quota check failed: %w", err)
		}
		if !allowed {
			return nil, ErrAnonymousQuotaMet
		}
	} else {
		// Registered user quota: 100 per calendar month
		monthWindow := time.Now().UTC().Format("2006-01")
		identityKey := fmt.Sprintf("user:%s", *ownerID)

		allowed, _, err := s.repo.CheckAndIncrementQuota(identityKey, false, monthWindow, s.cfg.RegisteredMonthlyQuota)
		if err != nil {
			if errors.Is(err, repository.ErrQuotaExceeded) {
				return nil, ErrRegisteredQuotaMet
			}
			return nil, fmt.Errorf("user quota check failed: %w", err)
		}
		if !allowed {
			return nil, ErrRegisteredQuotaMet
		}
	}

	// 4. Generate collision-free short code
	code, err := s.generateUniqueCode()
	if err != nil {
		return nil, err
	}

	linkID := hashIdentity(fmt.Sprintf("%s:%d", code, time.Now().UnixNano()))[:16]

	link := &models.Link{
		ID:             linkID,
		ShortCode:      code,
		DestinationURL: cleanURL,
		OwnerID:        ownerID,
		CreatedAt:      time.Now().UTC(),
		ExpiresAt:      expiresAt,
		Status:         models.StatusActive,
		AutoRenew:      false,
		ClickCount:     0,
	}

	if err := s.repo.CreateLink(link); err != nil {
		return nil, fmt.Errorf("failed to save link: %w", err)
	}

	baseURL := strings.TrimRight(s.cfg.BaseURL, "/")
	return &models.ShortenResponse{
		ShortCode: code,
		ShortURL:  fmt.Sprintf("%s/%s", baseURL, code),
		TargetURL: cleanURL,
		ExpiresAt: expiresAt,
	}, nil
}

// Resolve retrieves the destination link for redirection and tracks clicks.
func (s *LinkService) Resolve(code string, clientIP, referrer, userAgent string) (*models.Link, error) {
	link, err := s.repo.GetLinkByCode(code)
	if err != nil {
		if errors.Is(err, repository.ErrLinkNotFound) {
			return nil, ErrLinkNotFound
		}
		return nil, err
	}

	// Moderation check
	if link.Status == models.StatusDisabled {
		return nil, ErrLinkDisabled
	}

	// Expiration check
	if link.IsExpired() {
		if link.Status != models.StatusExpired {
			_ = s.repo.UpdateLinkStatus(link.ID, models.StatusExpired)
			link.Status = models.StatusExpired
		}
		return link, ErrLinkExpired
	}

	// Active link: increment clicks and asynchronously record analytics
	go func(linkID, ref, ua string) {
		_ = s.repo.IncrementClickCount(linkID)
		device, browser, osName := parseUserAgent(ua)
		_ = s.repo.RecordClick(linkID, "Unknown", ref, device, browser, osName)
	}(link.ID, referrer, userAgent)

	return link, nil
}

// GetInfo returns public metadata about a short code (for the expired or info page).
func (s *LinkService) GetInfo(code string) (*models.LinkInfoResponse, error) {
	link, err := s.repo.GetLinkByCode(code)
	if err != nil {
		return nil, ErrLinkNotFound
	}

	isExpired := link.IsExpired()
	status := link.Status
	if isExpired && status == models.StatusActive {
		status = models.StatusExpired
	}

	return &models.LinkInfoResponse{
		ShortCode: link.ShortCode,
		Status:    status,
		CreatedAt: link.CreatedAt,
		ExpiresAt: link.ExpiresAt,
		IsExpired: isExpired,
	}, nil
}

func (s *LinkService) resolveExpiration(exp string, isRegistered bool) (time.Time, error) {
	now := time.Now().UTC()
	exp = strings.ToLower(strings.TrimSpace(exp))

	var duration time.Duration
	switch exp {
	case "1h", "1hour", "1 hour":
		duration = 1 * time.Hour
	case "1d", "1day", "1 day":
		duration = 24 * time.Hour
	case "3d", "3days", "3 days":
		duration = 3 * 24 * time.Hour
	case "7d", "7days", "7 days", "":
		duration = 7 * 24 * time.Hour
	case "30d", "30days", "30 days", "1m", "1month":
		if !isRegistered {
			duration = 7 * 24 * time.Hour // clamp for anonymous
		} else {
			duration = 30 * 24 * time.Hour
		}
	case "90d", "90days", "3m", "3months", "3 months":
		if !isRegistered {
			duration = 7 * 24 * time.Hour
		} else {
			duration = 90 * 24 * time.Hour
		}
	case "180d", "180days", "6m", "6months", "6 months":
		if !isRegistered {
			duration = 7 * 24 * time.Hour
		} else {
			duration = 180 * 24 * time.Hour
		}
	case "365d", "365days", "1y", "1year", "1 year":
		if !isRegistered {
			duration = 7 * 24 * time.Hour
		} else {
			duration = 365 * 24 * time.Hour
		}
	default:
		// Default to 7 days
		duration = 7 * 24 * time.Hour
	}

	// Absolute clamp: anonymous max 7 days
	if !isRegistered && duration > time.Duration(s.cfg.AnonymousMaxExpirationDays)*24*time.Hour {
		duration = time.Duration(s.cfg.AnonymousMaxExpirationDays) * 24 * time.Hour
	}
	// Registered max 365 days
	if isRegistered && duration > time.Duration(s.cfg.RegisteredMaxExpirationDays)*24*time.Hour {
		duration = time.Duration(s.cfg.RegisteredMaxExpirationDays) * 24 * time.Hour
	}

	return now.Add(duration), nil
}

func (s *LinkService) generateUniqueCode() (string, error) {
	for retries := 0; retries < 5; retries++ {
		length := 6
		if retries >= 3 {
			length = 7 // escalate length on collision
		}
		code, err := shortcode.Generate(length)
		if err != nil {
			return "", err
		}

		_, err = s.repo.GetLinkByCode(code)
		if errors.Is(err, repository.ErrLinkNotFound) {
			return code, nil // Found unique code
		}
	}
	return "", ErrMaxRetriesExceeded
}

func hashIdentity(id string) string {
	h := sha256.Sum256([]byte(id))
	return hex.EncodeToString(h[:16])
}

func parseUserAgent(ua string) (device, browser, osName string) {
	uaLower := strings.ToLower(ua)
	if strings.Contains(uaLower, "mobile") || strings.Contains(uaLower, "android") || strings.Contains(uaLower, "iphone") {
		device = "Mobile"
	} else if strings.Contains(uaLower, "tablet") || strings.Contains(uaLower, "ipad") {
		device = "Tablet"
	} else {
		device = "Desktop"
	}

	if strings.Contains(uaLower, "firefox") {
		browser = "Firefox"
	} else if strings.Contains(uaLower, "edg") {
		browser = "Edge"
	} else if strings.Contains(uaLower, "chrome") {
		browser = "Chrome"
	} else if strings.Contains(uaLower, "safari") {
		browser = "Safari"
	} else {
		browser = "Other"
	}

	if strings.Contains(uaLower, "windows") {
		osName = "Windows"
	} else if strings.Contains(uaLower, "macintosh") || strings.Contains(uaLower, "mac os") {
		osName = "macOS"
	} else if strings.Contains(uaLower, "linux") {
		osName = "Linux"
	} else if strings.Contains(uaLower, "android") {
		osName = "Android"
	} else if strings.Contains(uaLower, "ios") || strings.Contains(uaLower, "iphone") || strings.Contains(uaLower, "ipad") {
		osName = "iOS"
	} else {
		osName = "Other"
	}

	return device, browser, osName
}

// RenewLink renews an expired link owned by the user, preserving the exact short code without consuming quota.
func (s *LinkService) RenewLink(shortCode string, ownerID string, expirationStr string) (*models.Link, error) {
	link, err := s.repo.GetLinkByCode(shortCode)
	if err != nil {
		return nil, ErrLinkNotFound
	}

	if link.OwnerID == nil || *link.OwnerID != ownerID {
		return nil, repository.ErrUnauthorized
	}

	newExpiresAt, err := s.resolveExpiration(expirationStr, true)
	if err != nil {
		return nil, err
	}

	if err := s.repo.RenewLink(shortCode, ownerID, newExpiresAt); err != nil {
		return nil, err
	}

	link.ExpiresAt = newExpiresAt
	link.Status = models.StatusActive
	return link, nil
}

// DeleteLink soft-deletes a user's shortened link.
func (s *LinkService) DeleteLink(shortCode string, ownerID string) error {
	link, err := s.repo.GetLinkByCode(shortCode)
	if err != nil {
		return ErrLinkNotFound
	}

	if link.OwnerID == nil || *link.OwnerID != ownerID {
		return repository.ErrUnauthorized
	}

	return s.repo.SoftDeleteLink(shortCode, ownerID)
}

// GetUserDashboard fetches user quota, statistics, and paginated links.
func (s *LinkService) GetUserDashboard(ownerID string, quotaLimit int, search, status string, page, limit int) (*models.DashboardResponse, error) {
	monthWindow := time.Now().UTC().Format("2006-01")
	stats, err := s.repo.GetUserStats(ownerID, quotaLimit, monthWindow)
	if err != nil {
		return nil, fmt.Errorf("failed to get user stats: %w", err)
	}

	links, total, err := s.repo.GetUserLinks(ownerID, search, status, page, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get user links: %w", err)
	}

	return &models.DashboardResponse{
		Stats: *stats,
		Links: links,
		Page:  page,
		Total: total,
	}, nil
}
