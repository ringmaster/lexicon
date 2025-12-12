package wikilink

import (
	"strings"

	"lexicon/internal/database"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// Parser is a Goldmark inline parser for wiki links.
type Parser struct{}

var _ parser.InlineParser = (*Parser)(nil)

// Trigger returns the characters that trigger this parser.
func (p *Parser) Trigger() []byte {
	return []byte{'['}
}

// Parse parses a wiki link [[target]] or [[target|display]].
func (p *Parser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, segment := block.PeekLine()
	if len(line) < 4 { // Minimum: [[x]]
		return nil
	}

	// Check for [[
	if line[0] != '[' || line[1] != '[' {
		return nil
	}

	// Find closing ]]
	end := -1
	for i := 2; i < len(line)-1; i++ {
		if line[i] == ']' && line[i+1] == ']' {
			end = i
			break
		}
		// Don't allow newlines in wiki links
		if line[i] == '\n' {
			return nil
		}
	}

	if end < 0 {
		return nil
	}

	// Extract content between [[ and ]]
	content := string(line[2:end])
	if content == "" {
		return nil
	}

	// Parse target and display text
	var target, displayText string
	if idx := strings.Index(content, "|"); idx >= 0 {
		target = strings.TrimSpace(content[:idx])
		displayText = strings.TrimSpace(content[idx+1:])
	} else {
		target = strings.TrimSpace(content)
		displayText = target
	}

	if target == "" {
		return nil
	}

	// Slugify the target
	slug := database.Slugify(target)
	if slug == "" {
		return nil
	}

	// Advance the reader past the wiki link
	block.Advance(segment.Start + end + 2)

	return NewWikiLink(slug, displayText)
}

// CloseBlock is not used for inline parsers.
func (p *Parser) CloseBlock(parent ast.Node, pc parser.Context) {
	// Not used for inline parsers
}
