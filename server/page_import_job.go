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

	fm, isNewPage, err := j.readOrInitFrontmatter(identifier)
	if err != nil {
		return err
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

	if err := j.writePageContent(identifier, isNewPage, record.Template, fm); err != nil {
		return err
	}

	j.trackResult(identifier, isNewPage)
	return nil
}

// readOrInitFrontmatter reads existing frontmatter for the page or initialises a
// fresh map for a brand-new page. Returns the map, whether the page is new, and
// any unexpected error.
func (j *SinglePageImportJob) readOrInitFrontmatter(identifier string) (map[string]any, bool, error) {
	_, existingFm, err := j.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(identifier))
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, false, fmt.Errorf("failed to read existing frontmatter for %s: %w", identifier, err)
		}
		fm := make(map[string]any)
		fm["identifier"] = identifier
		return fm, true, nil
	}
	return existingFm, false, nil
}

// writePageContent persists the frontmatter and, for new inv_item pages, the
// inventory markdown template.
func (j *SinglePageImportJob) writePageContent(identifier string, isNewPage bool, template string, fm map[string]any) error {
	if err := j.pageReaderMutator.WriteFrontMatter(wikipage.PageIdentifier(identifier), fm); err != nil {
		return fmt.Errorf("failed to write frontmatter: %w", err)
	}
	if isNewPage && template == pageimport.InvItemTemplate {
		markdown := inventory.BuildItemMarkdown()
		if err := j.pageReaderMutator.WriteMarkdown(wikipage.PageIdentifier(identifier), wikipage.Markdown(markdown)); err != nil {
			return fmt.Errorf("failed to write markdown: %w", err)
		}
	}
	return nil
}

// trackResult records the outcome of a page import in the accumulator and logs it.
func (j *SinglePageImportJob) trackResult(identifier string, isNewPage bool) {
	if isNewPage {
		j.resultAccumulator.RecordCreated(identifier)
		j.logger.Info("Created page: %s", identifier)
	} else {
		j.resultAccumulator.RecordUpdated(identifier)
		j.logger.Info("Updated page: %s", identifier)
	}
}

// mergeFrontmatter merges source frontmatter into target (upsert semantics).
func (j *SinglePageImportJob) mergeFrontmatter(target, source map[string]any) error {
	for key, value := range source {
		if nestedSource, ok := value.(map[string]any); ok {
			if err := j.mergeNestedMapEntry(target, key, nestedSource); err != nil {
				return err
			}
		} else {
			// Scalar value - overwrite
			target[key] = value
		}
	}
	return nil
}

// mergeNestedMapEntry merges a nested map value into target[key], recursing into
// an existing nested map or creating a new one.
func (j *SinglePageImportJob) mergeNestedMapEntry(target map[string]any, key string, nestedSource map[string]any) error {
	nestedTarget, ok := target[key].(map[string]any)
	if !ok {
		nestedTarget = make(map[string]any)
		target[key] = nestedTarget
	}
	return j.mergeFrontmatter(nestedTarget, nestedSource)
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

// navigateToParentMap traverses fm following the dotted-path segments, creating
// intermediate maps as needed, and returns the map that holds the final key.
func navigateToParentMap(fm map[string]any, parts []string) (map[string]any, error) {
	current := fm
	for _, part := range parts {
		if existing, exists := current[part]; exists {
			nested, ok := existing.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("cannot navigate through non-map value at '%s'", part)
			}
			current = nested
		} else {
			newNested := make(map[string]any)
			current[part] = newNested
			current = newNested
		}
	}
	return current, nil
}

// coerceToAnySlice returns the existing value at fieldName as a []any slice,
// converting []string if necessary. Returns nil when the field is absent.
func coerceToAnySlice(current map[string]any, fieldName string) ([]any, error) {
	existing, exists := current[fieldName]
	if !exists {
		return nil, nil
	}
	switch v := existing.(type) {
	case []any:
		return v, nil
	case []string:
		arr := make([]any, len(v))
		for i, s := range v {
			arr[i] = s
		}
		return arr, nil
	default:
		return nil, fmt.Errorf("field '%s' is not an array", fieldName)
	}
}

// ensureValueInArray returns arr with value appended if not already present.
func ensureValueInArray(arr []any, value string) []any {
	for _, item := range arr {
		if s, ok := item.(string); ok && s == value {
			return arr // already present
		}
	}
	return append(arr, value)
}

// removeValueFromArray returns a new slice with all occurrences of value removed.
func removeValueFromArray(arr []any, value string) []any {
	result := make([]any, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok && s == value {
			continue
		}
		result = append(result, item)
	}
	return result
}

// applyArrayOperation applies an array operation to frontmatter.
func (*SinglePageImportJob) applyArrayOperation(fm map[string]any, op pageimport.ArrayOperation) error {
	parts := strings.Split(op.FieldPath, ".")
	parentParts := parts[:len(parts)-1]
	fieldName := parts[len(parts)-1]

	current, err := navigateToParentMap(fm, parentParts)
	if err != nil {
		return err
	}

	arr, err := coerceToAnySlice(current, fieldName)
	if err != nil {
		return err
	}

	switch op.Operation {
	case pageimport.EnsureExists:
		arr = ensureValueInArray(arr, op.Value)
	case pageimport.DeleteValue:
		arr = removeValueFromArray(arr, op.Value)
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
	if err := j.pageReaderMutator.WriteFrontMatter(wikipage.PageIdentifier(PageImportReportPage), fm); err != nil {
		return fmt.Errorf("failed to write import report frontmatter: %w", err)
	}
	if err := j.pageReaderMutator.WriteMarkdown(wikipage.PageIdentifier(PageImportReportPage), wikipage.Markdown(report.String())); err != nil {
		return fmt.Errorf("failed to write import report markdown: %w", err)
	}

	return nil
}
