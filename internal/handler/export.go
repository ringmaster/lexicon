package handler

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Export generates a ZIP file with all pages as markdown.
func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	wikiTitle, _ := h.DB.WikiTitle()

	// Set headers for ZIP download
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="lexicon-export.zip"`)

	// Create ZIP writer
	zw := zip.NewWriter(w)
	defer zw.Close()

	// Get all non-phantom pages
	pages, err := h.DB.ListPages()
	if err != nil {
		http.Error(w, "Export failed", http.StatusInternalServerError)
		return
	}

	// Get all phantoms
	phantoms, err := h.DB.ListPhantoms()
	if err != nil {
		http.Error(w, "Export failed", http.StatusInternalServerError)
		return
	}

	// Export each page
	for _, page := range pages {
		rev, err := h.DB.GetCurrentRevision(page.ID)
		if err != nil {
			continue
		}

		revCount, _ := h.DB.RevisionCount(page.ID)

		content := fmt.Sprintf(`---
title: %s
slug: %s
created: %s
updated: %s
author: %s
revisions: %d
---

%s
`,
			page.Title,
			page.Slug,
			page.CreatedAt.Format(time.RFC3339),
			page.UpdatedAt.Format(time.RFC3339),
			rev.AuthorUsername,
			revCount,
			rev.Content,
		)

		f, err := zw.Create("pages/" + page.Slug + ".md")
		if err != nil {
			continue
		}
		f.Write([]byte(content))
	}

	// Create metadata.json
	metadata := struct {
		ExportedAt    string `json:"exported_at"`
		WikiTitle     string `json:"wiki_title"`
		TotalPages    int    `json:"total_pages"`
		TotalPhantoms int    `json:"total_phantoms"`
		Phantoms      []struct {
			Slug          string `json:"slug"`
			Title         string `json:"title"`
			FirstCitedBy  string `json:"first_cited_by"`
			FirstCitedIn  string `json:"first_cited_in"`
		} `json:"phantoms"`
	}{
		ExportedAt:    time.Now().Format(time.RFC3339),
		WikiTitle:     wikiTitle,
		TotalPages:    len(pages),
		TotalPhantoms: len(phantoms),
	}

	for _, phantom := range phantoms {
		var citedBy, citedIn string
		if phantom.FirstCitedByUserID != nil {
			if user, err := h.DB.GetUserByID(*phantom.FirstCitedByUserID); err == nil {
				citedBy = user.Username
			}
		}
		if phantom.FirstCitedInPageID != nil {
			if page, err := h.DB.GetPageByID(*phantom.FirstCitedInPageID); err == nil {
				citedIn = page.Slug
			}
		}

		metadata.Phantoms = append(metadata.Phantoms, struct {
			Slug          string `json:"slug"`
			Title         string `json:"title"`
			FirstCitedBy  string `json:"first_cited_by"`
			FirstCitedIn  string `json:"first_cited_in"`
		}{
			Slug:         phantom.Slug,
			Title:        phantom.Title,
			FirstCitedBy: citedBy,
			FirstCitedIn: citedIn,
		})
	}

	metaJSON, _ := json.MarshalIndent(metadata, "", "  ")
	f, err := zw.Create("metadata.json")
	if err == nil {
		f.Write(metaJSON)
	}
}
