package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/arc/go-shortener/internal/models"
)

var (
	ErrLinkNotFound  = errors.New("link not found")
	ErrQuotaExceeded = errors.New("link creation quota exceeded for this period")
	ErrUnauthorized  = errors.New("you do not have permission to manage this link")
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

// GetQuotaUsage returns current used count for an identity window.
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

// GetUserLinks retrieves paginated links owned by a user with search and status filtering.
func (r *LinkRepository) GetUserLinks(ownerID string, search string, filterStatus string, page int, limit int) ([]models.UserLinkItem, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var whereClauses []string
	var args []interface{}

	whereClauses = append(whereClauses, "owner_id = ?", "status != 'DELETED'")
	args = append(args, ownerID)

	if search != "" {
		whereClauses = append(whereClauses, "(short_code LIKE ? OR destination_url LIKE ?)")
		searchTerm := "%" + search + "%"
		args = append(args, searchTerm, searchTerm)
	}

	if filterStatus != "" && filterStatus != "all" {
		whereClauses = append(whereClauses, "status = ?")
		args = append(args, strings.ToUpper(filterStatus))
	}

	whereSQL := strings.Join(whereClauses, " AND ")

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM links WHERE %s", whereSQL)
	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT id, short_code, destination_url, created_at, expires_at, status, auto_renew, click_count
		FROM links
		WHERE %s
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, whereSQL)

	queryArgs := append(args, limit, offset)
	rows, err := r.db.Query(query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	now := time.Now().UTC()
	var links []models.UserLinkItem
	for rows.Next() {
		var item models.UserLinkItem
		var statusStr string
		err := rows.Scan(
			&item.ID,
			&item.ShortCode,
			&item.DestinationURL,
			&item.CreatedAt,
			&item.ExpiresAt,
			&statusStr,
			&item.AutoRenew,
			&item.ClickCount,
		)
		if err != nil {
			return nil, 0, err
		}

		item.Status = models.LinkStatus(statusStr)
		item.IsExpired = (!item.AutoRenew && now.After(item.ExpiresAt)) || item.Status == models.StatusExpired
		links = append(links, item)
	}

	return links, total, nil
}

// GetUserStats calculates dashboard statistics for a user.
func (r *LinkRepository) GetUserStats(ownerID string, quotaLimit int, monthWindow string) (*models.DashboardStats, error) {
	stats := &models.DashboardStats{
		QuotaLimit: quotaLimit,
	}

	// 1. Quota used in this month window
	quotaKey := fmt.Sprintf("user:%s", ownerID)
	used, err := r.GetQuotaUsage(quotaKey, monthWindow)
	if err != nil {
		return nil, err
	}
	stats.QuotaUsed = used

	// 2. Days until month reset
	now := time.Now().UTC()
	// Next month first day
	year, month, _ := now.Date()
	firstOfNextMonth := time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC)
	stats.DaysUntilReset = int(firstOfNextMonth.Sub(now).Hours() / 24)
	if stats.DaysUntilReset < 1 {
		stats.DaysUntilReset = 1
	}

	// 3. Link counts and clicks
	query := `
		SELECT 
			COUNT(*) as total,
			COALESCE(SUM(CASE WHEN (auto_renew = 1 OR expires_at > CURRENT_TIMESTAMP) AND status = 'ACTIVE' THEN 1 ELSE 0 END), 0) as active,
			COALESCE(SUM(CASE WHEN auto_renew = 0 AND (expires_at <= CURRENT_TIMESTAMP OR status = 'EXPIRED') AND status != 'DELETED' THEN 1 ELSE 0 END), 0) as expired,
			COALESCE(SUM(click_count), 0) as total_clicks
		FROM links
		WHERE owner_id = ? AND status != 'DELETED'
	`
	row := r.db.QueryRow(query, ownerID)
	err = row.Scan(&stats.TotalLinks, &stats.ActiveLinks, &stats.ExpiredLinks, &stats.TotalClicks)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// RenewLink sets a new expiration timestamp for an expired link and activates it.
func (r *LinkRepository) RenewLink(shortCode string, ownerID string, newExpiresAt time.Time) error {
	query := `
		UPDATE links
		SET expires_at = ?, status = 'ACTIVE'
		WHERE short_code = ? AND owner_id = ? AND status != 'DELETED'
	`
	res, err := r.db.Exec(query, newExpiresAt, shortCode, ownerID)
	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrUnauthorized
	}

	return nil
}

// SoftDeleteLink marks a user's link as DELETED.
func (r *LinkRepository) SoftDeleteLink(shortCode string, ownerID string) error {
	query := `
		UPDATE links
		SET status = 'DELETED'
		WHERE short_code = ? AND owner_id = ? AND status != 'DELETED'
	`
	res, err := r.db.Exec(query, shortCode, ownerID)
	if err != nil {
		return err
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrUnauthorized
	}

	return nil
}

