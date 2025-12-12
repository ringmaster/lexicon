package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// AdminDashboard renders the admin dashboard.
func (h *Handler) AdminDashboard(w http.ResponseWriter, r *http.Request) {
	pageCount, phantomCount, _ := h.DB.PageStats()
	userCount, _ := h.DB.UserCount()

	h.Render(w, r, "admin/dashboard.html", "Admin Dashboard", map[string]any{
		"PageCount":    pageCount,
		"PhantomCount": phantomCount,
		"UserCount":    userCount,
	})
}

// AdminSettings renders the settings page.
func (h *Handler) AdminSettings(w http.ResponseWriter, r *http.Request) {
	settings, _ := h.DB.GetAllSettings()

	h.Render(w, r, "admin/settings.html", "Settings", map[string]any{
		"Settings": settings,
	})
}

// AdminSaveSettings handles settings form submission.
func (h *Handler) AdminSaveSettings(w http.ResponseWriter, r *http.Request) {
	// Update each setting
	settings := map[string]string{
		"wiki_title":           r.FormValue("wiki_title"),
		"public_read_access":   boolToString(r.FormValue("public_read_access") == "true"),
		"registration_enabled": boolToString(r.FormValue("registration_enabled") == "true"),
		"registration_code":    r.FormValue("registration_code"),
	}

	for key, value := range settings {
		if err := h.DB.SetSetting(key, value); err != nil {
			h.AddFlash(r, "danger", "Failed to save settings")
			http.Redirect(w, r, "/admin/settings", http.StatusSeeOther)
			return
		}
	}

	h.AddFlash(r, "success", "Settings saved")
	http.Redirect(w, r, "/admin/settings", http.StatusSeeOther)
}

// AdminUsers renders the user management page.
func (h *Handler) AdminUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.DB.ListUsers()
	if err != nil {
		h.RenderError(w, r, http.StatusInternalServerError, "Database error")
		return
	}

	h.Render(w, r, "admin/users.html", "User Management", map[string]any{
		"Users": users,
	})
}

// AdminChangeRole handles role change requests.
func (h *Handler) AdminChangeRole(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "userID")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		h.AddFlash(r, "danger", "Invalid user ID")
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}

	role := r.FormValue("role")
	if role != "admin" && role != "user" {
		h.AddFlash(r, "danger", "Invalid role")
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}

	if err := h.DB.UpdateUserRole(userID, role); err != nil {
		h.AddFlash(r, "danger", "Failed to change role")
	} else {
		h.AddFlash(r, "success", "User role updated")
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

// AdminDeleteUser handles user deletion.
func (h *Handler) AdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	userIDStr := chi.URLParam(r, "userID")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		h.AddFlash(r, "danger", "Invalid user ID")
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}

	// Delete user sessions first
	h.DB.DeleteUserSessions(userID)

	if err := h.DB.DeleteUser(userID); err != nil {
		h.AddFlash(r, "danger", "Failed to delete user")
	} else {
		h.AddFlash(r, "success", "User deleted")
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

// AdminDeletedPages renders the deleted pages management page.
func (h *Handler) AdminDeletedPages(w http.ResponseWriter, r *http.Request) {
	pages, err := h.DB.ListDeletedPages()
	if err != nil {
		h.RenderError(w, r, http.StatusInternalServerError, "Database error")
		return
	}

	h.Render(w, r, "admin/deleted.html", "Deleted Pages", map[string]any{
		"Pages": pages,
	})
}

// AdminRestorePage handles page restoration.
func (h *Handler) AdminRestorePage(w http.ResponseWriter, r *http.Request) {
	pageIDStr := chi.URLParam(r, "pageID")
	pageID, err := strconv.ParseInt(pageIDStr, 10, 64)
	if err != nil {
		h.AddFlash(r, "danger", "Invalid page ID")
		http.Redirect(w, r, "/admin/deleted", http.StatusSeeOther)
		return
	}

	if err := h.DB.RestorePage(pageID); err != nil {
		h.AddFlash(r, "danger", "Failed to restore page")
	} else {
		h.AddFlash(r, "success", "Page restored")
	}

	http.Redirect(w, r, "/admin/deleted", http.StatusSeeOther)
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
