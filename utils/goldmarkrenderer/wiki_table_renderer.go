package goldmarkrenderer

import (
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// WikiTableRenderer wraps GFM tables in <wiki-table> custom elements
// for progressive enhancement with interactive features.
type WikiTableRenderer struct {
	html.Config
}

// NewWikiTableRenderer creates a new WikiTableRenderer
func NewWikiTableRenderer(opts ...html.Option) renderer.NodeRenderer {
	r := &WikiTableRenderer{Config: html.NewConfig()}
	for _, opt := range opts {
		opt.SetHTMLOption(&r.Config)
	}
	return r
}

// RegisterFuncs registers the table rendering function
func (r *WikiTableRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(east.KindTable, r.renderTable)
}

func (r *WikiTableRenderer) renderTable(w util.BufWriter, _ []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		if _, err := w.WriteString("<wiki-table><table"); err != nil {
			return ast.WalkStop, err
		}
		if node.Attributes() != nil {
			html.RenderAttributes(w, node, extension.TableAttributeFilter)
		}
		if _, err := w.WriteString(">\n"); err != nil {
			return ast.WalkStop, err
		}
	} else {
		if _, err := w.WriteString("</table>\n</wiki-table>\n"); err != nil {
			return ast.WalkStop, err
		}
	}
	return ast.WalkContinue, nil
}
