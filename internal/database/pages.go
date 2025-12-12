package database

import (
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var (
	ErrNotFound = errors.New("not found")
)

// Page represents a wiki page.
type Page struct {
	ID                 int64
	Slug               string
	Title              string
	IsPhantom          bool
	FirstCitedByUserID *int64
	FirstCitedInPageID *int64
	DeletedAt          *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Slugify converts a title into a URL-safe slug.
func Slugify(title string) string {
	// Normalize unicode
	s := norm.NFKD.String(title)

	// Lowercase
	s = strings.ToLower(s)

	// Replace spaces and underscores with hyphens
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")

	// Remove all characters except a-z, 0-9, hyphen
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		} else if unicode.Is(unicode.Mn, r) {
			// Skip combining marks (accents) from normalization
			continue
		}
	}
	s = result.String()

	// Collapse multiple hyphens
	re := regexp.MustCompile(`-+`)
	s = re.ReplaceAllString(s, "-")

	// Trim leading/trailing hyphens
	s = strings.Trim(s, "-")

	return s
}

// GetPageBySlug retrieves a page by its slug.
func (db *DB) GetPageBySlug(slug string) (*Page, error) {
	page := &Page{}
	err := db.QueryRow(`
		SELECT id, slug, title, is_phantom, first_cited_by_user_id, first_cited_in_page_id, deleted_at, created_at, updated_at
		FROM pages WHERE slug = ?
	`, slug).Scan(
		&page.ID, &page.Slug, &page.Title, &page.IsPhantom,
		&page.FirstCitedByUserID, &page.FirstCitedInPageID,
		&page.DeletedAt, &page.CreatedAt, &page.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return page, nil
}

// GetPageByID retrieves a page by its ID.
func (db *DB) GetPageByID(id int64) (*Page, error) {
	page := &Page{}
	err := db.QueryRow(`
		SELECT id, slug, title, is_phantom, first_cited_by_user_id, first_cited_in_page_id, deleted_at, created_at, updated_at
		FROM pages WHERE id = ?
	`, id).Scan(
		&page.ID, &page.Slug, &page.Title, &page.IsPhantom,
		&page.FirstCitedByUserID, &page.FirstCitedInPageID,
		&page.DeletedAt, &page.CreatedAt, &page.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return page, nil
}

// CreatePage creates a new page (non-phantom with content).
func (db *DB) CreatePage(slug, title, content string, authorID int64) (*Page, error) {
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Check if phantom exists
	var existingID int64
	var isPhantom bool
	err = tx.QueryRow("SELECT id, is_phantom FROM pages WHERE slug = ?", slug).Scan(&existingID, &isPhantom)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	var pageID int64
	now := time.Now()

	if err == sql.ErrNoRows {
		// Create new page
		result, err := tx.Exec(`
			INSERT INTO pages (slug, title, is_phantom, created_at, updated_at)
			VALUES (?, ?, 0, ?, ?)
		`, slug, title, now, now)
		if err != nil {
			return nil, err
		}
		pageID, _ = result.LastInsertId()
	} else if isPhantom {
		// Convert phantom to real page
		_, err = tx.Exec(`
			UPDATE pages SET title = ?, is_phantom = 0, updated_at = ?
			WHERE id = ?
		`, title, now, existingID)
		if err != nil {
			return nil, err
		}
		pageID = existingID
	} else {
		return nil, errors.New("page already exists")
	}

	// Create first revision
	_, err = tx.Exec(`
		INSERT INTO revisions (page_id, content, author_id, created_at)
		VALUES (?, ?, ?, ?)
	`, pageID, content, authorID, now)
	if err != nil {
		return nil, err
	}

	// Update FTS index
	_, err = tx.Exec(`
		INSERT INTO pages_fts (rowid, title, content) VALUES (?, ?, ?)
	`, pageID, title, content)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return db.GetPageByID(pageID)
}

// UpdatePage adds a new revision to an existing page.
func (db *DB) UpdatePage(pageID int64, title, content string, authorID int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now()

	// Update page metadata
	_, err = tx.Exec(`
		UPDATE pages SET title = ?, updated_at = ? WHERE id = ?
	`, title, now, pageID)
	if err != nil {
		return err
	}

	// Create new revision
	_, err = tx.Exec(`
		INSERT INTO revisions (page_id, content, author_id, created_at)
		VALUES (?, ?, ?, ?)
	`, pageID, content, authorID, now)
	if err != nil {
		return err
	}

	// Update FTS index
	_, err = tx.Exec(`DELETE FROM pages_fts WHERE rowid = ?`, pageID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
		INSERT INTO pages_fts (rowid, title, content) VALUES (?, ?, ?)
	`, pageID, title, content)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// CreatePhantom creates a phantom page entry.
func (db *DB) CreatePhantom(slug, title string, citedByUserID, citedInPageID int64) (*Page, error) {
	// Check if page already exists (as phantom or real)
	existing, err := db.GetPageBySlug(slug)
	if err == nil {
		return existing, nil // Already exists, return it
	}
	if err != ErrNotFound {
		return nil, err
	}

	now := time.Now()
	result, err := db.Exec(`
		INSERT INTO pages (slug, title, is_phantom, first_cited_by_user_id, first_cited_in_page_id, created_at, updated_at)
		VALUES (?, ?, 1, ?, ?, ?, ?)
	`, slug, title, citedByUserID, citedInPageID, now, now)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return db.GetPageByID(id)
}

// PageExists checks if a page exists (phantom or not).
func (db *DB) PageExists(slug string) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM pages WHERE slug = ?", slug).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// IsPhantom checks if a page is a phantom.
func (db *DB) IsPhantom(slug string) (bool, error) {
	var isPhantom bool
	err := db.QueryRow("SELECT is_phantom FROM pages WHERE slug = ?", slug).Scan(&isPhantom)
	if err == sql.ErrNoRows {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	return isPhantom, nil
}

// SoftDeletePage marks a page as deleted.
func (db *DB) SoftDeletePage(pageID int64) error {
	result, err := db.Exec(
		"UPDATE pages SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND deleted_at IS NULL",
		pageID,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}

	// Remove from FTS index
	db.Exec("DELETE FROM pages_fts WHERE rowid = ?", pageID)

	return nil
}

// RestorePage restores a soft-deleted page.
func (db *DB) RestorePage(pageID int64) error {
	// Get the page to restore
	var title, content string
	err := db.QueryRow(`
		SELECT p.title, COALESCE(r.content, '')
		FROM pages p
		LEFT JOIN revisions r ON r.page_id = p.id
		WHERE p.id = ? AND p.deleted_at IS NOT NULL
		ORDER BY r.created_at DESC LIMIT 1
	`, pageID).Scan(&title, &content)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	// Restore the page
	_, err = db.Exec(
		"UPDATE pages SET deleted_at = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		pageID,
	)
	if err != nil {
		return err
	}

	// Re-add to FTS index
	db.Exec("INSERT INTO pages_fts (rowid, title, content) VALUES (?, ?, ?)", pageID, title, content)

	return nil
}

// ListDeletedPages returns all soft-deleted pages.
func (db *DB) ListDeletedPages() ([]*Page, error) {
	rows, err := db.Query(`
		SELECT id, slug, title, is_phantom, first_cited_by_user_id, first_cited_in_page_id, deleted_at, created_at, updated_at
		FROM pages WHERE deleted_at IS NOT NULL ORDER BY deleted_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanPages(rows)
}

// ListPages returns all non-phantom, non-deleted pages ordered alphabetically.
func (db *DB) ListPages() ([]*Page, error) {
	rows, err := db.Query(`
		SELECT id, slug, title, is_phantom, first_cited_by_user_id, first_cited_in_page_id, deleted_at, created_at, updated_at
		FROM pages WHERE is_phantom = 0 AND deleted_at IS NULL ORDER BY title ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanPages(rows)
}

// PhantomWithSource represents a phantom page with its source page info.
type PhantomWithSource struct {
	*Page
	SourceSlug  string
	SourceTitle string
}

// ListPhantoms returns all non-deleted phantom pages ordered alphabetically.
func (db *DB) ListPhantoms() ([]*Page, error) {
	rows, err := db.Query(`
		SELECT id, slug, title, is_phantom, first_cited_by_user_id, first_cited_in_page_id, deleted_at, created_at, updated_at
		FROM pages WHERE is_phantom = 1 AND deleted_at IS NULL ORDER BY title ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanPages(rows)
}

// ListPhantomsWithSource returns all non-deleted phantom pages with their source page info.
func (db *DB) ListPhantomsWithSource() ([]*PhantomWithSource, error) {
	rows, err := db.Query(`
		SELECT p.id, p.slug, p.title, p.is_phantom, p.first_cited_by_user_id, p.first_cited_in_page_id, p.deleted_at, p.created_at, p.updated_at,
		       COALESCE(src.slug, '') as source_slug, COALESCE(src.title, '') as source_title
		FROM pages p
		LEFT JOIN pages src ON p.first_cited_in_page_id = src.id
		WHERE p.is_phantom = 1 AND p.deleted_at IS NULL
		ORDER BY p.title ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var phantoms []*PhantomWithSource
	for rows.Next() {
		page := &Page{}
		phantom := &PhantomWithSource{Page: page}
		err := rows.Scan(
			&page.ID, &page.Slug, &page.Title, &page.IsPhantom,
			&page.FirstCitedByUserID, &page.FirstCitedInPageID,
			&page.DeletedAt, &page.CreatedAt, &page.UpdatedAt,
			&phantom.SourceSlug, &phantom.SourceTitle,
		)
		if err != nil {
			return nil, err
		}
		phantoms = append(phantoms, phantom)
	}
	return phantoms, rows.Err()
}

// ListRecentPages returns recently modified non-deleted pages.
func (db *DB) ListRecentPages(limit int) ([]*Page, error) {
	rows, err := db.Query(`
		SELECT id, slug, title, is_phantom, first_cited_by_user_id, first_cited_in_page_id, deleted_at, created_at, updated_at
		FROM pages WHERE is_phantom = 0 AND deleted_at IS NULL ORDER BY updated_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanPages(rows)
}

// PageStats returns counts of non-deleted pages and phantoms.
func (db *DB) PageStats() (pages, phantoms int, err error) {
	err = db.QueryRow("SELECT COUNT(*) FROM pages WHERE is_phantom = 0 AND deleted_at IS NULL").Scan(&pages)
	if err != nil {
		return
	}
	err = db.QueryRow("SELECT COUNT(*) FROM pages WHERE is_phantom = 1 AND deleted_at IS NULL").Scan(&phantoms)
	return
}

func scanPages(rows *sql.Rows) ([]*Page, error) {
	var pages []*Page
	for rows.Next() {
		page := &Page{}
		err := rows.Scan(
			&page.ID, &page.Slug, &page.Title, &page.IsPhantom,
			&page.FirstCitedByUserID, &page.FirstCitedInPageID,
			&page.DeletedAt, &page.CreatedAt, &page.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		pages = append(pages, page)
	}
	return pages, rows.Err()
}
