// Package pageimport provides CSV parsing and validation for bulk page imports.
package pageimport

import (
	"encoding/csv"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
)

// MaxRows is the maximum number of data rows allowed in a CSV import.
const MaxRows = 200

// InvItemTemplate is the built-in template for inventory items.
// This template has special handling and doesn't require a wiki page to exist.
const InvItemTemplate = "inv_item"

// DeleteSentinel is the marker for deleting a scalar field entirely.
const DeleteSentinel = "[[DELETE]]"

// deleteValueRegex matches [[DELETE(value)]] and captures the value.
var deleteValueRegex = regexp.MustCompile(`^\[\[DELETE\((.+)\)\]\]$`)

// ArrayOpType represents the type of operation to perform on an array field.
type ArrayOpType int

const (
	// EnsureExists adds a value to an array if not already present.
	EnsureExists ArrayOpType = iota
	// DeleteValue removes a specific value from an array.
	DeleteValue
)

// ArrayOperation represents an operation on an array field.
type ArrayOperation struct {
	FieldPath string      // e.g., "tags" or "inventory.items" (without [] suffix)
	Operation ArrayOpType // EnsureExists or DeleteValue
	Value     string      // The value to add or remove
}

// ParsedRecord represents a single row from the CSV after parsing.
type ParsedRecord struct {
	RowNumber        int                // 1-indexed row number (excluding header)
	Identifier       string             // The page identifier
	Template         string             // Template to apply (optional)
	Frontmatter      map[string]any     // Scalar fields to set
	ArrayOps         []ArrayOperation   // Array field operations
	FieldsToDelete   []string           // Fields marked with [[DELETE]] (scalars and entire arrays)
	ValidationErrors []string           // Per-record validation errors
	Warnings         []string           // Per-record warnings (e.g., type coercion)
}

// ParseResult contains the result of parsing a CSV file.
type ParseResult struct {
	Records       []ParsedRecord // Parsed records
	ParsingErrors []string       // Global parsing errors (header issues, etc.)
	Headers       []string       // Original headers from CSV
}

// ColumnInfo describes a parsed column header.
type ColumnInfo struct {
	OriginalHeader string // Original header from CSV
	FieldPath      string // Field path without [] suffix
	IsArray        bool   // True if header ends with []
	ColumnIndex    int    // Index in CSV row
}

// ParseCSV parses CSV content and returns structured records.
// It validates the CSV structure and each record.
func ParseCSV(csvContent string) (*ParseResult, error) {
	if csvContent == "" {
		return nil, errors.New("CSV content is empty")
	}

	reader := csv.NewReader(strings.NewReader(csvContent))
	reader.FieldsPerRecord = -1 // Allow variable number of fields

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(records) == 0 {
		return nil, errors.New("CSV has no rows")
	}

	result := &ParseResult{
		Headers: records[0],
	}

	// Parse and validate headers
	columns, err := parseHeaders(records[0])
	if err != nil {
		result.ParsingErrors = append(result.ParsingErrors, err.Error())
		return result, nil
	}

	// Check for identifier column
	hasIdentifier := false
	for _, col := range columns {
		if strings.EqualFold(col.FieldPath, "identifier") && !col.IsArray {
			hasIdentifier = true
			break
		}
	}
	if !hasIdentifier {
		result.ParsingErrors = append(result.ParsingErrors, "CSV must have 'identifier' column")
		return result, nil
	}

	// Check row count (excluding header)
	dataRows := records[1:]
	if len(dataRows) == 0 {
		result.ParsingErrors = append(result.ParsingErrors, "CSV has no data rows")
		return result, nil
	}
	if len(dataRows) > MaxRows {
		result.ParsingErrors = append(result.ParsingErrors, fmt.Sprintf("CSV exceeds %d row limit (has %d rows)", MaxRows, len(dataRows)))
		return result, nil
	}

	// Track seen identifiers for duplicate detection
	seenIdentifiers := make(map[string]int) // identifier -> first row number

	// Parse each data row
	for i, row := range dataRows {
		rowNum := i + 1 // 1-indexed
		record := parseRow(rowNum, row, columns)

		// Check for duplicate identifiers
		if record.Identifier != "" {
			if firstRow, seen := seenIdentifiers[record.Identifier]; seen {
				record.ValidationErrors = append(record.ValidationErrors,
					fmt.Sprintf("duplicate identifier '%s' (also in row %d)", record.Identifier, firstRow))
			} else {
				seenIdentifiers[record.Identifier] = rowNum
			}
		}

		result.Records = append(result.Records, record)
	}

	return result, nil
}

// parseHeaders parses and validates CSV headers.
func parseHeaders(headers []string) ([]ColumnInfo, error) {
	var columns []ColumnInfo

	for i, header := range headers {
		header = strings.TrimSpace(header)
		if header == "" {
			continue // Skip empty headers
		}

		col := ColumnInfo{
			OriginalHeader: header,
			ColumnIndex:    i,
		}

		// Check for array suffix
		if strings.HasSuffix(header, "[]") {
			col.IsArray = true
			col.FieldPath = strings.TrimSuffix(header, "[]")
		} else {
			col.FieldPath = header
		}

		// Validate field path format
		if err := validateFieldPath(col.FieldPath); err != nil {
			return nil, fmt.Errorf("column '%s': %w", header, err)
		}

		columns = append(columns, col)
	}

	return columns, nil
}

// validateFieldPath validates that a field path is valid.
// Only allows single dot for table.field notation (no nested tables).
func validateFieldPath(path string) error {
	if path == "" {
		return errors.New("empty field name")
	}

	parts := strings.Split(path, ".")
	if len(parts) > 2 {
		return errors.New("nested tables not supported, use single dot only (e.g., 'table.field')")
	}

	for _, part := range parts {
		if part == "" {
			return errors.New("invalid path format (empty segment)")
		}
	}

	return nil
}

