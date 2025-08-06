package lazy

import (
	"regexp"
	"strings"
)

type TOMLTableSpacingMigration struct{}

func NewTOMLTableSpacingMigration() *TOMLTableSpacingMigration {
	return &TOMLTableSpacingMigration{}
}

func (*TOMLTableSpacingMigration) SupportedTypes() []FrontmatterType {
	return []FrontmatterType{FrontmatterTOML}
}

func (*TOMLTableSpacingMigration) AppliesTo(content []byte) bool {
	frontmatter := extractTOMLFrontmatter(content)
	if frontmatter == "" {
		return false
	}

	lines := strings.Split(frontmatter, newlineChar)
	tableRegex := regexp.MustCompile(`^\[([^\]]+)\]`)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check if this is a table header
		if tableRegex.MatchString(trimmed) {
			// Check if it's the first line (no blank line needed)
			if i == 0 {
				continue
			}

			// Check if previous line is blank
			prevLineIndex := i - 1
			for prevLineIndex >= 0 && strings.TrimSpace(lines[prevLineIndex]) == "" {
				prevLineIndex--
			}

			// If we found a non-empty line immediately before the table header
			if prevLineIndex >= 0 && prevLineIndex == i-1 {
				return true
			}
		}
	}

	return false
}

func (*TOMLTableSpacingMigration) Apply(content []byte) ([]byte, error) {
	parts := splitTOMLContent(content)
	if len(parts) != tomlDelimiterLength {
		// Not proper TOML frontmatter format
		return content, nil
	}

	frontmatter := parts[1]
	transformedFrontmatter := addBlankLinesBeforeTables(frontmatter)

	// Reconstruct the content - parts[1] includes trailing newline, don't add extra
	result := tomlDelimiter + transformedFrontmatter + tomlDelimiter + newlineChar + parts[2]
	return []byte(result), nil
}

func addBlankLinesBeforeTables(frontmatter string) string {
	lines := strings.Split(frontmatter, newlineChar)
	var result []string
	tableRegex := regexp.MustCompile(`^\[([^\]]+)\]`)

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Check if this is a table header and not the first line
		if tableRegex.MatchString(trimmed) && i > 0 {
			// Check if previous line is not blank
			prevLineIndex := i - 1
			for prevLineIndex >= 0 && strings.TrimSpace(lines[prevLineIndex]) == "" {
				prevLineIndex--
			}

			// If we found a non-empty line immediately before the table header, add blank line
			if prevLineIndex >= 0 && prevLineIndex == i-1 {
				result = append(result, "")
			}
		}

		result = append(result, line)
	}

	return strings.Join(result, newlineChar)
}