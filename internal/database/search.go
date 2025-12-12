package database

import (
	"strings"
)

// SearchResult represents a single search result.
type SearchResult struct {
	Slug    string
	Title   string
	Snippet string
}

// Search performs a full-text search on pages.
func (db *DB) Search(query string, limit int) ([]*SearchResult, error) {
	// Sanitize query for FTS5
	sanitized := sanitizeFTSQuery(query)
	if sanitized == "" {
		return nil, nil
	}

	rows, err := db.Query(`
		SELECT p.slug, p.title, COALESCE(snippet(pages_fts, 1, '<mark>', '</mark>', '...', 32), '') as snippet
		FROM pages_fts
		JOIN pages p ON pages_fts.rowid = p.id
		WHERE pages_fts MATCH ? AND p.deleted_at IS NULL
		ORDER BY rank
		LIMIT ?
	`, sanitized, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*SearchResult
	for rows.Next() {
		result := &SearchResult{}
		if err := rows.Scan(&result.Slug, &result.Title, &result.Snippet); err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return results, rows.Err()
}

// sanitizeFTSQuery prepares a query string for FTS5.
// Supports:
//   - Simple words: "dragon" matches "dragon", "dragons", "dragonfly" (with stemming)
//   - Prefix matching: "drag*" matches anything starting with "drag"
//   - Multiple terms: "dragon fire" requires both terms (AND)
func sanitizeFTSQuery(query string) string {
	// Remove most FTS5 special characters but preserve * for prefix search
	replacer := strings.NewReplacer(
		`"`, ``,
		`^`, ``,
		`:`, ``,
		`(`, ``,
		`)`, ``,
		`{`, ``,
		`}`, ``,
		`[`, ``,
		`]`, ``,
	)
	sanitized := replacer.Replace(query)

	// Split into words
	words := strings.Fields(sanitized)
	if len(words) == 0 {
		return ""
	}

	// Process each word
	var terms []string
	for _, word := range words {
		word = strings.TrimSpace(word)
		if word == "" || word == "*" || word == "-" {
			continue
		}

		// Check if it's a prefix search (ends with *)
		if strings.HasSuffix(word, "*") {
			// Prefix search: remove quotes, keep the *
			base := strings.TrimSuffix(word, "*")
			base = strings.Trim(base, "-")
			if base != "" {
				terms = append(terms, base+"*")
			}
		} else {
			// Regular word: quote it for exact token matching (stemming still applies)
			word = strings.Trim(word, "-")
			if word != "" {
				terms = append(terms, `"`+word+`"`)
			}
		}
	}

	if len(terms) == 0 {
		return ""
	}

	// Join with space (implicit AND in FTS5)
	return strings.Join(terms, " ")
}
