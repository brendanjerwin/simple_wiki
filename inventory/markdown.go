package inventory

import (
	"bytes"
	"fmt"

	"github.com/pelletier/go-toml/v2"
)

const (
	tomlDelimiter = "+++\n"
	newline       = "\n"
)

// ItemMarkdownTemplate is the markdown template for inventory item pages.
// It is exported so it can be used by both the server and the gRPC API layer.
const ItemMarkdownTemplate = `{{if .Description}}
{{.Description}}
{{end}}
{{if .Inventory.Container }}
### Goes in: {{LinkTo .Inventory.Container }}
{{end}}
{{if IsContainer .Identifier }}
## Contents
{{ ShowInventoryContentsOf .Identifier }}
{{ end }}
`

// BuildItemMarkdown creates the markdown content (without frontmatter) for an inventory item page.
func BuildItemMarkdown() string {
	var builder bytes.Buffer
	_, _ = builder.WriteString("# {{or .Title .Identifier}}")
	_, _ = builder.WriteString(newline)
	_, _ = builder.WriteString(ItemMarkdownTemplate)
	return builder.String()
}

// BuildItemPageText creates the full page text (frontmatter + markdown) for an inventory item.
func BuildItemPageText(fm map[string]any) (string, error) {
	fmBytes, err := toml.Marshal(fm)
	if err != nil {
		return "", fmt.Errorf("failed to marshal frontmatter to TOML: %w", err)
	}

	var builder bytes.Buffer

	if len(fmBytes) > 0 {
		_, _ = builder.WriteString(tomlDelimiter)
		_, _ = builder.Write(fmBytes)
		if !bytes.HasSuffix(fmBytes, []byte(newline)) {
			_, _ = builder.WriteString(newline)
		}
		_, _ = builder.WriteString(tomlDelimiter)
	}

	_, _ = builder.WriteString(newline)
	_, _ = builder.WriteString(BuildItemMarkdown())

	return builder.String(), nil
}
