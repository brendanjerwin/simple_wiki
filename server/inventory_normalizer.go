package server

import (
	"errors"
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

// inventoryNormalizerPageModifier is the atomic modifier contract for
// ensureIsContainerField. Implemented by pagestore.Store but not by the
// PageReaderMutator interface, so it is accessed via type assertion.
type inventoryNormalizerPageModifier interface {
	ModifyFrontMatterAndMarkdown(id wikipage.PageIdentifier, fn func(wikipage.FrontMatter, wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error), identity wikipage.Identity) error
}

// errInventorySkipWrite is returned from a ModifyFrontMatterAndMarkdown callback
// to abort the write without propagating an error to the caller.
// Used when the callback determines there is nothing to update (page does not
// exist, no inventory section, is_container already set, etc.).
var errInventorySkipWrite = errors.New("inventory: skip write")

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
		_, _, err := n.deps.ReadFrontMatter(wikipage.PageIdentifier(itemID))
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
// Uses ModifyFrontMatterAndMarkdown to perform the read-modify-write atomically
// under the page lock, preventing TOCTOU races with concurrent writers.
func (n *InventoryNormalizer) ensureIsContainerField(pageID wikipage.PageIdentifier) error {
	modifier, ok := n.deps.(inventoryNormalizerPageModifier)
	if !ok {
		return errors.New("page store does not support atomic frontmatter modification")
	}

	var updated bool
	err := modifier.ModifyFrontMatterAndMarkdown(pageID, func(fm wikipage.FrontMatter, md wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error) {
		if fm == nil {
			return nil, md, errInventorySkipWrite // page does not exist
		}

		inventoryData, ok := fm["inventory"].(map[string]any)
		if !ok {
			return fm, md, errInventorySkipWrite // No inventory section — no-op
		}

		itemsRaw, ok := inventoryData["items"]
		if !ok {
			return fm, md, errInventorySkipWrite // No items array — no-op
		}

		items, err := mungeItemIdentifiers(itemsRaw, pageID)
		if err != nil {
			return nil, md, fmt.Errorf("failed to get container items: %w", err)
		}
		if len(items) == 0 {
			return fm, md, errInventorySkipWrite // Empty items — no-op
		}

		if isContainer, ok := inventoryData["is_container"].(bool); ok && isContainer {
			return fm, md, errInventorySkipWrite // Already set — no-op
		}

		inventoryData["is_container"] = true
		updated = true
		return fm, md, nil
	}, wikipage.AnonymousIdentity)

	if errors.Is(err, errInventorySkipWrite) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to write frontmatter: %w", err)
	}

	if updated {
		n.logger.Info("Set is_container = true for page: %s", pageID)
	}
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

	return mungeItemIdentifiers(itemsRaw, containerID)
}

// mungeItemIdentifiers munges all item identifiers from a raw items value.
// Handles both []string and []any typed values.
// Returns an error if any item identifier is invalid.
func mungeItemIdentifiers(itemsRaw any, containerID wikipage.PageIdentifier) ([]string, error) {
	switch v := itemsRaw.(type) {
	case []string:
		return mungeStringSliceItems(v, containerID)
	case []any:
		return mungeAnySliceItems(v, containerID)
	default:
		return nil, nil
	}
}

// mungeStringSliceItems munges identifiers from a []string items array.
func mungeStringSliceItems(items []string, containerID wikipage.PageIdentifier) ([]string, error) {
	result := make([]string, 0, len(items))
	for _, s := range items {
		munged, err := wikiidentifiers.MungeIdentifier(s)
		if err != nil {
			return nil, fmt.Errorf("invalid item identifier %q in container %s: %w", s, containerID, err)
		}
		result = append(result, munged)
	}
	return result, nil
}

// mungeAnySliceItems munges identifiers from a []any items array, skipping non-string entries.
func mungeAnySliceItems(items []any, containerID wikipage.PageIdentifier) ([]string, error) {
	var result []string
	for _, item := range items {
		s, ok := item.(string)
		if !ok {
			continue
		}
		munged, err := wikiidentifiers.MungeIdentifier(s)
		if err != nil {
			return nil, fmt.Errorf("invalid item identifier %q in container %s: %w", s, containerID, err)
		}
		result = append(result, munged)
	}
	return result, nil
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
	if err := n.deps.WriteFrontMatter(wikipage.PageIdentifier(identifier), fm, wikipage.AnonymousIdentity); err != nil {
		return fmt.Errorf("failed to write frontmatter: %w", err)
	}

	// Build and write markdown
	markdown := inventory.BuildItemMarkdown()
	if err := n.deps.WriteMarkdown(wikipage.PageIdentifier(identifier), wikipage.Markdown(markdown), wikipage.AnonymousIdentity); err != nil {
		return fmt.Errorf("failed to write markdown: %w", err)
	}

	return nil
}
