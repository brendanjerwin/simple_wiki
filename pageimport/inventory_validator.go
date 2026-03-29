// Package pageimport provides CSV parsing and validation for bulk page imports.
package pageimport

import (
	"fmt"
	"strings"

	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
)

// PageExistenceChecker checks whether pages exist in the wiki.
type PageExistenceChecker interface {
	PageExists(identifier string) bool
}

// ContainerReferenceGetter retrieves the inventory.container value for a page.
type ContainerReferenceGetter interface {
	GetContainerReference(identifier string) string
}

// InventoryValidator validates inventory-specific constraints for page imports.
type InventoryValidator struct {
	pageChecker     PageExistenceChecker
	containerGetter ContainerReferenceGetter
}

// NewInventoryValidator creates a new InventoryValidator.
func NewInventoryValidator(pc PageExistenceChecker, cg ContainerReferenceGetter) *InventoryValidator {
	return &InventoryValidator{
		pageChecker:     pc,
		containerGetter: cg,
	}
}

// ValidateContainerIdentifier validates that inventory.container values are valid identifiers.
func (*InventoryValidator) ValidateContainerIdentifier(record *ParsedRecord) {
	containerValue := getInventoryContainer(record)
	if containerValue == "" {
		return
	}

	munged, err := wikiidentifiers.MungeIdentifier(containerValue)
	if err != nil {
		record.ValidationErrors = append(record.ValidationErrors,
			fmt.Sprintf("inventory.container '%s' is invalid: %v", containerValue, err))
		return
	}

	if munged != containerValue {
		record.ValidationErrors = append(record.ValidationErrors,
			fmt.Sprintf("inventory.container '%s' would be normalized to '%s'", containerValue, munged))
	}
}

// ValidateInventoryItemsIdentifiers validates that inventory.items[] values are valid identifiers.
func (*InventoryValidator) ValidateInventoryItemsIdentifiers(record *ParsedRecord) {
	for _, op := range record.ArrayOps {
		if op.FieldPath != "inventory.items" {
			continue
		}
		if op.Operation != EnsureExists {
			continue // Only validate items being added, not deleted
		}

		munged, err := wikiidentifiers.MungeIdentifier(op.Value)
		if err != nil {
			record.ValidationErrors = append(record.ValidationErrors,
				fmt.Sprintf("inventory.items[] value '%s' is invalid: %v", op.Value, err))
			continue
		}

		if munged != op.Value {
			record.ValidationErrors = append(record.ValidationErrors,
				fmt.Sprintf("inventory.items[] value '%s' would be normalized to '%s'", op.Value, munged))
		}
	}
}

// ValidateContainerExistence checks that referenced containers exist.
func (v *InventoryValidator) ValidateContainerExistence(records []ParsedRecord) {
	// Build set of munged identifiers being created in this import (excluding errored records)
	importIdentifiers := make(map[string]bool)
	for i := range records {
		record := &records[i]
		if record.Identifier == "" || record.HasErrors() {
			continue
		}
		munged, err := wikiidentifiers.MungeIdentifier(record.Identifier)
		if err != nil {
			continue
		}
		importIdentifiers[munged] = true
	}

	// Validate each record's container reference
	for i := range records {
		record := &records[i]
		containerValue := getInventoryContainer(record)
		if containerValue == "" {
			continue
		}

		// Munge the container value for comparison
		mungedContainer, err := wikiidentifiers.MungeIdentifier(containerValue)
		if err != nil {
			// Invalid identifier error already handled by ValidateContainerIdentifier
			continue
		}

		// Check if container exists in import OR in existing pages
		if !importIdentifiers[mungedContainer] && !v.pageChecker.PageExists(mungedContainer) {
			record.ValidationErrors = append(record.ValidationErrors,
				fmt.Sprintf("inventory.container references non-existent page '%s'", containerValue))
		}
	}
}

// DetectCircularReferences detects cycles in container relationships.
func (v *InventoryValidator) DetectCircularReferences(records []ParsedRecord) {
	importGraph := buildImportGraph(records)

	for i := range records {
		record := &records[i]
		if record.Identifier == "" || record.HasErrors() {
			continue
		}
		if getInventoryContainer(record) == "" {
			continue
		}
		mungedID, err := wikiidentifiers.MungeIdentifier(record.Identifier)
		if err != nil {
			continue
		}
		if cycle := v.findCycle(mungedID, importGraph); len(cycle) > 0 {
			record.ValidationErrors = append(record.ValidationErrors,
				fmt.Sprintf("circular reference detected: %s", strings.Join(cycle, " -> ")))
		}
	}
}

// buildImportGraph builds a container graph from import records (munged identifiers).
// Key: munged identifier, Value: munged container.
func buildImportGraph(records []ParsedRecord) map[string]string {
	importGraph := make(map[string]string)
	for i := range records {
		record := &records[i]
		if record.Identifier == "" || record.HasErrors() {
			continue
		}
		mungedID, err := wikiidentifiers.MungeIdentifier(record.Identifier)
		if err != nil {
			continue
		}
		containerValue := getInventoryContainer(record)
		if containerValue == "" {
			continue
		}
		mungedContainer, err := wikiidentifiers.MungeIdentifier(containerValue)
		if err != nil {
			continue
		}
		importGraph[mungedID] = mungedContainer
	}
	return importGraph
}

// findCycle performs DFS to find a cycle starting from startID.
func (v *InventoryValidator) findCycle(startID string, importGraph map[string]string) []string {
	visited := make(map[string]bool)
	path := []string{}

	current := startID
	for {
		if visited[current] {
			return extractCycle(path, current)
		}
		visited[current] = true
		path = append(path, current)
		current = v.resolveNextContainer(current, importGraph)
		if current == "" {
			return nil // No cycle - chain ends
		}
	}
}

// resolveNextContainer returns the next container in the chain for a given ID,
// checking the import graph first, then falling back to existing pages.
func (v *InventoryValidator) resolveNextContainer(current string, importGraph map[string]string) string {
	if container, inImport := importGraph[current]; inImport {
		return container
	}
	if v.containerGetter != nil {
		return v.containerGetter.GetContainerReference(current)
	}
	return ""
}

// extractCycle returns the cycle portion of a DFS path starting at the repeated node.
func extractCycle(path []string, current string) []string {
	for i, id := range path {
		if id == current {
			cycle := make([]string, len(path[i:])+1)
			copy(cycle, path[i:])
			cycle[len(cycle)-1] = current
			return cycle
		}
	}
	return nil
}

// getInventoryContainer extracts the inventory.container value from a record's frontmatter.
func getInventoryContainer(record *ParsedRecord) string {
	inventory, ok := record.Frontmatter["inventory"].(map[string]any)
	if !ok {
		return ""
	}
	container, ok := inventory["container"].(string)
	if !ok {
		return ""
	}
	return container
}
