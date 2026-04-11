package v1

import (
	"context"
	"fmt"
	"os"
	"strings"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/pageimport"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

// serverPageExistenceChecker implements pageimport.PageExistenceChecker.
type serverPageExistenceChecker struct {
	reader wikipage.PageReader
}

func (c *serverPageExistenceChecker) PageExists(identifier string) bool {
	_, _, err := c.reader.ReadFrontMatter(wikipage.PageIdentifier(identifier))
	return err == nil
}

// serverContainerReferenceGetter implements pageimport.ContainerReferenceGetter.
type serverContainerReferenceGetter struct {
	reader wikipage.PageReader
}

func (c *serverContainerReferenceGetter) GetContainerReference(identifier string) string {
	_, fm, err := c.reader.ReadFrontMatter(wikipage.PageIdentifier(identifier))
	if err != nil {
		return ""
	}
	inventory, ok := fm["inventory"].(map[string]any)
	if !ok {
		return ""
	}
	container, ok := inventory["container"].(string)
	if !ok {
		return ""
	}
	return container
}

// runInventoryValidations runs all inventory-specific validations on parsed records.
func (s *Server) runInventoryValidations(records []pageimport.ParsedRecord) {
	validator := pageimport.NewInventoryValidator(
		&serverPageExistenceChecker{reader: s.pageReaderMutator},
		&serverContainerReferenceGetter{reader: s.pageReaderMutator},
	)

	// Phase 1: Per-record validations
	for i := range records {
		validator.ValidateContainerIdentifier(&records[i])
		validator.ValidateInventoryItemsIdentifiers(&records[i])
	}

	// Phase 2: Cross-record validations
	validator.ValidateContainerExistence(records)
	validator.DetectCircularReferences(records)
}

// ParseCSVPreview implements the ParseCSVPreview RPC for the PageImportService.
// It parses CSV content and returns a preview of what would be imported.
func (s *Server) ParseCSVPreview(_ context.Context, req *apiv1.ParseCSVPreviewRequest) (*apiv1.ParseCSVPreviewResponse, error) {
	if req.CsvContent == "" {
		return nil, status.Error(codes.InvalidArgument, "csv_content cannot be empty")
	}

	parseResult, err := pageimport.ParseCSV(req.CsvContent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to parse CSV: %v", err)
	}

	// If there are global parsing errors, return them
	if parseResult.HasErrors() {
		return &apiv1.ParseCSVPreviewResponse{
			ParsingErrors: parseResult.ParsingErrors,
		}, nil
	}

	// Run inventory-specific validations
	s.runInventoryValidations(parseResult.Records)

	// Convert parsed records to protobuf and check page existence
	var records []*apiv1.PageImportRecord
	var errorCount, updateCount, createCount int32

	for _, parsed := range parseResult.Records {
		record, err := convertParsedRecordToProto(parsed)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to convert record: %v", err)
		}

		// Check if the page exists
		_, _, readErr := s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(parsed.Identifier))
		pageExists := readErr == nil
		record.PageExists = pageExists

		// Validate template if specified (skip the built-in inv_item template)
		record.ValidationErrors = append(record.ValidationErrors, s.validateRecordTemplate(parsed)...)

		// Count errors, updates, and creates
		if len(record.ValidationErrors) > 0 {
			errorCount++
		} else if pageExists {
			updateCount++
		} else {
			createCount++
		}

		records = append(records, record)
	}

	return &apiv1.ParseCSVPreviewResponse{
		Records:      records,
		TotalRecords: int32(len(records)),
		ErrorCount:   errorCount,
		UpdateCount:  updateCount,
		CreateCount:  createCount,
	}, nil
}

// validateRecordTemplate validates the template referenced by a parsed record.
// Returns any validation error strings; returns nil when no validation is needed.
func (s *Server) validateRecordTemplate(parsed pageimport.ParsedRecord) []string {
	if parsed.Template == "" || parsed.HasErrors() || parsed.Template == pageimport.InvItemTemplate {
		return nil
	}
	_, templateFm, err := s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(parsed.Template))
	if err != nil {
		if os.IsNotExist(err) {
			return []string{fmt.Sprintf("template '%s' does not exist", parsed.Template)}
		}
		return []string{fmt.Sprintf("failed to read template '%s': %v", parsed.Template, err)}
	}
	if !isTemplatePage(templateFm) {
		return []string{fmt.Sprintf("page '%s' is not a template (missing template: true)", parsed.Template)}
	}
	return nil
}

