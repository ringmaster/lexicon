package markdown

import (
	"bytes"

	"lexicon/internal/markdown/wikilink"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Renderer handles markdown rendering with wiki-link support.
type Renderer struct {
	md          goldmark.Markdown
	pageChecker wikilink.PageChecker
}

// New creates a new markdown renderer with the given page checker.
func New(pageChecker wikilink.PageChecker) *Renderer {
	r := &Renderer{
		pageChecker: pageChecker,
	}

	// Create goldmark instance with wiki-link extension
	r.md = goldmark.New(
		goldmark.WithParserOptions(
			parser.WithInlineParsers(
				util.Prioritized(&wikilink.Parser{}, 100),
			),
		),
		goldmark.WithRendererOptions(
			renderer.WithNodeRenderers(
				util.Prioritized(wikilink.NewRenderer(pageChecker), 100),
			),
		),
	)

	return r
}

// Render converts markdown content to HTML.
func (r *Renderer) Render(content string) (string, error) {
	var buf bytes.Buffer
	if err := r.md.Convert([]byte(content), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ExtractLinks parses content and returns all wiki-link targets.
func (r *Renderer) ExtractLinks(content string) []LinkInfo {
	p := goldmark.New(
		goldmark.WithParserOptions(
			parser.WithInlineParsers(
				util.Prioritized(&wikilink.Parser{}, 100),
			),
		),
	).Parser()

	reader := text.NewReader([]byte(content))
	doc := p.Parse(reader)

	var links []LinkInfo
	ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if wl, ok := node.(*wikilink.WikiLink); ok {
			links = append(links, LinkInfo{
				Target:      wl.Target,
				DisplayText: wl.DisplayText,
			})
		}
		return ast.WalkContinue, nil
	})

	return links
}

// LinkInfo holds information about an extracted wiki link.
type LinkInfo struct {
	Target      string
	DisplayText string
}

// UniqueTargets returns deduplicated link targets.
func UniqueTargets(links []LinkInfo) []string {
	seen := make(map[string]bool)
	var targets []string
	for _, link := range links {
		if !seen[link.Target] {
			seen[link.Target] = true
			targets = append(targets, link.Target)
		}
	}
	return targets
}
