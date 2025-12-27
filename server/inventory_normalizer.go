package server

import (
	"fmt"
	"strings"

	"github.com/brendanjerwin/simple_wiki/inventory"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
	"github.com/stoewer/go-strcase"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// InventoryNormalizer provides shared normalization logic for inventory pages.
// Both the full InventoryNormalizationJob and the per-page PageInventoryNormalizationJob use this.
type InventoryNormalizer struct {
	deps   InventoryNormalizationDependencies
	logger lumber.Logger
}

// NewInventoryNormalizer creates a new InventoryNormalizer.
func NewInventoryNormalizer(deps InventoryNormalizationDependencies, logger lumber.Logger) *InventoryNormalizer {
	return &InventoryNormalizer{
		deps:   deps,
		logger: logger,
	}
}

// NormalizePage runs normalization for a single page.
// If the page has inventory.items, it ensures is_container = true and creates missing item pages.
// Returns the list of pages created.
func (n *InventoryNormalizer) NormalizePage(pageID wikipage.PageIdentifier) ([]string, error) {
	var createdPages []string

	// Step 1: Ensure is_container is set if page has items
	if err := n.ensureIsContainerField(pageID); err != nil {
		n.logger.Error("Failed to ensure is_container for page %s: %v", pageID, err)
	}

	// Step 2: Create missing item pages
	items := n.GetContainerItems(pageID)
	for _, itemID := range items {
		// Check if page exists
		_, _, err := n.deps.ReadFrontMatter(itemID)
		if err == nil {
			continue // Page exists
		}

		// Create the missing page
		if createErr := n.CreateItemPage(itemID, string(pageID)); createErr != nil {
			n.logger.Error("Failed to create page for item %s: %v", itemID, createErr)
		} else {
			createdPages = append(createdPages, itemID)
			n.logger.Info("Created page for item: %s in container: %s", itemID, pageID)
		}
	}

	return createdPages, nil
}

// ensureIsContainerField sets is_container = true if the page has inventory.items.
func (n *InventoryNormalizer) ensureIsContainerField(pageID wikipage.PageIdentifier) error {
	_, fm, err := n.deps.ReadFrontMatter(pageID)
	if err != nil {
		return nil // Page doesn't exist, nothing to do
	}

	// Check if page has inventory.items with actual content
	inventoryData, ok := fm["inventory"].(map[string]any)
	if !ok {
		return nil // No inventory section
	}

	// Check if items array exists and is non-empty
	items := n.GetContainerItems(pageID)
	if len(items) == 0 {
		return nil // No items or empty items array
	}

	// Check if is_container is already set
	if isContainer, ok := inventoryData["is_container"].(bool); ok && isContainer {
		return nil // Already set
	}

	// Set is_container = true
	inventoryData["is_container"] = true

	// Write back frontmatter
	if err := n.deps.WriteFrontMatter(pageID, fm); err != nil {
		return fmt.Errorf("failed to write frontmatter: %w", err)
	}

	n.logger.Info("Set is_container = true for page: %s", pageID)
	return nil
}

// GetContainerItems gets items listed in a container's inventory.items array.
func (n *InventoryNormalizer) GetContainerItems(containerID wikipage.PageIdentifier) []string {
	_, fm, err := n.deps.ReadFrontMatter(containerID)
	if err != nil {
		return nil
	}

	inventoryData, ok := fm["inventory"].(map[string]any)
	if !ok {
		return nil
	}

	itemsRaw, ok := inventoryData["items"]
	if !ok {
		return nil
	}

	// Handle both []string and []any
	var items []string
	switch v := itemsRaw.(type) {
	case []string:
		items = v
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				items = append(items, wikiidentifiers.MungeIdentifier(s))
			}
		}
	}

	return items
}

// CreateItemPage creates a new inventory item page.
func (n *InventoryNormalizer) CreateItemPage(itemID, containerID string) error {
	identifier := wikiidentifiers.MungeIdentifier(itemID)

	// Build frontmatter
	fm := make(map[string]any)
	fm["identifier"] = identifier

	// Generate title from identifier
	titleCaser := cases.Title(language.AmericanEnglish)
	snaked := strcase.SnakeCase(identifier)
	// Replace underscores with spaces for a nicer title
	titleStr := strings.ReplaceAll(snaked, "_", " ")
	fm["title"] = titleCaser.String(titleStr)

	// Set up inventory structure - only add container reference, not items array
	// Items array and is_container are only for actual containers
	inventoryData := make(map[string]any)
	if containerID != "" {
		inventoryData["container"] = wikiidentifiers.MungeIdentifier(containerID)
	}
	fm["inventory"] = inventoryData

	// Write frontmatter
	if err := n.deps.WriteFrontMatter(identifier, fm); err != nil {
		return fmt.Errorf("failed to write frontmatter: %w", err)
	}

	// Build and write markdown
	markdown := inventory.BuildItemMarkdown()
	if err := n.deps.WriteMarkdown(identifier, markdown); err != nil {
		return fmt.Errorf("failed to write markdown: %w", err)
	}

	return nil
}
