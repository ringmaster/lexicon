package database

import "database/sql"

// GetSetting retrieves a setting value by key.
func (db *DB) GetSetting(key string) (string, error) {
	var value string
	err := db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

// SetSetting updates or creates a setting.
func (db *DB) SetSetting(key, value string) error {
	_, err := db.Exec(`
		INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	return err
}

// GetAllSettings returns all settings as a map.
func (db *DB) GetAllSettings() (map[string]string, error) {
	rows, err := db.Query("SELECT key, value FROM settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		settings[key] = value
	}
	return settings, rows.Err()
}

// Convenience methods for typed settings

// PublicReadAccess returns whether unauthenticated users can read pages.
func (db *DB) PublicReadAccess() (bool, error) {
	val, err := db.GetSetting("public_read_access")
	if err != nil {
		return false, err
	}
	return val == "true", nil
}

// RegistrationEnabled returns whether new user registration is allowed.
func (db *DB) RegistrationEnabled() (bool, error) {
	val, err := db.GetSetting("registration_enabled")
	if err != nil {
		return false, err
	}
	return val == "true", nil
}

// RegistrationCode returns the required code for registration (empty = no code).
func (db *DB) RegistrationCode() (string, error) {
	return db.GetSetting("registration_code")
}

// WikiTitle returns the wiki title for display.
func (db *DB) WikiTitle() (string, error) {
	return db.GetSetting("wiki_title")
}
