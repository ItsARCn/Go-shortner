package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// InitDB initializes SQLite database connection and runs initial schema migrations.
func InitDB(dbPath string) (*sql.DB, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Configure connection pool for SQLite
	db.SetMaxOpenConns(1) // Single writer for SQLite to avoid lock contention

	// Pragmas for performance and data integrity
	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA foreign_keys=ON;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA cache_size=-20000;", // 20MB cache
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return nil, fmt.Errorf("failed to execute %s: %w", pragma, err)
		}
	}

	if err := migrate(db); err != nil {
		return nil, fmt.Errorf("migration failed: %w", err)
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	schema := `
	-- Links table
	CREATE TABLE IF NOT EXISTS links (
		id TEXT PRIMARY KEY,
		short_code TEXT UNIQUE NOT NULL,
		destination_url TEXT NOT NULL,
		owner_id TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME NOT NULL,
		deleted_at DATETIME,
		status TEXT NOT NULL DEFAULT 'ACTIVE',
		auto_renew INTEGER NOT NULL DEFAULT 0,
		click_count INTEGER NOT NULL DEFAULT 0
	);

	CREATE INDEX IF NOT EXISTS idx_links_short_code ON links(short_code);
	CREATE INDEX IF NOT EXISTS idx_links_owner_id ON links(owner_id);
	CREATE INDEX IF NOT EXISTS idx_links_expires_at ON links(expires_at);
	CREATE INDEX IF NOT EXISTS idx_links_status ON links(status);
	CREATE INDEX IF NOT EXISTS idx_links_deleted_at ON links(deleted_at);

	-- Users table
	CREATE TABLE IF NOT EXISTS users (
		id TEXT PRIMARY KEY,
		first_name TEXT NOT NULL,
		last_name TEXT NOT NULL,
		email TEXT UNIQUE NOT NULL,
		password_hash TEXT,
		auth_provider TEXT NOT NULL DEFAULT 'email',
		firebase_uid TEXT,
		role TEXT NOT NULL DEFAULT 'user',
		status TEXT NOT NULL DEFAULT 'active',
		timeout_until DATETIME,
		timeout_reason TEXT,
		ban_reason TEXT,
		quota_limit INTEGER NOT NULL DEFAULT 100,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_login_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
	CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
	CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);

	-- Link clicks (privacy conscious)
	CREATE TABLE IF NOT EXISTS link_clicks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		link_id TEXT NOT NULL REFERENCES links(id) ON DELETE CASCADE,
		clicked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		country TEXT DEFAULT 'Unknown',
		referrer TEXT,
		device_type TEXT,
		browser TEXT,
		os TEXT
	);

	CREATE INDEX IF NOT EXISTS idx_link_clicks_link_id ON link_clicks(link_id);
	CREATE INDEX IF NOT EXISTS idx_link_clicks_clicked_at ON link_clicks(clicked_at);

	-- Quota tracking table
	CREATE TABLE IF NOT EXISTS quota_usage (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		identity_key TEXT NOT NULL,
		is_anonymous INTEGER NOT NULL,
		window_start TEXT NOT NULL,
		count INTEGER NOT NULL DEFAULT 1,
		UNIQUE(identity_key, window_start)
	);

	CREATE INDEX IF NOT EXISTS idx_quota_identity ON quota_usage(identity_key, window_start);

	-- Abuse reports
	CREATE TABLE IF NOT EXISTS reports (
		id TEXT PRIMARY KEY,
		link_id TEXT NOT NULL REFERENCES links(id) ON DELETE CASCADE,
		short_code TEXT NOT NULL,
		reason TEXT NOT NULL,
		details TEXT,
		reporter_ip_hash TEXT,
		status TEXT NOT NULL DEFAULT 'pending',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	-- Permanent link requests
	CREATE TABLE IF NOT EXISTS permanent_link_requests (
		id TEXT PRIMARY KEY,
		link_id TEXT NOT NULL REFERENCES links(id) ON DELETE CASCADE,
		user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		reason TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		reviewed_by TEXT REFERENCES users(id),
		reviewed_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	-- Login audit records
	CREATE TABLE IF NOT EXISTS login_records (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		account_email TEXT NOT NULL,
		auth_method TEXT NOT NULL,
		result TEXT NOT NULL,
		ip_hash TEXT,
		user_agent TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	-- Admin audit logs
	CREATE TABLE IF NOT EXISTS admin_audit_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		admin_id TEXT NOT NULL REFERENCES users(id),
		admin_email TEXT NOT NULL,
		action TEXT NOT NULL,
		target_type TEXT NOT NULL,
		target_id TEXT NOT NULL,
		reason TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	-- System settings
	CREATE TABLE IF NOT EXISTS system_settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	`

	if _, err := db.Exec(schema); err != nil {
		return err
	}

	// Backward compatibility: ensure deleted_at column exists in links table
	var hasDeletedAt bool
	if rows, err := db.Query("PRAGMA table_info(links);"); err == nil {
		for rows.Next() {
			var cid int
			var name, ctype string
			var notnull, pk int
			var dflt *string
			if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err == nil {
				if name == "deleted_at" {
					hasDeletedAt = true
				}
			}
		}
		rows.Close()
		if !hasDeletedAt {
			_, _ = db.Exec("ALTER TABLE links ADD COLUMN deleted_at DATETIME;")
			_, _ = db.Exec("CREATE INDEX IF NOT EXISTS idx_links_deleted_at ON links(deleted_at);")
		}
	}

	return nil
}
