package service

import (
	"errors"
	"strings"
	"time"

	"github.com/arc/go-shortener/internal/models"
	"github.com/arc/go-shortener/internal/repository"
)

var (
	ErrInvalidDuration     = errors.New("invalid timeout duration")
	ErrInvalidReportReason = errors.New("invalid abuse report reason")
	ErrCannotModerateAdmin = errors.New("super administrators cannot be restricted or banned")
)

type AdminService struct {
	userRepo *repository.UserRepository
	linkRepo *repository.LinkRepository
}

func NewAdminService(userRepo *repository.UserRepository, linkRepo *repository.LinkRepository) *AdminService {
	return &AdminService{
		userRepo: userRepo,
		linkRepo: linkRepo,
	}
}

// GetOverviewStats returns system-wide metrics for the admin overview.
func (s *AdminService) GetOverviewStats() (*models.AdminOverviewStats, error) {
	return s.userRepo.GetAdminOverviewStats()
}

// ListUsers retrieves paginated users.
func (s *AdminService) ListUsers(search, role, status string, page, limit int) ([]models.AdminUserItem, int, error) {
	return s.userRepo.AdminGetUsers(search, role, status, page, limit)
}

// TimeoutUser places a user under a temporary restriction.
func (s *AdminService) TimeoutUser(userID string, durationStr string, reason string) (*time.Time, error) {
	targetUser, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	if targetUser.Role == models.RoleSuperAdmin {
		return nil, ErrCannotModerateAdmin
	}

	dur, err := parseTimeoutDuration(durationStr)
	if err != nil {
		return nil, err
	}

	until := time.Now().UTC().Add(dur)
	if err := s.userRepo.SetUserTimeout(userID, &until, reason); err != nil {
		return nil, err
	}

	return &until, nil
}

// BanUser permanently bans a user and optionally bulk-deactivates their active links.
func (s *AdminService) BanUser(userID string, reason string, disableLinks bool) error {
	targetUser, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return err
	}
	if targetUser.Role == models.RoleSuperAdmin {
		return ErrCannotModerateAdmin
	}

	if err := s.userRepo.SetUserBan(userID, true, reason); err != nil {
		return err
	}

	if disableLinks {
		_ = s.linkRepo.BulkDisableUserLinks(userID)
	}

	return nil
}

// UnbanUser restores user account to active status and clears any timeouts or bans.
func (s *AdminService) UnbanUser(userID string) error {
	_ = s.userRepo.SetUserTimeout(userID, nil, "")
	return s.userRepo.SetUserBan(userID, false, "")
}

// ListLinks retrieves paginated links for moderation.
func (s *AdminService) ListLinks(search, status string, page, limit int) ([]models.AdminLinkItem, int, error) {
	return s.linkRepo.AdminGetLinks(search, status, page, limit)
}

// DisableLink deactivates a link immediately.
func (s *AdminService) DisableLink(shortCode string) error {
	return s.linkRepo.AdminSetLinkStatus(shortCode, models.StatusDisabled)
}

// EnableLink re-activates a disabled link.
func (s *AdminService) EnableLink(shortCode string) error {
	return s.linkRepo.AdminSetLinkStatus(shortCode, models.StatusActive)
}

// DeleteLink soft-deletes a link.
func (s *AdminService) DeleteLink(shortCode string) error {
	return s.linkRepo.AdminSetLinkStatus(shortCode, models.StatusDeleted)
}

// SubmitReport creates an abuse report against a shortened link.
func (s *AdminService) SubmitReport(shortCode, reason, details, clientIP string) error {
	link, err := s.linkRepo.GetLinkByCode(shortCode)
	if err != nil {
		return ErrLinkNotFound
	}

	reason = strings.ToLower(strings.TrimSpace(reason))
	validReasons := map[string]bool{
		"phishing": true, "malware": true, "scam": true, "spam": true, "illegal": true, "other": true,
	}
	if !validReasons[reason] {
		return ErrInvalidReportReason
	}

	reportID := generateID()
	ipHash := hashIdentity(clientIP)

	return s.linkRepo.CreateReport(reportID, link.ID, shortCode, reason, details, ipHash)
}

// ListReports retrieves abuse reports.
func (s *AdminService) ListReports(status string, page, limit int) ([]models.ReportItem, int, error) {
	return s.linkRepo.GetReports(status, page, limit)
}

// ResolveReport marks a report as reviewed or dismissed.
func (s *AdminService) ResolveReport(reportID string, status string) error {
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "reviewed" && status != "dismissed" {
		status = "reviewed"
	}
	return s.linkRepo.ResolveReport(reportID, status)
}

// GetLoginAuditRecords queries security audit entries.
func (s *AdminService) GetLoginAuditRecords(filterResult, filterMethod string, page, limit int) ([]models.LoginRecordItem, int, error) {
	return s.userRepo.GetLoginRecords(filterResult, filterMethod, page, limit)
}

func parseTimeoutDuration(durStr string) (time.Duration, error) {
	switch strings.ToLower(strings.TrimSpace(durStr)) {
	case "30s", "30sec", "30 seconds":
		return 30 * time.Second, nil
	case "1m", "1min", "1 minute":
		return 1 * time.Minute, nil
	case "5m", "5min", "5 minutes":
		return 5 * time.Minute, nil
	case "30m", "30min", "30 minutes":
		return 30 * time.Minute, nil
	case "1h", "1hour", "1 hour":
		return 1 * time.Hour, nil
	case "6h", "6hours", "6 hours":
		return 6 * time.Hour, nil
	case "12h", "12hours", "12 hours":
		return 12 * time.Hour, nil
	case "1d", "1day", "1 day":
		return 24 * time.Hour, nil
	case "3d", "3days", "3 days":
		return 3 * 24 * time.Hour, nil
	case "7d", "7days", "7 days":
		return 7 * 24 * time.Hour, nil
	default:
		// Attempt standard duration parsing e.g. "10m"
		d, err := time.ParseDuration(durStr)
		if err != nil {
			return 0, ErrInvalidDuration
		}
		return d, nil
	}
}

// ListPermanentRequests retrieves permanent link requests.
func (s *AdminService) ListPermanentRequests(status string, page, limit int) ([]models.PermanentLinkRequestItem, int, error) {
	return s.linkRepo.GetPermanentLinkRequests(status, page, limit)
}

// ResolvePermanentRequest approves or rejects a permanent request.
func (s *AdminService) ResolvePermanentRequest(reqID string, approved bool, adminID string) error {
	return s.linkRepo.ResolvePermanentLinkRequest(reqID, approved, adminID)
}

// UpdateUserRole changes a user's role (super_admin, moderator, user).
func (s *AdminService) UpdateUserRole(adminID string, targetUserID string, newRole models.UserRole) error {
	if adminID == targetUserID {
		return errors.New("cannot change your own role")
	}

	validRoles := map[models.UserRole]bool{
		models.RoleUser: true, models.RoleModerator: true, models.RoleSuperAdmin: true,
	}
	if !validRoles[newRole] {
		return errors.New("invalid role")
	}

	return s.userRepo.UpdateUserRole(targetUserID, newRole)
}
