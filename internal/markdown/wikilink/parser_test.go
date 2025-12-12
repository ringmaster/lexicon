package wikilink

import (
	"testing"

	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

func TestParser(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantTarget  string
		wantDisplay string
		wantNil     bool
	}{
		{
			name:        "simple link",
			input:       "[[Page Name]]",
			wantTarget:  "page-name",
			wantDisplay: "Page Name",
		},
		{
			name:        "link with display text",
			input:       "[[Page Name|Display Text]]",
			wantTarget:  "page-name",
			wantDisplay: "Display Text",
		},
		{
			name:        "link with spaces trimmed",
			input:       "[[ Page Name | Display Text ]]",
			wantTarget:  "page-name",
			wantDisplay: "Display Text",
		},
		{
			name:        "link with special characters",
			input:       "[[The Battle of Foo!]]",
			wantTarget:  "the-battle-of-foo",
			wantDisplay: "The Battle of Foo!",
		},
		{
			name:    "empty link",
			input:   "[[]]",
			wantNil: true,
		},
		{
			name:    "incomplete link start",
			input:   "[Page",
			wantNil: true,
		},
		{
			name:    "single bracket",
			input:   "[Page Name]",
			wantNil: true,
		},
		{
			name:    "unclosed link",
			input:   "[[Page Name",
			wantNil: true,
		},
		{
			name:        "link with numbers",
			input:       "[[Chapter 1]]",
			wantTarget:  "chapter-1",
			wantDisplay: "Chapter 1",
		},
		{
			name:        "link with hyphens",
			input:       "[[Some-Thing]]",
			wantTarget:  "some-thing",
			wantDisplay: "Some-Thing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Parser{}
			reader := text.NewReader([]byte(tt.input))
			pc := parser.NewContext()

			node := p.Parse(nil, reader, pc)

			if tt.wantNil {
				if node != nil {
					t.Errorf("expected nil, got %v", node)
				}
				return
			}

			if node == nil {
				t.Fatal("expected non-nil node")
			}

			wl, ok := node.(*WikiLink)
			if !ok {
				t.Fatalf("expected WikiLink, got %T", node)
			}

			if wl.Target != tt.wantTarget {
				t.Errorf("target = %q, want %q", wl.Target, tt.wantTarget)
			}
			if wl.DisplayText != tt.wantDisplay {
				t.Errorf("display = %q, want %q", wl.DisplayText, tt.wantDisplay)
			}
		})
	}
}
