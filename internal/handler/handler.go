package handler

import (
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"sync"

	"lexicon/internal/config"
	"lexicon/internal/database"
	"lexicon/internal/markdown"
	"lexicon/internal/middleware"
)

// Flash represents a flash message.
type Flash struct {
	Type    string // "success", "warning", "danger", "info"
	Message string
}

// Handler provides HTTP handlers for the application.
type Handler struct {
	DB        *database.DB
	Config    *config.Config
	templates map[string]*template.Template
	Markdown  *markdown.Renderer
	CSRFStore *middleware.CSRFStore

	flashMu sync.RWMutex
	flashes map[string][]Flash // sessionID -> flashes
}

// New creates a new Handler.
func New(cfg *config.Config, db *database.DB, tmplFS fs.FS) (*Handler, error) {
	h := &Handler{
		DB:        db,
		Config:    cfg,
		CSRFStore: middleware.NewCSRFStore(),
		flashes:   make(map[string][]Flash),
		templates: make(map[string]*template.Template),
	}

	// Create markdown renderer with page checker
	h.Markdown = markdown.New(func(slug string) (bool, bool) {
		exists, err := db.PageExists(slug)
		if err != nil || !exists {
			return false, false
		}
		isPhantom, err := db.IsPhantom(slug)
		if err != nil {
			return true, false
		}
		return true, isPhantom
	})

	// Template functions
	funcMap := template.FuncMap{
		"safe": func(s string) template.HTML {
			return template.HTML(s)
		},
	}

	// Read layout template
	layoutContent, err := fs.ReadFile(tmplFS, "layout.html")
	if err != nil {
		return nil, err
	}

	// Find all page templates (skip layout.html)
	err = fs.WalkDir(tmplFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".html") || path == "layout.html" {
			return nil
		}

		// Read page template
		content, err := fs.ReadFile(tmplFS, path)
		if err != nil {
			return err
		}

		// Create template with layout + page content
		// Use path as-is for the template name
		tmpl, err := template.New("layout").Funcs(funcMap).Parse(string(layoutContent))
		if err != nil {
			return err
		}
		tmpl, err = tmpl.Parse(string(content))
		if err != nil {
			return err
		}

		h.templates[path] = tmpl
		log.Printf("Loaded template: %s", path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return h, nil
}

// TemplateData holds common data passed to templates.
type TemplateData struct {
	Title     string
	WikiTitle string
	User      *database.User
	CSRFToken string
	Flashes   []Flash
	Data      any
}

// Render renders a template with the given data.
func (h *Handler) Render(w http.ResponseWriter, r *http.Request, tmpl string, title string, data any) {
	wikiTitle, _ := h.DB.WikiTitle()
	if wikiTitle == "" {
		wikiTitle = "Lexicon Wiki"
	}

	td := TemplateData{
		Title:     title,
		WikiTitle: wikiTitle,
		User:      middleware.GetUser(r),
		CSRFToken: middleware.GetCSRFToken(r),
		Flashes:   h.GetFlashes(r),
		Data:      data,
	}

	t, ok := h.templates[tmpl]
	if !ok {
		log.Printf("Template not found: %s (available: %v)", tmpl, h.templateNames())
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout", td); err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) templateNames() []string {
	names := make([]string, 0, len(h.templates))
	for name := range h.templates {
		names = append(names, name)
	}
	return names
}

// AddFlash adds a flash message for the current session.
func (h *Handler) AddFlash(r *http.Request, typ, message string) {
	session := middleware.GetSession(r)
	if session == nil {
		return
	}

	h.flashMu.Lock()
	defer h.flashMu.Unlock()
	h.flashes[session.ID] = append(h.flashes[session.ID], Flash{Type: typ, Message: message})
}

// GetFlashes returns and clears flash messages for the current session.
func (h *Handler) GetFlashes(r *http.Request) []Flash {
	session := middleware.GetSession(r)
	if session == nil {
		return nil
	}

	h.flashMu.Lock()
	defer h.flashMu.Unlock()

	flashes := h.flashes[session.ID]
	delete(h.flashes, session.ID)
	return flashes
}

// RenderError renders an error page.
func (h *Handler) RenderError(w http.ResponseWriter, r *http.Request, status int, message string) {
	w.WriteHeader(status)
	h.Render(w, r, "errors/error.html", "Error", map[string]any{
		"Status":  status,
		"Message": message,
	})
}

// NotFound renders a 404 page.
func (h *Handler) NotFound(w http.ResponseWriter, r *http.Request) {
	h.RenderError(w, r, http.StatusNotFound, "Page not found")
}

// Forbidden renders a 403 page.
func (h *Handler) Forbidden(w http.ResponseWriter, r *http.Request) {
	h.RenderError(w, r, http.StatusForbidden, "Access denied")
}
