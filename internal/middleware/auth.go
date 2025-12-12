package middleware

import (
	"context"
	"net/http"

	"lexicon/internal/database"
)

type contextKey string

const (
	userContextKey    contextKey = "user"
	sessionContextKey contextKey = "session"
)

// SessionMiddleware loads the user from session cookie and adds to context.
func SessionMiddleware(db *database.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session")
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			session, err := db.GetSession(cookie.Value)
			if err != nil {
				// Invalid or expired session, clear cookie
				http.SetCookie(w, &http.Cookie{
					Name:     "session",
					Value:    "",
					Path:     "/",
					MaxAge:   -1,
					HttpOnly: true,
				})
				next.ServeHTTP(w, r)
				return
			}

			user, err := db.GetUserByID(session.UserID)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			// Clean up expired sessions periodically (1% of requests)
			// This is a simple approach; in production you might use a background job
			if r.URL.Path == "/" {
				go db.CleanExpiredSessions()
			}

			// Add user and session to context
			ctx := context.WithValue(r.Context(), userContextKey, user)
			ctx = context.WithValue(ctx, sessionContextKey, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireAuth ensures the user is logged in.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if GetUser(r) == nil {
			http.Redirect(w, r, "/login?redirect="+r.URL.Path, http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAdmin ensures the user is an admin.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r)
		if user == nil {
			http.Redirect(w, r, "/login?redirect="+r.URL.Path, http.StatusSeeOther)
			return
		}
		if !user.IsAdmin() {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GetUser returns the current user from context, or nil if not logged in.
func GetUser(r *http.Request) *database.User {
	user, _ := r.Context().Value(userContextKey).(*database.User)
	return user
}

// GetSession returns the current session from context, or nil if not logged in.
func GetSession(r *http.Request) *database.Session {
	session, _ := r.Context().Value(sessionContextKey).(*database.Session)
	return session
}

// IsLoggedIn returns true if a user is logged in.
func IsLoggedIn(r *http.Request) bool {
	return GetUser(r) != nil
}
