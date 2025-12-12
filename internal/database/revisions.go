package database

import (
	"database/sql"
	"time"
)

// Revision represents a page revision.
type Revision struct {
	ID        int64
	PageID    int64
	Content   string
	AuthorID  int64
	CreatedAt time.Time

	// Joined fields (not always populated)
	AuthorUsername string
}

// GetCurrentRevision returns the most recent revision for a page.
func (db *DB) GetCurrentRevision(pageID int64) (*Revision, error) {
	rev := &Revision{}
	err := db.QueryRow(`
		SELECT r.id, r.page_id, r.content, r.author_id, r.created_at, u.username
		FROM revisions r
		JOIN users u ON r.author_id = u.id
		WHERE r.page_id = ?
		ORDER BY r.created_at DESC
		LIMIT 1
	`, pageID).Scan(&rev.ID, &rev.PageID, &rev.Content, &rev.AuthorID, &rev.CreatedAt, &rev.AuthorUsername)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return rev, nil
}

// GetRevisionByID returns a specific revision.
func (db *DB) GetRevisionByID(revisionID int64) (*Revision, error) {
	rev := &Revision{}
	err := db.QueryRow(`
		SELECT r.id, r.page_id, r.content, r.author_id, r.created_at, u.username
		FROM revisions r
		JOIN users u ON r.author_id = u.id
		WHERE r.id = ?
	`, revisionID).Scan(&rev.ID, &rev.PageID, &rev.Content, &rev.AuthorID, &rev.CreatedAt, &rev.AuthorUsername)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return rev, nil
}

// ListRevisions returns all revisions for a page, newest first.
func (db *DB) ListRevisions(pageID int64) ([]*Revision, error) {
	rows, err := db.Query(`
		SELECT r.id, r.page_id, r.content, r.author_id, r.created_at, u.username
		FROM revisions r
		JOIN users u ON r.author_id = u.id
		WHERE r.page_id = ?
		ORDER BY r.created_at DESC
	`, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var revisions []*Revision
	for rows.Next() {
		rev := &Revision{}
		err := rows.Scan(&rev.ID, &rev.PageID, &rev.Content, &rev.AuthorID, &rev.CreatedAt, &rev.AuthorUsername)
		if err != nil {
			return nil, err
		}
		revisions = append(revisions, rev)
	}
	return revisions, rows.Err()
}

// RevisionCount returns the number of revisions for a page.
func (db *DB) RevisionCount(pageID int64) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM revisions WHERE page_id = ?", pageID).Scan(&count)
	return count, err
}
