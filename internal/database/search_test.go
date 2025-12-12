package database

import (
	"os"
	"testing"
)

func TestSearch(t *testing.T) {
	// Create temporary database
	tmpFile, err := os.CreateTemp("", "lexicon-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Create a test user
	user, err := db.CreateUser("testuser", "password123", "user")
	if err != nil {
		t.Fatal(err)
	}

	// Create some test pages
	page1, err := db.CreatePage("the-first-age", "The First Age", "This is content about the first age of the world.", user.ID)
	if err != nil {
		t.Fatalf("Failed to create page1: %v", err)
	}
	t.Logf("Created page1: ID=%d, slug=%s", page1.ID, page1.Slug)

	page2, err := db.CreatePage("dragons", "Dragons", "Dragons are mythical creatures that breathe fire.", user.ID)
	if err != nil {
		t.Fatalf("Failed to create page2: %v", err)
	}
	t.Logf("Created page2: ID=%d, slug=%s", page2.ID, page2.Slug)

	page3, err := db.CreatePage("the-war", "The War", "The great war happened in the first age.", user.ID)
	if err != nil {
		t.Fatalf("Failed to create page3: %v", err)
	}
	t.Logf("Created page3: ID=%d, slug=%s", page3.ID, page3.Slug)

	// Debug: Check FTS table contents
	rows, err := db.Query("SELECT rowid, title, content FROM pages_fts")
	if err != nil {
		t.Logf("Error querying FTS table: %v", err)
	} else {
		defer rows.Close()
		t.Log("FTS table contents:")
		for rows.Next() {
			var rowid int64
			var title, content string
			rows.Scan(&rowid, &title, &content)
			t.Logf("  rowid=%d, title=%q, content=%q", rowid, title, content)
		}
	}

	// Test searches
	tests := []struct {
		query       string
		wantResults int
		wantSlugs   []string
	}{
		{"first", 2, []string{"the-first-age", "the-war"}},
		{"dragons", 1, []string{"dragons"}},
		{"fire", 1, []string{"dragons"}},
		{"age", 2, []string{"the-first-age", "the-war"}},
		{"nonexistent", 0, nil},

		// Stemming tests - porter stemmer should match word variations
		{"dragon", 1, []string{"dragons"}},           // singular matches plural
		{"creature", 1, []string{"dragons"}},         // singular matches plural "creatures"
		{"mythical", 1, []string{"dragons"}},         // exact match still works
		{"breathing", 1, []string{"dragons"}},        // "breathing" stems to "breath" matches "breathe"
		{"happened", 1, []string{"the-war"}},         // past tense matches
		{"wars", 1, []string{"the-war"}},             // plural matches singular "war"

		// Prefix search tests
		{"drag*", 1, []string{"dragons"}},            // prefix matches "dragons"
		{"fir*", 3, []string{"the-first-age", "the-war", "dragons"}}, // matches "first" and "fire"

		// Multiple terms (AND logic)
		{"great war", 1, []string{"the-war"}},        // both terms required
		{"first age", 2, []string{"the-first-age", "the-war"}}, // both in two docs
	}

	for _, tt := range tests {
		t.Run("search_"+tt.query, func(t *testing.T) {
			results, err := db.Search(tt.query, 50)
			if err != nil {
				t.Fatalf("Search(%q) error: %v", tt.query, err)
			}

			t.Logf("Search(%q) returned %d results", tt.query, len(results))
			for _, r := range results {
				t.Logf("  - %s: %s | snippet: %q", r.Slug, r.Title, r.Snippet)
			}

			if len(results) != tt.wantResults {
				t.Errorf("Search(%q) got %d results, want %d", tt.query, len(results), tt.wantResults)
			}

			if tt.wantSlugs != nil {
				for _, wantSlug := range tt.wantSlugs {
					found := false
					for _, r := range results {
						if r.Slug == wantSlug {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Search(%q) missing expected slug %q", tt.query, wantSlug)
					}
				}
			}
		})
	}
}
