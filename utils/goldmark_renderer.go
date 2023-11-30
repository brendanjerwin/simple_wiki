package utils

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark-emoji"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"

	"go.abhg.dev/goldmark/mermaid"
)

type GoldmarkRenderer struct{}

func (b GoldmarkRenderer) Render(input []byte) ([]byte, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM, 
			emoji.Emoji,
			$mermaid.Extender{},
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
