package rollingmigrations

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v2"
)

const (
	yamlDelimiter       = "---"
	yamlDelimiterLine   = "---\n"
	tomlDelimiterLine   = "+++\n"
	yamlDelimiterLength = 3
	closingDelimiterLen = 5 // Length of "\n---\n"
	yamlEndOnlyLength   = 4 // Length of "\n---"
)

type YAMLToTOMLMigration struct{}

func NewYAMLToTOMLMigration() *YAMLToTOMLMigration {
	return &YAMLToTOMLMigration{}
}

func (*YAMLToTOMLMigration) SupportedTypes() []FrontmatterType {
	return []FrontmatterType{FrontmatterYAML}
}

func (*YAMLToTOMLMigration) AppliesTo(content []byte) bool {
	if len(content) < yamlDelimiterLength {
		return false
	}
	return bytes.HasPrefix(content, []byte(yamlDelimiter))
}

func (*YAMLToTOMLMigration) Apply(content []byte) ([]byte, error) {
	if !bytes.HasPrefix(content, []byte(yamlDelimiter)) {
		// Not YAML frontmatter, return unchanged
		return content, nil
	}

	parts, err := extractYAMLFrontmatter(content)
	if err != nil {
		return content, fmt.Errorf("failed to extract YAML frontmatter: %w", err)
	}

	if parts.frontmatter == "" {
		// No frontmatter content, return unchanged
		return content, nil
	}

	// Parse YAML frontmatter 
	var yamlData any
	if err := yaml.Unmarshal([]byte(parts.frontmatter), &yamlData); err != nil {
		return content, fmt.Errorf("failed to parse YAML frontmatter: %w", err)
	}

	// Convert all maps to have string keys for TOML compatibility
	convertedData := convertMapsToStringKeys(yamlData)

	// Convert to TOML
	tomlData, err := toml.Marshal(convertedData)
	if err != nil {
		return content, fmt.Errorf("failed to convert to TOML: %w", err)
	}

	// Reconstruct the content with TOML frontmatter
	var result strings.Builder
	_, _ = result.WriteString(tomlDelimiterLine)
	_, _ = result.Write(tomlData)
	_, _ = result.WriteString(tomlDelimiterLine)
	_, _ = result.WriteString(parts.body)

	return []byte(result.String()), nil
}

type yamlParts struct {
	frontmatter string
	body        string
}

func extractYAMLFrontmatter(content []byte) (*yamlParts, error) {
	str := string(content)
	
	if !strings.HasPrefix(str, yamlDelimiter) {
		return nil, errors.New("content does not start with YAML delimiter")
	}

	// Find the closing delimiter
	rest := str[len(yamlDelimiter):]
	if !strings.HasPrefix(rest, "\n") {
		return nil, errors.New("YAML delimiter not followed by newline")
	}
	
	rest = rest[1:] // Skip the newline after opening delimiter
	
	closingIndex := strings.Index(rest, "\n"+yamlDelimiter)
	if closingIndex == -1 {
		return nil, errors.New("closing YAML delimiter not found")
	}

	frontmatter := rest[:closingIndex]
	
	// Find the body content after the closing delimiter
	// remainingContent starts with "\n---\n" or "\n---"
	remainingContent := rest[closingIndex:]
	
	// Look for content after the closing delimiter
	// We expect format like: "\n---\n<body content>"
	if strings.HasPrefix(remainingContent, "\n---\n") {
		// Normal case with body content
		body := remainingContent[closingDelimiterLen:] // Skip "\n---\n"
		return &yamlParts{
			frontmatter: frontmatter,
			body:        body,
		}, nil
	} else if strings.HasPrefix(remainingContent, "\n---") && len(remainingContent) == yamlEndOnlyLength {
		// Case where there's no content after frontmatter
		return &yamlParts{
			frontmatter: frontmatter,
			body:        "",
		}, nil
	}
	
	// Fallback for other cases
	return &yamlParts{
		frontmatter: frontmatter,
		body:        "",
	}, nil
}

// convertMapsToStringKeys recursively converts all map[any]any to map[string]any
// This is necessary because YAML unmarshaling creates any keys but TOML requires string keys
func convertMapsToStringKeys(data any) any {
	switch v := data.(type) {
	case map[any]any:
		// Convert map[any]any to map[string]any
		result := make(map[string]any)
		for key, value := range v {
			// Convert key to string
			strKey := fmt.Sprintf("%v", key)
			result[strKey] = convertMapsToStringKeys(value)
		}
		return result
	case map[string]any:
		// Already has string keys, but process values recursively
		result := make(map[string]any)
		for key, value := range v {
			result[key] = convertMapsToStringKeys(value)
		}
		return result
	case []any:
		// Process slice elements recursively
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = convertMapsToStringKeys(item)
		}
		return result
	default:
		// Return primitive types as-is
		return v
	}
}