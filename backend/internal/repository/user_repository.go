package repository

import (
	"database/sql"
	"errors"
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
