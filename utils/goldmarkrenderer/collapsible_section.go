package goldmarkrenderer

import (
	"strconv"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// KindCollapsibleSection is the node kind for collapsible section container elements.
// These wrap a CollapsibleHeadingNode and the block content that follows it
// until the next heading of equal or higher level.
var KindCollapsibleSection = ast.NewNodeKind("CollapsibleSection")

// CollapsibleSectionNode wraps a collapsible heading and its section content.
// It renders as <collapsible-heading heading-level="N">...</collapsible-heading>.
type CollapsibleSectionNode struct {
	ast.BaseBlock
	Level int
}

// Kind implements ast.Node.Kind.
func (*CollapsibleSectionNode) Kind() ast.NodeKind { //nolint:revive // receiver unused by design
	return KindCollapsibleSection
}

// Dump implements ast.Node.Dump.
func (n *CollapsibleSectionNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{
		"Level": strconv.Itoa(n.Level),
	}, nil)
}

// NewCollapsibleSectionNode creates a new CollapsibleSectionNode with the given heading level.
func NewCollapsibleSectionNode(level int) *CollapsibleSectionNode {
	return &CollapsibleSectionNode{Level: level}
}

// collapsibleSectionTransformer is a goldmark AST transformer that wraps each
// CollapsibleHeadingNode and its following block content into a CollapsibleSectionNode.
// It runs after inline parsing so all nodes have their inline children resolved.
type collapsibleSectionTransformer struct{}

// NewCollapsibleSectionTransformer creates an AST transformer for collapsible sections.
func NewCollapsibleSectionTransformer() parser.ASTTransformer {
	return &collapsibleSectionTransformer{}
}

// Transform finds all CollapsibleHeadingNodes and wraps each one with its following
// sibling content (until the next heading of equal or higher level) in a
// CollapsibleSectionNode.
func (*collapsibleSectionTransformer) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) { //nolint:revive // receiver unused by design
	// Collect all collapsible heading nodes first to avoid modifying while iterating.
	var headings []*CollapsibleHeadingNode
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if heading, ok := n.(*CollapsibleHeadingNode); ok {
			headings = append(headings, heading)
			// Skip children: no nested collapsible headings at parse time
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})

	for _, heading := range headings {
		wrapInSection(heading)
	}
}

// wrapInSection replaces a CollapsibleHeadingNode in its parent with a
// CollapsibleSectionNode that contains the heading and all following siblings
// up to (but not including) the next heading of equal or higher level.
func wrapInSection(heading *CollapsibleHeadingNode) {
	parent := heading.Parent()
	if parent == nil {
		return
	}

	section := NewCollapsibleSectionNode(heading.Level)

	// Replace heading with section at the same position in the parent
	parent.ReplaceChild(parent, heading, section)
	section.AppendChild(section, heading)

	// Collect following siblings until a heading of equal or higher level
	next := section.NextSibling()
	for next != nil {
		if isCollapsibleSectionStop(next, heading.Level) {
			break
		}
		toMove := next
		next = next.NextSibling()
		parent.RemoveChild(parent, toMove)
		section.AppendChild(section, toMove)
	}
}

// isCollapsibleSectionStop returns true if the given node should stop section collection.
// A node stops collection if it is a heading (standard or collapsible) at a level
// equal to or higher (numerically lower) than the current section level.
func isCollapsibleSectionStop(n ast.Node, sectionLevel int) bool {
	if h, ok := n.(*ast.Heading); ok {
		return h.Level <= sectionLevel
	}
	if h, ok := n.(*CollapsibleHeadingNode); ok {
		return h.Level <= sectionLevel
	}
	if s, ok := n.(*CollapsibleSectionNode); ok {
		return s.Level <= sectionLevel
	}
	return false
}
