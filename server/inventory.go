package server

import (
	"fmt"

	"github.com/brendanjerwin/simple_wiki/inventory"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/brendanjerwin/simple_wiki/wikipage"
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

// InventoryItemMarkdownTemplate is deprecated. Use inventory.ItemMarkdownTemplate instead.
// This is kept for backwards compatibility.
const InventoryItemMarkdownTemplate = inventory.ItemMarkdownTemplate

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

	// Set up inventory structure - only add container reference, not items array
	// Items array and is_container are only for actual containers
	inventoryData := make(map[string]any)
	if params.Container != "" {
		inventoryData["container"] = wikiidentifiers.MungeIdentifier(params.Container)
	}
	fm[inventoryKeyPath] = inventoryData

	// Build page content
	pageText, err := inventory.BuildItemPageText(fm)
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

// EnsureInventoryFrontmatterStructure ensures the frontmatter has the proper inventory structure.
// This is used when creating inventory items from URL params.
// Note: This only ensures the inventory map exists, not the items array.
// Items array and is_container are only added for actual containers.
func EnsureInventoryFrontmatterStructure(fm map[string]any) {
	if _, exists := fm[inventoryKeyPath]; !exists {
		fm[inventoryKeyPath] = make(map[string]any)
	}
}
