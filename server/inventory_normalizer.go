package server

import (
	"fmt"
	"os"
	"strings"

	"github.com/brendanjerwin/simple_wiki/inventory"
	"github.com/brendanjerwin/simple_wiki/pkg/logging"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/stoewer/go-strcase"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// FailedPageCreation represents a page that failed to be created during normalization.
type FailedPageCreation struct {
	ItemID      string
	ContainerID string
	Error       error
}

// NormalizeResult contains the results of a page normalization operation.
type NormalizeResult struct {
	CreatedPages []string
	FailedPages  []FailedPageCreation
}

// InventoryNormalizer provides shared normalization logic for inventory pages.
// Both the full InventoryNormalizationJob and the per-page PageInventoryNormalizationJob use this.
type InventoryNormalizer struct {
	deps   InventoryNormalizationDependencies
	logger logging.Logger
}

// NewInventoryNormalizer creates a new InventoryNormalizer.
func NewInventoryNormalizer(deps InventoryNormalizationDependencies, logger logging.Logger) *InventoryNormalizer {
	return &InventoryNormalizer{
		deps:   deps,
		logger: logger,
	}
}

// NormalizePage runs normalization for a single page.
// If the page has inventory.items, it ensures is_container = true and creates missing item pages.
// Returns a NormalizeResult containing both created and failed pages.
func (n *InventoryNormalizer) NormalizePage(pageID wikipage.PageIdentifier) (*NormalizeResult, error) {
	result := &NormalizeResult{}

	// Step 1: Ensure is_container is set if page has items
	if err := n.ensureIsContainerField(pageID); err != nil {
		return nil, fmt.Errorf("failed to ensure is_container for page %s: %w", pageID, err)
	}

	// Step 2: Create missing item pages
	items, err := n.GetContainerItems(pageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get container items for %s: %w", pageID, err)
	}

	containerIDStr := string(pageID)
	for _, itemID := range items {
		// Check if page exists
		_, _, err := n.deps.ReadFrontMatter(itemID)
		if err == nil {
			continue // Page exists
		}

		// Create the missing page
		if createErr := n.CreateItemPage(itemID, containerIDStr); createErr != nil {
			result.FailedPages = append(result.FailedPages, FailedPageCreation{
				ItemID:      itemID,
				ContainerID: containerIDStr,
				Error:       createErr,
			})
			n.logger.Error("Failed to create page for item %s: %v", itemID, createErr)
		} else {
			result.CreatedPages = append(result.CreatedPages, itemID)
			n.logger.Info("Created page for item: %s in container: %s", itemID, pageID)
		}
	}

	return result, nil
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
	items, err := n.GetContainerItems(pageID)
	if err != nil {
		return fmt.Errorf("failed to get container items: %w", err)
	}
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
// Returns an error if any item identifier is invalid.
func (n *InventoryNormalizer) GetContainerItems(containerID wikipage.PageIdentifier) ([]string, error) {
	_, fm, err := n.deps.ReadFrontMatter(containerID)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Container doesn't exist, no items
		}
		return nil, fmt.Errorf("failed to read frontmatter for container %s: %w", containerID, err)
	}

	inventoryData, ok := fm["inventory"].(map[string]any)
	if !ok {
		return nil, nil // No inventory section
	}

	itemsRaw, ok := inventoryData["items"]
	if !ok {
		return nil, nil // No items array
	}

	// Handle both []string and []any - munge all identifiers for consistency
	var items []string
	switch v := itemsRaw.(type) {
	case []string:
		for _, s := range v {
			munged, err := wikiidentifiers.MungeIdentifier(s)
			if err != nil {
				return nil, fmt.Errorf("invalid item identifier %q in container %s: %w", s, containerID, err)
			}
			items = append(items, munged)
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				munged, err := wikiidentifiers.MungeIdentifier(s)
				if err != nil {
					return nil, fmt.Errorf("invalid item identifier %q in container %s: %w", s, containerID, err)
				}
				items = append(items, munged)
			}
		}
	}

	return items, nil
}

// CreateItemPage creates a new inventory item page.
func (n *InventoryNormalizer) CreateItemPage(itemID, containerID string) error {
	identifier, err := wikiidentifiers.MungeIdentifier(itemID)
	if err != nil {
		return fmt.Errorf("invalid item identifier %q: %w", itemID, err)
	}

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
		mungedContainer, err := wikiidentifiers.MungeIdentifier(containerID)
		if err != nil {
			return fmt.Errorf("invalid container identifier %q: %w", containerID, err)
		}
		inventoryData["container"] = mungedContainer
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
