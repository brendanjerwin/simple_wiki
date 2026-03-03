package server

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/brendanjerwin/simple_wiki/inventory"
	"github.com/brendanjerwin/simple_wiki/pageimport"
	"github.com/brendanjerwin/simple_wiki/pkg/logging"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

const (
	// PageImportJobName is the name used for page import jobs and their queue.
	PageImportJobName = "PageImportJob"

	// PageImportReportPage is the identifier for the page import report page.
	PageImportReportPage = "page_import_report"
)

// PageImportResult contains the result of page import job(s) execution.
type PageImportResult struct {
	CreatedPages  []string
	UpdatedPages  []string
	FailedRecords []FailedPageImport
}

// FailedPageImport represents a record that failed to import.
type FailedPageImport struct {
	RowNumber  int
	Identifier string
	Error      string
}

// PageImportResultAccumulator accumulates results from multiple SinglePageImportJobs.
// It is thread-safe and can be shared across multiple jobs.
type PageImportResultAccumulator struct {
	mu            sync.Mutex
	CreatedPages  []string
	UpdatedPages  []string
	FailedRecords []FailedPageImport
}

// NewPageImportResultAccumulator creates a new result accumulator.
func NewPageImportResultAccumulator() *PageImportResultAccumulator {
	return &PageImportResultAccumulator{
		CreatedPages:  []string{},
		UpdatedPages:  []string{},
		FailedRecords: []FailedPageImport{},
	}
}

// RecordCreated records a successfully created page.
func (a *PageImportResultAccumulator) RecordCreated(identifier string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.CreatedPages = append(a.CreatedPages, identifier)
}

// RecordUpdated records a successfully updated page.
func (a *PageImportResultAccumulator) RecordUpdated(identifier string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.UpdatedPages = append(a.UpdatedPages, identifier)
}

// RecordFailed records a failed import.
func (a *PageImportResultAccumulator) RecordFailed(failure FailedPageImport) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.FailedRecords = append(a.FailedRecords, failure)
}

// GetResult returns a snapshot of the accumulated results.
func (a *PageImportResultAccumulator) GetResult() PageImportResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	return PageImportResult{
		CreatedPages:  append([]string{}, a.CreatedPages...),
		UpdatedPages:  append([]string{}, a.UpdatedPages...),
		FailedRecords: append([]FailedPageImport{}, a.FailedRecords...),
	}
}

// SinglePageImportJob imports a single page from a parsed CSV record.
type SinglePageImportJob struct {
	record            pageimport.ParsedRecord
	pageReaderMutator wikipage.PageReaderMutator
	logger            logging.Logger
	resultAccumulator *PageImportResultAccumulator
}

// NewSinglePageImportJob creates a new single page import job.
// Returns an error if any required dependency is nil.
func NewSinglePageImportJob(
	record pageimport.ParsedRecord,
	pageReaderMutator wikipage.PageReaderMutator,
	logger logging.Logger,
	resultAccumulator *PageImportResultAccumulator,
) (*SinglePageImportJob, error) {
	if pageReaderMutator == nil {
		return nil, errors.New("pageReaderMutator is required")
	}
	if logger == nil {
		return nil, errors.New("logger is required")
	}
	if resultAccumulator == nil {
		return nil, errors.New("resultAccumulator is required")
	}
	return &SinglePageImportJob{
		record:            record,
		pageReaderMutator: pageReaderMutator,
		logger:            logger,
		resultAccumulator: resultAccumulator,
	}, nil
}

// Execute runs the single page import job.
func (j *SinglePageImportJob) Execute() error {
	record := j.record

	// Handle records with validation errors
	if record.HasErrors() {
		j.resultAccumulator.RecordFailed(FailedPageImport{
			RowNumber:  record.RowNumber,
			Identifier: record.Identifier,
			Error:      strings.Join(record.ValidationErrors, "; "),
		})
		return nil
	}

	// Process the single record
	if err := j.processRecord(record); err != nil {
		j.logger.Error("Failed to process record row %d (%s): %v", record.RowNumber, record.Identifier, err)
		j.resultAccumulator.RecordFailed(FailedPageImport{
			RowNumber:  record.RowNumber,
			Identifier: record.Identifier,
			Error:      err.Error(),
		})
		return nil // Don't return the error - we've recorded it
	}

	return nil
}

// GetName returns the job name.
func (*SinglePageImportJob) GetName() string {
	return PageImportJobName
}

// GetRecord returns the record being imported.
func (j *SinglePageImportJob) GetRecord() pageimport.ParsedRecord {
	return j.record
}

