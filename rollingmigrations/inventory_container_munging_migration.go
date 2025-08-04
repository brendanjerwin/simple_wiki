package rollingmigrations

import (
	"bytes"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
)

// InventoryContainerMungingMigration ensures inventory.container values are munged.
type InventoryContainerMungingMigration struct{}

func NewInventoryContainerMungingMigration() *InventoryContainerMungingMigration {
	return &InventoryContainerMungingMigration{}
}

func (*InventoryContainerMungingMigration) SupportedTypes() []FrontmatterType {
	return []FrontmatterType{FrontmatterTOML}
}

func (*InventoryContainerMungingMigration) AppliesTo(content []byte) bool {
	// Extract frontmatter using existing helper
	frontmatter := extractTOMLFrontmatter(content)
	if frontmatter == "" {
		return false
	}

	// Parse TOML to check for inventory.container
	var data map[string]any
	if err := toml.Unmarshal([]byte(frontmatter), &data); err != nil {
		return false
	}

	// Check if inventory.container exists and needs munging
	inventory, ok := data["inventory"].(map[string]any)
	if !ok {
		return false
	}

	container, ok := inventory["container"].(string)
	if !ok {
		return false
	}

	// Apply if the container value is different when munged
	munged := wikiidentifiers.MungeIdentifier(container)
	return container != munged
}

func (*InventoryContainerMungingMigration) Apply(content []byte) ([]byte, error) {
	// Extract frontmatter and body using existing helper
	parts := splitTOMLContent(content)
	if len(parts) != tomlDelimiterLength {
		return content, nil // Invalid format, return unchanged
	}
	
	frontmatter := parts[1]
	body := parts[2]

	// Parse TOML
	var data map[string]any
	if err := toml.Unmarshal([]byte(frontmatter), &data); err != nil {
		return content, err
	}

	// Check if inventory.container exists
	inventory, ok := data["inventory"].(map[string]any)
	if !ok {
		return content, nil // No inventory section
	}

	container, ok := inventory["container"].(string)
	if !ok {
		return content, nil // No container field
	}

	// Munge the container value
	munged := wikiidentifiers.MungeIdentifier(container)
	if container == munged {
		return content, nil // Already munged
	}

	// Update the container value
	inventory["container"] = munged

	// Marshal back to TOML
	var buf bytes.Buffer
	encoder := toml.NewEncoder(&buf)
	if err := encoder.Encode(data); err != nil {
		return content, err
	}

	// Reconstruct the full content
	newFrontmatter := strings.TrimSpace(buf.String())
	result := "+++\n" + newFrontmatter + "\n+++\n" + body
	return []byte(result), nil
}