// GetLinkAnalytics aggregates click metrics for a link.
func (r *LinkRepository) GetLinkAnalytics(linkID string, shortCode string, destinationURL string) (*models.LinkAnalyticsResponse, error) {
	resp := &models.LinkAnalyticsResponse{
		ShortCode:      shortCode,
		DestinationURL: destinationURL,
	}

	timeQuery := `
		SELECT
			COUNT(*) as total,
			COALESCE(SUM(CASE WHEN clicked_at >= datetime('now', 'start of day') THEN 1 ELSE 0 END), 0) as today,
			COALESCE(SUM(CASE WHEN clicked_at >= datetime('now', '-7 days') THEN 1 ELSE 0 END), 0) as week,
			COALESCE(SUM(CASE WHEN clicked_at >= datetime('now', '-30 days') THEN 1 ELSE 0 END), 0) as month
		FROM link_clicks
		WHERE link_id = ?
	`
	err := r.db.QueryRow(timeQuery, linkID).Scan(
		&resp.TotalClicks,
		&resp.ClicksToday,
		&resp.ClicksThisWeek,
		&resp.ClicksThisMonth,
	)
	if err != nil {
		return nil, err
	}

	total := float64(resp.TotalClicks)
	if total == 0 {
		total = 1.0
	}

	resp.Devices = r.queryBreakdown("device_type", linkID, total)
	resp.Browsers = r.queryBreakdown("browser", linkID, total)
	resp.OperatingSystems = r.queryBreakdown("os", linkID, total)
	resp.TopReferrers = r.queryReferrers(linkID, total)

	return resp, nil
}

func (r *LinkRepository) queryBreakdown(column, linkID string, total float64) []models.AnalyticsBreakdownItem {
	query := fmt.Sprintf(`
		SELECT COALESCE(NULLIF(%s, ''), 'Unknown') as name, COUNT(*) as count
		FROM link_clicks
		WHERE link_id = ?
		GROUP BY name
		ORDER BY count DESC
		LIMIT 5
	`, column)

	rows, err := r.db.Query(query, linkID)
	if err != nil {
		return []models.AnalyticsBreakdownItem{}
	}
	defer rows.Close()

	var items []models.AnalyticsBreakdownItem
	for rows.Next() {
		var item models.AnalyticsBreakdownItem
		if err := rows.Scan(&item.Name, &item.Count); err == nil {
			item.Percentage = (float64(item.Count) / total) * 100
			items = append(items, item)
		}
	}
	return items
}

func (r *LinkRepository) queryReferrers(linkID string, total float64) []models.AnalyticsBreakdownItem {
	query := `
		SELECT COALESCE(NULLIF(referrer, ''), 'Direct / None') as name, COUNT(*) as count
		FROM link_clicks
		WHERE link_id = ?
		GROUP BY name
		ORDER BY count DESC
		LIMIT 5
	`
	rows, err := r.db.Query(query, linkID)
	if err != nil {
		return []models.AnalyticsBreakdownItem{}
	}
	defer rows.Close()

	var items []models.AnalyticsBreakdownItem
	for rows.Next() {
		var item models.AnalyticsBreakdownItem
		if err := rows.Scan(&item.Name, &item.Count); err == nil {
			item.Percentage = (float64(item.Count) / total) * 100
			items = append(items, item)
		}
	}
	return items
}

// CreateReport inserts an abuse report for a link.
func (r *LinkRepository) CreateReport(id, linkID, shortCode, reason, details, reporterIPHash string) error {
	query := `
		INSERT INTO reports (id, link_id, short_code, reason, details, reporter_ip_hash, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, 'pending', CURRENT_TIMESTAMP)
	`
	_, err := r.db.Exec(query, id, linkID, shortCode, reason, details, reporterIPHash)
	return err
}