// processRecord processes a single parsed record.
func (j *SinglePageImportJob) processRecord(record pageimport.ParsedRecord) error {
	identifier := record.Identifier

	// Check if page exists
	_, existingFm, err := j.pageReaderMutator.ReadFrontMatter(identifier)
	var isNewPage bool
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read existing frontmatter for %s: %w", identifier, err)
		}
		isNewPage = true
	}

	var fm map[string]any
	if isNewPage {
		fm = make(map[string]any)
		fm["identifier"] = identifier
	} else {
		fm = existingFm
	}

	// Apply template logic for inv_item
	if record.Template == pageimport.InvItemTemplate {
		EnsureInventoryFrontmatterStructure(fm)
	}

	// Merge frontmatter from record (upsert semantics)
	if err := j.mergeFrontmatter(fm, record.Frontmatter); err != nil {
		return fmt.Errorf("failed to merge frontmatter: %w", err)
	}

	// Handle fields to delete
	for _, fieldPath := range record.FieldsToDelete {
		j.deleteField(fm, fieldPath)
	}

	// Handle array operations
	for _, op := range record.ArrayOps {
		if err := j.applyArrayOperation(fm, op); err != nil {
			return fmt.Errorf("failed to apply array operation on %s: %w", op.FieldPath, err)
		}
	}

	// Write frontmatter
	if err := j.pageReaderMutator.WriteFrontMatter(identifier, fm); err != nil {
		return fmt.Errorf("failed to write frontmatter: %w", err)
	}

	// For new pages with inv_item template, also write the inventory markdown
	if isNewPage && record.Template == pageimport.InvItemTemplate {
		markdown := inventory.BuildItemMarkdown()
		if err := j.pageReaderMutator.WriteMarkdown(identifier, markdown); err != nil {
			return fmt.Errorf("failed to write markdown: %w", err)
		}
	}

	// Track result
	if isNewPage {
		j.resultAccumulator.RecordCreated(identifier)
		j.logger.Info("Created page: %s", identifier)
	} else {
		j.resultAccumulator.RecordUpdated(identifier)
		j.logger.Info("Updated page: %s", identifier)
	}

	return nil
}

// mergeFrontmatter merges source frontmatter into target (upsert semantics).
func (j *SinglePageImportJob) mergeFrontmatter(target, source map[string]any) error {
	for key, value := range source {
		if nestedSource, ok := value.(map[string]any); ok {
			// Handle nested maps
			if existing, exists := target[key]; exists {
				if nestedTarget, ok := existing.(map[string]any); ok {
					// Recursively merge nested maps
					if err := j.mergeFrontmatter(nestedTarget, nestedSource); err != nil {
						return err
					}
					continue
				}
			}
			// Create new nested map
			newNested := make(map[string]any)
			if err := j.mergeFrontmatter(newNested, nestedSource); err != nil {
				return err
			}
			target[key] = newNested
		} else {
			// Scalar value - overwrite
			target[key] = value
		}
	}
	return nil
}

// deleteField removes a field from frontmatter using dotted path notation.
func (*SinglePageImportJob) deleteField(fm map[string]any, fieldPath string) {
	parts := strings.Split(fieldPath, ".")
	current := fm

	// Navigate to parent of field to delete
	for i := 0; i < len(parts)-1; i++ {
		nested, ok := current[parts[i]].(map[string]any)
		if !ok {
			return // Path doesn't exist, nothing to delete
		}
		current = nested
	}

	// Delete the field
	delete(current, parts[len(parts)-1])
}

// applyArrayOperation applies an array operation to frontmatter.
func (*SinglePageImportJob) applyArrayOperation(fm map[string]any, op pageimport.ArrayOperation) error {
	parts := strings.Split(op.FieldPath, ".")
	current := fm

	// Navigate to parent, creating nested maps as needed
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		if existing, exists := current[part]; exists {
			nested, ok := existing.(map[string]any)
			if !ok {
				return fmt.Errorf("cannot navigate through non-map value at '%s'", part)
			}
			current = nested
		} else {
			newNested := make(map[string]any)
			current[part] = newNested
			current = newNested
		}
	}

	fieldName := parts[len(parts)-1]

	// Get or create the array
	var arr []any
	if existing, exists := current[fieldName]; exists {
		switch v := existing.(type) {
		case []any:
			arr = v
		case []string:
			// Convert []string to []any
			arr = make([]any, len(v))
			for i, s := range v {
				arr[i] = s
			}
		default:
			return fmt.Errorf("field '%s' is not an array", fieldName)
		}
	}

	switch op.Operation {
	case pageimport.EnsureExists:
		// Add value if not already present
		found := false
		for _, item := range arr {
			if s, ok := item.(string); ok && s == op.Value {
				found = true
				break
			}
		}
		if !found {
			arr = append(arr, op.Value)
		}

	case pageimport.DeleteValue:
		// Remove value if present
		newArr := make([]any, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok && s == op.Value {
				continue // Skip this value
			}
			newArr = append(newArr, item)
		}
		arr = newArr

	default:
		return fmt.Errorf("unknown array operation type: %d", op.Operation)
	}

	current[fieldName] = arr
	return nil
}

