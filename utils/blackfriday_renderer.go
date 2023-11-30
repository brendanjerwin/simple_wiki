package utils

import (
	"github.com/russross/blackfriday/v2"
)

// BlackfridayRenderer is an implementation of Renderer that uses the blackfriday library
type BlackfridayRenderer struct{}

func (b BlackfridayRenderer) Render(input []byte) ([]byte, error) {
	r := blackfriday.NewHTMLRenderer(blackfriday.HTMLRendererParameters{
		Flags: blackfriday.CommonHTMLFlags,
	})
	return blackfriday.Run(input, blackfriday.WithRenderer(r)), nil
}
