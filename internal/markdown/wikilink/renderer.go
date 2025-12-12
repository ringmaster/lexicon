package wikilink

import (
	"html"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// PageChecker is a function that checks if a page exists and whether it's a phantom.
type PageChecker func(slug string) (exists bool, isPhantom bool)

// Renderer renders WikiLink nodes to HTML.
type Renderer struct {
	PageChecker PageChecker
}

// NewRenderer creates a new WikiLink renderer.
func NewRenderer(checker PageChecker) *Renderer {
	return &Renderer{PageChecker: checker}
}

// RegisterFuncs registers the renderer functions.
func (r *Renderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(Kind, r.renderWikiLink)
}

func (r *Renderer) renderWikiLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	n := node.(*WikiLink)

	// Determine link class based on page status
	class := "wiki-link"
	if r.PageChecker != nil {
		exists, isPhantom := r.PageChecker(n.Target)
		if !exists || isPhantom {
			class = "wiki-link phantom"
		}
	}

	// Escape values for HTML
	escapedTarget := html.EscapeString(n.Target)
	escapedDisplay := html.EscapeString(n.DisplayText)

	// Write the HTML
	w.WriteString(`<a href="/`)
	w.WriteString(escapedTarget)
	w.WriteString(`" class="`)
	w.WriteString(class)
	w.WriteString(`">`)
	w.WriteString(escapedDisplay)
	w.WriteString(`</a>`)

	return ast.WalkContinue, nil
}
