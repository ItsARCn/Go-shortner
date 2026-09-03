package database

import (
	"database/sql"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrationAddsDeletedAtColumnToExistingDatabase(t *testing.T) {
	tmpDB, err := os.CreateTemp("", "go-short-old-*.sqlite")
	if err != nil {
		t.Fatalf("failed to create temp db: %v", err)
	}
	tmpDBPath := tmpDB.Name()
	tmpDB.Close()
	defer func() {
		os.Remove(tmpDBPath)
		os.Remove(tmpDBPath + "-wal")
		os.Remove(tmpDBPath + "-shm")
	}()

	// 1. Manually create an older schema WITHOUT deleted_at column
	db, err := sql.Open("sqlite", tmpDBPath)
	if err != nil {
		t.Fatalf("failed to open raw sqlite: %v", err)
	}
	oldSchema := `
	CREATE TABLE links (
		id TEXT PRIMARY KEY,
		short_code TEXT UNIQUE NOT NULL,
		destination_url TEXT NOT NULL,
		owner_id TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		expires_at DATETIME NOT NULL,
		status TEXT NOT NULL DEFAULT 'ACTIVE',
		auto_renew INTEGER NOT NULL DEFAULT 0,
		click_count INTEGER NOT NULL DEFAULT 0
	);
	CREATE INDEX idx_links_short_code ON links(short_code);
	CREATE INDEX idx_links_owner_id ON links(owner_id);
	`
	if _, err := db.Exec(oldSchema); err != nil {
		db.Close()
		t.Fatalf("failed to setup old schema: %v", err)
	}
	db.Close()

	// 2. Now run InitDB on the existing database
	migratedDB, err := InitDB(tmpDBPath)
	if err != nil {
		t.Fatalf("InitDB failed on existing database without deleted_at: %v", err)
	}
	defer migratedDB.Close()

	// 3. Verify that deleted_at exists and can be queried
	var hasDeletedAt bool
	rows, err := migratedDB.Query("PRAGMA table_info(links);")
	if err != nil {
		t.Fatalf("failed to query table info: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err == nil {
			if name == "deleted_at" {
				hasDeletedAt = true
			}
		}
	}

	if !hasDeletedAt {
		t.Errorf("expected deleted_at column to be added by migration, but it was not found")
	}
}
