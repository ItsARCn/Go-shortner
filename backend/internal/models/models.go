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

// IsExpired checks if the link has passed its expiration time
func (l *Link) IsExpired() bool {
	if l.AutoRenew {
		return false
	}
	return time.Now().After(l.ExpiresAt)
}

// ShortenRequest is the payload for creating a shortened link
type ShortenRequest struct {
	URL        string `json:"url"`
	Expiration string `json:"expiration"` // "1h", "1d", "3d", "7d", "30d", "90d", "180d", "365d"
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
