package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"sync"
)

const csrfTokenContextKey contextKey = "csrf_token"

// CSRFStore manages CSRF tokens in memory.
type CSRFStore struct {
	mu     sync.RWMutex
	tokens map[string]bool
}

// NewCSRFStore creates a new CSRF token store.
func NewCSRFStore() *CSRFStore {
	return &CSRFStore{
		tokens: make(map[string]bool),
	}
}

// Generate creates a new CSRF token.
func (s *CSRFStore) Generate() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := base64.URLEncoding.EncodeToString(bytes)

	s.mu.Lock()
	s.tokens[token] = true
	s.mu.Unlock()

	return token, nil
}

// Validate checks if a token is valid and removes it.
func (s *CSRFStore) Validate(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.tokens[token] {
		delete(s.tokens, token)
		return true
	}
	return false
}

// CSRFMiddleware adds CSRF protection.
func CSRFMiddleware(store *CSRFStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Generate token for all requests (so templates can use it)
			token, err := store.Generate()
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			// Add token to context
			ctx := context.WithValue(r.Context(), csrfTokenContextKey, token)
			r = r.WithContext(ctx)

			// Validate token on POST requests
			if r.Method == http.MethodPost {
				formToken := r.FormValue("csrf_token")
				// For POST, we check the submitted token (not the one we just generated)
				if formToken == "" || !store.Validate(formToken) {
					http.Error(w, "Invalid request", http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetCSRFToken returns the CSRF token from context.
func GetCSRFToken(r *http.Request) string {
	token, _ := r.Context().Value(csrfTokenContextKey).(string)
	return token
}
