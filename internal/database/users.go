package database

import (
	"database/sql"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User represents a user account.
type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Role         string // "admin" or "user"
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// IsAdmin returns true if the user has admin role.
func (u *User) IsAdmin() bool {
	return u.Role == "admin"
}

// CreateUser creates a new user with hashed password.
func (db *DB) CreateUser(username, password, role string) (*User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	result, err := db.Exec(`
		INSERT INTO users (username, password_hash, role, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, username, string(hash), role, now, now)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return db.GetUserByID(id)
}

// GetUserByID retrieves a user by ID.
func (db *DB) GetUserByID(id int64) (*User, error) {
	user := &User{}
	err := db.QueryRow(`
		SELECT id, username, password_hash, role, created_at, updated_at
		FROM users WHERE id = ?
	`, id).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserByUsername retrieves a user by username.
func (db *DB) GetUserByUsername(username string) (*User, error) {
	user := &User{}
	err := db.QueryRow(`
		SELECT id, username, password_hash, role, created_at, updated_at
		FROM users WHERE username = ?
	`, username).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// AuthenticateUser verifies credentials and returns the user if valid.
func (db *DB) AuthenticateUser(username, password string) (*User, error) {
	user, err := db.GetUserByUsername(username)
	if err != nil {
		return nil, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return nil, ErrNotFound // Don't reveal whether user exists
	}

	return user, nil
}

// ListUsers returns all users.
func (db *DB) ListUsers() ([]*User, error) {
	rows, err := db.Query(`
		SELECT id, username, password_hash, role, created_at, updated_at
		FROM users ORDER BY username ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		user := &User{}
		err := rows.Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

// UpdateUserRole changes a user's role.
func (db *DB) UpdateUserRole(userID int64, role string) error {
	_, err := db.Exec(`
		UPDATE users SET role = ?, updated_at = ? WHERE id = ?
	`, role, time.Now(), userID)
	return err
}

// UpdatePassword changes a user's password after verifying the current one.
func (db *DB) UpdatePassword(userID int64, currentPassword, newPassword string) error {
	// Get current user
	user, err := db.GetUserByID(userID)
	if err != nil {
		return err
	}

	// Verify current password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword))
	if err != nil {
		return ErrNotFound // Current password doesn't match
	}

	// Hash new password
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Update password
	_, err = db.Exec(`
		UPDATE users SET password_hash = ?, updated_at = ? WHERE id = ?
	`, string(hash), time.Now(), userID)
	return err
}

// DeleteUser removes a user.
func (db *DB) DeleteUser(userID int64) error {
	_, err := db.Exec("DELETE FROM users WHERE id = ?", userID)
	return err
}

// UserCount returns the total number of users.
func (db *DB) UserCount() (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}
