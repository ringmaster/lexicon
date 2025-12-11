# Lexicon Wiki Specification

A purpose-built wiki for facilitating LexiconRPG games. This document provides complete implementation guidance for Claude Code.

## Project Overview

### Purpose

Lexicon is a collaborative worldbuilding game where players write encyclopedia entries about a fictional world, citing each other's work and creating "phantom" entries that other players must eventually write. This wiki supports that gameplay loop with:

- Wiki-style page creation and editing with revision history
- Phantom link tracking (cited-but-unwritten entries)
- Markdown content with `[[wiki-links]]`
- Per-page comment threads
- Full-text search
- Simple user authentication

### Design Philosophy

- **Minimal dependencies**: Use well-maintained libraries with clear documentation
- **Readable code**: Favor explicitness over magic; someone should understand the codebase by reading it
- **No JavaScript requirement**: The interface must function fully without client-side JS
- **Single-binary deployment**: Compile to one executable with embedded assets
- **SQLite storage**: Single-file database, easy to backup and restore

---

## Technical Stack

| Component | Choice | Rationale |
|-----------|--------|-----------|
| Language | Go 1.22+ | Method routing, embed directive |
| Router | chi | Minimal, stdlib-compatible, readable |
| Markdown | goldmark | Extensible, well-maintained, CommonMark compliant |
| Database | SQLite + FTS5 (pure Go) | Simple, embedded, no CGO required |
| CSS | Bulma | Clean, no-JS, good component vocabulary |
| TLS | autocert | Automatic Let's Encrypt certificate management |
| Password hashing | bcrypt | Industry standard, in stdlib.org/x |

### Go Module Dependencies

```
github.com/go-chi/chi/v5
github.com/yuin/goldmark
modernc.org/sqlite
golang.org/x/crypto/bcrypt
golang.org/x/crypto/acme/autocert
```

Note: `modernc.org/sqlite` is a pure-Go SQLite implementation requiring no CGO or C compiler.

---

## Configuration

All configuration via environment variables. No config files.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `LEXICON_DOMAIN` | Yes* | — | Domain for Let's Encrypt (e.g., `wiki.example.com`) |
| `LEXICON_DATA_DIR` | No | `./data` | Directory for SQLite database and autocert cache |
| `LEXICON_SESSION_SECRET` | Yes | — | 32+ character secret for session signing |
| `LEXICON_ADMIN_EMAIL` | Yes* | — | Email for Let's Encrypt registration |
| `LEXICON_HTTP_MODE` | No | `false` | If `true`, run plain HTTP (for reverse proxy or dev) |
| `LEXICON_PORT` | No | `443`/`8080` | Port (defaults to 443 for HTTPS, 8080 for HTTP mode) |

*Required unless `LEXICON_HTTP_MODE=true`

### Server Modes

**Direct HTTPS (default)**: Handles TLS termination via Let's Encrypt autocert. Binds ports 80 (redirect) and 443.

**HTTP mode** (`LEXICON_HTTP_MODE=true`): Runs plain HTTP on a single port. Use this when:
- Running behind a reverse proxy (Caddy, nginx) that terminates TLS
- Local development

Example with Caddy reverse proxy:
```
wiki.example.com {
    reverse_proxy localhost:8080
}
```

### Startup Behavior

1. Load environment variables
2. Validate required config (fail fast with clear error messages)
3. Initialize database (create tables if not exist, run migrations)
4. Start server (HTTPS with autocert, or HTTP in HTTP mode)

---

## Data Model

### Schema

