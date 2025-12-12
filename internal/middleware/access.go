package middleware

import (
	"net/http"
	"strings"

	"lexicon/internal/database"
)

// PublicAccessMiddleware checks public_read_access setting for read-only routes.
func PublicAccessMiddleware(db *database.DB) func(http.Handler) http.Handler {
	// Routes that are always accessible
	publicPaths := map[string]bool{
		"/login":    true,
		"/logout":   true,
		"/register": true,
		"/static/":  true,
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path is always public
			for path := range publicPaths {
				if strings.HasPrefix(r.URL.Path, path) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// If user is logged in, allow access
			if GetUser(r) != nil {
				next.ServeHTTP(w, r)
				return
			}

			// Check public read access setting
			publicAccess, err := db.PublicReadAccess()
			if err != nil || !publicAccess {
				// If not public or error, redirect to login
				http.Redirect(w, r, "/login?redirect="+r.URL.Path, http.StatusSeeOther)
				return
			}

			// Public access enabled, but only for GET requests (reading)
			if r.Method != http.MethodGet {
				http.Redirect(w, r, "/login?redirect="+r.URL.Path, http.StatusSeeOther)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
