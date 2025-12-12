package database

import (
	"database/sql"
	"time"
)

// Comment represents a comment on a page.
type Comment struct {
	ID        int64
	PageID    int64
	AuthorID  int64
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time

	// Joined fields (not always populated)
	AuthorUsername string
}

// CreateComment adds a comment to a page.
func (db *DB) CreateComment(pageID, authorID int64, content string) (*Comment, error) {
	now := time.Now()
	result, err := db.Exec(`
		INSERT INTO comments (page_id, author_id, content, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, pageID, authorID, content, now, now)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return db.GetCommentByID(id)
}

// GetCommentByID retrieves a comment by ID.
func (db *DB) GetCommentByID(id int64) (*Comment, error) {
	comment := &Comment{}
	err := db.QueryRow(`
		SELECT c.id, c.page_id, c.author_id, c.content, c.created_at, c.updated_at, u.username
		FROM comments c
		JOIN users u ON c.author_id = u.id
		WHERE c.id = ?
	`, id).Scan(
		&comment.ID, &comment.PageID, &comment.AuthorID, &comment.Content,
		&comment.CreatedAt, &comment.UpdatedAt, &comment.AuthorUsername,
	)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return comment, nil
}

// ListComments returns all comments for a page, oldest first.
func (db *DB) ListComments(pageID int64) ([]*Comment, error) {
	rows, err := db.Query(`
		SELECT c.id, c.page_id, c.author_id, c.content, c.created_at, c.updated_at, u.username
		FROM comments c
		JOIN users u ON c.author_id = u.id
		WHERE c.page_id = ?
		ORDER BY c.created_at ASC
	`, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []*Comment
	for rows.Next() {
		comment := &Comment{}
		err := rows.Scan(
			&comment.ID, &comment.PageID, &comment.AuthorID, &comment.Content,
			&comment.CreatedAt, &comment.UpdatedAt, &comment.AuthorUsername,
		)
		if err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}
	return comments, rows.Err()
}

// UpdateComment modifies a comment's content.
func (db *DB) UpdateComment(commentID int64, content string) error {
	_, err := db.Exec(`
		UPDATE comments SET content = ?, updated_at = ? WHERE id = ?
	`, content, time.Now(), commentID)
	return err
}

// DeleteComment removes a comment.
func (db *DB) DeleteComment(commentID int64) error {
	_, err := db.Exec("DELETE FROM comments WHERE id = ?", commentID)
	return err
}

// CommentCount returns the number of comments for a page.
func (db *DB) CommentCount(pageID int64) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM comments WHERE page_id = ?", pageID).Scan(&count)
	return count, err
}
