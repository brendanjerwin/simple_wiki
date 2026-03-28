package goldmarkrenderer

import (
	"strconv"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// maxHeadingLevel is the maximum ATX heading level per the CommonMark spec.
const maxHeadingLevel = 6

// KindCollapsibleHeading is the node kind for collapsible heading elements.
// These nodes render as <hN slot="heading"> inside a CollapsibleSectionNode.
var KindCollapsibleHeading = ast.NewNodeKind("CollapsibleHeading")

// CollapsibleHeadingNode represents a heading parsed from #^ syntax.
// It renders as <hN slot="heading" id="..."> with auto-generated id.
type CollapsibleHeadingNode struct {
	ast.BaseBlock
	Level int
}

// Kind implements ast.Node.Kind.
func (*CollapsibleHeadingNode) Kind() ast.NodeKind { //nolint:revive // receiver unused by design
	return KindCollapsibleHeading
}

// Dump implements ast.Node.Dump.
func (n *CollapsibleHeadingNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{
		"Level": strconv.Itoa(n.Level),
	}, nil)
}

// NewCollapsibleHeadingNode creates a new CollapsibleHeadingNode with the given level.
func NewCollapsibleHeadingNode(level int) *CollapsibleHeadingNode {
	return &CollapsibleHeadingNode{Level: level}
}

// collapsibleHeadingBlockParser parses the #^ Heading syntax into CollapsibleHeadingNodes.
// It must be registered with a priority lower than 600 (ATX heading parser priority)
// so it runs first and can intercept the #^ pattern.
type collapsibleHeadingBlockParser struct{}

// NewCollapsibleHeadingBlockParser creates a block parser for #^ heading syntax.
func NewCollapsibleHeadingBlockParser() parser.BlockParser {
	return &collapsibleHeadingBlockParser{}
}

// Trigger returns '#' to intercept lines that might be collapsible headings.
func (*collapsibleHeadingBlockParser) Trigger() []byte { //nolint:revive // receiver unused by design
	return []byte{'#'}
}

// Open parses a line in the form "#{1-6}^ text" and returns a CollapsibleHeadingNode.
// Returns nil if the line does not match the collapsible heading syntax.
func (*collapsibleHeadingBlockParser) Open(_ ast.Node, reader text.Reader, pc parser.Context) (ast.Node, parser.State) { //nolint:revive // receiver unused by design
	line, segment := reader.PeekLine()
	pos := pc.BlockOffset()
	if pos < 0 {
		return nil, parser.NoChildren
	}

	// Count # characters (1-6 allowed)
	i := pos
	for i < len(line) && line[i] == '#' {
		i++
	}
	level := i - pos
	if level == 0 || level > maxHeadingLevel {
		return nil, parser.NoChildren
	}

	// Must have ^ immediately after the # characters
	if i >= len(line) || line[i] != '^' {
		return nil, parser.NoChildren
	}
	i++ // skip ^

	// Must have exactly one space or tab after ^
	if i >= len(line) || (line[i] != ' ' && line[i] != '\t') {
		return nil, parser.NoChildren
	}
	i++ // skip the space/tab

	node := NewCollapsibleHeadingNode(level)

	// Set text segment for inline parsing — the heading text after "#^ "
	hl := text.NewSegment(
		segment.Start+i-segment.Padding,
		segment.Start+len(line)-segment.Padding,
	)
	hl = hl.TrimRightSpace(reader.Source())
	if hl.Len() > 0 {
		node.Lines().Append(hl)
	}
	reader.AdvanceToEOL()
	return node, parser.NoChildren
}

// Continue closes the node immediately (ATX headings are always single-line).
func (*collapsibleHeadingBlockParser) Continue(_ ast.Node, _ text.Reader, _ parser.Context) parser.State { //nolint:revive // receiver unused by design
	return parser.Close
}

// Close generates an auto heading ID for the node, matching goldmark's standard behavior.
func (*collapsibleHeadingBlockParser) Close(node ast.Node, reader text.Reader, pc parser.Context) { //nolint:revive // receiver unused by design
	n, ok := node.(*CollapsibleHeadingNode)
	if !ok {
		return
	}
	lastIndex := n.Lines().Len() - 1
	if lastIndex >= 0 {
		lastLine := n.Lines().At(lastIndex)
		line := lastLine.Value(reader.Source())
		// Use ast.KindHeading so IDs share the deduplication namespace with regular headings
		headingID := pc.IDs().Generate(line, ast.KindHeading)
		n.SetAttribute([]byte("id"), headingID)
	}
}

// CanInterruptParagraph returns true so the parser works like standard ATX headings.
func (*collapsibleHeadingBlockParser) CanInterruptParagraph() bool { //nolint:revive // receiver unused by design
	return true
}

// CanAcceptIndentedLine returns false, matching standard ATX heading behavior.
func (*collapsibleHeadingBlockParser) CanAcceptIndentedLine() bool { //nolint:revive // receiver unused by design
	return false
}
