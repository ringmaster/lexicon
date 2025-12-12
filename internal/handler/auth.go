package handler

import (
	"crypto/subtle"
	"net/http"
	"regexp"

	"lexicon/internal/database"
	"lexicon/internal/middleware"
)

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{3,50}$`)

// LoginForm renders the login page.
func (h *Handler) LoginForm(w http.ResponseWriter, r *http.Request) {
	if middleware.IsLoggedIn(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	redirect := r.URL.Query().Get("redirect")
	registrationEnabled, _ := h.DB.GetSetting("registration_enabled")

	h.Render(w, r, "auth/login.html", "Login", map[string]any{
		"Redirect":            redirect,
		"RegistrationEnabled": registrationEnabled == "true",
	})
}

// Login handles login form submission.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	redirect := r.FormValue("redirect")

	if redirect == "" || redirect[0] != '/' {
		redirect = "/"
	}

	user, err := h.DB.AuthenticateUser(username, password)
	if err != nil {
		h.AddFlash(r, "danger", "Invalid username or password")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	session, err := h.DB.CreateSession(user.ID)
	if err != nil {
		h.AddFlash(r, "danger", "Failed to create session")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    session.ID,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60, // 30 days
		HttpOnly: true,
		Secure:   !h.Config.HTTPMode,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

// Logout handles logout.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSession(r)
	if session != nil {
		h.DB.DeleteSession(session.ID)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// RegisterForm renders the registration page.
func (h *Handler) RegisterForm(w http.ResponseWriter, r *http.Request) {
	if middleware.IsLoggedIn(r) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	enabled, _ := h.DB.RegistrationEnabled()
	if !enabled {
		h.RenderError(w, r, http.StatusForbidden, "Registration is disabled")
		return
	}

	regCode, _ := h.DB.RegistrationCode()
	h.Render(w, r, "auth/register.html", "Register", map[string]any{
		"RequireCode": regCode != "",
	})
}

// Register handles registration form submission.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	enabled, _ := h.DB.RegistrationEnabled()
	if !enabled {
		h.RenderError(w, r, http.StatusForbidden, "Registration is disabled")
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")
	confirm := r.FormValue("confirm")
	code := r.FormValue("code")

	// Validate username
	if !usernameRegex.MatchString(username) {
		h.AddFlash(r, "danger", "Username must be 3-50 alphanumeric characters")
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}

	// Validate password
	if len(password) < 8 {
		h.AddFlash(r, "danger", "Password must be at least 8 characters")
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}

	if password != confirm {
		h.AddFlash(r, "danger", "Passwords do not match")
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}

	// Check registration code
	regCode, _ := h.DB.RegistrationCode()
	if regCode != "" {
		if subtle.ConstantTimeCompare([]byte(code), []byte(regCode)) != 1 {
			// Generic error to not reveal if username was taken vs wrong code
			h.AddFlash(r, "danger", "Registration failed")
			http.Redirect(w, r, "/register", http.StatusSeeOther)
			return
		}
	}

	// Check if username exists
	_, err := h.DB.GetUserByUsername(username)
	if err == nil {
		// Generic error
		h.AddFlash(r, "danger", "Registration failed")
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}
	if err != database.ErrNotFound {
		h.AddFlash(r, "danger", "Registration failed")
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}

	// Create user
	user, err := h.DB.CreateUser(username, password, "user")
	if err != nil {
		h.AddFlash(r, "danger", "Registration failed")
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}

	// Create session
	session, err := h.DB.CreateSession(user.ID)
	if err != nil {
		h.AddFlash(r, "danger", "Registration succeeded but login failed")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    session.ID,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   !h.Config.HTTPMode,
		SameSite: http.SameSiteLaxMode,
	})

	h.AddFlash(r, "success", "Welcome to the wiki!")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ChangePasswordForm renders the change password page.
func (h *Handler) ChangePasswordForm(w http.ResponseWriter, r *http.Request) {
	h.Render(w, r, "auth/change-password.html", "Change Password", nil)
}

// ChangePassword handles password change form submission.
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)

	currentPassword := r.FormValue("current_password")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	// Validate new password
	if len(newPassword) < 8 {
		h.AddFlash(r, "danger", "New password must be at least 8 characters")
		http.Redirect(w, r, "/account/password", http.StatusSeeOther)
		return
	}

	if newPassword != confirmPassword {
		h.AddFlash(r, "danger", "New passwords do not match")
		http.Redirect(w, r, "/account/password", http.StatusSeeOther)
		return
	}

	// Update password
	err := h.DB.UpdatePassword(user.ID, currentPassword, newPassword)
	if err == database.ErrNotFound {
		h.AddFlash(r, "danger", "Current password is incorrect")
		http.Redirect(w, r, "/account/password", http.StatusSeeOther)
		return
	}
	if err != nil {
		h.AddFlash(r, "danger", "Failed to update password")
		http.Redirect(w, r, "/account/password", http.StatusSeeOther)
		return
	}

	h.AddFlash(r, "success", "Password updated successfully")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
