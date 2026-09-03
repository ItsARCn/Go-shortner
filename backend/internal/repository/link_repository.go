package repository

import (
	"database/sql"
	"errors"

	"github.com/arc/go-shortener/internal/models"
)

var (
	ErrLinkNotFound = errors.New("link not found")
	ErrQuotaExceeded = errors.New("link creation quota exceeded for this period")
)

type LinkRepository struct {
	db *sql.DB
}

func NewLinkRepository(db *sql.DB) *LinkRepository {
	return &LinkRepository{db: db}
}

// CreateLink persists a new link in the database.
func (r *LinkRepository) CreateLink(l *models.Link) error {
	query := `
		INSERT INTO links (id, short_code, destination_url, owner_id, created_at, expires_at, status, auto_renew, click_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(query,
		l.ID,
		l.ShortCode,
		l.DestinationURL,
		l.OwnerID,
		l.CreatedAt,
		l.ExpiresAt,
		string(l.Status),
		l.AutoRenew,
		l.ClickCount,
	)
	return err
}

// GetLinkByCode retrieves a link by its short code.
func (r *LinkRepository) GetLinkByCode(code string) (*models.Link, error) {
	query := `
		SELECT id, short_code, destination_url, owner_id, created_at, expires_at, status, auto_renew, click_count
		FROM links
		WHERE short_code = ? AND status != 'DELETED'
		LIMIT 1
	`
	row := r.db.QueryRow(query, code)
	var l models.Link
	var statusStr string
	var ownerID sql.NullString

	err := row.Scan(
		&l.ID,
		&l.ShortCode,
		&l.DestinationURL,
		&ownerID,
		&l.CreatedAt,
		&l.ExpiresAt,
		&statusStr,
		&l.AutoRenew,
		&l.ClickCount,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrLinkNotFound
		}
		return nil, err
	}

	l.Status = models.LinkStatus(statusStr)
	if ownerID.Valid {
		l.OwnerID = &ownerID.String
	}

	return &l, nil
}

// IncrementClickCount atomically increments the click counter for a link.
func (r *LinkRepository) IncrementClickCount(linkID string) error {
	query := `UPDATE links SET click_count = click_count + 1 WHERE id = ?`
	_, err := r.db.Exec(query, linkID)
	return err
}

// RecordClick asynchronously records click analytics.
func (r *LinkRepository) RecordClick(linkID, country, referrer, device, browser, osName string) error {
	query := `
		INSERT INTO link_clicks (link_id, clicked_at, country, referrer, device_type, browser, os)
		VALUES (?, CURRENT_TIMESTAMP, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(query, linkID, country, referrer, device, browser, osName)
	return err
}

// CheckAndIncrementQuota checks if quota is within limit and increments atomically.
func (r *LinkRepository) CheckAndIncrementQuota(identityKey string, isAnonymous bool, windowStart string, limit int) (bool, int, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return false, 0, err
	}
	defer tx.Rollback()

	var currentCount int
	query := `SELECT count FROM quota_usage WHERE identity_key = ? AND window_start = ?`
	err = tx.QueryRow(query, identityKey, windowStart).Scan(&currentCount)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No quota used yet in this window
			insertQuery := `INSERT INTO quota_usage (identity_key, is_anonymous, window_start, count) VALUES (?, ?, ?, 1)`
			_, err = tx.Exec(insertQuery, identityKey, isAnonymous, windowStart)
			if err != nil {
				return false, 0, err
			}
			if err := tx.Commit(); err != nil {
				return false, 0, err
			}
			return true, 1, nil
		}
		return false, 0, err
	}

	if currentCount >= limit {
		return false, currentCount, ErrQuotaExceeded
	}

	updateQuery := `UPDATE quota_usage SET count = count + 1 WHERE identity_key = ? AND window_start = ?`
	_, err = tx.Exec(updateQuery, identityKey, windowStart)
	if err != nil {
		return false, currentCount, err
	}

	if err := tx.Commit(); err != nil {
		return false, currentCount, err
	}

	return true, currentCount + 1, nil
}

// GetQuotaUsage returns current used count and total limit for an identity window.
func (r *LinkRepository) GetQuotaUsage(identityKey string, windowStart string) (int, error) {
	query := `SELECT count FROM quota_usage WHERE identity_key = ? AND window_start = ?`
	var count int
	err := r.db.QueryRow(query, identityKey, windowStart).Scan(&count)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return count, nil
}

// UpdateLinkStatus updates the status of a link (e.g. EXPIRED, ACTIVE, DISABLED)
func (r *LinkRepository) UpdateLinkStatus(linkID string, status models.LinkStatus) error {
	query := `UPDATE links SET status = ? WHERE id = ?`
	_, err := r.db.Exec(query, string(status), linkID)
	return err
}
