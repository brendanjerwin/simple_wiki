// Package v1 provides the implementation of gRPC services for version 1 of the API
package v1

import (
	"bytes"
	"context"
	"fmt"
	"maps"
	"os"
	"reflect"
	"strings"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/index/bleve"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
	"github.com/pelletier/go-toml/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	pageReadWriterNotAvailableError = "PageReaderMutator not available"
	identifierKey                   = "identifier"
	pageNotFoundErrFmt              = "page not found: %s"
	failedToReadFrontmatterErrFmt   = "failed to read frontmatter: %v"
	failedToBuildPageTextErrFmt     = "failed to build page text: %v"
	maxUniqueIdentifierAttempts     = 1000
)

// filterIdentifierKey removes the identifier key from a frontmatter map.
func filterIdentifierKey(fm map[string]any) map[string]any {
	if fm == nil {
		return nil
	}

	filtered := make(map[string]any)
	for k, v := range fm {
		if k != identifierKey {
			filtered[k] = v
		}
	}
	return filtered
}

// validateNoIdentifierKey checks if the frontmatter contains an identifier key.
func validateNoIdentifierKey(fm map[string]any) error {
	if fm == nil {
		return nil
	}

	if _, exists := fm[identifierKey]; exists {
		return status.Error(codes.InvalidArgument, "identifier key cannot be modified")
	}
	return nil
}

// isIdentifierKeyPath checks if the given path targets the identifier key at the root level.
func isIdentifierKeyPath(path []*apiv1.PathComponent) bool {
	if len(path) != 1 {
		return false
	}

	keyComp, ok := path[0].Component.(*apiv1.PathComponent_Key)
	if !ok {
		return false
	}

	return keyComp.Key == identifierKey
}

// Server is the implementation of the gRPC services.
type Server struct {
	apiv1.UnimplementedSystemInfoServiceServer
	apiv1.UnimplementedFrontmatterServer
	apiv1.UnimplementedPageManagementServiceServer
	apiv1.UnimplementedSearchServiceServer
	apiv1.UnimplementedInventoryManagementServiceServer
	Commit                  string
	BuildTime               time.Time
	PageReaderMutator       wikipage.PageReaderMutator
	BleveIndexQueryer       bleve.IQueryBleveIndex
	JobProgressProvider     jobs.IProvideJobProgress
	Logger                  *lumber.ConsoleLogger
	MarkdownRenderer        wikipage.IRenderMarkdownToHTML
	TemplateExecutor        wikipage.IExecuteTemplate
	FrontmatterIndexQueryer wikipage.IQueryFrontmatterIndex
}

// MergeFrontmatter implements the MergeFrontmatter RPC.
func (s *Server) MergeFrontmatter(_ context.Context, req *apiv1.MergeFrontmatterRequest) (resp *apiv1.MergeFrontmatterResponse, err error) {
	v := reflect.ValueOf(s.PageReaderMutator)
	if s.PageReaderMutator == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return nil, status.Error(codes.Internal, pageReadWriterNotAvailableError)
	}

	// Validate that the request doesn't contain an identifier key
	if req.Frontmatter != nil {
		newFm := req.Frontmatter.AsMap()
		if err := validateNoIdentifierKey(newFm); err != nil {
			return nil, err
		}
	}

	_, existingFm, err := s.PageReaderMutator.ReadFrontMatter(req.Page)
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, failedToReadFrontmatterErrFmt, err)
	}

	if existingFm == nil {
		existingFm = make(map[string]any)
	}

	if req.Frontmatter != nil {
		newFm := req.Frontmatter.AsMap()
		maps.Copy(existingFm, newFm)
	}

	err = s.PageReaderMutator.WriteFrontMatter(req.Page, existingFm)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to write frontmatter: %v", err)
	}

	// Filter out the identifier key from the response
	filteredFm := filterIdentifierKey(existingFm)
	mergedFmStruct, err := structpb.NewStruct(filteredFm)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert merged frontmatter to struct: %v", err)
	}

	return &apiv1.MergeFrontmatterResponse{
		Frontmatter: mergedFmStruct,
	}, nil
}

