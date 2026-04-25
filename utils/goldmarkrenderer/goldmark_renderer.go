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

// collapsibleHeadingParserPriority must be lower than the ATX heading parser (600)
// so the collapsible heading parser runs first and intercepts #^ syntax.
const collapsibleHeadingParserPriority = 550

// collapsibleSectionRendererPriority for the custom collapsible section renderer.
const collapsibleSectionRendererPriority = 100

// alertTransformerPriority for the alert AST transformer.
// Uses the same priority as collapsibleSectionRendererPriority; both handle
// different node types so ordering between them does not matter.
const alertTransformerPriority = 100

// alertRendererPriority for the custom alert node renderer.
const alertRendererPriority = 500

// hashtagInlineParserPriority must run after the link parser so `(#anchor)`
// inside markdown link syntax `[text](#anchor)` is consumed as part of the
// link before the hashtag parser sees it. The default link parser is at
// priority 200, so we use a larger number (lower priority).
const hashtagInlineParserPriority = 999

// hashtagRendererPriority for the custom hashtag node renderer.
const hashtagRendererPriority = 500

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
			parser.WithBlockParsers(
				util.Prioritized(NewCollapsibleHeadingBlockParser(), collapsibleHeadingParserPriority),
			),
			parser.WithInlineParsers(
				util.Prioritized(NewHashtagInlineParser(), hashtagInlineParserPriority),
			),
			parser.WithASTTransformers(
				util.Prioritized(NewCollapsibleSectionTransformer(), collapsibleSectionRendererPriority),
				util.Prioritized(NewAlertTransformer(), alertTransformerPriority),
			),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
			html.WithUnsafe(),
			renderer.WithNodeRenderers(
				util.Prioritized(NewWikiImageRenderer(html.WithUnsafe()), wikiImageRendererPriority),
				util.Prioritized(NewWikiTableRenderer(html.WithUnsafe()), wikiTableRendererPriority),
				util.Prioritized(NewCollapsibleSectionRenderer(html.WithUnsafe()), collapsibleSectionRendererPriority),
				util.Prioritized(NewAlertRenderer(html.WithUnsafe()), alertRendererPriority),
				util.Prioritized(NewHashtagRenderer(html.WithUnsafe()), hashtagRendererPriority),
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
	// Allow wiki-survey custom element
	p.AllowElements("wiki-survey")
	p.AllowAttrs("name", "page").OnElements("wiki-survey")
	// Allow wiki-blog custom element and its server-rendered fallback children
	p.AllowElements("wiki-blog")
	p.AllowAttrs("blog-id", "max-articles", "page", "hide-new-post").OnElements("wiki-blog")
	// Allow collapsible-heading custom element for #^ syntax
	p.AllowElements("collapsible-heading")
	p.AllowAttrs("heading-level").OnElements("collapsible-heading")
	// Allow slot attribute on heading elements for the collapsible-heading named slot
	p.AllowAttrs("slot").OnElements("h1", "h2", "h3", "h4", "h5", "h6")
	p.AllowAttrs("class").OnElements("span", "a")
	// Allow hashtag pills emitted by the hashtag inline parser/renderer.
	p.AllowAttrs("class").Matching(regexp.MustCompile(`^hashtag-pill$`)).OnElements("a")
	// Allow GitHub-style alert/admonition blocks rendered by the alert transformer.
	p.AllowElements("div")
	p.AllowAttrs("class").Matching(regexp.MustCompile(`^markdown-alert(?: markdown-alert-(?:note|tip|important|warning|caution))?$`)).OnElements("div")
	p.AllowAttrs("role").Matching(regexp.MustCompile(`^note$`)).OnElements("div")
	p.AllowAttrs("class").Matching(regexp.MustCompile(`^markdown-alert-title$`)).OnElements("p")
	p.AllowAttrs("aria-hidden").Matching(regexp.MustCompile(`^true$`)).OnElements("span")
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
