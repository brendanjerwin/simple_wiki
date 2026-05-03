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
//
// One exception: when the previous non-blank line is itself a table header that
// is an ancestor of this one (e.g. [wiki] immediately followed by [wiki.connectors]),
// we don't insert a blank line. pelletier/go-toml/v2 marshals nested empty parent
// tables as consecutive headers; inserting blank lines between them creates a
// write-loop because every connector write strips them again on re-marshal.
func tableHeaderNeedsBlankLineBefore(lines []string, i int, tableRegex *regexp.Regexp) bool {
	trimmed := strings.TrimSpace(lines[i])
	matches := tableRegex.FindStringSubmatch(trimmed)
	if matches == nil {
		return false
	}
	if !needsBlankLineBefore(lines, i) {
		return false
	}
	// If the immediately-previous line is a table header that is an ancestor of
	// this one (parent or higher), skip the blank-line requirement.
	prevTrimmed := strings.TrimSpace(lines[i-1])
	prevMatches := tableRegex.FindStringSubmatch(prevTrimmed)
	if prevMatches != nil && isAncestorTablePath(prevMatches[1], matches[1]) {
		return false
	}
	return true
}

// isAncestorTablePath reports whether ancestor is a strict ancestor of descendant
// in TOML dotted-path semantics. Both inputs are the un-bracketed table key
// (e.g. "wiki" and "wiki.connectors"). Array-of-tables headers ([[...]]) are
// captured by the same regex with a leading "[" — those don't form ancestor
// relationships with regular tables, so they fall through to the default
// (blank-line required), which matches existing behavior.
func isAncestorTablePath(ancestor, descendant string) bool {
	if ancestor == "" || descendant == "" {
		return false
	}
	if strings.HasPrefix(ancestor, "[") || strings.HasPrefix(descendant, "[") {
		return false
	}
	if ancestor == descendant {
		return false
	}
	prefix := ancestor + "."
	return strings.HasPrefix(descendant, prefix)
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