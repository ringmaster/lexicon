package handler

import (
	"net/http"
)

// Search handles search requests.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	var results []*struct {
		Slug    string
		Title   string
		Snippet string
	}

	if query != "" {
		dbResults, err := h.DB.Search(query, 50)
		if err == nil {
			for _, r := range dbResults {
				results = append(results, &struct {
					Slug    string
					Title   string
					Snippet string
				}{
					Slug:    r.Slug,
					Title:   r.Title,
					Snippet: r.Snippet,
				})
			}
		}
	}

	h.Render(w, r, "search.html", "Search", map[string]any{
		"Query":   query,
		"Results": results,
	})
}
