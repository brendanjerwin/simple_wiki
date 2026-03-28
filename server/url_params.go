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
	frontmatter["identifier"] = identifier

	skipParams := map[string]bool{
		"tmpl": true,
	}

	for key, values := range params {
		if shouldSkipURLParam(key, skipParams) {
			continue
		}

		value, ok := resolveParamValue(values)
		if !ok {
			continue
		}

		if strings.Contains(key, ".") {
			if err := setNestedValue(frontmatter, key, value); err != nil {
				return nil, err
			}
		} else {
			if err := setFlatValue(frontmatter, key, value); err != nil {
				return nil, err
			}
		}
	}

	return frontmatter, nil
}

// shouldSkipURLParam returns true if the parameter should be excluded from frontmatter.
func shouldSkipURLParam(key string, skipParams map[string]bool) bool {
	return skipParams[key] || (len(key) > 0 && key[0] == '_') || key == "identifier"
}

// resolveParamValue returns the value to use for a parameter and whether it's valid.
func resolveParamValue(values []string) (any, bool) {
	switch len(values) {
	case 1:
		return values[0], true
	case 0:
		return nil, false
	default:
		return values, true
	}
}

// setFlatValue sets a simple (non-dotted) key in the frontmatter map, returning an error
// if the key already exists as a nested table structure.
func setFlatValue(frontmatter map[string]any, key string, value any) error {
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