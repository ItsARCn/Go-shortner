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
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("an account with this email address already exists")
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// CreateUser inserts a new user record.
func (r *UserRepository) CreateUser(u *models.User) error {
	query := `
		INSERT INTO users (
			id, first_name, last_name, email, password_hash, auth_provider,
			firebase_uid, role, status, quota_limit, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.Exec(query,
		u.ID,
		u.FirstName,
		u.LastName,
		strings.ToLower(strings.TrimSpace(u.Email)),
		u.PasswordHash,
		u.AuthProvider,
		u.FirebaseUID,
		string(u.Role),
		string(u.Status),
		u.QuotaLimit,
		u.CreatedAt,
		u.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: users.email") {
			return ErrUserAlreadyExists
		}
		return err
	}
	return nil
}

// GetUserByEmail queries a user by email.
func (r *UserRepository) GetUserByEmail(email string) (*models.User, error) {
	query := `
		SELECT id, first_name, last_name, email, password_hash, auth_provider,
		       firebase_uid, role, status, timeout_until, timeout_reason, ban_reason,
		       quota_limit, created_at, updated_at, last_login_at
		FROM users
		WHERE email = ?
		LIMIT 1
	`
	row := r.db.QueryRow(query, strings.ToLower(strings.TrimSpace(email)))
	return scanUser(row)
}

// GetUserByID queries a user by ID.
func (r *UserRepository) GetUserByID(id string) (*models.User, error) {
	query := `
		SELECT id, first_name, last_name, email, password_hash, auth_provider,
		       firebase_uid, role, status, timeout_until, timeout_reason, ban_reason,
		       quota_limit, created_at, updated_at, last_login_at
		FROM users
		WHERE id = ?
		LIMIT 1
	`
	row := r.db.QueryRow(query, id)
	return scanUser(row)
}

// UpdateLastLogin updates the timestamp of last successful login.
func (r *UserRepository) UpdateLastLogin(userID string) error {
	query := `UPDATE users SET last_login_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, time.Now().UTC(), userID)
	return err
}

// RecordLoginAttempt records an audit event in login_records.
func (r *UserRepository) RecordLoginAttempt(email, method, result, ipHash, userAgent string) error {
	query := `
		INSERT INTO login_records (account_email, auth_method, result, ip_hash, user_agent, created_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`
	_, err := r.db.Exec(query, strings.ToLower(strings.TrimSpace(email)), method, result, ipHash, userAgent)
	return err
}