// ReplaceFrontmatter implements the ReplaceFrontmatter RPC.
func (s *Server) ReplaceFrontmatter(_ context.Context, req *apiv1.ReplaceFrontmatterRequest) (resp *apiv1.ReplaceFrontmatterResponse, err error) {
	v := reflect.ValueOf(s.PageReaderMutator)
	if s.PageReaderMutator == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return nil, status.Error(codes.Internal, pageReadWriterNotAvailableError)
	}

	var fm map[string]any
	if req.Frontmatter != nil {
		fm = req.Frontmatter.AsMap()
		// Filter out any user-provided identifier key and set the correct one
		fm = filterIdentifierKey(fm)
		fm[identifierKey] = req.Page
	}

	err = s.PageReaderMutator.WriteFrontMatter(req.Page, fm)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to write frontmatter: %v", err)
	}

	// Return the frontmatter without the identifier key
	var responseFm map[string]any
	if fm != nil {
		responseFm = filterIdentifierKey(fm)
	}

	var responseFmStruct *structpb.Struct
	if len(responseFm) > 0 {
		responseFmStruct, err = structpb.NewStruct(responseFm)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to convert frontmatter to struct: %v", err)
		}
	}

	return &apiv1.ReplaceFrontmatterResponse{
		Frontmatter: responseFmStruct,
	}, nil
}

// RemoveKeyAtPath implements the RemoveKeyAtPath RPC.
func (s *Server) RemoveKeyAtPath(_ context.Context, req *apiv1.RemoveKeyAtPathRequest) (*apiv1.RemoveKeyAtPathResponse, error) {
	v := reflect.ValueOf(s.PageReaderMutator)
	if s.PageReaderMutator == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return nil, status.Error(codes.Internal, pageReadWriterNotAvailableError)
	}

	if len(req.GetKeyPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "key_path cannot be empty")
	}

	// Validate that the path is not targeting the identifier key
	if isIdentifierKeyPath(req.GetKeyPath()) {
		return nil, status.Error(codes.InvalidArgument, "identifier key cannot be removed")
	}

	_, fm, err := s.PageReaderMutator.ReadFrontMatter(req.Page)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, pageNotFoundErrFmt, req.Page)
		}
		return nil, status.Errorf(codes.Internal, failedToReadFrontmatterErrFmt, err)
	}

	if fm == nil {
		// Attempting to remove from a non-existent frontmatter. The path will not be found.
		fm = make(map[string]any)
	}

	updatedFm, err := removeAtPath(fm, req.GetKeyPath())
	if err != nil {
		return nil, err
	}

	err = s.PageReaderMutator.WriteFrontMatter(req.Page, updatedFm.(map[string]any))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to write frontmatter: %v", err)
	}

	// Filter out the identifier key from the response
	filteredFm := filterIdentifierKey(updatedFm.(map[string]any))
	updatedFmStruct, err := structpb.NewStruct(filteredFm)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert updated frontmatter to struct: %v", err)
	}

	return &apiv1.RemoveKeyAtPathResponse{
		Frontmatter: updatedFmStruct,
	}, nil
}

// removeAtPath recursively traverses the data structure according to the path
// and removes the element at the end of the path. It returns the modified data
// structure. For slices, this may be a new slice instance.
func removeAtPath(data any, path []*apiv1.PathComponent) (any, error) {
	if len(path) == 0 {
		// This should be caught by the public-facing method, but as a safeguard:
		return nil, status.Error(codes.InvalidArgument, "path cannot be empty")
	}

	component := path[0]
	remainingPath := path[1:]

	switch v := data.(type) {
	case map[string]any:
		keyComp, ok := component.Component.(*apiv1.PathComponent_Key)
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "path component is not a key for a map: %T", component.Component)
		}
		key := keyComp.Key

		value, exists := v[key]
		if !exists {
			return nil, status.Errorf(codes.NotFound, "key '%s' not found", key)
		}

		if len(remainingPath) == 0 {
			// Base case: remove key from map
			delete(v, key)
			return v, nil // return modified map
		}

		// Recursive step
		newValue, err := removeAtPath(value, remainingPath)
		if err != nil {
			return nil, err
		}
		v[key] = newValue // Update map with potentially modified child.
		return v, nil

	case []any:
		indexComp, ok := component.Component.(*apiv1.PathComponent_Index)
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "path component is not an index for a slice: %T", component.Component)
		}
		idx := int(indexComp.Index)

		if idx < 0 || idx >= len(v) {
			return nil, status.Errorf(codes.OutOfRange, "index %d is out of range for slice of length %d", idx, len(v))
		}

		if len(remainingPath) == 0 {
			// Base case: remove item from slice
			newSlice := append(v[:idx], v[idx+1:]...)
			return newSlice, nil // Return the new slice
		}

		// Recursive step
		value := v[idx]
		newValue, err := removeAtPath(value, remainingPath)
		if err != nil {
			return nil, err
		}
		v[idx] = newValue // Update slice with potentially modified child.
		return v, nil

	default:
		// Trying to traverse deeper, but `data` is a primitive.
		return nil, status.Error(codes.InvalidArgument, "path is deeper than data structure")
	}
}

