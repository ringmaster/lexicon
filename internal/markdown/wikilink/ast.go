package wikilink

import (
	"github.com/yuin/goldmark/ast"
)

// Kind is the kind of WikiLink AST node.
var Kind = ast.NewNodeKind("WikiLink")

// WikiLink represents a wiki-style link in the AST.
// Syntax: [[Page Name]] or [[Page Name|Display Text]]
type WikiLink struct {
	ast.BaseInline
	// Target is the slugified page reference
	Target string
	// DisplayText is what to show the user
	DisplayText string
}

// Kind returns the kind of this node.
func (n *WikiLink) Kind() ast.NodeKind {
	return Kind
}

// Dump dumps the WikiLink node for debugging.
func (n *WikiLink) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{
		"Target":      n.Target,
		"DisplayText": n.DisplayText,
	}, nil)
}

// NewWikiLink creates a new WikiLink node.
func NewWikiLink(target, displayText string) *WikiLink {
	return &WikiLink{
		Target:      target,
		DisplayText: displayText,
	}
}
