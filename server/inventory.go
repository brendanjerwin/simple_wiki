package server

import (
	"bytes"
	"fmt"

	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/pelletier/go-toml/v2"
	"github.com/stoewer/go-strcase"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const (
	inventoryKeyPath = "inventory"
)

// InventoryItemParams contains the parameters for creating an inventory item page.
type InventoryItemParams struct {
	Identifier string
	Container  string // Optional: the container this item belongs to
	Title      string // Optional: human-readable title (auto-generated if empty)
}

// InventoryItemMarkdownTemplate is the markdown template for inventory item pages.
// It is exported so it can be used by both the server and the gRPC API layer.
const InventoryItemMarkdownTemplate = `
{{if IsContainer .Identifier }}
## Contents
{{ ShowInventoryContentsOf .Identifier }}
{{ end }}
`

// CreateInventoryItemPage creates a new inventory item page with the inv_item template structure.
// If the page already exists, it returns an error.
func (s *Site) CreateInventoryItemPage(params InventoryItemParams) (*wikipage.Page, error) {
	// Munge the identifier to ensure consistency
	identifier := wikiidentifiers.MungeIdentifier(params.Identifier)

	// Check if page already exists
	p, err := s.ReadPage(identifier)
	if err != nil {
		return nil, fmt.Errorf("failed to read page %s: %w", identifier, err)
	}

	if !p.IsNew() {
		return nil, fmt.Errorf("page already exists: %s", identifier)
	}

	// Build frontmatter
	fm := make(map[string]any)
	fm["identifier"] = identifier

	// Set title - auto-generate from identifier if not provided
	title := params.Title
	if title == "" {
		titleCaser := cases.Title(language.AmericanEnglish)
		title = titleCaser.String(strcase.SnakeCase(identifier))
	}
	fm["title"] = title

	// Set up inventory structure
	inventory := make(map[string]any)
	if params.Container != "" {
		inventory["container"] = wikiidentifiers.MungeIdentifier(params.Container)
	}
	inventory["items"] = []string{}
	fm[inventoryKeyPath] = inventory

	// Build page content
	pageText, err := buildInventoryItemPageText(fm)
	if err != nil {
		return nil, fmt.Errorf("failed to build page text: %w", err)
	}

	p.Text = pageText

	// Render the page
	if renderErr := p.Render(s, s.MarkdownRenderer, TemplateExecutor{}, s.FrontmatterIndexQueryer); renderErr != nil {
		s.Logger.Error("Error rendering new inventory item page: %v", renderErr)
	}

	// Save the page
	if err := s.savePageAndIndex(p); err != nil {
		return nil, fmt.Errorf("failed to save inventory item page '%s': %w", identifier, err)
	}

	return p, nil
}

// buildInventoryItemPageText creates the full page text for an inventory item.
func buildInventoryItemPageText(fm map[string]any) (string, error) {
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
	_, _ = builder.WriteString("# {{or .Title .Identifier}}")
	_, _ = builder.WriteString(newline)
	_, _ = builder.WriteString(InventoryItemMarkdownTemplate)

	return builder.String(), nil
}

// EnsureInventoryFrontmatterStructure ensures the frontmatter has the proper inventory structure.
// This is used when creating inventory items from URL params.
func EnsureInventoryFrontmatterStructure(fm map[string]any) {
	if _, exists := fm[inventoryKeyPath]; !exists {
		fm[inventoryKeyPath] = make(map[string]any)
	}
	if inventory, ok := fm[inventoryKeyPath].(map[string]any); ok {
		if _, exists := inventory["items"]; !exists {
			inventory["items"] = []string{}
		}
	}
}
