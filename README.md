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

```bash
go build -o lexicon ./cmd/lexicon

LEXICON_HTTP_MODE=true \
LEXICON_SESSION_SECRET=change-me-to-something-secure-32chars \
./lexicon
```

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
| `LEXICON_ADMIN_USERNAME` | No | Initial admin username (first run) |
| `LEXICON_ADMIN_PASSWORD` | No | Initial admin password (first run) |

## Server Modes

### Direct HTTPS (default)

Handles TLS termination via Let's Encrypt autocert. Binds ports 80 (redirect) and 443.

```bash
LEXICON_DOMAIN=wiki.example.com \
LEXICON_ADMIN_EMAIL=admin@example.com \
LEXICON_SESSION_SECRET=your-secret-here \
./lexicon
```

### HTTP Mode

Runs plain HTTP on a single port. Use when:
- Running behind a reverse proxy (Caddy, nginx) that terminates TLS
- Local development

```bash
LEXICON_HTTP_MODE=true \
LEXICON_SESSION_SECRET=your-secret-here \
./lexicon
```

Example with Caddy reverse proxy:
```
wiki.example.com {
    reverse_proxy localhost:8080
}
```

## Wiki Links

Link to other entries using wiki-link syntax:

- `[[Page Name]]` — links to "page-name", displays "Page Name"
- `[[Page Name|Display Text]]` — links to "page-name", displays "Display Text"

Links to unwritten entries appear in red and create "phantom" pages that track who first cited them.

## Restricting Registration

In Admin > Settings, set a **Registration Code**. Users must enter this passcode to create accounts. Share the code with your players out-of-band. Change it anytime without restarting.

## Export

Admins can export all content as markdown files via Admin > Export.

## Building

```bash
go build -o lexicon ./cmd/lexicon
```

No CGO required. Cross-compilation works normally.

## Data

All persistent data lives in `LEXICON_DATA_DIR` (default `./data`):
- `lexicon.db` — SQLite database
- `autocert/` — Let's Encrypt certificate cache (if using direct HTTPS)

## License

MIT