func scanUser(row *sql.Row) (*models.User, error) {
	var u models.User
	var roleStr, statusStr string
	var firebaseUID, timeoutReason, banReason sql.NullString
	var timeoutUntil, lastLoginAt sql.NullTime

	err := row.Scan(
		&u.ID,
		&u.FirstName,
		&u.LastName,
		&u.Email,
		&u.PasswordHash,
		&u.AuthProvider,
		&firebaseUID,
		&roleStr,
		&statusStr,
		&timeoutUntil,
		&timeoutReason,
		&banReason,
		&u.QuotaLimit,
		&u.CreatedAt,
		&u.UpdatedAt,
		&lastLoginAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	u.Role = models.UserRole(roleStr)
	u.Status = models.UserStatus(statusStr)
	if firebaseUID.Valid {
		u.FirebaseUID = &firebaseUID.String
	}
	if timeoutReason.Valid {
		u.TimeoutReason = &timeoutReason.String
	}
	if banReason.Valid {
		u.BanReason = &banReason.String
	}
	if timeoutUntil.Valid {
		u.TimeoutUntil = &timeoutUntil.Time
	}
	if lastLoginAt.Valid {
		u.LastLoginAt = &lastLoginAt.Time
	}

	return &u, nil
}

// LinkGoogleAccount links a Firebase UID to an existing user account.
func (r *UserRepository) LinkGoogleAccount(userID string, firebaseUID string) error {
	query := `UPDATE users SET firebase_uid = ?, auth_provider = 'google', updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := r.db.Exec(query, firebaseUID, userID)
	return err
}

// AdminGetUsers lists users with filtering by search, role, and status.
func (r *UserRepository) AdminGetUsers(search, role, status string, page, limit int) ([]models.AdminUserItem, int, error) {
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
		whereClauses = append(whereClauses, "(u.email LIKE ? OR u.first_name LIKE ? OR u.last_name LIKE ?)")
		searchTerm := "%" + search + "%"
		args = append(args, searchTerm, searchTerm, searchTerm)
	}

	if role != "" && role != "all" {
		whereClauses = append(whereClauses, "u.role = ?")
		args = append(args, role)
	}

	if status != "" && status != "all" {
		whereClauses = append(whereClauses, "u.status = ?")
		args = append(args, status)
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM users u %s", whereSQL)
	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT u.id, u.first_name, u.last_name, u.email, u.auth_provider, u.role, u.status,
		       u.timeout_until, u.quota_limit, u.created_at, u.last_login_at,
		       (SELECT COUNT(*) FROM links l WHERE l.owner_id = u.id AND l.status != 'DELETED') as link_count
		FROM users u
		%s
		ORDER BY u.created_at DESC
		LIMIT ? OFFSET ?
	`, whereSQL)

	queryArgs := append(args, limit, offset)
	rows, err := r.db.Query(query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []models.AdminUserItem
	for rows.Next() {
		var item models.AdminUserItem
		var roleStr, statusStr string
		var timeoutUntil, lastLoginAt sql.NullTime

		err := rows.Scan(
			&item.ID,
			&item.FirstName,
			&item.LastName,
			&item.Email,
			&item.AuthProvider,
			&roleStr,
			&statusStr,
			&timeoutUntil,
			&item.QuotaLimit,
			&item.CreatedAt,
			&lastLoginAt,
			&item.LinkCount,
		)
		if err != nil {
			return nil, 0, err
		}

		item.Role = models.UserRole(roleStr)
		item.Status = models.UserStatus(statusStr)
		if timeoutUntil.Valid {
			item.TimeoutUntil = &timeoutUntil.Time
		}
		if lastLoginAt.Valid {
			item.LastLoginAt = &lastLoginAt.Time
		}
		users = append(users, item)
	}

	return users, total, nil
}

// SetUserTimeout sets or removes a temporary user restriction.
func (r *UserRepository) SetUserTimeout(userID string, until *time.Time, reason string) error {
	if until == nil {
		// Lift timeout
		query := `UPDATE users SET status = 'active', timeout_until = NULL, timeout_reason = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
		_, err := r.db.Exec(query, userID)
		return err
	}

	query := `UPDATE users SET status = 'timed_out', timeout_until = ?, timeout_reason = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := r.db.Exec(query, until, reason, userID)
	return err
}

// SetUserBan sets or removes a permanent ban.
func (r *UserRepository) SetUserBan(userID string, banned bool, reason string) error {
	if !banned {
		// Lift ban
		query := `UPDATE users SET status = 'active', ban_reason = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
		_, err := r.db.Exec(query, userID)
		return err
	}

	query := `UPDATE users SET status = 'banned', ban_reason = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := r.db.Exec(query, reason, userID)
	return err
}

// GetAdminOverviewStats aggregates platform metrics for the admin dashboard.
func (r *UserRepository) GetAdminOverviewStats() (*models.AdminOverviewStats, error) {
	stats := &models.AdminOverviewStats{}

	// Users counts
	userQuery := `
		SELECT
			COUNT(*) as total,
			COALESCE(SUM(CASE WHEN status = 'banned' THEN 1 ELSE 0 END), 0) as banned,
			COALESCE(SUM(CASE WHEN status = 'timed_out' AND (timeout_until IS NULL OR timeout_until > CURRENT_TIMESTAMP) THEN 1 ELSE 0 END), 0) as timed_out
		FROM users
	`
	err := r.db.QueryRow(userQuery).Scan(&stats.TotalUsers, &stats.BannedUsers, &stats.TimedOutUsers)
	if err != nil {
		return nil, err
	}

	// Links counts
	linkQuery := `
		SELECT
			COUNT(*) as total,
			COALESCE(SUM(CASE WHEN (auto_renew = 1 OR expires_at > CURRENT_TIMESTAMP) AND status = 'ACTIVE' THEN 1 ELSE 0 END), 0) as active,
			COALESCE(SUM(CASE WHEN auto_renew = 0 AND (expires_at <= CURRENT_TIMESTAMP OR status = 'EXPIRED') AND status != 'DELETED' THEN 1 ELSE 0 END), 0) as expired
		FROM links
		WHERE status != 'DELETED'
	`
	err = r.db.QueryRow(linkQuery).Scan(&stats.TotalLinks, &stats.ActiveLinks, &stats.ExpiredLinks)
	if err != nil {
		return nil, err
	}

	// Reports count
	reportQuery := `SELECT COUNT(*) FROM reports WHERE status = 'pending'`
	_ = r.db.QueryRow(reportQuery).Scan(&stats.ReportsCount)

	return stats, nil
}

// GetLoginRecords retrieves audit entries from login_records.
func (r *UserRepository) GetLoginRecords(filterResult, filterMethod string, page, limit int) ([]models.LoginRecordItem, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 25
	}
	offset := (page - 1) * limit

	var whereClauses []string
	var args []interface{}

	if filterResult != "" && filterResult != "all" {
		whereClauses = append(whereClauses, "result = ?")
		args = append(args, strings.ToUpper(filterResult))
	}

	if filterMethod != "" && filterMethod != "all" {
		whereClauses = append(whereClauses, "auth_method = ?")
		args = append(args, strings.ToLower(filterMethod))
	}

	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM login_records %s", whereSQL)
	var total int
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`
		SELECT id, account_email, auth_method, result, COALESCE(ip_hash, ''), COALESCE(user_agent, ''), created_at
		FROM login_records
		%s
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, whereSQL)

	queryArgs := append(args, limit, offset)
	rows, err := r.db.Query(query, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var records []models.LoginRecordItem
	for rows.Next() {
		var item models.LoginRecordItem
		err := rows.Scan(
			&item.ID,
			&item.AccountEmail,
			&item.AuthMethod,
			&item.Result,
			&item.IPHash,
			&item.UserAgent,
			&item.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		records = append(records, item)
	}

	return records, total, nil
}

// EnsureSuperAdminBootstrap creates an initial Super Admin if no admin user exists.
func (r *UserRepository) EnsureSuperAdminBootstrap(email, passwordHash string) error {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'super_admin'").Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 || email == "" || passwordHash == "" {
		return nil // Admin already exists or no credentials configured
	}

	query := `
		INSERT INTO users (
			id, first_name, last_name, email, password_hash, auth_provider,
			role, status, quota_limit, created_at, updated_at
		) VALUES ('admin-bootstrap-1', 'System', 'Admin', ?, ?, 'email', 'super_admin', 'active', 999999, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`
	_, err = r.db.Exec(query, strings.ToLower(strings.TrimSpace(email)), passwordHash)
	return err
}
