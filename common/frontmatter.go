package common

import (
	"bytes"

	toml "github.com/pelletier/go-toml/v2"
)

type PageIdentifier = string
type Markdown = string
type FrontMatter map[string]interface{}

type IReadPages interface {
	ReadFrontMatter(requested_identifier string) (PageIdentifier, FrontMatter, error)
	ReadMarkdown(requested_identifier string) (PageIdentifier, Markdown, error)
}

func (f FrontMatter) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf).
		SetArraysMultiline(true).
		SetTablesInline(false).
		SetIndentTables(true)

	err := enc.Encode(f)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