// NewServer creates a new debug server
func NewServer(
	commit string,
	buildTime time.Time,
	pageReadWriter wikipage.PageReaderMutator,
	bleveIndexQueryer bleve.IQueryBleveIndex,
	jobProgressProvider jobs.IProvideJobProgress,
	logger *lumber.ConsoleLogger,
	markdownRenderer wikipage.IRenderMarkdownToHTML,
	templateExecutor wikipage.IExecuteTemplate,
	frontmatterIndexQueryer wikipage.IQueryFrontmatterIndex,
) *Server {
	return &Server{
		Commit:                  commit,
		BuildTime:               buildTime,
		PageReaderMutator:       pageReadWriter,
		BleveIndexQueryer:       bleveIndexQueryer,
		JobProgressProvider:     jobProgressProvider,
		Logger:                  logger,
		MarkdownRenderer:        markdownRenderer,
		TemplateExecutor:        templateExecutor,
		FrontmatterIndexQueryer: frontmatterIndexQueryer,
	}
}

// RegisterWithServer registers the gRPC services with the given gRPC server.
func (s *Server) RegisterWithServer(grpcServer *grpc.Server) {
	apiv1.RegisterSystemInfoServiceServer(grpcServer, s)
	apiv1.RegisterFrontmatterServer(grpcServer, s)
	apiv1.RegisterPageManagementServiceServer(grpcServer, s)
	apiv1.RegisterSearchServiceServer(grpcServer, s)
	apiv1.RegisterInventoryManagementServiceServer(grpcServer, s)
}

// GetVersion implements the GetVersion RPC.
func (s *Server) GetVersion(_ context.Context, _ *apiv1.GetVersionRequest) (*apiv1.GetVersionResponse, error) {
	return &apiv1.GetVersionResponse{
		Commit:    s.Commit,
		BuildTime: timestamppb.New(s.BuildTime),
	}, nil
}

// GetJobStatus implements the GetJobStatus RPC.
func (s *Server) GetJobStatus(_ context.Context, _ *apiv1.GetJobStatusRequest) (*apiv1.GetJobStatusResponse, error) {
	return s.buildJobStatusResponse(), nil
}

// StreamJobStatus implements the StreamJobStatus RPC for real-time job queue updates.
func (s *Server) StreamJobStatus(req *apiv1.StreamJobStatusRequest, stream apiv1.SystemInfoService_StreamJobStatusServer) error {
	// Default to 1-second intervals, allow client to customize
	interval := time.Duration(req.GetUpdateIntervalMs()) * time.Millisecond
	if interval == 0 {
		interval = 1 * time.Second
	}

	// Minimum interval to prevent excessive server load
	const minIntervalMs = 100
	if interval < minIntervalMs*time.Millisecond {
		interval = minIntervalMs * time.Millisecond
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Send initial status immediately
	response := s.buildJobStatusResponse()
	if err := stream.Send(response); err != nil {
		return err
	}

	// Stream updates at the specified interval
	for {
		select {
		case <-ticker.C:
			response := s.buildJobStatusResponse()
			if err := stream.Send(response); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

// buildJobStatusResponse builds a GetJobStatusResponse from the job progress provider.
func (s *Server) buildJobStatusResponse() *apiv1.GetJobStatusResponse {
	if s.JobProgressProvider == nil {
		return &apiv1.GetJobStatusResponse{
			JobQueues: []*apiv1.JobQueueStatus{},
		}
	}

	progress := s.JobProgressProvider.GetJobProgress()
	var protoQueues []*apiv1.JobQueueStatus

	for _, queueStats := range progress.QueueStats {
		protoQueue := &apiv1.JobQueueStatus{
			Name:          queueStats.QueueName,
			JobsRemaining: queueStats.JobsRemaining,
			HighWaterMark: queueStats.HighWaterMark,
			IsActive:      queueStats.IsActive,
		}
		protoQueues = append(protoQueues, protoQueue)
	}

	return &apiv1.GetJobStatusResponse{
		JobQueues: protoQueues,
	}
}

// GetFrontmatter implements the GetFrontmatter RPC.
func (s *Server) GetFrontmatter(_ context.Context, req *apiv1.GetFrontmatterRequest) (resp *apiv1.GetFrontmatterResponse, err error) {
	v := reflect.ValueOf(s.PageReaderMutator)
	if s.PageReaderMutator == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return nil, status.Error(codes.Internal, pageReadWriterNotAvailableError)
	}

	var fm map[string]any
	_, fm, err = s.PageReaderMutator.ReadFrontMatter(req.Page)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, pageNotFoundErrFmt, req.Page)
		}
		return nil, status.Errorf(codes.Internal, failedToReadFrontmatterErrFmt, err)
	}

	// Filter out the identifier key from the response
	filteredFm := filterIdentifierKey(fm)

	var structFm *structpb.Struct
	structFm, err = structpb.NewStruct(filteredFm)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert frontmatter to struct: %v", err)
	}

	return &apiv1.GetFrontmatterResponse{
		Frontmatter: structFm,
	}, nil
}

