package utils

import (
	"bytes"
	"net/url"

	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"

	"github.com/stoewer/go-strcase"
	"go.abhg.dev/goldmark/mermaid"
	"go.abhg.dev/goldmark/wikilink"
)

type GoldmarkRenderer struct{}

func (b GoldmarkRenderer) Render(input []byte) ([]byte, error) {
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
		),
	)

	var buf bytes.Buffer
	if err := md.Convert(input, &buf); err != nil {
		return []byte{}, err
	}
	return buf.Bytes(), nil
}

type wikilinkResolver struct{}

func (wikilinkResolver) ResolveWikilink(n *wikilink.Node) ([]byte, error) {
	sourceTarget := string(n.Target)
	snakeTarget := strcase.SnakeCase(sourceTarget)
	urlTarget := url.QueryEscape(sourceTarget)
	relativeTarget := "/" + snakeTarget + "?title=" + urlTarget

	return []byte(relativeTarget), nil
}