```sql
-- Users table
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin', 'user')),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Pages table
CREATE TABLE pages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT UNIQUE NOT NULL,
    title TEXT NOT NULL,
    is_phantom INTEGER NOT NULL DEFAULT 0,
    first_cited_by_user_id INTEGER REFERENCES users(id),
    first_cited_in_page_id INTEGER REFERENCES pages(id),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Revisions table (full content per revision)
CREATE TABLE revisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    page_id INTEGER NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    author_id INTEGER NOT NULL REFERENCES users(id),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Comments table
CREATE TABLE comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    page_id INTEGER NOT NULL REFERENCES pages(id) ON DELETE CASCADE,
    author_id INTEGER NOT NULL REFERENCES users(id),
    content TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Sessions table
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Settings table (key-value)
CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Full-text search virtual table
CREATE VIRTUAL TABLE pages_fts USING fts5(
    title,
    content,
    content='',
    contentless_delete=1
);

-- Index for common queries
CREATE INDEX idx_pages_is_phantom ON pages(is_phantom);
CREATE INDEX idx_revisions_page_id ON revisions(page_id);
CREATE INDEX idx_comments_page_id ON comments(page_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
```

### Settings Keys

| Key | Values | Default | Description |
|-----|--------|---------|-------------|
| `public_read_access` | `true`/`false` | `false` | Allow unauthenticated page viewing |
| `registration_enabled` | `true`/`false` | `false` | Allow new user registration |
| `registration_code` | string | empty | If non-empty, required to create new accounts |
| `wiki_title` | string | `Lexicon Wiki` | Displayed in header and title tags |

---

## URL Routes

### Public Routes (if public read enabled, otherwise redirect to login)

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Home page (recent entries, statistics) |
| GET | `/page/{slug}` | View page (or phantom placeholder) |
| GET | `/page/{slug}/history` | Revision history |
| GET | `/page/{slug}/revision/{revisionID}` | View specific revision |
| GET | `/pages` | Index of all written pages (alphabetical) |
| GET | `/pages/phantoms` | Index of all phantom pages |
| GET | `/pages/recent` | Recently modified pages |
| GET | `/search` | Search form |
| GET | `/search?q={query}` | Search results |

### Authentication Routes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/login` | Login form |
| POST | `/login` | Process login |
| POST | `/logout` | End session |
| GET | `/register` | Registration form (if enabled) |
| POST | `/register` | Process registration (validates passcode if configured) |

### Authenticated User Routes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/page/{slug}/edit` | Edit form (creates page if phantom/new) |
| POST | `/page/{slug}` | Save page content |
| GET | `/page/{slug}/comments` | View comments (also shown on page view) |
| POST | `/page/{slug}/comments` | Add comment |

### Admin Routes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/admin` | Admin dashboard |
| GET | `/admin/settings` | Settings form |
| POST | `/admin/settings` | Update settings |
| GET | `/admin/users` | User list |
| POST | `/admin/users/{userID}/role` | Change user role |
| POST | `/admin/users/{userID}/delete` | Delete user |
| GET | `/admin/export` | Trigger markdown export (downloads zip) |

---

## Wiki-Link Parsing

### Syntax

| Input | Target Slug | Display Text |
|-------|-------------|--------------|
| `[[Page Name]]` | `page-name` | `Page Name` |
| `[[Page Name\|The Page]]` | `page-name` | `The Page` |
| `[[The Battle of Foo]]` | `the-battle-of-foo` | `The Battle of Foo` |

### Slug Generation

```go
func Slugify(title string) string {
    // Lowercase
    // Replace spaces and underscores with hyphens
    // Remove all characters except a-z, 0-9, hyphen
    // Collapse multiple hyphens
    // Trim leading/trailing hyphens
}
```

### Goldmark Extension

Implement a custom Goldmark parser extension:

1. **Inline parser**: Detect `[[` and `]]` delimiters
2. **AST node**: Create a `WikiLink` node type with `Target` and `DisplayText` fields
3. **Renderer**: Output appropriate HTML based on link state

```go
// WikiLink AST node
type WikiLink struct {
    ast.BaseInline
    Target      string // The slugified page reference
    DisplayText string // What to show the user
}

// Renderer output examples:
// Existing page: <a href="/page/foo" class="wiki-link">Foo</a>
// Phantom page:  <a href="/page/foo" class="wiki-link phantom">Foo</a>
// Note: renderer needs access to page existence lookup
```

### Link Extraction on Save

When saving a page revision:

