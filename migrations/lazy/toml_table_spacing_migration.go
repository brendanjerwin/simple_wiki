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

	for i := range lines {
		if tableHeaderNeedsBlankLineBefore(lines, i, tableRegex) {
			return true
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

// needsBlankLineBefore returns true if there is a non-blank line immediately before line i.
func needsBlankLineBefore(lines []string, i int) bool {
	if i == 0 {
		return false
	}
	prevLineIndex := i - 1
	for prevLineIndex >= 0 && strings.TrimSpace(lines[prevLineIndex]) == "" {
		prevLineIndex--
	}
	return prevLineIndex >= 0 && prevLineIndex == i-1
}

// tableHeaderNeedsBlankLineBefore returns true if line i is a TOML table header
// that has a non-blank line immediately before it (i.e., it needs a blank line inserted).
func tableHeaderNeedsBlankLineBefore(lines []string, i int, tableRegex *regexp.Regexp) bool {
	trimmed := strings.TrimSpace(lines[i])
	if !tableRegex.MatchString(trimmed) {
		return false
	}
	return needsBlankLineBefore(lines, i)
}

func addBlankLinesBeforeTables(frontmatter string) string {
	lines := strings.Split(frontmatter, newlineChar)
	var result []string
	tableRegex := regexp.MustCompile(`^\[([^\]]+)\]`)

	for i, line := range lines {
		if tableHeaderNeedsBlankLineBefore(lines, i, tableRegex) {
			result = append(result, "")
		}
		result = append(result, line)
	}

	return strings.Join(result, newlineChar)
}