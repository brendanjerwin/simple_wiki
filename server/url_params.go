package server

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// BuildFrontmatterFromURLParams converts URL query parameters into a frontmatter map suitable for TOML marshaling.
// It handles dotted keys (e.g., "inventory.container") by creating nested map structures.
// Special parameters that shouldn't be in frontmatter (like "tmpl") are filtered out.
func BuildFrontmatterFromURLParams(identifier string, params url.Values) (wikipage.FrontMatter, error) {
	frontmatter := make(wikipage.FrontMatter)
	
	// Always include the identifier
	frontmatter["identifier"] = identifier
	
	// List of parameters to skip
	skipParams := map[string]bool{
		"tmpl": true,
		// Add other special parameters here as needed
	}
	
	// Process each parameter
	for key, values := range params {
		// Skip special parameters
		if skipParams[key] {
			continue
		}
		
		// Skip parameters starting with underscore
		if len(key) > 0 && key[0] == '_' {
			continue
		}
		
		// Skip identifier parameter (we already set it from the function argument)
		if key == "identifier" {
			continue
		}
		
		// Determine the value to use
		var value any
		if len(values) == 1 {
			value = values[0]
		} else if len(values) > 1 {
			value = values
		} else {
			continue // Skip empty values
		}
		
		// Handle dotted keys
		if strings.Contains(key, ".") {
			if err := setNestedValue(frontmatter, key, value); err != nil {
				return nil, err
			}
		} else {
			frontmatter[key] = value
		}
	}
	
	return frontmatter, nil
}

// setNestedValue sets a value in a nested map structure based on a dotted key path.
// For example, "inventory.container" creates {"inventory": {"container": value}}
// Returns an error if there's a conflict between a simple value and a nested structure.
func setNestedValue(m map[string]any, dottedKey string, value any) error {
	parts := strings.Split(dottedKey, ".")
	current := m
	
	// Navigate/create the nested structure
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		
		if existing, exists := current[part]; exists {
			// If it exists and is a map, use it
			nestedMap, ok := existing.(map[string]any)
			if !ok {
				// If it exists but is not a map, this is an error - TOML cannot have both
				// a simple value and a table with the same key
				return fmt.Errorf("parameter '%s' cannot be both a value and a table", part)
			}
			current = nestedMap
		} else {
			// Create a new nested map
			newMap := make(map[string]any)
			current[part] = newMap
			current = newMap
		}
	}
	
	// Set the final value
	finalKey := parts[len(parts)-1]
	current[finalKey] = value
	return nil
}