// GetReports retrieves paginated abuse reports.
func (r *LinkRepository) GetReports(status string, page, limit int) ([]models.ReportItem, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var whereClauses []string
	var args []interface{}

	if status != "" && status != "all" {
		whereClauses = append(whereClauses, "r.status = ?")
		args = append(args, status)
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM reports r %s", whereSQL)
	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT r.id, r.link_id, r.short_code, COALESCE(l.destination_url, ''), r.reason, COALESCE(r.details, ''), r.reporter_ip_hash, r.status, r.created_at
		FROM reports r
		LEFT JOIN links l ON r.link_id = l.id
		%s
		ORDER BY r.created_at DESC
		LIMIT ? OFFSET ?
	`, whereSQL)

	queryArgs := append(args, limit, offset)
	rows, err := r.db.Query(query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var reports []models.ReportItem
	for rows.Next() {
		var item models.ReportItem
		err := rows.Scan(
			&item.ID,
			&item.LinkID,
			&item.ShortCode,
			&item.DestinationURL,
			&item.Reason,
			&item.Details,
			&item.ReporterIPHash,
			&item.Status,
			&item.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		reports = append(reports, item)
	}

	return reports, total, nil
}

// ResolveReport updates report status (e.g. reviewed, dismissed).
func (r *LinkRepository) ResolveReport(reportID string, status string) error {
	query := `UPDATE reports SET status = ? WHERE id = ?`
	_, err := r.db.Exec(query, status, reportID)
	return err
}

// AdminGetLinks lists all links across the platform with search, status filter, and report counts.
func (r *LinkRepository) AdminGetLinks(search, status string, page, limit int) ([]models.AdminLinkItem, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var whereClauses []string
	var args []interface{}

	if search != "" {
		whereClauses = append(whereClauses, "(l.short_code LIKE ? OR l.destination_url LIKE ? OR u.email LIKE ?)")
		searchTerm := "%" + search + "%"
		args = append(args, searchTerm, searchTerm, searchTerm)
	}

	if status != "" && status != "all" {
		whereClauses = append(whereClauses, "l.status = ?")
		args = append(args, strings.ToUpper(status))
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM links l
		LEFT JOIN users u ON l.owner_id = u.id
		%s
	`, whereSQL)

	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT l.id, l.short_code, l.destination_url, COALESCE(u.email, 'Anonymous'),
		       l.created_at, l.expires_at, l.status, l.auto_renew, l.click_count,
		       (SELECT COUNT(*) FROM reports r WHERE r.link_id = l.id) as report_count
		FROM links l
		LEFT JOIN users u ON l.owner_id = u.id
		%s
		ORDER BY l.created_at DESC
		LIMIT ? OFFSET ?
	`, whereSQL)

	queryArgs := append(args, limit, offset)
	rows, err := r.db.Query(query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var links []models.AdminLinkItem
	for rows.Next() {
		var item models.AdminLinkItem
		var statusStr string
		err := rows.Scan(
			&item.ID,
			&item.ShortCode,
			&item.DestinationURL,
			&item.OwnerEmail,
			&item.CreatedAt,
			&item.ExpiresAt,
			&statusStr,
			&item.AutoRenew,
			&item.ClickCount,
			&item.ReportCount,
		)
		if err != nil {
			return nil, 0, err
		}
		item.Status = models.LinkStatus(statusStr)
		links = append(links, item)
	}

	return links, total, nil
}

// AdminSetLinkStatus overrides status of any link (e.g. DISABLED, ACTIVE, DELETED).
func (r *LinkRepository) AdminSetLinkStatus(shortCode string, status models.LinkStatus) error {
	query := `UPDATE links SET status = ? WHERE short_code = ?`
	res, err := r.db.Exec(query, string(status), shortCode)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrLinkNotFound
	}
	return nil
}

// BulkDisableUserLinks disables all active links belonging to a user (used during banning).
func (r *LinkRepository) BulkDisableUserLinks(ownerID string) error {
	query := `UPDATE links SET status = 'DISABLED' WHERE owner_id = ? AND status = 'ACTIVE'`
	_, err := r.db.Exec(query, ownerID)
	return err
}

// CheckUserRestriction checks if a user is currently timed out or banned.
func (r *LinkRepository) CheckUserRestriction(userID string) (bool, string, *time.Time, error) {
	query := `SELECT status, timeout_until, timeout_reason, ban_reason FROM users WHERE id = ?`
	var status string
	var timeoutUntil sql.NullTime
	var timeoutReason, banReason sql.NullString

	err := r.db.QueryRow(query, userID).Scan(&status, &timeoutUntil, &timeoutReason, &banReason)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, "", nil, nil
		}
		return false, "", nil, err
	}

	if status == "banned" {
		reason := "Account has been suspended"
		if banReason.Valid && banReason.String != "" {
			reason = fmt.Sprintf("Account suspended: %s", banReason.String)
		}
		return true, reason, nil, nil
	}

	if status == "timed_out" && timeoutUntil.Valid {
		if time.Now().UTC().Before(timeoutUntil.Time) {
			reason := "Account is temporarily restricted"
			if timeoutReason.Valid && timeoutReason.String != "" {
				reason = fmt.Sprintf("Account restricted until %s: %s", timeoutUntil.Time.Format("2006-01-02 15:04:05 UTC"), timeoutReason.String)
			}
			return true, reason, &timeoutUntil.Time, nil
		}
	}

	return false, "", nil, nil
}
