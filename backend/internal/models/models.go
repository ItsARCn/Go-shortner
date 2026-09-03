package models

import "time"

// LinkStatus represents the state of a shortened URL
type LinkStatus string

const (
	StatusActive   LinkStatus = "ACTIVE"
	StatusExpired  LinkStatus = "EXPIRED"
	StatusDisabled LinkStatus = "DISABLED"
	StatusDeleted  LinkStatus = "DELETED"
)

// Link represents a shortened URL record
type Link struct {
	ID             string     `json:"id"`
	ShortCode      string     `json:"short_code"`
	DestinationURL string     `json:"destination_url"`
	OwnerID        *string    `json:"owner_id,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ExpiresAt      time.Time  `json:"expires_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
	Status         LinkStatus `json:"status"`
	AutoRenew      bool       `json:"auto_renew"`
	ClickCount     int        `json:"click_count"`
}

// IsExpired checks if the link has passed its expiration time or is marked expired
func (l *Link) IsExpired() bool {
	if l.AutoRenew {
		return false
	}
	return l.Status == StatusExpired || time.Now().After(l.ExpiresAt)
}

// ShortenRequest is the payload for creating a shortened link
type ShortenRequest struct {
	URL            string `json:"url"`
	Expiration     string `json:"expiration"` // "1h", "1d", "3d", "7d", "30d", "90d", "180d", "365d"
	TurnstileToken string `json:"turnstile_token,omitempty"`
}

// ShortenResponse is the response returned upon successful creation
type ShortenResponse struct {
	ShortCode string    `json:"short_code"`
	ShortURL  string    `json:"short_url"`
	TargetURL string    `json:"destination_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// LinkInfoResponse contains public information for the expired/info page
type LinkInfoResponse struct {
	ShortCode string     `json:"short_code"`
	Status    LinkStatus `json:"status"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	IsExpired bool       `json:"is_expired"`
}

// UserRole represents RBAC permission levels
type UserRole string

const (
	RoleUser       UserRole = "user"
	RoleModerator  UserRole = "moderator"
	RoleSuperAdmin UserRole = "super_admin"
)

// UserStatus represents user account state
type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusTimedOut UserStatus = "timed_out"
	UserStatusBanned   UserStatus = "banned"
)

