package handler

import (
	"fmt"
	"net/http"

	"lexicon/internal/database"
	"lexicon/internal/markdown"
	"lexicon/internal/middleware"

	"github.com/go-chi/chi/v5"
)

// Home renders the home page.
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	recentPages, _ := h.DB.ListRecentPages(10)
	pageCount, phantomCount, _ := h.DB.PageStats()

	data := map[string]any{
		"RecentPages":  recentPages,
		"PageCount":    pageCount,
		"PhantomCount": phantomCount,
	}

	// Check for a "home-page" wiki page to display as main content
	if homePage, err := h.DB.GetPageBySlug("home-page"); err == nil && !homePage.IsPhantom {
		if revision, err := h.DB.GetCurrentRevision(homePage.ID); err == nil {
			if html, err := h.Markdown.Render(revision.Content); err == nil {
				data["HomePageContent"] = html
				data["HomePageExists"] = true
			}
		}
	}

	h.Render(w, r, "home.html", "Home", data)
}

// ViewPage renders a page or phantom placeholder.
func (h *Handler) ViewPage(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	page, err := h.DB.GetPageBySlug(slug)
	if err == database.ErrNotFound {
		// Page doesn't exist - redirect to edit if logged in
		if middleware.IsLoggedIn(r) {
			http.Redirect(w, r, "/"+slug+"/edit", http.StatusSeeOther)
			return
		}
		h.NotFound(w, r)
		return
	}
	if err != nil {
		h.RenderError(w, r, http.StatusInternalServerError, "Database error")
		return
	}

	if page.IsPhantom {
		// Phantom page - redirect to edit if logged in
		if middleware.IsLoggedIn(r) {
			http.Redirect(w, r, "/"+slug+"/edit", http.StatusSeeOther)
			return
		}
		h.renderPhantom(w, r, page)
		return
	}

	// Check if deleted
	if page.DeletedAt != nil {
		h.renderDeleted(w, r, page)
		return
	}

	// Get current revision
	revision, err := h.DB.GetCurrentRevision(page.ID)
	if err != nil {
		h.RenderError(w, r, http.StatusInternalServerError, "Database error")
		return
	}

	// Render markdown
	html, err := h.Markdown.Render(revision.Content)
	if err != nil {
		h.RenderError(w, r, http.StatusInternalServerError, "Markdown error")
		return
	}

	// Get comments
	comments, _ := h.DB.ListComments(page.ID)
	revisionCount, _ := h.DB.RevisionCount(page.ID)

	h.Render(w, r, "page/view.html", page.Title, map[string]any{
		"Page":          page,
		"Content":       html,
		"Revision":      revision,
		"Comments":      comments,
		"RevisionCount": revisionCount,
	})
}

func (h *Handler) renderPhantom(w http.ResponseWriter, r *http.Request, page *database.Page) {
	var citedByUser *database.User
	var citedInPage *database.Page

	if page.FirstCitedByUserID != nil {
		citedByUser, _ = h.DB.GetUserByID(*page.FirstCitedByUserID)
	}
	if page.FirstCitedInPageID != nil {
		citedInPage, _ = h.DB.GetPageByID(*page.FirstCitedInPageID)
	}

	h.Render(w, r, "page/phantom.html", page.Title, map[string]any{
		"Page":         page,
		"CitedByUser":  citedByUser,
		"CitedInPage":  citedInPage,
		"CanEdit":      middleware.IsLoggedIn(r),
	})
}

func (h *Handler) renderDeleted(w http.ResponseWriter, r *http.Request, page *database.Page) {
	user := middleware.GetUser(r)
	isAdmin := user != nil && user.Role == "admin"

	h.Render(w, r, "page/deleted.html", page.Title, map[string]any{
		"Page":    page,
		"IsAdmin": isAdmin,
	})
}

