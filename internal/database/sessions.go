package database

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"time"
)

const sessionDuration = 30 * 24 * time.Hour // 30 days

// Session represents a user session.
type Session struct {
	ID        string
	UserID    int64
	ExpiresAt time.Time
	CreatedAt time.Time
}

// CreateSession creates a new session for a user.
func (db *DB) CreateSession(userID int64) (*Session, error) {
	// Generate random session ID
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return nil, err
	}
	sessionID := base64.URLEncoding.EncodeToString(bytes)

	now := time.Now()
	expiresAt := now.Add(sessionDuration)

	_, err := db.Exec(`
		INSERT INTO sessions (id, user_id, expires_at, created_at)
		VALUES (?, ?, ?, ?)
	`, sessionID, userID, expiresAt, now)
	if err != nil {
		return nil, err
	}

	return &Session{
		ID:        sessionID,
		UserID:    userID,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}, nil
}

// GetSession retrieves a session by ID if it hasn't expired.
func (db *DB) GetSession(sessionID string) (*Session, error) {
	session := &Session{}
	err := db.QueryRow(`
		SELECT id, user_id, expires_at, created_at
		FROM sessions
		WHERE id = ? AND expires_at > ?
	`, sessionID, time.Now()).Scan(&session.ID, &session.UserID, &session.ExpiresAt, &session.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return session, nil
}

// DeleteSession removes a session.
func (db *DB) DeleteSession(sessionID string) error {
	_, err := db.Exec("DELETE FROM sessions WHERE id = ?", sessionID)
	return err
}

// DeleteUserSessions removes all sessions for a user.
func (db *DB) DeleteUserSessions(userID int64) error {
	_, err := db.Exec("DELETE FROM sessions WHERE user_id = ?", userID)
	return err
}

// CleanExpiredSessions removes all expired sessions.
func (db *DB) CleanExpiredSessions() error {
	_, err := db.Exec("DELETE FROM sessions WHERE expires_at < ?", time.Now())
	return err
}

// ExtendSession updates the expiration time.
func (db *DB) ExtendSession(sessionID string) error {
	newExpiry := time.Now().Add(sessionDuration)
	_, err := db.Exec("UPDATE sessions SET expires_at = ? WHERE id = ?", newExpiry, sessionID)
	return err
}
