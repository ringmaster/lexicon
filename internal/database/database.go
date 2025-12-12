package database

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB wraps the database connection and provides application-specific methods.
type DB struct {
	*sql.DB
}

// Open creates a new database connection and initializes the schema.
func Open(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db := &DB{sqlDB}

	if err := db.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}

func (db *DB) migrate() error {
	schema := `
	-- Users table
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user')),
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	-- Pages table
	CREATE TABLE IF NOT EXISTS pages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		slug TEXT UNIQUE NOT NULL,
		title TEXT NOT NULL,
		is_phantom INTEGER NOT NULL DEFAULT 0,
		first_cited_by_user_id INTEGER REFERENCES users(id),
		first_cited_in_page_id INTEGER REFERENCES pages(id),
		deleted_at DATETIME,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	-- Revisions table (full content per revision)
	CREATE TABLE IF NOT EXISTS revisions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		page_id INTEGER NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
		content TEXT NOT NULL,
		author_id INTEGER NOT NULL REFERENCES users(id),
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	-- Comments table
	CREATE TABLE IF NOT EXISTS comments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		page_id INTEGER NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
		author_id INTEGER NOT NULL REFERENCES users(id),
		content TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	-- Sessions table
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		expires_at DATETIME NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	-- Settings table (key-value)
	CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	-- Indexes
	CREATE INDEX IF NOT EXISTS idx_pages_is_phantom ON pages(is_phantom);
	CREATE INDEX IF NOT EXISTS idx_pages_deleted_at ON pages(deleted_at);
	CREATE INDEX IF NOT EXISTS idx_revisions_page_id ON revisions(page_id);
	CREATE INDEX IF NOT EXISTS idx_comments_page_id ON comments(page_id);
	CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
	`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Run migrations for existing databases
	if err := db.runMigrations(); err != nil {
		return err
	}

	// Create FTS5 virtual table (separate because CREATE VIRTUAL TABLE IF NOT EXISTS is tricky)
	if err := db.createFTS(); err != nil {
		return err
	}

	// Insert default settings
	if err := db.ensureDefaultSettings(); err != nil {
		return err
	}

	return nil
}

func (db *DB) runMigrations() error {
	// Migration: Add deleted_at column to pages table if it doesn't exist
	var colCount int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('pages') WHERE name='deleted_at'
	`).Scan(&colCount)
	if err != nil {
		return fmt.Errorf("failed to check for deleted_at column: %w", err)
	}
	if colCount == 0 {
		_, err = db.Exec(`ALTER TABLE pages ADD COLUMN deleted_at DATETIME`)
		if err != nil {
			return fmt.Errorf("failed to add deleted_at column: %w", err)
		}
	}

	return nil
}

func (db *DB) createFTS() error {
	// Check if FTS table exists
	var name string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='pages_fts'").Scan(&name)
	if err == nil {
		return nil // Table exists
	}
	if err != sql.ErrNoRows {
		return fmt.Errorf("failed to check FTS table: %w", err)
	}

	_, err = db.Exec(`
		CREATE VIRTUAL TABLE pages_fts USING fts5(
			title,
			content,
			tokenize='porter unicode61'
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create FTS table: %w", err)
	}

	return nil
}

func (db *DB) ensureDefaultSettings() error {
	defaults := map[string]string{
		"public_read_access":   "false",
		"registration_enabled": "false",
		"registration_code":    "",
		"wiki_title":           "Lexicon Wiki",
	}

	for key, value := range defaults {
		_, err := db.Exec(
			"INSERT OR IGNORE INTO settings (key, value) VALUES (?, ?)",
			key, value,
		)
		if err != nil {
			return fmt.Errorf("failed to insert default setting %s: %w", key, err)
		}
	}

	return nil
}

// NeedsAdminSetup returns true if no admin user exists.
func (db *DB) NeedsAdminSetup() (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}
