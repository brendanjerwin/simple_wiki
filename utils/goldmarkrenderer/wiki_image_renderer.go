package goldmarkrenderer

import (
	"bytes"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// WikiImageRenderer renders images as <wiki-image> custom elements
type WikiImageRenderer struct {
	html.Config
}

// NewWikiImageRenderer creates a new WikiImageRenderer
func NewWikiImageRenderer(opts ...html.Option) renderer.NodeRenderer {
	r := &WikiImageRenderer{Config: html.NewConfig()}
	for _, opt := range opts {
		opt.SetHTMLOption(&r.Config)
	}
	return r
}

// RegisterFuncs registers the image rendering function
func (r *WikiImageRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindImage, r.renderImage)
}

// extractAltText extracts the alt text from an image node's children
func extractAltText(n *ast.Image, source []byte) []byte {
	var buf bytes.Buffer
	for child := n.FirstChild(); child != nil; child = child.NextSibling() {
		if text, ok := child.(*ast.Text); ok {
			// bytes.Buffer.Write never returns an error per Go documentation
			_, _ = buf.Write(text.Segment.Value(source))
		}
	}
	return buf.Bytes()
}

// renderImage renders an image node as a wiki-image custom element.
// The entering parameter is required by the goldmark NodeRendererFunc interface.
func (r *WikiImageRenderer) renderImage(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}

	n, ok := node.(*ast.Image)
	if !ok {
		return ast.WalkContinue, nil
	}

	// Write opening tag with src attribute
	if _, err := w.WriteString(`<wiki-image src="`); err != nil {
		return ast.WalkStop, err
	}
	if r.Unsafe || !html.IsDangerousURL(n.Destination) {
		if _, err := w.Write(util.EscapeHTML(util.URLEscape(n.Destination, true))); err != nil {
			return ast.WalkStop, err
		}
	}

	// Write alt attribute
	if _, err := w.WriteString(`" alt="`); err != nil {
		return ast.WalkStop, err
	}
	altText := extractAltText(n, source)
	if _, err := w.Write(util.EscapeHTML(altText)); err != nil {
		return ast.WalkStop, err
	}
	if err := w.WriteByte('"'); err != nil {
		return ast.WalkStop, err
	}

	// Write title attribute if present
	if n.Title != nil {
		if _, err := w.WriteString(` title="`); err != nil {
			return ast.WalkStop, err
		}
		r.Writer.Write(w, n.Title)
		if err := w.WriteByte('"'); err != nil {
			return ast.WalkStop, err
		}
	}

	// Close tag
	if _, err := w.WriteString(`></wiki-image>`); err != nil {
		return ast.WalkStop, err
	}

	return ast.WalkSkipChildren, nil
}