// convertParsedRecordToProto converts a pageimport.ParsedRecord to apiv1.PageImportRecord.
func convertParsedRecordToProto(parsed pageimport.ParsedRecord) (*apiv1.PageImportRecord, error) {
	record := &apiv1.PageImportRecord{
		RowNumber:        int32(parsed.RowNumber),
		Identifier:       parsed.Identifier,
		Template:         parsed.Template,
		FieldsToDelete:   parsed.FieldsToDelete,
		ValidationErrors: parsed.ValidationErrors,
		Warnings:         parsed.Warnings,
	}

	// Convert frontmatter to protobuf Struct
	if len(parsed.Frontmatter) > 0 {
		fmStruct, err := structpb.NewStruct(parsed.Frontmatter)
		if err != nil {
			return nil, fmt.Errorf("failed to convert frontmatter for row %d: %w", parsed.RowNumber, err)
		}
		record.Frontmatter = fmStruct
	}

	// Convert array operations
	for _, op := range parsed.ArrayOps {
		protoOp := &apiv1.ArrayOperation{
			FieldPath: op.FieldPath,
			Value:     op.Value,
		}
		switch op.Operation {
		case pageimport.EnsureExists:
			protoOp.Operation = apiv1.ArrayOpType_ARRAY_OP_TYPE_ENSURE_EXISTS
		case pageimport.DeleteValue:
			protoOp.Operation = apiv1.ArrayOpType_ARRAY_OP_TYPE_DELETE_VALUE
		default:
			protoOp.Operation = apiv1.ArrayOpType_ARRAY_OP_TYPE_UNSPECIFIED
		}
		record.ArrayOps = append(record.ArrayOps, protoOp)
	}

	return record, nil
}

// StartPageImportJob implements the StartPageImportJob RPC for the PageImportService.
// It starts background jobs to import pages from the CSV content - one job per page.
func (s *Server) StartPageImportJob(ctx context.Context, req *apiv1.StartPageImportJobRequest) (*apiv1.StartPageImportJobResponse, error) {
	if req.CsvContent == "" {
		return nil, status.Error(codes.InvalidArgument, "csv_content cannot be empty")
	}

	if s.jobQueueCoordinator == nil {
		return nil, status.Error(codes.Unavailable, "job queue coordinator not available")
	}

	parseResult, err := pageimport.ParseCSV(req.CsvContent)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to parse CSV: %v", err)
	}

	// Check for global parsing errors
	if parseResult.HasErrors() {
		return nil, status.Errorf(codes.InvalidArgument, "CSV has parsing errors: %s", strings.Join(parseResult.ParsingErrors, "; "))
	}

	// Run inventory-specific validations
	s.runInventoryValidations(parseResult.Records)

	// Get all records (we'll handle validation errors in individual jobs)
	allRecords := parseResult.Records
	if len(allRecords) == 0 {
		return nil, status.Error(codes.InvalidArgument, "no records to import")
	}

	// Create a shared result accumulator for all jobs
	resultAccumulator := server.NewPageImportResultAccumulator()

	if err := s.enqueueImportJobs(allRecords, resultAccumulator); err != nil {
		return nil, err
	}

	identity := tailscale.IdentityFromContext(ctx)
	s.logger.Info("[AUDIT] import | records: %d | user: %q", len(allRecords), identity.ForLog())

	return &apiv1.StartPageImportJobResponse{
		Success:     true,
		JobId:       server.PageImportJobName,
		RecordCount: int32(len(allRecords)),
	}, nil
}

// makeReportJobCallback creates a completion callback that enqueues a report generation job
// after all import jobs have completed.
func (s *Server) makeReportJobCallback(resultAccumulator *server.PageImportResultAccumulator) func(error) {
	return func(_ error) {
		// All jobs complete when this runs (queue processes in order)
		reportJob, createErr := server.NewPageImportReportJob(s.pageReaderMutator, s.logger, resultAccumulator)
		if createErr != nil {
			s.logger.Error("Failed to create report job: %v", createErr)
			return
		}
		if enqueueErr := s.jobQueueCoordinator.EnqueueJob(reportJob); enqueueErr != nil {
			s.logger.Error("Failed to enqueue report job: %v", enqueueErr)
		}
	}
}

// enqueueImportJobs creates and enqueues a job for each import record.
// The last record's job is enqueued with a completion callback to trigger report generation.
func (s *Server) enqueueImportJobs(allRecords []pageimport.ParsedRecord, resultAccumulator *server.PageImportResultAccumulator) error {
	for i, record := range allRecords {
		job, err := server.NewSinglePageImportJob(record, s.pageReaderMutator, s.logger, resultAccumulator)
		if err != nil {
			return status.Errorf(codes.Internal, "failed to create import job for record %d: %v", i+1, err)
		}

		// For the last job, use completion callback to trigger report generation
		if i == len(allRecords)-1 {
			err = s.jobQueueCoordinator.EnqueueJobWithCompletion(job, s.makeReportJobCallback(resultAccumulator))
		} else {
			err = s.jobQueueCoordinator.EnqueueJob(job)
		}

		if err != nil {
			return status.Errorf(codes.Internal, "failed to enqueue import job for record %d: %v", i+1, err)
		}
	}
	return nil
}