// User represents a registered account record
type User struct {
	ID            string     `json:"id"`
	FirstName     string     `json:"first_name"`
	LastName      string     `json:"last_name"`
	Email         string     `json:"email"`
	PasswordHash  string     `json:"-"`
	AuthProvider  string     `json:"auth_provider"`
	FirebaseUID   *string    `json:"firebase_uid,omitempty"`
	Role          UserRole   `json:"role"`
	Status        UserStatus `json:"status"`
	TimeoutUntil  *time.Time `json:"timeout_until,omitempty"`
	TimeoutReason *string    `json:"timeout_reason,omitempty"`
	BanReason     *string    `json:"ban_reason,omitempty"`
	QuotaLimit    int        `json:"quota_limit"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	LastLoginAt   *time.Time `json:"last_login_at,omitempty"`
}

// RegisterRequest holds registration fields
type RegisterRequest struct {
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
}

// LoginRequest holds login fields
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse is sent upon successful login or registration
type AuthResponse struct {
	Token string `json:"token"`
	User  *User  `json:"user"`
}

// TokenClaims represents the payload in our JWT session token
type TokenClaims struct {
	UserID string   `json:"uid"`
	Email  string   `json:"email"`
	Role   UserRole `json:"role"`
	Exp    int64    `json:"exp"`
}

// RenewLinkRequest holds renewal options for expired links
type RenewLinkRequest struct {
	Expiration string `json:"expiration"` // "7d", "30d", "90d", "180d", "365d"
}

// UserLinkItem represents a link row in the dashboard
type UserLinkItem struct {
	ID                 string     `json:"id"`
	ShortCode          string     `json:"short_code"`
	DestinationURL     string     `json:"destination_url"`
	CreatedAt          time.Time  `json:"created_at"`
	ExpiresAt          time.Time  `json:"expires_at"`
	DeletedAt          *time.Time `json:"deleted_at,omitempty"`
	DaysRemainingInBin *int       `json:"days_remaining_in_bin,omitempty"`
	Status             LinkStatus `json:"status"`
	AutoRenew          bool       `json:"auto_renew"`
	ClickCount         int        `json:"click_count"`
	IsExpired          bool       `json:"is_expired"`
}

// RestoreLinkRequest holds payload for restoring a deleted link from bin
type RestoreLinkRequest struct {
	ShortCode string `json:"short_code"`
}

// DashboardStats provides user analytics and quota usage
type DashboardStats struct {
	TotalLinks     int `json:"total_links"`
	ActiveLinks    int `json:"active_links"`
	ExpiredLinks   int `json:"expired_links"`
	BinLinks       int `json:"bin_links"`
	TotalClicks    int `json:"total_clicks"`
	QuotaUsed      int `json:"quota_used"`
	QuotaLimit     int `json:"quota_limit"`
	DaysUntilReset int `json:"days_until_reset"`
}

// DashboardResponse is the payload returned to the user dashboard
type DashboardResponse struct {
	Stats DashboardStats `json:"stats"`
	Links []UserLinkItem `json:"links"`
	Page  int            `json:"page"`
	Total int            `json:"total"`
}

// GoogleLoginRequest represents a Google authentication request
type GoogleLoginRequest struct {
	IDToken        string `json:"id_token"`
	TurnstileToken string `json:"turnstile_token,omitempty"`
}

// AnalyticsBreakdownItem represents a category slice in click analytics
type AnalyticsBreakdownItem struct {
	Name       string  `json:"name"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

// LinkAnalyticsResponse provides privacy-conscious metrics for a single link
type LinkAnalyticsResponse struct {
	ShortCode        string                   `json:"short_code"`
	DestinationURL   string                   `json:"destination_url"`
	TotalClicks      int                      `json:"total_clicks"`
	ClicksToday      int                      `json:"clicks_today"`
	ClicksThisWeek   int                      `json:"clicks_this_week"`
	ClicksThisMonth  int                      `json:"clicks_this_month"`
	Devices          []AnalyticsBreakdownItem `json:"devices"`
	Browsers         []AnalyticsBreakdownItem `json:"browsers"`
	OperatingSystems []AnalyticsBreakdownItem `json:"operating_systems"`
	TopReferrers     []AnalyticsBreakdownItem `json:"top_referrers"`
}

// CreateReportRequest holds payload for reporting link abuse
type CreateReportRequest struct {
	ShortCode string `json:"short_code"`
	Reason    string `json:"reason"` // "phishing", "malware", "scam", "spam", "illegal", "other"
	Details   string `json:"details"`
}

// ReportItem represents an abuse report row
type ReportItem struct {
	ID             string    `json:"id"`
	LinkID         string    `json:"link_id"`
	ShortCode      string    `json:"short_code"`
	DestinationURL string    `json:"destination_url"`
	Reason         string    `json:"reason"`
	Details        string    `json:"details"`
	ReporterIPHash string    `json:"reporter_ip_hash"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// AdminOverviewStats aggregates system-wide counts for the admin dashboard
type AdminOverviewStats struct {
	TotalUsers    int `json:"total_users"`
	TotalLinks    int `json:"total_links"`
	ActiveLinks   int `json:"active_links"`
	ExpiredLinks  int `json:"expired_links"`
	ReportsCount  int `json:"reports_count"`
	BannedUsers   int `json:"banned_users"`
	TimedOutUsers int `json:"timed_out_users"`
}

// AdminUserItem represents a user in the admin user list
type AdminUserItem struct {
	ID           string     `json:"id"`
	FirstName    string     `json:"first_name"`
	LastName     string     `json:"last_name"`
	Email        string     `json:"email"`
	AuthProvider string     `json:"auth_provider"`
	Role         UserRole   `json:"role"`
	Status       UserStatus `json:"status"`
	TimeoutUntil *time.Time `json:"timeout_until,omitempty"`
	LinkCount    int        `json:"link_count"`
	QuotaLimit   int        `json:"quota_limit"`
	CreatedAt    time.Time  `json:"created_at"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
}

// AdminLinkItem represents a link in the admin link moderation list
type AdminLinkItem struct {
	ID             string     `json:"id"`
	ShortCode      string     `json:"short_code"`
	DestinationURL string     `json:"destination_url"`
	OwnerEmail     string     `json:"owner_email"`
	CreatedAt      time.Time  `json:"created_at"`
	ExpiresAt      time.Time  `json:"expires_at"`
	Status         LinkStatus `json:"status"`
	AutoRenew      bool       `json:"auto_renew"`
	ClickCount     int        `json:"click_count"`
	ReportCount    int        `json:"report_count"`
}

// TimeoutUserRequest holds duration and reason for a temporary timeout
type TimeoutUserRequest struct {
	Duration string `json:"duration"` // "30s", "1m", "5m", "30m", "1h", "6h", "12h", "1d", "3d", "7d"
	Reason   string `json:"reason"`
}

// BanUserRequest holds reason and link deactivation flag for banning
type BanUserRequest struct {
	Reason       string `json:"reason"`
	DisableLinks bool   `json:"disable_links"`
}

// LoginRecordItem represents a security audit event
type LoginRecordItem struct {
	ID           int       `json:"id"`
	AccountEmail string    `json:"account_email"`
	AuthMethod   string    `json:"auth_method"`
	Result       string    `json:"result"`
	IPHash       string    `json:"ip_hash"`
	UserAgent    string    `json:"user_agent"`
	CreatedAt    time.Time `json:"created_at"`
}

// PermanentLinkRequestItem represents a user request to make a link permanent
type PermanentLinkRequestItem struct {
	ID             string     `json:"id"`
	LinkID         string     `json:"link_id"`
	ShortCode      string     `json:"short_code"`
	DestinationURL string     `json:"destination_url"`
	UserID         string     `json:"user_id"`
	UserEmail      string     `json:"user_email"`
	Reason         string     `json:"reason"`
	Status         string     `json:"status"` // pending, approved, rejected
	ReviewedBy     *string    `json:"reviewed_by,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ReviewedAt     *time.Time `json:"reviewed_at,omitempty"`
}

// CreatePermanentRequest holds user input for requesting permanent status
type CreatePermanentRequest struct {
	Reason string `json:"reason"`
}

// ResolvePermanentRequest holds admin decision on permanent link request
type ResolvePermanentRequest struct {
	Approved bool `json:"approved"`
}

// UpdateUserRoleRequest holds new role for a user
type UpdateUserRoleRequest struct {
	Role UserRole `json:"role"`
}
