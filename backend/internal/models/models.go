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
	ID             string     `json:"id"`
	ShortCode      string     `json:"short_code"`
	DestinationURL string     `json:"destination_url"`
	CreatedAt      time.Time  `json:"created_at"`
	ExpiresAt      time.Time  `json:"expires_at"`
	Status         LinkStatus `json:"status"`
	AutoRenew      bool       `json:"auto_renew"`
	ClickCount     int        `json:"click_count"`
	IsExpired      bool       `json:"is_expired"`
}

// DashboardStats provides user analytics and quota usage
type DashboardStats struct {
	TotalLinks     int `json:"total_links"`
	ActiveLinks    int `json:"active_links"`
	ExpiredLinks   int `json:"expired_links"`
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
