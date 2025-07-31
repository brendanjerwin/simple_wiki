package rollingmigrations

import (
	"regexp"
	"sort"
	"strings"
)

const (
	// TOML frontmatter delimiter length (length of "+++")
	tomlDelimiterLength = 3
	// TOML frontmatter start position after delimiter and newline (length of "\n+++\n")
	tomlStartOffset = 5
	// Newline character for TOML formatting
	newlineChar = "\n"
	// Dot separator for TOML key paths
	dotSeparator = "."
	// TOML frontmatter delimiter
	tomlDelimiter = "+++"
)

type TOMLDotNotationMigration struct{}

func NewTOMLDotNotationMigration() *TOMLDotNotationMigration {
	return &TOMLDotNotationMigration{}
}

func (*TOMLDotNotationMigration) SupportedTypes() []FrontmatterType {
	return []FrontmatterType{FrontmatterTOML}
}

func (*TOMLDotNotationMigration) AppliesTo(content []byte) bool {
	frontmatter := extractTOMLFrontmatter(content)
	if frontmatter == "" {
		return false
	}

	// Apply migration if any dot notation is found - we want consistent table syntax
	dotNotationPrefixes := findDotNotationPrefixes(frontmatter)
	return len(dotNotationPrefixes) > 0
}

func (*TOMLDotNotationMigration) Apply(content []byte) ([]byte, error) {
	parts := splitTOMLContent(content)
	if len(parts) != tomlDelimiterLength {
		// Not proper TOML frontmatter format
		return content, nil
	}

	frontmatter := parts[1]
	transformedFrontmatter := transformTOMLDotNotation(frontmatter)

	// Reconstruct the content - ensure proper newline formatting
	result := tomlDelimiter + newlineChar + transformedFrontmatter + newlineChar + tomlDelimiter + newlineChar + parts[2]
	return []byte(result), nil
}

// Helper functions

func extractTOMLFrontmatter(content []byte) string {
	parts := splitTOMLContent(content)
	if len(parts) != tomlDelimiterLength {
		return ""
	}
	return parts[1]
}

func splitTOMLContent(content []byte) []string {
	str := string(content)
	if !strings.HasPrefix(str, tomlDelimiter) {
		return nil
	}

	// Validate string length before slicing to prevent panic
	if len(str) < tomlDelimiterLength {
		return nil
	}

	// Find the closing +++
	rest := str[tomlDelimiterLength:]
	closingIndex := strings.Index(rest, "\n+++\n")
	if closingIndex == -1 {
		return nil
	}

	frontmatter := rest[:closingIndex+1] // Include the newline before +++
	bodyContent := rest[closingIndex+tomlStartOffset:] // Skip "\n+++\n"

	return []string{tomlDelimiter, frontmatter, bodyContent}
}

func findDotNotationPrefixes(frontmatter string) []string {
	dotRegex := regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*(?:\.[a-zA-Z_][a-zA-Z0-9_]*)+)\s*=`)
	lines := strings.Split(frontmatter, newlineChar)
	prefixSet := make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		matches := dotRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			dotKey := matches[1]
			// Extract the top-level prefix (everything before the first dot)
			parts := strings.Split(dotKey, dotSeparator)
			if len(parts) > 1 {
				prefix := strings.Join(parts[:len(parts)-1], dotSeparator)
				prefixSet[prefix] = true
			}
		}
	}

	var prefixes []string
	for prefix := range prefixSet {
		prefixes = append(prefixes, prefix)
	}
	return prefixes
}

func findTablePrefixes(frontmatter string) []string {
	tableRegex := regexp.MustCompile(`^\[([^\]]+)\]`)
	lines := strings.Split(frontmatter, newlineChar)
	prefixSet := make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		matches := tableRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			prefixSet[matches[1]] = true
		}
	}

	var prefixes []string
	for prefix := range prefixSet {
		prefixes = append(prefixes, prefix)
	}
	return prefixes
}

// tomlParseResult holds the parsed TOML structure
type tomlParseResult struct {
	dotAssignments  map[string][]string // prefix -> []assignments
	tableContent    map[string][]string // table -> []content lines
	ungroupedLines  []string
}

func transformTOMLDotNotation(frontmatter string) string {
	lines := strings.Split(frontmatter, newlineChar)
	parsed := parseTOMLLines(lines)
	return buildTransformedTOML(parsed)
}

func parseTOMLLines(lines []string) *tomlParseResult {
	result := &tomlParseResult{
		dotAssignments: make(map[string][]string),
		tableContent:   make(map[string][]string),
		ungroupedLines: []string{},
	}
	
	currentTable := ""
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		if processTableHeader(trimmed, &currentTable, result.tableContent) {
			continue
		}

		if processDotNotationAssignment(trimmed, result.dotAssignments) {
			continue
		}

		// Regular assignment within a table or standalone
		if currentTable != "" {
			result.tableContent[currentTable] = append(result.tableContent[currentTable], trimmed)
		} else {
			result.ungroupedLines = append(result.ungroupedLines, trimmed)
		}
	}
	
	return result
}

func processTableHeader(trimmed string, currentTable *string, tableContent map[string][]string) bool {
	tableRegex := regexp.MustCompile(`^\[([^\]]+)\]`)
	if matches := tableRegex.FindStringSubmatch(trimmed); len(matches) > 1 {
		*currentTable = matches[1]
		if tableContent[*currentTable] == nil {
			tableContent[*currentTable] = []string{}
		}
		return true
	}
	return false
}

func processDotNotationAssignment(trimmed string, dotAssignments map[string][]string) bool {
	dotRegex := regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*(?:\.[a-zA-Z_][a-zA-Z0-9_]*)+)\s*=\s*(.+)`)
	matches := dotRegex.FindStringSubmatch(trimmed)
	if len(matches) <= 2 {
		return false
	}
	
	fullKey := matches[1]
	value := matches[2]
	
	// Extract prefix and key
	parts := strings.Split(fullKey, dotSeparator)
	if len(parts) > 1 {
		prefix := strings.Join(parts[:len(parts)-1], dotSeparator)
		key := parts[len(parts)-1]
		assignment := key + " = " + value
		dotAssignments[prefix] = append(dotAssignments[prefix], assignment)
	}
	return true
}

func buildTransformedTOML(parsed *tomlParseResult) string {
	var result []string
	
	// Add ungrouped lines first (if any)
	if len(parsed.ungroupedLines) > 0 {
		result = append(result, parsed.ungroupedLines...)
	}

	// Get all unique table names
	allTables := collectAllTableNames(parsed.tableContent, parsed.dotAssignments)
	
	// Sort tables for consistent output
	var sortedTables []string
	for table := range allTables {
		sortedTables = append(sortedTables, table)
	}
	sort.Strings(sortedTables)

	// Build each table section
	for _, table := range sortedTables {
		result = append(result, "["+table+"]")
		
		// Add dot notation assignments first
		if assignments, exists := parsed.dotAssignments[table]; exists {
			result = append(result, assignments...)
		}
		
		// Add existing table content
		if content, exists := parsed.tableContent[table]; exists {
			result = append(result, content...)
		}
	}

	return strings.Join(result, newlineChar)
}

func collectAllTableNames(tableContent, dotAssignments map[string][]string) map[string]bool {
	allTables := make(map[string]bool)
	for table := range tableContent {
		allTables[table] = true
	}
	for prefix := range dotAssignments {
		allTables[prefix] = true
	}
	return allTables
}