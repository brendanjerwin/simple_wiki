package goldmarkrenderer

import (
	"fmt"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// alertMeta holds display metadata for each alert type.
type alertMeta struct {
	label string
	icon  string // Unicode symbol, rendered inside aria-hidden span
}

// alertMetaByType maps each AlertType to its display label and icon.
var alertMetaByType = map[AlertType]alertMeta{
	AlertNote:      {label: "Note", icon: "\u2139\ufe0f"},      // ℹ️
	AlertTip:       {label: "Tip", icon: "\U0001f4a1"},         // 💡
	AlertImportant: {label: "Important", icon: "\U0001f4cc"},   // 📌
	AlertWarning:   {label: "Warning", icon: "\u26a0\ufe0f"},   // ⚠️
	AlertCaution:   {label: "Caution", icon: "\U0001f6d1"},     // 🛑
}

// alertNodeRenderer renders AlertNode elements as styled GitHub-style callout boxes.
type alertNodeRenderer struct {
	html.Config
}

// NewAlertRenderer creates a renderer for AlertNode elements.
func NewAlertRenderer(opts ...html.Option) renderer.NodeRenderer {
	r := &alertNodeRenderer{Config: html.NewConfig()}
	for _, opt := range opts {
		opt.SetHTMLOption(&r.Config)
	}
	return r
}

// RegisterFuncs registers the rendering function for AlertNode.
func (*alertNodeRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) { //nolint:revive // receiver unused by design
	reg.Register(KindAlert, renderAlertNode)
}

// renderAlertNode renders an AlertNode as a <div class="markdown-alert ..."> element.
// The title paragraph and icon are injected on entry; the closing div is written on exit.
func renderAlertNode(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n, ok := node.(*AlertNode)
	if !ok {
		return ast.WalkContinue, nil
	}

	if !entering {
		_, err := w.WriteString("</div>\n")
		return ast.WalkContinue, err
	}

	meta := alertMetaByType[n.Variant]
	_, err := fmt.Fprintf(w,
		"<div class=\"markdown-alert markdown-alert-%s\" role=\"note\">\n"+
			"<p class=\"markdown-alert-title\">"+
			"<span class=\"markdown-alert-icon\" aria-hidden=\"true\">%s</span> %s"+
			"</p>\n",
		string(n.Variant),
		meta.icon,
		meta.label,
	)
	return ast.WalkContinue, err
}