1. Parse content with Goldmark
2. Walk AST to collect all WikiLink nodes
3. For each target slug:
   - If page exists: no action
   - If page doesn't exist: create phantom record (if not already phantom)
     - Set `first_cited_by_user_id` to current user
     - Set `first_cited_in_page_id` to current page
     - Only set these fields if this is the *first* citation (don't overwrite)

### Phantom-to-Page Transition

When a user edits a phantom page (creating real content):

1. Set `is_phantom = 0`
2. Create first revision with content
3. Preserve `first_cited_by_user_id` and `first_cited_in_page_id` for historical record
4. Update FTS index

---

## Search Implementation

### Indexing

On page save:
```sql
-- Delete old entry
DELETE FROM pages_fts WHERE rowid = ?;

-- Insert new entry (only for non-phantom pages)
INSERT INTO pages_fts (rowid, title, content) VALUES (?, ?, ?);
```

### Query

```sql
SELECT p.slug, p.title, snippet(pages_fts, 1, '<mark>', '</mark>', '...', 32) as snippet
FROM pages_fts
JOIN pages p ON pages_fts.rowid = p.id
WHERE pages_fts MATCH ?
ORDER BY rank
LIMIT 50;
```

Use FTS5 query syntax. Sanitize user input by escaping special characters or using phrase queries.

---

## Authentication

### Password Handling

```go
import "golang.org/x/crypto/bcrypt"

// Hash on registration/password change
hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

// Verify on login
err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password))
```

### Registration Passcode

If `registration_code` setting is non-empty:

1. Registration form displays a "Passcode" field
2. On submit, compare submitted passcode to stored setting (constant-time comparison)
3. Reject registration with generic error if passcode doesn't match

```go
regCode, _ := db.GetSetting("registration_code")
if regCode != "" {
    if subtle.ConstantTimeCompare([]byte(formCode), []byte(regCode)) != 1 {
        // Return generic "registration failed" error
        // Don't reveal whether username was taken vs wrong passcode
    }
}
```

Admins can update the passcode at any time via Admin > Settings without restarting the server.

### Sessions

- Generate 32-byte random session ID using `crypto/rand`
- Store in `sessions` table with expiration (30 days from creation)
- Set HTTP-only, Secure (in production), SameSite=Lax cookie
- Clean expired sessions periodically (on each request, or on timer)

### Middleware Stack

```go
r := chi.NewRouter()

// Global middleware
r.Use(middleware.RealIP)
r.Use(middleware.Logger)
r.Use(middleware.Recoverer)
r.Use(SessionMiddleware)      // Populates context with user if logged in
r.Use(PublicAccessMiddleware) // Checks public_read_access setting

// Protected routes
r.Group(func(r chi.Router) {
    r.Use(RequireAuth)
    // ... user routes
})

r.Group(func(r chi.Router) {
    r.Use(RequireAdmin)
    // ... admin routes
})
```

---

## Templates

### Structure

```
templates/
├── layout.html          # Base layout with Bulma structure
├── home.html            # Landing page
├── page/
│   ├── view.html        # Page display with rendered markdown
│   ├── edit.html        # Edit form
│   ├── history.html     # Revision list
│   └── revision.html    # Single revision view
├── pages/
│   ├── index.html       # All pages listing
│   ├── phantoms.html    # Phantom pages listing
│   └── recent.html      # Recent changes
├── search.html          # Search form and results
├── auth/
│   ├── login.html
│   └── register.html
├── admin/
│   ├── dashboard.html
│   ├── settings.html
│   └── users.html
└── partials/
    ├── nav.html         # Navigation component
    ├── flash.html       # Flash message display
    └── comments.html    # Comment thread component
```

### Base Layout Pattern

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>{{.Title}} - {{.WikiTitle}}</title>
    <link rel="stylesheet" href="/static/bulma.min.css">
    <link rel="stylesheet" href="/static/lexicon.css">
</head>
<body>
    <nav class="navbar" role="navigation">
        {{template "nav" .}}
    </nav>
    
    <main class="section">
        <div class="container">
            {{template "flash" .}}
            {{template "content" .}}
        </div>
    </main>
    
    <footer class="footer">
        <div class="content has-text-centered">
            <p>Powered by Lexicon Wiki</p>
        </div>
    </footer>
</body>
</html>
```

### Custom CSS (`lexicon.css`)

Minimal additions to Bulma:

```css
/* Wiki link styling */
a.wiki-link {
    /* Normal links: default Bulma styling */
}

a.wiki-link.phantom {
    color: #c0392b;  /* Red for unwritten entries */
}

a.wiki-link.phantom:hover {
    color: #e74c3c;
}

/* Page content styling */
.page-content {
    line-height: 1.7;
}

.page-content h1,
.page-content h2,
.page-content h3 {
    margin-top: 1.5em;
    margin-bottom: 0.5em;
}

/* Revision diff could use background highlighting */
.revision-content {
    background: #f8f9fa;
    padding: 1rem;
    border-radius: 4px;
}
```

---

## Static Assets

Embed static assets in binary using Go 1.16+ embed directive:

```go
//go:embed static templates
var embeddedFS embed.FS
```

### Filesystem Overlay

Use an overlay filesystem that checks local disk first, then falls back to embedded:

```go
type OverlayFS struct {
    disk     string // local directory path (e.g., "./templates")
    embedded fs.FS
}

func (o OverlayFS) Open(name string) (fs.File, error) {
    if o.disk != "" {
        path := filepath.Join(o.disk, name)
        if f, err := os.Open(path); err == nil {
            return f, nil
        }
    }
    return o.embedded.Open(name)
}
```

Apply this to both `templates/` and `static/` directories. This allows runtime template or CSS edits without rebuilding — place a modified file at `./templates/page/view.html` relative to the working directory and it takes precedence.

If no local files exist, behavior is identical to pure embedded. The binary remains fully self-contained and portable.

### Static Files

```
static/
├── bulma.min.css    # Bulma CSS (vendor, specific version)
├── lexicon.css      # Custom styles
└── favicon.ico
```

Download and vendor Bulma rather than using CDN (allows offline operation).

---

## Page View Logic

```go
func (h *Handler) ViewPage(w http.ResponseWriter, r *http.Request) {
    slug := chi.URLParam(r, "slug")
    
    page, err := h.db.GetPageBySlug(slug)
    if err == ErrNotFound {
        // Check if it's a valid slug format for potential creation
        http.Error(w, "Page not found", 404)
        return
    }
    
    if page.IsPhantom {
        // Render phantom template showing:
        // - This page hasn't been written yet
        // - First cited by [user] in [[page]]
        // - [Write this entry] button (if logged in)
        h.renderPhantom(w, r, page)
        return
    }
    
    // Get current revision
    revision, _ := h.db.GetCurrentRevision(page.ID)
    
    // Render markdown to HTML
    // Note: renderer needs page existence map for link styling
    html := h.renderMarkdown(revision.Content)
    
    // Get comments
    comments, _ := h.db.GetComments(page.ID)
    
    h.render(w, "page/view.html", map[string]any{
        "Page":     page,
        "Content":  template.HTML(html),
        "Revision": revision,
        "Comments": comments,
    })
}
```

---

## Export Functionality

Admin-only feature to dump all pages as markdown files.

### Export Format

```
export/
├── pages/
│   ├── the-first-age.md
│   ├── the-battle-of-foo.md
│   └── ...
└── metadata.json
```

### Page File Format

```markdown
---
title: The First Age
slug: the-first-age
created: 2024-01-15T10:30:00Z
updated: 2024-01-20T14:22:00Z
author: username
revisions: 5
---

The First Age began when the world was young...

[[The Elder Gods]] walked among mortals in those days...
```

### metadata.json

```json
{
  "exported_at": "2024-01-21T12:00:00Z",
  "wiki_title": "The Chronicles of Elsewhere",
  "total_pages": 47,
  "total_phantoms": 12,
  "phantoms": [
    {
      "slug": "the-elder-gods",
      "title": "The Elder Gods",
      "first_cited_by": "alice",
      "first_cited_in": "the-first-age"
    }
  ]
}
```

### Implementation

```go
func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
    // Create zip in memory or temp file
    // Add all pages as .md files with frontmatter
    // Add metadata.json
    // Set headers for download
    w.Header().Set("Content-Type", "application/zip")
    w.Header().Set("Content-Disposition", 
        `attachment; filename="lexicon-export.zip"`)
    // Write zip to response
}
```

---

## Project Structure

```
lexicon/
├── cmd/
│   └── lexicon/
│       └── main.go           # Entry point, config loading
├── internal/
│   ├── config/
│   │   └── config.go         # Environment variable parsing
│   ├── database/
│   │   ├── database.go       # Connection, migrations
│   │   ├── pages.go          # Page CRUD operations
│   │   ├── users.go          # User CRUD operations
│   │   ├── revisions.go      # Revision operations
│   │   ├── comments.go       # Comment operations
│   │   ├── sessions.go       # Session management
│   │   └── settings.go       # Settings key-value store
│   ├── handler/
│   │   ├── handler.go        # Handler struct, shared utilities
│   │   ├── pages.go          # Page view, edit, save handlers
│   │   ├── auth.go           # Login, logout, register handlers
│   │   ├── search.go         # Search handler
│   │   ├── admin.go          # Admin handlers
│   │   └── export.go         # Export handler
│   ├── markdown/
│   │   ├── markdown.go       # Goldmark setup
│   │   └── wikilink/
│   │       ├── ast.go        # WikiLink AST node
│   │       ├── parser.go     # WikiLink parser
│   │       └── renderer.go   # WikiLink HTML renderer
│   ├── middleware/
│   │   ├── auth.go           # Session, RequireAuth, RequireAdmin
│   │   └── access.go         # Public access control
│   └── server/
│       └── server.go         # HTTP server setup, TLS config
├── templates/                 # HTML templates (see Templates section)
├── static/                    # CSS, favicon (see Static Assets section)
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## First Run / Setup