// LoggingInterceptor returns a gRPC unary interceptor for logging method calls.
func (s *Server) LoggingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		// Call the method
		resp, err := handler(ctx, req)

		// Log the request in a format similar to Gin
		duration := time.Since(start)
		statusCode := codes.OK
		if err != nil {
			if st, ok := status.FromError(err); ok {
				statusCode = st.Code()
			}
		}

		if s.Logger != nil {
			s.Logger.Warn("[GRPC] %s | %s | %v",
				statusCode,
				duration,
				info.FullMethod,
			)
		}

		return resp, err
	}
}

// DeletePage implements the DeletePage RPC.
func (s *Server) DeletePage(_ context.Context, req *apiv1.DeletePageRequest) (*apiv1.DeletePageResponse, error) {
	v := reflect.ValueOf(s.PageReaderMutator)
	if s.PageReaderMutator == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return nil, status.Error(codes.Internal, pageReadWriterNotAvailableError)
	}

	err := s.PageReaderMutator.DeletePage(req.PageName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "page not found: %s", req.PageName)
		}
		return nil, status.Errorf(codes.Internal, "failed to delete page: %v", err)
	}

	return &apiv1.DeletePageResponse{
		Success: true,
		Error:   "",
	}, nil
}

// SearchContent implements the SearchContent RPC.
//
//revive:disable:cognitive-complexity
//revive:disable:cyclomatic
//revive:disable:function-length
func (s *Server) SearchContent(_ context.Context, req *apiv1.SearchContentRequest) (*apiv1.SearchContentResponse, error) {
	if err := s.validateSearchRequest(req); err != nil {
		return nil, err
	}

	searchResults, err := s.BleveIndexQueryer.Query(req.Query)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to search: %v", err)
	}

	includeFilterSets, err := s.buildIncludeFilterSets(req.FrontmatterKeyIncludeFilters)
	if err != nil {
		return nil, err
	}

	excludedPages, err := s.buildExcludedPagesSet(req.FrontmatterKeyExcludeFilters)
	if err != nil {
		return nil, err
	}

	results := s.filterAndConvertResults(searchResults, includeFilterSets, excludedPages, req.FrontmatterKeysToReturnInResults)

	return &apiv1.SearchContentResponse{Results: results}, nil
}

// validateSearchRequest validates the search request and index availability.
func (s *Server) validateSearchRequest(req *apiv1.SearchContentRequest) error {
	v := reflect.ValueOf(s.BleveIndexQueryer)
	if s.BleveIndexQueryer == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return status.Error(codes.Internal, "Search index is not available")
	}
	if req.Query == "" {
		return status.Error(codes.InvalidArgument, "query cannot be empty")
	}
	return nil
}

// buildIncludeFilterSets builds sets of pages for each include filter key.
func (s *Server) buildIncludeFilterSets(filterKeys []string) ([]map[wikipage.PageIdentifier]bool, error) {
	if len(filterKeys) == 0 {
		return nil, nil
	}
	if err := s.validateFrontmatterIndexAvailable(); err != nil {
		return nil, err
	}

	var filterSets []map[wikipage.PageIdentifier]bool
	for _, filterKey := range filterKeys {
		pageIDs := s.FrontmatterIndexQueryer.QueryKeyExistence(wikipage.DottedKeyPath(filterKey))
		pageSet := make(map[wikipage.PageIdentifier]bool, len(pageIDs))
		for _, id := range pageIDs {
			pageSet[id] = true
		}
		filterSets = append(filterSets, pageSet)
	}
	return filterSets, nil
}