// EditPage renders the edit form.
func (h *Handler) EditPage(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	page, err := h.DB.GetPageBySlug(slug)

	var title, content string
	if err == database.ErrNotFound {
		// New page - use slug as initial title
		title = slug
	} else if err != nil {
		h.RenderError(w, r, http.StatusInternalServerError, "Database error")
		return
	} else if page.IsPhantom {
		// Phantom page - use phantom's title
		title = page.Title
	} else {
		// Existing page - load current content
		title = page.Title
		rev, err := h.DB.GetCurrentRevision(page.ID)
		if err == nil {
			content = rev.Content
		}
	}

	h.Render(w, r, "page/edit.html", "Edit: "+title, map[string]any{
		"Slug":    slug,
		"Title":   title,
		"Content": content,
		"IsNew":   page == nil || page.IsPhantom,
	})
}

// SavePage handles page creation/update.
func (h *Handler) SavePage(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	user := middleware.GetUser(r)

	title := r.FormValue("title")
	content := r.FormValue("content")

	if title == "" {
		h.AddFlash(r, "danger", "Title is required")
		http.Redirect(w, r, "/"+slug+"/edit", http.StatusSeeOther)
		return
	}

	// Validate lengths
	if len(title) > 500 {
		h.AddFlash(r, "danger", "Title is too long (max 500 characters)")
		http.Redirect(w, r, "/"+slug+"/edit", http.StatusSeeOther)
		return
	}
	if len(content) > 500*1024 {
		h.AddFlash(r, "danger", "Content is too long (max 500KB)")
		http.Redirect(w, r, "/"+slug+"/edit", http.StatusSeeOther)
		return
	}

	page, err := h.DB.GetPageBySlug(slug)
	if err == database.ErrNotFound || (page != nil && page.IsPhantom) {
		// Create new page
		page, err = h.DB.CreatePage(slug, title, content, user.ID)
		if err != nil {
			h.AddFlash(r, "danger", "Failed to create page")
			http.Redirect(w, r, "/"+slug+"/edit", http.StatusSeeOther)
			return
		}
	} else if err != nil {
		h.RenderError(w, r, http.StatusInternalServerError, "Database error")
		return
	} else {
		// Update existing page
		err = h.DB.UpdatePage(page.ID, title, content, user.ID)
		if err != nil {
			h.AddFlash(r, "danger", "Failed to update page")
			http.Redirect(w, r, "/"+slug+"/edit", http.StatusSeeOther)
			return
		}
	}

	// Process wiki links and create phantoms
	h.processWikiLinks(content, user.ID, page.ID)

	h.AddFlash(r, "success", "Page saved")
	http.Redirect(w, r, "/"+slug, http.StatusSeeOther)
}

func (h *Handler) processWikiLinks(content string, userID, pageID int64) {
	links := h.Markdown.ExtractLinks(content)
	targets := markdown.UniqueTargets(links)

	for _, target := range targets {
		exists, err := h.DB.PageExists(target)
		if err != nil || exists {
			continue
		}

		// Find display text for this target
		var displayText string
		for _, link := range links {
			if link.Target == target {
				displayText = link.DisplayText
				break
			}
		}

		h.DB.CreatePhantom(target, displayText, userID, pageID)
	}
}

// PageHistory shows revision history.
func (h *Handler) PageHistory(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	page, err := h.DB.GetPageBySlug(slug)
	if err == database.ErrNotFound {
		h.NotFound(w, r)
		return
	}
	if err != nil {
		h.RenderError(w, r, http.StatusInternalServerError, "Database error")
		return
	}

	revisions, err := h.DB.ListRevisions(page.ID)
	if err != nil {
		h.RenderError(w, r, http.StatusInternalServerError, "Database error")
		return
	}

	h.Render(w, r, "page/history.html", "History: "+page.Title, map[string]any{
		"Page":      page,
		"Revisions": revisions,
	})
}