On first run with empty database:

1. Create all tables
2. Insert default settings
3. Prompt for admin account creation OR use environment variables:
   - `LEXICON_ADMIN_USERNAME`
   - `LEXICON_ADMIN_PASSWORD`

If neither interactive prompt nor environment variables are available, fail with clear instructions.

### Interactive Setup (if stdin is terminal)

```
Lexicon Wiki - First Time Setup
================================
No admin user found. Please create one.

Admin username: █
Admin password: 
Confirm password: 

Admin user created. Starting server...
```

---

## Error Handling

### User-Facing Errors

- Use flash messages for action feedback (stored in session)
- Render friendly error pages for 404, 403, 500
- Never expose stack traces or internal details

### Flash Messages

Store in session, clear after display:

```go
type Flash struct {
    Type    string // "success", "warning", "danger", "info"
    Message string
}
```

Display using Bulma notification component:

```html
{{range .Flashes}}
<div class="notification is-{{.Type}}">
    <button class="delete"></button>
    {{.Message}}
</div>
{{end}}
```

Note: The delete button requires JS to dismiss. Without JS, messages simply remain visible (acceptable tradeoff).

---

## Security Considerations

### Input Validation

- Slugs: Alphanumeric and hyphens only, max 200 characters
- Titles: Max 500 characters
- Page content: Max 500KB
- Comments: Max 10KB
- Usernames: Alphanumeric, 3-50 characters
- Passwords: Minimum 8 characters
- Registration passcode: Constant-time comparison against stored setting, no length hints in errors