// parseRow parses a single CSV row into a ParsedRecord.
func parseRow(rowNum int, row []string, columns []ColumnInfo) ParsedRecord {
	record := ParsedRecord{
		RowNumber:   rowNum,
		Frontmatter: make(map[string]any),
	}

	// Track array values for duplicate detection within this row
	arrayValues := make(map[string]map[string]bool) // fieldPath -> set of values

	for _, col := range columns {
		if col.ColumnIndex >= len(row) {
			continue // Row doesn't have this column
		}

		value := strings.TrimSpace(row[col.ColumnIndex])
		if value == "" {
			continue // Skip empty cells
		}

		// Handle special columns
		if strings.EqualFold(col.FieldPath, "identifier") && !col.IsArray {
			record.Identifier = value
			continue
		}
		if strings.EqualFold(col.FieldPath, "template") && !col.IsArray {
			record.Template = value
			continue
		}

		if col.IsArray {
			// Array column handling
			parseArrayCell(&record, col, value, arrayValues)
		} else {
			// Scalar column handling
			parseScalarCell(&record, col, value)
		}
	}

	// Validate identifier
	if record.Identifier == "" {
		record.ValidationErrors = append(record.ValidationErrors, "identifier cannot be empty")
	} else {
		// Check if identifier is valid format
		munged, err := wikiidentifiers.MungeIdentifier(record.Identifier)
		if err != nil {
			record.ValidationErrors = append(record.ValidationErrors,
				fmt.Sprintf("identifier '%s' is invalid: %v", record.Identifier, err))
		} else if munged != record.Identifier {
			record.ValidationErrors = append(record.ValidationErrors,
				fmt.Sprintf("identifier '%s' contains invalid characters (would be normalized to '%s')", record.Identifier, munged))
		}
	}

	return record
}

// parseScalarCell handles a scalar (non-array) column value.
func parseScalarCell(record *ParsedRecord, col ColumnInfo, value string) {
	// Check for [[DELETE(value)]] in scalar column - invalid
	if deleteValueRegex.MatchString(value) {
		record.ValidationErrors = append(record.ValidationErrors,
			fmt.Sprintf("[[DELETE(value)]] only valid for array columns (add [] suffix to '%s')", col.FieldPath))
		return
	}

	// Check for [[DELETE]]
	if value == DeleteSentinel {
		record.FieldsToDelete = append(record.FieldsToDelete, col.FieldPath)
		return
	}

	// Set the scalar value
	if err := setNestedValue(record.Frontmatter, col.FieldPath, value); err != nil {
		record.ValidationErrors = append(record.ValidationErrors,
			fmt.Sprintf("failed to set '%s': %v", col.FieldPath, err))
	}
}

// parseArrayCell handles an array column value.
func parseArrayCell(record *ParsedRecord, col ColumnInfo, value string, arrayValues map[string]map[string]bool) {
	// Check for [[DELETE]] in array column - invalid
	if value == DeleteSentinel {
		record.ValidationErrors = append(record.ValidationErrors,
			fmt.Sprintf("use [[DELETE(value)]] for array columns, or remove [] suffix to delete entire field '%s'", col.FieldPath))
		return
	}

	// Check for [[DELETE(value)]]
	if matches := deleteValueRegex.FindStringSubmatch(value); matches != nil {
		deleteValue := matches[1]

		// Check for duplicate in this row
		if arrayValues[col.FieldPath] != nil && arrayValues[col.FieldPath][deleteValue] {
			record.ValidationErrors = append(record.ValidationErrors,
				fmt.Sprintf("duplicate value '%s' for %s[]", deleteValue, col.FieldPath))
			return
		}

		// Track this value
		if arrayValues[col.FieldPath] == nil {
			arrayValues[col.FieldPath] = make(map[string]bool)
		}
		arrayValues[col.FieldPath][deleteValue] = true

		record.ArrayOps = append(record.ArrayOps, ArrayOperation{
			FieldPath: col.FieldPath,
			Operation: DeleteValue,
			Value:     deleteValue,
		})
		return
	}

	// Regular value - ensure exists in array
	// Check for duplicate in this row
	if arrayValues[col.FieldPath] != nil && arrayValues[col.FieldPath][value] {
		record.ValidationErrors = append(record.ValidationErrors,
			fmt.Sprintf("duplicate value '%s' for %s[]", value, col.FieldPath))
		return
	}

	// Track this value
	if arrayValues[col.FieldPath] == nil {
		arrayValues[col.FieldPath] = make(map[string]bool)
	}
	arrayValues[col.FieldPath][value] = true

	record.ArrayOps = append(record.ArrayOps, ArrayOperation{
		FieldPath: col.FieldPath,
		Operation: EnsureExists,
		Value:     value,
	})
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
				return fmt.Errorf("'%s' cannot be both a value and a table", part)
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

// HasErrors returns true if the record has any validation errors.
func (r *ParsedRecord) HasErrors() bool {
	return len(r.ValidationErrors) > 0
}

// HasErrors returns true if the parse result has any global errors.
func (r *ParseResult) HasErrors() bool {
	return len(r.ParsingErrors) > 0
}

// ErrorCount returns the number of records with validation errors.
func (r *ParseResult) ErrorCount() int {
	count := 0
	for _, record := range r.Records {
		if record.HasErrors() {
			count++
		}
	}
	return count
}

// ValidRecords returns only records without validation errors.
func (r *ParseResult) ValidRecords() []ParsedRecord {
	var valid []ParsedRecord
	for _, record := range r.Records {
		if !record.HasErrors() {
			valid = append(valid, record)
		}
	}
	return valid
}
