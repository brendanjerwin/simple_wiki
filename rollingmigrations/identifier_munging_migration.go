package rollingmigrations

import (
	"bytes"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
)

// IdentifierMungingMigration ensures identifier values are munged.
type IdentifierMungingMigration struct{}

func NewIdentifierMungingMigration() *IdentifierMungingMigration {
	return &IdentifierMungingMigration{}
}

func (*IdentifierMungingMigration) SupportedTypes() []FrontmatterType {
	return []FrontmatterType{FrontmatterTOML}
}

func (*IdentifierMungingMigration) AppliesTo(content []byte) bool {
	// Extract frontmatter using existing helper
	frontmatter := extractTOMLFrontmatter(content)
	if frontmatter == "" {
		return false
	}

	// Parse TOML to check for identifier
	var data map[string]any
	if err := toml.Unmarshal([]byte(frontmatter), &data); err != nil {
		return false
	}

	// Check if identifier exists and needs munging
	identifier, ok := data["identifier"].(string)
	if !ok {
		return false
	}

	// Apply if the identifier value is different when munged
	munged := wikiidentifiers.MungeIdentifier(identifier)
	return identifier != munged
}

func (*IdentifierMungingMigration) Apply(content []byte) ([]byte, error) {
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

	// Check if identifier exists
	identifier, ok := data["identifier"].(string)
	if !ok {
		return content, nil // No identifier field
	}

	// Munge the identifier value
	munged := wikiidentifiers.MungeIdentifier(identifier)
	if identifier == munged {
		return content, nil // Already munged
	}

	// Update the identifier value
	data["identifier"] = munged

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