### HTML Sanitization

Goldmark with `goldmark-unsafe` disabled (default) does not allow raw HTML in markdown. This is the desired behavior — no user-supplied HTML.

### CSRF Protection

For state-changing operations (POST), include a CSRF token:

```go
// Generate token (store in session)
token := base64.URLEncoding.EncodeToString(randomBytes(32))

// Include in forms
<input type="hidden" name="csrf_token" value="{{.CSRFToken}}">

// Validate in handler
if r.FormValue("csrf_token") != session.CSRFToken {
    http.Error(w, "Invalid request", 403)
    return
}
```

### Rate Limiting

Basic rate limiting on authentication endpoints:

- Login: 5 attempts per minute per IP
- Registration: 3 attempts per hour per IP

Use in-memory counter with cleanup (not worth external dependency for this scale).

---

## Operations

### Building

```bash
go build -o lexicon ./cmd/lexicon
```

No CGO required. Cross-compilation works normally.

### Running

**Development / behind reverse proxy:**
```bash
LEXICON_HTTP_MODE=true \
LEXICON_SESSION_SECRET=your-secret-here \
./lexicon
```

**Direct HTTPS:**
```bash
LEXICON_DOMAIN=wiki.example.com \
LEXICON_ADMIN_EMAIL=admin@example.com \
LEXICON_SESSION_SECRET=your-secret-here \
./lexicon
```

