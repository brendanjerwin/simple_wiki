package goldmarkrenderer

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// KindAlert is the AST node kind for GitHub-style alert blocks.
// These are blockquotes whose first line is a [!TYPE] marker.
var KindAlert = ast.NewNodeKind("Alert")

// AlertType represents one of the five supported alert types.
type AlertType string

const (
	// AlertNote is the informational note alert type.
	AlertNote AlertType = "note"
	// AlertTip is the helpful tip alert type.
	AlertTip AlertType = "tip"
	// AlertImportant is the important information alert type.
	AlertImportant AlertType = "important"
	// AlertWarning is the urgent warning alert type.
	AlertWarning AlertType = "warning"
	// AlertCaution is the cautionary alert type advising about risks.
	AlertCaution AlertType = "caution"
)

// minAlertMarkerLen is the minimum number of bytes needed for a valid [!X] marker.
const minAlertMarkerLen = 4 // "[", "!", at least one letter, "]"

// alertTypes maps lowercased type names to their AlertType constants.
var alertTypes = map[string]AlertType{
	"note":      AlertNote,
	"tip":       AlertTip,
	"important": AlertImportant,
	"warning":   AlertWarning,
	"caution":   AlertCaution,
}

// AlertNode wraps blockquote content that was parsed from a [!TYPE] marker.
// It renders as a styled <div class="markdown-alert markdown-alert-TYPE"> element.
// The field is named Variant (not Type) to avoid shadowing the Type() method
// provided by the embedded ast.BaseBlock.
type AlertNode struct {
	ast.BaseBlock
	// Variant is the alert variant (note, tip, important, warning, caution).
	Variant AlertType
}

// Kind implements ast.Node.Kind.
func (*AlertNode) Kind() ast.NodeKind { //nolint:revive // receiver unused by design
	return KindAlert
}

// Dump implements ast.Node.Dump.
func (n *AlertNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{
		"Variant": string(n.Variant),
	}, nil)
}

// NewAlertNode creates a new AlertNode with the given alert type.
func NewAlertNode(alertType AlertType) *AlertNode {
	return &AlertNode{Variant: alertType}
}

// alertTransformer is a goldmark AST transformer that detects blockquotes whose
// first line is a GitHub-style [!TYPE] marker and replaces them with AlertNodes.
type alertTransformer struct{}

// NewAlertTransformer creates an AST transformer for GitHub-style alert blocks.
func NewAlertTransformer() parser.ASTTransformer {
	return &alertTransformer{}
}

// Transform walks the document finding Blockquote nodes with [!TYPE] markers
// and replaces them with AlertNode instances containing the blockquote's content.
func (*alertTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) { //nolint:revive // receiver unused by design
	source := reader.Source()

	var blockquotes []*ast.Blockquote
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if bq, ok := n.(*ast.Blockquote); ok {
			blockquotes = append(blockquotes, bq)
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})

	for _, bq := range blockquotes {
		alertType, ok := detectAlertType(bq, source)
		if !ok {
			continue
		}
		convertBlockquoteToAlert(bq, alertType)
	}
}

// detectAlertType checks if the blockquote's first paragraph line is a [!TYPE]
// marker. Returns the AlertType and true if valid, or empty string and false otherwise.
func detectAlertType(bq *ast.Blockquote, source []byte) (AlertType, bool) {
	firstChild := bq.FirstChild()
	if firstChild == nil {
		return "", false
	}

	para, ok := firstChild.(*ast.Paragraph)
	if !ok {
		return "", false
	}

	if para.Lines().Len() == 0 {
		return "", false
	}

	// Extract the first line and check for [!TYPE] syntax.
	// text.Segment.Value has a pointer receiver so we assign to a variable first.
	firstSeg := para.Lines().At(0)
	firstLine := firstSeg.Value(source)
	firstLine = bytes.TrimRight(firstLine, "\r\n")
	firstLine = bytes.TrimSpace(firstLine)

	if len(firstLine) < minAlertMarkerLen || firstLine[0] != '[' || firstLine[1] != '!' {
		return "", false
	}

	// Find the closing bracket for [!TYPE].
	tail := firstLine[2:]
	closeIdx := bytes.IndexByte(tail, ']')
	if closeIdx < 0 {
		return "", false
	}

	// The closing bracket must end the content (no trailing text allowed).
	if closeIdx != len(tail)-1 {
		return "", false
	}

	typeName := strings.ToLower(string(tail[:closeIdx]))
	alertType, valid := alertTypes[typeName]
	return alertType, valid
}

// convertBlockquoteToAlert replaces a Blockquote node with an AlertNode,
// stripping the [!TYPE] marker from the first paragraph's inline children.
// AST transformers run after inline parsing, so raw segments have already been
// expanded into inline nodes (ast.Text, ast.SoftLineBreak, etc.); we must
// modify the inline children rather than the raw line segments.
func convertBlockquoteToAlert(bq *ast.Blockquote, alertType AlertType) {
	parent := bq.Parent()
	if parent == nil {
		return
	}

	// Remove the [!TYPE] inline content from the leading paragraph.
	// In goldmark, a soft line break is represented as a flag on an ast.Text node
	// (Text.SoftLineBreak()), not as a separate AST node type. Removing the first
	// child (the Text containing "[!TYPE]") is sufficient.
	if para, ok := bq.FirstChild().(*ast.Paragraph); ok {
		firstInline := para.FirstChild()
		if firstInline != nil {
			para.RemoveChild(para, firstInline)
		}
		// If the paragraph is now empty, remove it from the blockquote.
		if para.FirstChild() == nil {
			bq.RemoveChild(bq, para)
		}
	}

	alert := NewAlertNode(alertType)

	// Move all remaining blockquote children into the alert node.
	for child := bq.FirstChild(); child != nil; {
		next := child.NextSibling()
		bq.RemoveChild(bq, child)
		alert.AppendChild(alert, child)
		child = next
	}

	parent.ReplaceChild(parent, bq, alert)
}
