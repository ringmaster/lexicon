package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Domain for Let's Encrypt certificate (e.g., wiki.example.com)
	Domain string

	// Directory for SQLite database and autocert cache
	DataDir string

	// Secret for session signing (32+ characters)
	SessionSecret string

	// Email for Let's Encrypt registration
	AdminEmail string

	// If true, run plain HTTP instead of HTTPS
	HTTPMode bool

	// Port to listen on (defaults to 443 for HTTPS, 8080 for HTTP)
	Port int

	// Optional: admin credentials for first-run setup
	AdminUsername string
	AdminPassword string
}

// Load reads configuration from environment variables and validates required fields.
func Load() (*Config, error) {
	cfg := &Config{
		Domain:        os.Getenv("LEXICON_DOMAIN"),
		DataDir:       getEnvDefault("LEXICON_DATA_DIR", "./data"),
		SessionSecret: os.Getenv("LEXICON_SESSION_SECRET"),
		AdminEmail:    os.Getenv("LEXICON_ADMIN_EMAIL"),
		HTTPMode:      os.Getenv("LEXICON_HTTP_MODE") == "true",
		AdminUsername: os.Getenv("LEXICON_ADMIN_USERNAME"),
		AdminPassword: os.Getenv("LEXICON_ADMIN_PASSWORD"),
	}

	// Parse port
	portStr := os.Getenv("LEXICON_PORT")
	if portStr != "" {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("LEXICON_PORT must be a valid integer: %w", err)
		}
		cfg.Port = port
	} else {
		if cfg.HTTPMode {
			cfg.Port = 8080
		} else {
			cfg.Port = 443
		}
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.SessionSecret == "" {
		return errors.New("LEXICON_SESSION_SECRET is required (32+ characters)")
	}
	if len(c.SessionSecret) < 32 {
		return errors.New("LEXICON_SESSION_SECRET must be at least 32 characters")
	}

	if !c.HTTPMode {
		if c.Domain == "" {
			return errors.New("LEXICON_DOMAIN is required for HTTPS mode (or set LEXICON_HTTP_MODE=true)")
		}
		if c.AdminEmail == "" {
			return errors.New("LEXICON_ADMIN_EMAIL is required for HTTPS mode (or set LEXICON_HTTP_MODE=true)")
		}
	}

	return nil
}

// ListenAddr returns the address to listen on based on configuration.
func (c *Config) ListenAddr() string {
	return fmt.Sprintf(":%d", c.Port)
}

// DatabasePath returns the full path to the SQLite database file.
func (c *Config) DatabasePath() string {
	return c.DataDir + "/lexicon.db"
}

// AutocertDir returns the directory for Let's Encrypt certificate cache.
func (c *Config) AutocertDir() string {
	return c.DataDir + "/autocert"
}

func getEnvDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
