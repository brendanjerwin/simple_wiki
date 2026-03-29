package goldmarkrenderer

import (
	"fmt"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// collapsibleSectionRenderer renders CollapsibleSectionNode and CollapsibleHeadingNode.
type collapsibleSectionRenderer struct {
	html.Config
}

// NewCollapsibleSectionRenderer creates a renderer for collapsible section nodes.
func NewCollapsibleSectionRenderer(opts ...html.Option) renderer.NodeRenderer {
	r := &collapsibleSectionRenderer{Config: html.NewConfig()}
	for _, opt := range opts {
		opt.SetHTMLOption(&r.Config)
	}
	return r
}

// RegisterFuncs registers rendering functions for both collapsible node types.
func (*collapsibleSectionRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) { //nolint:revive // receiver unused by design
	reg.Register(KindCollapsibleSection, renderSection)
	reg.Register(KindCollapsibleHeading, renderHeading)
}

// renderSection renders a CollapsibleSectionNode as a <collapsible-heading> custom element.
func renderSection(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		_, err := w.WriteString("</collapsible-heading>\n")
		return ast.WalkContinue, err
	}
	n, ok := node.(*CollapsibleSectionNode)
	if !ok {
		return ast.WalkContinue, nil
	}
	if _, err := fmt.Fprintf(w, "<collapsible-heading heading-level=\"%d\">\n", n.Level); err != nil {
		return ast.WalkStop, err
	}
	return ast.WalkContinue, nil
}

// renderHeading renders a CollapsibleHeadingNode as <hN slot="heading" id="...">.
// The slot="heading" attribute projects this element into the named slot of the
// <collapsible-heading> web component's shadow DOM.
func renderHeading(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		n, ok := node.(*CollapsibleHeadingNode)
		if !ok {
			return ast.WalkContinue, nil
		}
		_, err := fmt.Fprintf(w, "</h%d>\n", n.Level)
		return ast.WalkContinue, err
	}
	n, ok := node.(*CollapsibleHeadingNode)
	if !ok {
		return ast.WalkContinue, nil
	}
	if _, err := fmt.Fprintf(w, "<h%d slot=\"heading\"", n.Level); err != nil {
		return ast.WalkStop, err
	}
	if n.Attributes() != nil {
		html.RenderAttributes(w, node, html.HeadingAttributeFilter)
	}
	if err := w.WriteByte('>'); err != nil {
		return ast.WalkStop, err
	}
	return ast.WalkContinue, nil
}
