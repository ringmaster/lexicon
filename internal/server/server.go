package server

import (
	"context"
	"crypto/tls"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"lexicon/internal/config"
	"lexicon/internal/database"
	"lexicon/internal/handler"
	"lexicon/internal/middleware"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"golang.org/x/crypto/acme/autocert"
)

// Server wraps the HTTP server and its dependencies.
type Server struct {
	config     *config.Config
	db         *database.DB
	handler    *handler.Handler
	router     *chi.Mux
	embeddedFS fs.FS
}

// New creates a new Server.
func New(cfg *config.Config, db *database.DB, embeddedFS fs.FS) *Server {
	return &Server{
		config:     cfg,
		db:         db,
		embeddedFS: embeddedFS,
	}
}

// Run starts the HTTP server.
func (s *Server) Run() error {
	// Create overlay filesystem for templates
	tmplFS := newOverlayFS("templates", mustSubFS(s.embeddedFS, "templates"))
	staticFS := newOverlayFS("static", mustSubFS(s.embeddedFS, "static"))

	// Create handler
	var err error
	s.handler, err = handler.New(s.config, s.db, tmplFS)
	if err != nil {
		return err
	}

	// Set up router
	s.router = chi.NewRouter()

	// Global middleware
	s.router.Use(chimw.RealIP)
	s.router.Use(chimw.Logger)
	s.router.Use(chimw.Recoverer)
	s.router.Use(middleware.SessionMiddleware(s.db))
	s.router.Use(middleware.CSRFMiddleware(s.handler.CSRFStore))

	// Static files
	s.router.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Auth routes (always public)
	s.router.Get("/login", s.handler.LoginForm)
	s.router.With(middleware.RateLimitMiddleware(middleware.LoginLimiter)).Post("/login", s.handler.Login)
	s.router.Post("/logout", s.handler.Logout)
	s.router.Get("/register", s.handler.RegisterForm)
	s.router.With(middleware.RateLimitMiddleware(middleware.RegisterLimiter)).Post("/register", s.handler.Register)

	// Public routes (access controlled by PublicAccessMiddleware)
	s.router.Group(func(r chi.Router) {
		r.Use(middleware.PublicAccessMiddleware(s.db))

		r.Get("/", s.handler.Home)
		r.Get("/pages", s.handler.ListPages)
		r.Get("/pages/phantoms", s.handler.ListPhantoms)
		r.Get("/pages/recent", s.handler.RecentPages)
		r.Get("/search", s.handler.Search)

		// Page routes at root level (must be after specific routes)
		r.Get("/{slug}", s.handler.ViewPage)
		r.Get("/{slug}/history", s.handler.PageHistory)
		r.Get("/{slug}/revision/{revisionID}", s.handler.ViewRevision)
	})

	// Authenticated user routes
	s.router.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth)

		r.Get("/account/password", s.handler.ChangePasswordForm)
		r.Post("/account/password", s.handler.ChangePassword)
		r.Get("/{slug}/edit", s.handler.EditPage)
		r.Post("/{slug}", s.handler.SavePage)
		r.Post("/{slug}/comments", s.handler.AddComment)
	})

	// Admin routes
	s.router.Group(func(r chi.Router) {
		r.Use(middleware.RequireAdmin)

		r.Get("/admin", s.handler.AdminDashboard)
		r.Get("/admin/settings", s.handler.AdminSettings)
		r.Post("/admin/settings", s.handler.AdminSaveSettings)
		r.Get("/admin/users", s.handler.AdminUsers)
		r.Post("/admin/users/{userID}/role", s.handler.AdminChangeRole)
		r.Post("/admin/users/{userID}/delete", s.handler.AdminDeleteUser)
		r.Get("/admin/export", s.handler.Export)
		r.Get("/admin/deleted", s.handler.AdminDeletedPages)
		r.Post("/admin/deleted/{pageID}/restore", s.handler.AdminRestorePage)
		r.Post("/{slug}/delete", s.handler.DeletePage)
	})

	// Create HTTP server
	srv := &http.Server{
		Addr:         s.config.ListenAddr(),
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Handle graceful shutdown
	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Server is shutting down...")

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		srv.SetKeepAlivesEnabled(false)
		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("Could not gracefully shutdown: %v", err)
		}
		close(done)
	}()

	if s.config.HTTPMode {
		log.Printf("Starting HTTP server on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
	} else {
		return s.runHTTPS(srv)
	}

	<-done
	return nil
}

func (s *Server) runHTTPS(srv *http.Server) error {
	// Set up autocert manager
	certManager := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(s.config.Domain),
		Cache:      autocert.DirCache(s.config.AutocertDir()),
		Email:      s.config.AdminEmail,
	}

	// TLS config
	srv.TLSConfig = &tls.Config{
		GetCertificate: certManager.GetCertificate,
		MinVersion:     tls.VersionTLS12,
	}

	// Start HTTP->HTTPS redirect server
	go func() {
		redirectSrv := &http.Server{
			Addr:    ":80",
			Handler: certManager.HTTPHandler(nil),
		}
		log.Printf("Starting HTTP redirect server on :80")
		if err := redirectSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP redirect server error: %v", err)
		}
	}()

	log.Printf("Starting HTTPS server on %s", srv.Addr)
	if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// overlayFS checks local disk first, then falls back to embedded.
type overlayFS struct {
	disk     string
	embedded fs.FS
}

func newOverlayFS(diskPath string, embedded fs.FS) fs.FS {
	return &overlayFS{
		disk:     diskPath,
		embedded: embedded,
	}
}

func (o *overlayFS) Open(name string) (fs.File, error) {
	if o.disk != "" {
		path := filepath.Join(o.disk, name)
		if f, err := os.Open(path); err == nil {
			return f, nil
		}
	}
	return o.embedded.Open(name)
}

func mustSubFS(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