// ViewRevision shows a specific revision.
func (h *Handler) ViewRevision(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	revisionID := chi.URLParam(r, "revisionID")

	page, err := h.DB.GetPageBySlug(slug)
	if err == database.ErrNotFound {
		h.NotFound(w, r)
		return
	}
	if err != nil {
		h.RenderError(w, r, http.StatusInternalServerError, "Database error")
		return
	}

	var revID int64
	if _, err := fmt.Sscanf(revisionID, "%d", &revID); err != nil {
		h.NotFound(w, r)
		return
	}

	revision, err := h.DB.GetRevisionByID(revID)
	if err == database.ErrNotFound {
		h.NotFound(w, r)
		return
	}
	if err != nil {
		h.RenderError(w, r, http.StatusInternalServerError, "Database error")
		return
	}

	// Verify revision belongs to this page
	if revision.PageID != page.ID {
		h.NotFound(w, r)
		return
	}

	html, _ := h.Markdown.Render(revision.Content)

	h.Render(w, r, "page/revision.html", "Revision: "+page.Title, map[string]any{
		"Page":     page,
		"Revision": revision,
		"Content":  html,
	})
}

// ListPages shows all pages.
func (h *Handler) ListPages(w http.ResponseWriter, r *http.Request) {
	pages, err := h.DB.ListPages()
	if err != nil {
		h.RenderError(w, r, http.StatusInternalServerError, "Database error")
		return
	}

	h.Render(w, r, "pages/index.html", "All Pages", map[string]any{
		"Pages": pages,
	})
}

// ListPhantoms shows all phantom pages.
func (h *Handler) ListPhantoms(w http.ResponseWriter, r *http.Request) {
	phantoms, err := h.DB.ListPhantomsWithSource()
	if err != nil {
		h.RenderError(w, r, http.StatusInternalServerError, "Database error")
		return
	}

	h.Render(w, r, "pages/phantoms.html", "Phantom Pages", map[string]any{
		"Phantoms": phantoms,
	})
}

// RecentPages shows recently modified pages.
func (h *Handler) RecentPages(w http.ResponseWriter, r *http.Request) {
	pages, err := h.DB.ListRecentPages(50)
	if err != nil {
		h.RenderError(w, r, http.StatusInternalServerError, "Database error")
		return
	}

	h.Render(w, r, "pages/recent.html", "Recent Pages", map[string]any{
		"Pages": pages,
	})
}

// AddComment handles new comment submission.
func (h *Handler) AddComment(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	user := middleware.GetUser(r)

	page, err := h.DB.GetPageBySlug(slug)
	if err == database.ErrNotFound {
		h.NotFound(w, r)
		return
	}
	if err != nil {
		h.RenderError(w, r, http.StatusInternalServerError, "Database error")
		return
	}

	content := r.FormValue("content")
	if content == "" {
		h.AddFlash(r, "danger", "Comment cannot be empty")
		http.Redirect(w, r, "/"+slug, http.StatusSeeOther)
		return
	}

	if len(content) > 10*1024 {
		h.AddFlash(r, "danger", "Comment is too long (max 10KB)")
		http.Redirect(w, r, "/"+slug, http.StatusSeeOther)
		return
	}

	_, err = h.DB.CreateComment(page.ID, user.ID, content)
	if err != nil {
		h.AddFlash(r, "danger", "Failed to add comment")
	} else {
		h.AddFlash(r, "success", "Comment added")
	}

	http.Redirect(w, r, "/"+slug+"#comments", http.StatusSeeOther)
}

// DeletePage handles soft deletion of a page.
func (h *Handler) DeletePage(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	user := middleware.GetUser(r)

	page, err := h.DB.GetPageBySlug(slug)
	if err == database.ErrNotFound {
		h.NotFound(w, r)
		return
	}
	if err != nil {
		h.RenderError(w, r, http.StatusInternalServerError, "Database error")
		return
	}

	// Only admins can delete pages
	if user.Role != "admin" {
		h.RenderError(w, r, http.StatusForbidden, "Only admins can delete pages")
		return
	}

	err = h.DB.SoftDeletePage(page.ID)
	if err != nil {
		h.AddFlash(r, "danger", "Failed to delete page")
		http.Redirect(w, r, "/"+slug, http.StatusSeeOther)
		return
	}

	h.AddFlash(r, "success", "Page deleted")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