On first run, you'll be prompted to create an admin account (or set `LEXICON_ADMIN_USERNAME` and `LEXICON_ADMIN_PASSWORD` environment variables).

### Data

All persistent data lives in `LEXICON_DATA_DIR` (default `./data`):
- `lexicon.db` — SQLite database
- `autocert/` — Let's Encrypt certificate cache (if using direct HTTPS)

---

## README.md Template

The repository should include a README covering:

```markdown
# Lexicon Wiki

A purpose-built wiki for LexiconRPG games.

## Features

- Markdown editing with `[[wiki-links]]`
- Phantom link tracking for unwritten entries
- Revision history
- Per-page comments
- Full-text search
- Automatic HTTPS via Let's Encrypt (or run behind reverse proxy)
- Single-binary deployment — no external files required, no CGO

## Quick Start

\`\`\`bash
go build -o lexicon ./cmd/lexicon

LEXICON_HTTP_MODE=true \
LEXICON_SESSION_SECRET=change-me \
./lexicon
\`\`\`

Opens at http://localhost:8080. Create admin account on first run.

The compiled binary is fully self-contained (templates and CSS are embedded). Copy it anywhere and run. To customize templates or styles without rebuilding, place modified files in `./templates/` or `./static/` relative to the working directory.

## Configuration

| Variable | Required | Description |
|----------|----------|-------------|
| `LEXICON_SESSION_SECRET` | Yes | Random secret (32+ chars) |
| `LEXICON_DOMAIN` | HTTPS only | Domain for TLS certificate |
| `LEXICON_ADMIN_EMAIL` | HTTPS only | Email for Let's Encrypt |
| `LEXICON_HTTP_MODE` | No | Run plain HTTP (for reverse proxy) |
| `LEXICON_PORT` | No | Port (default: 443 or 8080) |
| `LEXICON_DATA_DIR` | No | Data directory (default: ./data) |

## Restricting Registration

In Admin > Settings, set a **Registration Code**. Users must enter this passcode to create accounts. Share the code with your players out-of-band. Change it anytime without restarting.

## Export

Admins can export all content as markdown files via Admin > Export.

## License

[Your chosen license]
```

---

## Testing Expectations

Unit tests for core logic only. Keep test surface minimal.

### Required Unit Tests

- `internal/markdown/wikilink`: Parser extracts link targets and display text correctly
- `internal/database/pages.go`: Slug generation handles edge cases (spaces, special chars, unicode)
- `internal/middleware/auth.go`: Session validation logic

### Not Required

- Integration tests
- HTTP handler tests
- End-to-end tests

Manual verification during development is sufficient for handler behavior.

---

## Future Considerations

Not in scope for initial implementation, but noted for potential future work:

- **Turn tracking**: Track game rounds, enforce citation rules
- **Scholar personas**: Separate in-fiction author from player account
- **Entry claiming**: Reserve phantom entries before writing
- **Diff view**: Side-by-side revision comparison
- **Image uploads**: Attach images to pages
- **API**: JSON endpoints for external tools
- **Import**: Restore from export zip
