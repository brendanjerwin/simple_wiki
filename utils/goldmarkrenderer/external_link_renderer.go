package goldmarkrenderer

import (
	"bytes"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// externalLinkIndicator is the HTML fragment appended after the text of an
// external link. The span carries aria-hidden="true" so screen readers skip
// the decorative glyph and only hear the link's own text.
const externalLinkIndicator = `<span aria-hidden="true">↗</span>`

// externalLinkRenderer adds a ↗ indicator to links whose destination is an
// absolute URL pointing to an external host (http:// or https://).
// Internal wiki links, relative paths, and anchors are left unchanged.
type externalLinkRenderer struct {
	html.Config
}

// NewExternalLinkRenderer creates a renderer that appends a ↗ indicator to
// external links.
func NewExternalLinkRenderer(opts ...html.Option) renderer.NodeRenderer {
	r := &externalLinkRenderer{Config: html.NewConfig()}
	for _, opt := range opts {
		opt.SetHTMLOption(&r.Config)
	}
	return r
}

// RegisterFuncs registers the rendering functions for link and autolink nodes,
// overriding the default goldmark HTML renderer for those node kinds.
func (r *externalLinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindLink, r.renderLink)
	reg.Register(ast.KindAutoLink, r.renderAutoLink)
}

// isExternalURL returns true when dest is an absolute URL to an external host.
// Only http:// and https:// are considered external; relative paths, anchors,
// and protocol-relative URLs are treated as internal.
func isExternalURL(dest []byte) bool {
	return bytes.HasPrefix(dest, []byte("http://")) || bytes.HasPrefix(dest, []byte("https://"))
}

// renderLink replaces the default goldmark link renderer so that external
// links receive the ↗ indicator just before the closing </a> tag.
func (r *externalLinkRenderer) renderLink(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n, ok := node.(*ast.Link)
	if !ok {
		return ast.WalkContinue, nil
	}
	if !entering {
		return r.renderLinkExit(w, n)
	}
	return r.renderLinkEnter(w, n)
}

func (r *externalLinkRenderer) renderLinkEnter(w util.BufWriter, n *ast.Link) (ast.WalkStatus, error) {
	if _, err := w.WriteString(`<a href="`); err != nil {
		return ast.WalkStop, err
	}
	dest := util.URLEscape(n.Destination, true)
	if r.Unsafe || !html.IsDangerousURL(dest) {
		if _, err := w.Write(util.EscapeHTML(dest)); err != nil {
			return ast.WalkStop, err
		}
	}
	if err := w.WriteByte('"'); err != nil {
		return ast.WalkStop, err
	}
	if n.Title != nil {
		if _, err := w.WriteString(` title="`); err != nil {
			return ast.WalkStop, err
		}
		r.Writer.Write(w, n.Title)
		if err := w.WriteByte('"'); err != nil {
			return ast.WalkStop, err
		}
	}
	if n.Attributes() != nil {
		html.RenderAttributes(w, n, html.LinkAttributeFilter)
	}
	if err := w.WriteByte('>'); err != nil {
		return ast.WalkStop, err
	}
	return ast.WalkContinue, nil
}

func (*externalLinkRenderer) renderLinkExit(w util.BufWriter, n *ast.Link) (ast.WalkStatus, error) { //nolint:revive // receiver unused by design
	if isExternalURL(n.Destination) {
		if _, err := w.WriteString(externalLinkIndicator); err != nil {
			return ast.WalkStop, err
		}
	}
	_, err := w.WriteString("</a>")
	return ast.WalkContinue, err
}

// renderAutoLink replaces the default goldmark autolink renderer so that
// external autolinks receive the ↗ indicator just before the closing </a> tag.
func (r *externalLinkRenderer) renderAutoLink(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n, ok := node.(*ast.AutoLink)
	if !ok {
		return ast.WalkContinue, nil
	}
	rawURL := n.URL(source)
	escapedURL := util.URLEscape(rawURL, false)
	label := n.Label(source)

	if _, err := w.WriteString(`<a href="`); err != nil {
		return ast.WalkStop, err
	}
	if err := r.writeAutoLinkScheme(w, n, escapedURL); err != nil {
		return ast.WalkStop, err
	}
	if err := writeAutoLinkHrefClose(w, n); err != nil {
		return ast.WalkStop, err
	}
	if _, err := w.Write(util.EscapeHTML(label)); err != nil {
		return ast.WalkStop, err
	}
	if isExternalURL(rawURL) {
		if _, err := w.WriteString(externalLinkIndicator); err != nil {
			return ast.WalkStop, err
		}
	}
	if _, err := w.WriteString(`</a>`); err != nil {
		return ast.WalkStop, err
	}
	return ast.WalkContinue, nil
}

// writeAutoLinkScheme writes the href value (scheme + escaped URL) into w.
// For email autolinks that don't already carry a mailto: prefix, the prefix
// is prepended. Dangerous URLs are omitted unless the renderer is in unsafe mode.
func (r *externalLinkRenderer) writeAutoLinkScheme(w util.BufWriter, n *ast.AutoLink, escapedURL []byte) error {
	if n.AutoLinkType == ast.AutoLinkEmail && !bytes.HasPrefix(bytes.ToLower(escapedURL), []byte("mailto:")) {
		if _, err := w.WriteString("mailto:"); err != nil {
			return err
		}
	}
	if r.Unsafe || !html.IsDangerousURL(escapedURL) {
		if _, err := w.Write(util.EscapeHTML(escapedURL)); err != nil {
			return err
		}
	}
	return nil
}

// writeAutoLinkHrefClose closes the href attribute quote and opens the anchor
// tag body. When the node carries additional attributes they are rendered
// between the closing quote and the '>'; otherwise the compact `">` form is used.
func writeAutoLinkHrefClose(w util.BufWriter, n *ast.AutoLink) error {
	if n.Attributes() == nil {
		_, err := w.WriteString(`">`)
		return err
	}
	if err := w.WriteByte('"'); err != nil {
		return err
	}
	html.RenderAttributes(w, n, html.LinkAttributeFilter)
	return w.WriteByte('>')
}