// PageImportReportJob generates the import report after all page imports complete.
type PageImportReportJob struct {
	pageReaderMutator wikipage.PageReaderMutator
	logger            logging.Logger
	resultAccumulator *PageImportResultAccumulator
}

// NewPageImportReportJob creates a new report generation job.
// Returns an error if any required dependency is nil.
func NewPageImportReportJob(
	pageReaderMutator wikipage.PageReaderMutator,
	logger logging.Logger,
	resultAccumulator *PageImportResultAccumulator,
) (*PageImportReportJob, error) {
	if pageReaderMutator == nil {
		return nil, errors.New("pageReaderMutator is required")
	}
	if logger == nil {
		return nil, errors.New("logger is required")
	}
	if resultAccumulator == nil {
		return nil, errors.New("resultAccumulator is required")
	}
	return &PageImportReportJob{
		pageReaderMutator: pageReaderMutator,
		logger:            logger,
		resultAccumulator: resultAccumulator,
	}, nil
}

// Execute generates the import report.
func (j *PageImportReportJob) Execute() error {
	result := j.resultAccumulator.GetResult()

	j.logger.Info("Generating page import report: %d created, %d updated, %d failed",
		len(result.CreatedPages), len(result.UpdatedPages), len(result.FailedRecords))

	return j.generateReport(result)
}

// GetName returns the job name.
func (*PageImportReportJob) GetName() string {
	return PageImportJobName
}

// generateReport creates the import report page.
func (j *PageImportReportJob) generateReport(result PageImportResult) error {
	var report bytes.Buffer

	_, _ = report.WriteString("# Page Import Report\n\n")
	_, _ = fmt.Fprintf(&report, "*Last updated: %s*\n\n", time.Now().Format(time.RFC3339))

	// Summary
	_, _ = report.WriteString("## Summary\n\n")
	_, _ = fmt.Fprintf(&report, "- **Pages created:** %d\n", len(result.CreatedPages))
	_, _ = fmt.Fprintf(&report, "- **Pages updated:** %d\n", len(result.UpdatedPages))
	_, _ = fmt.Fprintf(&report, "- **Failed records:** %d\n\n", len(result.FailedRecords))

	// Created Pages
	if len(result.CreatedPages) > 0 {
		_, _ = report.WriteString("## Pages Created\n\n")
		for _, pageID := range result.CreatedPages {
			_, _ = fmt.Fprintf(&report, "- [[%s]]\n", pageID)
		}
		_, _ = report.WriteString("\n")
	}

	// Updated Pages
	if len(result.UpdatedPages) > 0 {
		_, _ = report.WriteString("## Pages Updated\n\n")
		for _, pageID := range result.UpdatedPages {
			_, _ = fmt.Fprintf(&report, "- [[%s]]\n", pageID)
		}
		_, _ = report.WriteString("\n")
	}

	// Failed Records
	if len(result.FailedRecords) > 0 {
		_, _ = report.WriteString("## Failed Records\n\n")
		for _, failure := range result.FailedRecords {
			identifier := failure.Identifier
			if identifier == "" {
				identifier = "(no identifier)"
			}
			_, _ = fmt.Fprintf(&report, "- **Row %d** (%s): %s\n", failure.RowNumber, identifier, failure.Error)
		}
		_, _ = report.WriteString("\n")
	} else {
		_, _ = report.WriteString("## Failed Records\n\n")
		_, _ = report.WriteString("No failures.\n\n")
	}

	// Build frontmatter
	fm := map[string]any{
		"identifier": PageImportReportPage,
		"title":      "Page Import Report",
	}

	// Write frontmatter and markdown
	if err := j.pageReaderMutator.WriteFrontMatter(PageImportReportPage, fm); err != nil {
		return fmt.Errorf("failed to write import report frontmatter: %w", err)
	}
	if err := j.pageReaderMutator.WriteMarkdown(PageImportReportPage, report.String()); err != nil {
		return fmt.Errorf("failed to write import report markdown: %w", err)
	}

	return nil
}