// buildExcludedPagesSet builds the set of pages to exclude based on filter keys.
func (s *Server) buildExcludedPagesSet(filterKeys []string) (map[wikipage.PageIdentifier]bool, error) {
	if len(filterKeys) == 0 {
		return nil, nil
	}
	if err := s.validateFrontmatterIndexAvailable(); err != nil {
		return nil, err
	}

	excludedPages := make(map[wikipage.PageIdentifier]bool)
	for _, filterKey := range filterKeys {
		pageIDs := s.FrontmatterIndexQueryer.QueryKeyExistence(wikipage.DottedKeyPath(filterKey))
		for _, id := range pageIDs {
			excludedPages[id] = true
		}
	}
	return excludedPages, nil
}

// validateFrontmatterIndexAvailable checks if the frontmatter index is available.
func (s *Server) validateFrontmatterIndexAvailable() error {
	v := reflect.ValueOf(s.FrontmatterIndexQueryer)
	if s.FrontmatterIndexQueryer == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return status.Error(codes.Internal, "Frontmatter index not available for filtering")
	}
	return nil
}

// filterAndConvertResults filters search results and converts them to API format.
func (s *Server) filterAndConvertResults(
	searchResults []bleve.SearchResult,
	includeFilterSets []map[wikipage.PageIdentifier]bool,
	excludedPages map[wikipage.PageIdentifier]bool,
	fmKeysToReturn []string,
) []*apiv1.SearchResult {
	var results []*apiv1.SearchResult
	for _, result := range searchResults {
		mungedID := wikipage.PageIdentifier(wikiidentifiers.MungeIdentifier(string(result.Identifier)))

		if !matchesAllIncludeFilters(mungedID, includeFilterSets) {
			continue
		}
		if excludedPages[mungedID] {
			continue
		}

		apiResult := s.convertSearchResult(result, mungedID, fmKeysToReturn)
		results = append(results, apiResult)
	}
	return results
}

// matchesAllIncludeFilters checks if a page matches all include filter sets.
func matchesAllIncludeFilters(pageID wikipage.PageIdentifier, filterSets []map[wikipage.PageIdentifier]bool) bool {
	for _, filterSet := range filterSets {
		if !filterSet[pageID] {
			return false
		}
	}
	return true
}

// convertSearchResult converts a bleve search result to an API search result.
func (s *Server) convertSearchResult(result bleve.SearchResult, mungedID wikipage.PageIdentifier, fmKeysToReturn []string) *apiv1.SearchResult {
	var highlights []*apiv1.HighlightSpan
	for _, hl := range result.Highlights {
		highlights = append(highlights, &apiv1.HighlightSpan{Start: hl.Start, End: hl.End})
	}

	apiResult := &apiv1.SearchResult{
		Identifier: string(result.Identifier),
		Title:      result.Title,
		Fragment:   result.Fragment,
		Highlights: highlights,
	}

	if len(fmKeysToReturn) > 0 {
		apiResult.Frontmatter = make(map[string]string)
		for _, key := range fmKeysToReturn {
			if value := s.FrontmatterIndexQueryer.GetValue(mungedID, key); value != "" {
				apiResult.Frontmatter[key] = value
			}
		}
	}
	return apiResult
}

