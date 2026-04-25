package goldmarkrenderer

import (
	"fmt"

	"github.com/brendanjerwin/simple_wiki/internal/hashtags"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// KindHashtag is the AST node kind for inline `#tag` tokens.
var KindHashtag = ast.NewNodeKind("Hashtag")

// HashtagNode is an inline AST node representing a single `#tag` token. It
// carries both the original raw spelling (for display) and the canonical
// normalized form (used as the search-link target).
type HashtagNode struct {
	ast.BaseInline
	// RawTag is the tag spelling as it appeared in the source, without the
	// leading `#`.
	RawTag string
	// NormalizedTag is the canonical form used for search/index keys.
	NormalizedTag string
}

// Kind implements ast.Node.Kind.
func (*HashtagNode) Kind() ast.NodeKind { //nolint:revive // receiver unused by design
	return KindHashtag
}

// Dump implements ast.Node.Dump for debugging.
func (n *HashtagNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{
		"RawTag":        n.RawTag,
		"NormalizedTag": n.NormalizedTag,
	}, nil)
}

// hashtagInlineParser parses `#tag` tokens inside paragraphs, lists, and
// other inline contexts. Goldmark's standard backslash-escape parser already
// handles `\#` (it strips the backslash and emits a literal `#`) before this
// parser is consulted, so we don't need to special-case escapes here.
type hashtagInlineParser struct{}

// NewHashtagInlineParser constructs the inline parser for `#tag` tokens.
func NewHashtagInlineParser() parser.InlineParser {
	return &hashtagInlineParser{}
}

// Trigger reports the byte that triggers this parser. Goldmark calls Parse
// any time it sees `#` in inline context.
func (*hashtagInlineParser) Trigger() []byte { //nolint:revive // receiver unused by design
	return []byte{'#'}
}

// Parse attempts to consume a `#tag` token at the reader's current position.
// Returns nil to mean "not a hashtag here" — Goldmark falls through to the
// next parser (or treats the `#` as literal text).
func (*hashtagInlineParser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node { //nolint:revive // receiver unused by design
	prev := block.PrecendingCharacter()
	if !isHashtagBoundary(prev) {
		return nil
	}

	line, _ := block.PeekLine()
	if len(line) < 2 || line[0] != '#' {
		return nil
	}

	// Skip the `#` and consume tag chars.
	tagBytes := line[1:]
	end := 0
	for end < len(tagBytes) && isHashtagByte(tagBytes[end]) {
		end++
	}

	if end == 0 {
		return nil
	}

	raw := string(tagBytes[:end])
	normalized := hashtags.Normalize(raw)
	if normalized == "" {
		return nil
	}

	// Advance the reader past `#` + tag chars.
	block.Advance(1 + end)

	return &HashtagNode{
		RawTag:        raw,
		NormalizedTag: normalized,
	}
}

// isHashtagBoundary mirrors hashtags.IsTagBoundary but treats a Goldmark
// "no preceding character" sentinel (rune(0)) as start-of-string.
func isHashtagBoundary(prev rune) bool {
	if prev == 0 {
		return true
	}
	return hashtags.IsTagBoundary(prev)
}

// isHashtagByte reports whether b is a permissible byte inside a tag body.
// Tag chars are letters/digits, `-`, and `_`. We accept all bytes >= 0x80 so
// multi-byte UTF-8 sequences (e.g. accented letters) pass through; the final
// Normalize() call rejects anything that isn't a Unicode letter/digit.
func isHashtagByte(b byte) bool {
	switch {
	case b == '-' || b == '_':
		return true
	case b >= '0' && b <= '9':
		return true
	case b >= 'A' && b <= 'Z':
		return true
	case b >= 'a' && b <= 'z':
		return true
	case b >= 0x80:
		return true
	default:
		return false
	}
}

// hashtagNodeRenderer renders HashtagNode elements as styled clickable pills.
type hashtagNodeRenderer struct {
	html.Config
}

// NewHashtagRenderer creates a renderer for HashtagNode elements.
func NewHashtagRenderer(opts ...html.Option) renderer.NodeRenderer {
	r := &hashtagNodeRenderer{Config: html.NewConfig()}
	for _, opt := range opts {
		opt.SetHTMLOption(&r.Config)
	}
	return r
}

// RegisterFuncs registers the rendering function for HashtagNode.
func (*hashtagNodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) { //nolint:revive // receiver unused by design
	reg.Register(KindHashtag, renderHashtagNode)
}

// renderHashtagNode emits `<wiki-hashtag tag="TAG">#TAG</wiki-hashtag>`
// where the `tag` attribute is the canonical normalized form and the
// slotted text preserves the original spelling so the pill displays as the
// user wrote it.
//
// The actual click handling lives in the `<wiki-hashtag>` Lit web component
// (see static/js/web-components/wiki-hashtag.ts). When the component is
// upgraded, a click dispatches a `wiki-search-open` event that the
// page-level `<wiki-search>` listens for and runs as a search — same UX
// as typing into the menu search bar. If the script fails to load, the
// custom element renders its inline text content unchanged: visible but
// not clickable.
func renderHashtagNode(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n, ok := node.(*HashtagNode)
	if !ok {
		return ast.WalkContinue, nil
	}

	_, err := fmt.Fprintf(
		w,
		`<wiki-hashtag tag="%s">#%s</wiki-hashtag>`,
		util.EscapeHTML([]byte(n.NormalizedTag)),
		util.EscapeHTML([]byte(n.RawTag)),
	)
	return ast.WalkSkipChildren, err
}
