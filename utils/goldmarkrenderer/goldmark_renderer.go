package goldmarkrenderer

import (
	"bytes"
	"net/url"
	"regexp"

	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"

	"go.abhg.dev/goldmark/mermaid"
	"go.abhg.dev/goldmark/wikilink"
)

// wikiImageRendererPriority sets the priority for the custom image renderer.
// Higher priority means it runs before default renderers.
const wikiImageRendererPriority = 100

// wikiTableRendererPriority must be lower than the GFM default table renderer (500)
// because goldmark registers renderers from highest to lowest priority,
// and the last registration wins.
const wikiTableRendererPriority = 400

type GoldmarkRenderer struct{}

// Render renders the input markdown to HTML.
func (GoldmarkRenderer) Render(input []byte) ([]byte, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			emoji.Emoji,
			&mermaid.Extender{},
			&wikilink.Extender{
				Resolver: wikilinkResolver{},
			},
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
			html.WithUnsafe(),
			renderer.WithNodeRenderers(
				util.Prioritized(NewWikiImageRenderer(html.WithUnsafe()), wikiImageRendererPriority),
				util.Prioritized(NewWikiTableRenderer(html.WithUnsafe()), wikiTableRendererPriority),
			),
		),
	)

	var buf bytes.Buffer
	if err := md.Convert(input, &buf); err != nil {
		return []byte{}, err
	}
	p := bluemonday.UGCPolicy()
	// Allow GFM task list checkboxes
	p.AllowElements("input")
	p.AllowAttrs("type").Matching(regexp.MustCompile(`^checkbox$`)).OnElements("input")
	p.AllowAttrs("disabled", "checked").OnElements("input")
	// Allow wiki-image custom element
	p.AllowElements("wiki-image")
	p.AllowAttrs("src", "alt", "title").OnElements("wiki-image")
	// Allow wiki-table custom element (AllowNoAttrs is required for bluemonday to preserve the element)
	p.AllowNoAttrs().OnElements("wiki-table")
	// Allow wiki-checklist custom element
	p.AllowElements("wiki-checklist")
	p.AllowAttrs("list-name", "page").OnElements("wiki-checklist")
	// Allow wiki-blog custom element and its server-rendered fallback children
	p.AllowElements("wiki-blog")
	p.AllowAttrs("blog-id", "max-articles", "page", "hide-new-post").OnElements("wiki-blog")
	p.AllowAttrs("class").OnElements("span", "a")
	return p.SanitizeBytes(buf.Bytes()), nil
}

type wikilinkResolver struct{}

func (wikilinkResolver) ResolveWikilink(n *wikilink.Node) ([]byte, error) {
	sourceTarget := string(n.Target)
	mungedTarget, err := wikiidentifiers.MungeIdentifier(sourceTarget)
	if err != nil {
		// Invalid identifier - use URL-escaped original as fallback
		mungedTarget = url.PathEscape(sourceTarget)
	}
	urlTarget := url.QueryEscape(sourceTarget)
	relativeTarget := "/" + mungedTarget + "?title=" + urlTarget

	return []byte(relativeTarget), nil
}