// ReadPage implements the ReadPage RPC.
func (s *Server) ReadPage(_ context.Context, req *apiv1.ReadPageRequest) (*apiv1.ReadPageResponse, error) {
	v := reflect.ValueOf(s.PageReaderMutator)
	if s.PageReaderMutator == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return nil, status.Error(codes.Internal, "PageReaderMutator not available")
	}

	// Read the page markdown and frontmatter
	_, markdown, err := s.PageReaderMutator.ReadMarkdown(wikipage.PageIdentifier(req.PageName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, pageNotFoundErrFmt, req.PageName)
		}
		return nil, status.Errorf(codes.Internal, "failed to read page: %v", err)
	}

	_, frontmatter, err := s.PageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(req.PageName))
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, failedToReadFrontmatterErrFmt, err)
	}

	// Convert frontmatter to TOML
	frontmatterToml, err := toml.Marshal(frontmatter)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to marshal frontmatter: %v", err)
	}

	// Build the page text with frontmatter
	var pageTextBuilder strings.Builder
	if len(frontmatter) > 0 {
		if _, err := pageTextBuilder.WriteString("+++\n"); err != nil {
			return nil, status.Errorf(codes.Internal, failedToBuildPageTextErrFmt, err)
		}
		if _, err := pageTextBuilder.Write(frontmatterToml); err != nil {
			return nil, status.Errorf(codes.Internal, failedToBuildPageTextErrFmt, err)
		}
		if !bytes.HasSuffix(frontmatterToml, []byte("\n")) {
			if _, err := pageTextBuilder.WriteString("\n"); err != nil {
				return nil, status.Errorf(codes.Internal, failedToBuildPageTextErrFmt, err)
			}
		}
		if _, err := pageTextBuilder.WriteString("+++\n"); err != nil {
			return nil, status.Errorf(codes.Internal, failedToBuildPageTextErrFmt, err)
		}
	}
	if _, err := pageTextBuilder.WriteString(string(markdown)); err != nil {
		return nil, status.Errorf(codes.Internal, failedToBuildPageTextErrFmt, err)
	}

	// Create a Page object and render it
	page := &wikipage.Page{
		Identifier: req.PageName,
		Text:       pageTextBuilder.String(),
	}

	// Render the page if rendering dependencies are available
	var renderedHTML string
	var renderedMarkdown string

	if s.MarkdownRenderer != nil && s.TemplateExecutor != nil {
		renderErr := page.Render(s.PageReaderMutator, s.MarkdownRenderer, s.TemplateExecutor, s.FrontmatterIndexQueryer)
		if renderErr != nil {
			return nil, status.Errorf(codes.Internal, "failed to render page: %v", renderErr)
		}
		renderedHTML = string(page.RenderedPage)
		renderedMarkdown = string(page.RenderedMarkdown)
	}

	return &apiv1.ReadPageResponse{
		ContentMarkdown:         string(markdown),
		FrontMatterToml:         string(frontmatterToml),
		RenderedContentHtml:     renderedHTML,
		RenderedContentMarkdown: renderedMarkdown,
	}, nil
}

// GenerateIdentifier implements the GenerateIdentifier RPC.
// Converts text to wiki identifier format and checks if it's available.
func (s *Server) GenerateIdentifier(_ context.Context, req *apiv1.GenerateIdentifierRequest) (*apiv1.GenerateIdentifierResponse, error) {
	if req.Text == "" {
		return nil, status.Error(codes.InvalidArgument, "text is required")
	}

	// Generate the base identifier
	identifier := wikiidentifiers.MungeIdentifier(req.Text)

	// Check if page exists
	isUnique, existingPage := s.checkIdentifierAvailability(identifier)

	// If ensure_unique is requested and page exists, find a unique suffix
	if req.EnsureUnique && !isUnique {
		identifier = s.findUniqueIdentifier(identifier)
		isUnique = true
		existingPage = nil
	}

	return &apiv1.GenerateIdentifierResponse{
		Identifier:   identifier,
		IsUnique:     isUnique,
		ExistingPage: existingPage,
	}, nil
}

// checkIdentifierAvailability checks if an identifier is available and returns info about existing page if not.
func (s *Server) checkIdentifierAvailability(identifier string) (bool, *apiv1.ExistingPageInfo) {
	v := reflect.ValueOf(s.PageReaderMutator)
	if s.PageReaderMutator == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		// If we can't check, assume it's unique
		return true, nil
	}

	_, fm, err := s.PageReaderMutator.ReadFrontMatter(identifier)
	if err != nil {
		// Page doesn't exist
		return true, nil
	}

	// Page exists, build info
	existingPage := &apiv1.ExistingPageInfo{
		Identifier: identifier,
	}

	// Get title from frontmatter
	if title, ok := fm["title"].(string); ok {
		existingPage.Title = title
	}

	// Get container from inventory.container
	if inv, ok := fm["inventory"].(map[string]any); ok {
		if container, ok := inv["container"].(string); ok {
			existingPage.Container = container
		}
	}

	return false, existingPage
}

// findUniqueIdentifier finds a unique identifier by adding numeric suffixes.
func (s *Server) findUniqueIdentifier(baseIdentifier string) string {
	v := reflect.ValueOf(s.PageReaderMutator)
	if s.PageReaderMutator == nil || (v.Kind() == reflect.Ptr && v.IsNil()) {
		return baseIdentifier
	}

	// Try suffixes _1, _2, _3, etc.
	for i := 1; i < maxUniqueIdentifierAttempts; i++ {
		candidate := fmt.Sprintf("%s_%d", baseIdentifier, i)

		_, _, err := s.PageReaderMutator.ReadFrontMatter(candidate)
		if err != nil {
			// Page doesn't exist, we found a unique identifier
			return candidate
		}
	}

	// Fallback: return with a high number
	return baseIdentifier + "_999"
}
