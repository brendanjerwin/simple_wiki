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
		if shouldSkipParam(key, skipParams) {
			continue
		}

		value, ok := extractParamValue(values)
		if !ok {
			continue
		}

		if err := setFrontmatterParam(frontmatter, key, value); err != nil {
			return nil, err
		}
	}

	return frontmatter, nil
}

// shouldSkipParam returns true if the parameter should be excluded from frontmatter.
func shouldSkipParam(key string, skipParams map[string]bool) bool {
	return skipParams[key] || (len(key) > 0 && key[0] == '_') || key == "identifier"
}

// extractParamValue returns the value to use for a parameter and whether it should be processed.
func extractParamValue(values []string) (any, bool) {
	if len(values) == 1 {
		return values[0], true
	}
	if len(values) > 1 {
		return values, true
	}
	return nil, false
}

// setFrontmatterParam sets a key-value pair in the frontmatter map, handling dotted keys and conflict detection.
func setFrontmatterParam(frontmatter map[string]any, key string, value any) error {
	if strings.Contains(key, ".") {
		return setNestedValue(frontmatter, key, value)
	}
	if existing, exists := frontmatter[key]; exists {
		if _, isMap := existing.(map[string]any); isMap {
			return fmt.Errorf("parameter '%s' cannot be both a value and a table", key)
		}
	}
	frontmatter[key] = value
	return nil
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