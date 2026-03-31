// Package v1 provides the implementation of gRPC services for version 1 of the API
package v1

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/filestore"
	"github.com/brendanjerwin/simple_wiki/index/bleve"
	"github.com/brendanjerwin/simple_wiki/pageimport"
	"github.com/brendanjerwin/simple_wiki/pkg/chatbuffer"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/templating"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
	"github.com/pelletier/go-toml/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	identifierKey                  = "identifier"
	pageNotFoundErrFmt             = "page not found: %s"
	failedToReadFrontmatterErrFmt  = "failed to read frontmatter: %v"
	failedToWriteFrontmatterErrFmt = "failed to write frontmatter: %v"
	failedToWriteMarkdownErrFmt    = "failed to write markdown: %v"
	pageNameRequiredErr            = "page_name is required"
	maxUniqueIdentifierAttempts    = 1000
	invalidTemplateErrFmt          = "invalid template in page content: %v"
)

// ChatBufferManager defines the interface for managing chat message buffers.
type ChatBufferManager interface {
	AddUserMessage(page, content, senderName string) (string, error)
	AddAssistantMessage(page, content, replyToID string) (string, error)
	EditMessage(messageID, newContent string) error
	AddReaction(messageID, emoji, reactor string) error
	GetMessages(page string) []*chatbuffer.Message
	SubscribeToPage(page string) (<-chan chatbuffer.Event, func())
	SubscribeToPageWithReplay(page string) ([]*chatbuffer.Message, <-chan chatbuffer.Event, func())
	SubscribeToChannel() (<-chan *chatbuffer.Message, func())
	HasChannelSubscribers() bool
}

// computeContentHash computes a SHA256 hash of the given markdown content,
// returned as a lowercase hex string. Used for optimistic concurrency control.
func computeContentHash(markdown wikipage.Markdown) string {
	h := sha256.Sum256([]byte(markdown))
	return hex.EncodeToString(h[:])
}

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

func mergeFrontmatterDeep(target, source map[string]any) {
	for key, value := range source {
		nestedSource, ok := value.(map[string]any)
		if !ok {
			target[key] = value
			continue
		}

		existingValue := target[key]
		nestedTarget, ok := existingValue.(map[string]any)
		if !ok {
			nestedTarget = make(map[string]any)
			target[key] = nestedTarget
		}

		mergeFrontmatterDeep(nestedTarget, nestedSource)
	}
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

// BuildInfo contains version and build metadata for the server.
type BuildInfo struct {
	Commit    string
	BuildTime time.Time
}

// Server is the implementation of the gRPC services.
type Server struct {
	apiv1.UnimplementedSystemInfoServiceServer
	apiv1.UnimplementedFrontmatterServer
	apiv1.UnimplementedPageManagementServiceServer
	apiv1.UnimplementedSearchServiceServer
	apiv1.UnimplementedInventoryManagementServiceServer
	apiv1.UnimplementedPageImportServiceServer
	apiv1.UnimplementedFileStorageServiceServer
	apiv1.UnimplementedChatServiceServer
	commit                  string
	buildTime               time.Time
	pageReaderMutator       wikipage.PageReaderMutator
	bleveIndexQueryer       bleve.IQueryBleveIndex
	jobQueueCoordinator     jobs.JobCoordinator
	logger                  *lumber.ConsoleLogger
	markdownRenderer        wikipage.IRenderMarkdownToHTML
	templateExecutor        wikipage.IExecuteTemplate
	frontmatterIndexQueryer wikipage.IQueryFrontmatterIndex
	fileStorer              filestore.FileStorer
	chatBufferManager       ChatBufferManager
	pageOpener              wikipage.PageOpener
}

// MergeFrontmatter implements the MergeFrontmatter RPC.
func (s *Server) MergeFrontmatter(_ context.Context, req *apiv1.MergeFrontmatterRequest) (resp *apiv1.MergeFrontmatterResponse, err error) {
	// Validate that the request doesn't contain an identifier key
	if req.Frontmatter != nil {
		newFm := req.Frontmatter.AsMap()
		if err := validateNoIdentifierKey(newFm); err != nil {
			return nil, err
		}
	}

	_, existingFm, err := s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(req.Page))
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, failedToReadFrontmatterErrFmt, err)
	}

	if existingFm == nil {
		existingFm = make(map[string]any)
	}

	if req.Frontmatter != nil {
		newFm := req.Frontmatter.AsMap()
		mergeFrontmatterDeep(existingFm, newFm)
	}

	err = s.pageReaderMutator.WriteFrontMatter(wikipage.PageIdentifier(req.Page), existingFm)
	if err != nil {
		return nil, status.Errorf(codes.Internal, failedToWriteFrontmatterErrFmt, err)
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
	var fm map[string]any
	if req.Frontmatter != nil {
		fm = req.Frontmatter.AsMap()
		// Filter out any user-provided identifier key and set the correct one
		fm = filterIdentifierKey(fm)
		fm[identifierKey] = req.Page
	}

	err = s.pageReaderMutator.WriteFrontMatter(wikipage.PageIdentifier(req.Page), fm)
	if err != nil {
		return nil, status.Errorf(codes.Internal, failedToWriteFrontmatterErrFmt, err)
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
	if len(req.GetKeyPath()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "key_path cannot be empty")
	}

	// Validate that the path is not targeting the identifier key
	if isIdentifierKeyPath(req.GetKeyPath()) {
		return nil, status.Error(codes.InvalidArgument, "identifier key cannot be removed")
	}

	_, fm, err := s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(req.Page))
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

	updatedFm, err := removeAtPath(map[string]any(fm), req.GetKeyPath())
	if err != nil {
		return nil, err
	}

	err = s.pageReaderMutator.WriteFrontMatter(wikipage.PageIdentifier(req.Page), wikipage.FrontMatter(updatedFm.(map[string]any)))
	if err != nil {
		return nil, status.Errorf(codes.Internal, failedToWriteFrontmatterErrFmt, err)
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
		return removeAtPathFromMap(v, component, remainingPath)
	case []any:
		return removeAtPathFromSlice(v, component, remainingPath)
	default:
		// Trying to traverse deeper, but `data` is a primitive.
		return nil, status.Error(codes.InvalidArgument, "path is deeper than data structure")
	}
}

// removeAtPathFromMap handles the map case for removeAtPath.
func removeAtPathFromMap(v map[string]any, component *apiv1.PathComponent, remainingPath []*apiv1.PathComponent) (any, error) {
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
		return v, nil
	}

	// Recursive step
	newValue, err := removeAtPath(value, remainingPath)
	if err != nil {
		return nil, err
	}
	v[key] = newValue
	return v, nil
}

// removeAtPathFromSlice handles the slice case for removeAtPath.
func removeAtPathFromSlice(v []any, component *apiv1.PathComponent, remainingPath []*apiv1.PathComponent) (any, error) {
	indexComp, ok := component.Component.(*apiv1.PathComponent_Index)
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "path component is not an index for a slice: %T", component.Component)
	}
	idx := int(indexComp.Index)

	if idx < 0 || idx >= len(v) {
		return nil, status.Errorf(codes.OutOfRange, "index %d is out of range for slice of length %d", idx, len(v))
	}

	if len(remainingPath) == 0 {
		// Base case: remove item from slice, zeroing the vacated slot to prevent memory leaks.
		copy(v[idx:], v[idx+1:])
		v[len(v)-1] = nil
		return v[:len(v)-1], nil
	}

	// Recursive step
	newValue, err := removeAtPath(v[idx], remainingPath)
	if err != nil {
		return nil, err
	}
	v[idx] = newValue
	return v, nil
}

// NewServer creates a new gRPC server with the given dependencies.
// Required dependencies: pageReaderMutator, bleveIndexQueryer, frontmatterIndexQueryer, logger, chatBufferManager, pageOpener.
// Optional dependencies can be set via WithJobQueueCoordinator, WithMarkdownRenderer, WithTemplateExecutor, WithFileStorer.
func NewServer(
	buildInfo BuildInfo,
	pageReaderMutator wikipage.PageReaderMutator,
	bleveIndexQueryer bleve.IQueryBleveIndex,
	frontmatterIndexQueryer wikipage.IQueryFrontmatterIndex,
	logger *lumber.ConsoleLogger,
	chatBufferManager ChatBufferManager,
	pageOpener wikipage.PageOpener,
) (*Server, error) {
	if pageReaderMutator == nil {
		return nil, errors.New("pageReaderMutator is required")
	}
	if bleveIndexQueryer == nil {
		return nil, errors.New("bleveIndexQueryer is required")
	}
	if frontmatterIndexQueryer == nil {
		return nil, errors.New("frontmatterIndexQueryer is required")
	}
	if logger == nil {
		return nil, errors.New("logger is required")
	}
	if chatBufferManager == nil {
		return nil, errors.New("chatBufferManager is required")
	}
	if pageOpener == nil {
		return nil, errors.New("pageOpener is required")
	}
	return &Server{
		commit:                  buildInfo.Commit,
		buildTime:               buildInfo.BuildTime,
		pageReaderMutator:       pageReaderMutator,
		bleveIndexQueryer:       bleveIndexQueryer,
		frontmatterIndexQueryer: frontmatterIndexQueryer,
		logger:                  logger,
		chatBufferManager:       chatBufferManager,
		pageOpener:              pageOpener,
	}, nil
}

// WithJobQueueCoordinator sets the optional job queue coordinator and returns the server for chaining.
func (s *Server) WithJobQueueCoordinator(jqc jobs.JobCoordinator) *Server {
	s.jobQueueCoordinator = jqc
	return s
}

// WithMarkdownRenderer sets the optional markdown renderer and returns the server for chaining.
func (s *Server) WithMarkdownRenderer(r wikipage.IRenderMarkdownToHTML) *Server {
	s.markdownRenderer = r
	return s
}

// WithTemplateExecutor sets the optional template executor and returns the server for chaining.
func (s *Server) WithTemplateExecutor(e wikipage.IExecuteTemplate) *Server {
	s.templateExecutor = e
	return s
}

// WithFileStorer sets the optional file storer and returns the server for chaining.
func (s *Server) WithFileStorer(fs filestore.FileStorer) *Server {
	s.fileStorer = fs
	return s
}

// RegisterWithServer registers the gRPC services with the given gRPC server.
func (s *Server) RegisterWithServer(grpcServer *grpc.Server) {
	apiv1.RegisterSystemInfoServiceServer(grpcServer, s)
	apiv1.RegisterFrontmatterServer(grpcServer, s)
	apiv1.RegisterPageManagementServiceServer(grpcServer, s)
	apiv1.RegisterSearchServiceServer(grpcServer, s)
	apiv1.RegisterInventoryManagementServiceServer(grpcServer, s)
	apiv1.RegisterPageImportServiceServer(grpcServer, s)
	apiv1.RegisterFileStorageServiceServer(grpcServer, s)
	apiv1.RegisterChatServiceServer(grpcServer, s)
}

// GetVersion implements the GetVersion RPC.
func (s *Server) GetVersion(ctx context.Context, _ *apiv1.GetVersionRequest) (*apiv1.GetVersionResponse, error) {
	response := &apiv1.GetVersionResponse{
		Commit:    s.commit,
		BuildTime: timestamppb.New(s.buildTime),
	}

	// Add Tailscale identity if available
	identity := tailscale.IdentityFromContext(ctx)
	if !identity.IsAnonymous() {
		response.TailscaleIdentity = &apiv1.TailscaleIdentity{
			LoginName:   proto.String(identity.LoginName()),
			DisplayName: proto.String(identity.DisplayName()),
			NodeName:    proto.String(identity.NodeName()),
		}
	}

	return response, nil
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

// buildJobStatusResponse builds a GetJobStatusResponse from the job queue coordinator.
func (s *Server) buildJobStatusResponse() *apiv1.GetJobStatusResponse {
	if s.jobQueueCoordinator == nil {
		return &apiv1.GetJobStatusResponse{
			JobQueues: []*apiv1.JobQueueStatus{},
		}
	}

	progress := s.jobQueueCoordinator.GetJobProgress()
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
	var fm map[string]any
	_, fm, err = s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(req.Page))
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

		// Get identity if available
		identity := tailscale.IdentityFromContext(ctx)
		identityStr := "anonymous"
		if !identity.IsAnonymous() {
			identityStr = identity.ForLog()
		}

		if s.logger != nil {
			s.logger.Warn("[GRPC] %s | %s | %v | %s",
				statusCode,
				duration,
				info.FullMethod,
				identityStr,
			)
		}

		return resp, err
	}
}

// DeletePage implements the DeletePage RPC.
func (s *Server) DeletePage(ctx context.Context, req *apiv1.DeletePageRequest) (*apiv1.DeletePageResponse, error) {
	err := s.pageReaderMutator.DeletePage(wikipage.PageIdentifier(req.PageName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "page not found: %s", req.PageName)
		}
		return nil, status.Errorf(codes.Internal, "failed to delete page: %v", err)
	}

	identity := tailscale.IdentityFromContext(ctx)
	s.logger.Info("[AUDIT] delete | page: %q | user: %q", req.PageName, identity.ForLog())

	return &apiv1.DeletePageResponse{
		Success: true,
		Error:   "",
	}, nil
}

// SearchContent implements the SearchContent RPC.
func (s *Server) SearchContent(_ context.Context, req *apiv1.SearchContentRequest) (*apiv1.SearchContentResponse, error) {
	if err := s.validateSearchRequest(req); err != nil {
		return nil, err
	}

	searchResults, err := s.bleveIndexQueryer.Query(req.Query)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to search: %v", err)
	}

	includeFilterSets := s.buildIncludeFilterSets(req.FrontmatterKeyIncludeFilters)
	excludedPages := s.buildExcludedPagesSet(req.FrontmatterKeyExcludeFilters)
	results, err := s.filterAndConvertResults(searchResults, includeFilterSets, excludedPages, req.FrontmatterKeysToReturnInResults)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to filter search results: %v", err)
	}

	// Return total unfiltered count when filters are applied (for inventory filter warning)
	// When no filters are applied, return 0 to indicate no filtering occurred
	totalUnfilteredCount := int32(0)
	hasFilters := len(req.FrontmatterKeyIncludeFilters) > 0 || len(req.FrontmatterKeyExcludeFilters) > 0
	if hasFilters {
		totalUnfilteredCount = int32(len(searchResults))
	}

	return &apiv1.SearchContentResponse{
		Results:              results,
		TotalUnfilteredCount: totalUnfilteredCount,
	}, nil
}

// ListPagesByFrontmatter lists pages matching a frontmatter key-value pair, sorted by another frontmatter key.
func (s *Server) ListPagesByFrontmatter(_ context.Context, req *apiv1.ListPagesByFrontmatterRequest) (*apiv1.ListPagesByFrontmatterResponse, error) {
	if req.MatchKey == "" {
		return nil, status.Error(codes.InvalidArgument, "match_key is required")
	}
	if req.SortByKey == "" {
		return nil, status.Error(codes.InvalidArgument, "sort_by_key is required")
	}
	if req.MaxResults < 0 {
		return nil, status.Error(codes.InvalidArgument, "max_results must be >= 0")
	}
	if req.ContentExcerptMaxChars < 0 {
		return nil, status.Error(codes.InvalidArgument, "content_excerpt_max_chars must be >= 0")
	}

	pageIDs := s.frontmatterIndexQueryer.QueryExactMatchSortedBy(
		req.MatchKey,
		req.MatchValue,
		req.SortByKey,
		req.SortAscending,
		int(req.MaxResults),
	)

	results := make([]*apiv1.FrontmatterQueryResult, 0, len(pageIDs))
	for _, id := range pageIDs {
		fmValues := make(map[string]string, len(req.FrontmatterKeysToReturn))
		for _, key := range req.FrontmatterKeysToReturn {
			fmValues[key] = s.frontmatterIndexQueryer.GetValue(id, key)
		}

		contentExcerpt := s.readContentExcerpt(id, req.ContentExcerptMaxChars)

		results = append(results, &apiv1.FrontmatterQueryResult{
			Identifier:        string(id),
			FrontmatterValues: fmValues,
			ContentExcerpt:    contentExcerpt,
		})
	}

	return &apiv1.ListPagesByFrontmatterResponse{
		Results: results,
	}, nil
}

// readContentExcerpt reads up to maxChars runes of markdown content for the given page.
// Returns an empty string when maxChars is 0, the page cannot be read, or the content is empty.
func (s *Server) readContentExcerpt(id wikipage.PageIdentifier, maxChars int32) string {
	if maxChars <= 0 {
		return ""
	}
	_, markdown, err := s.pageReaderMutator.ReadMarkdown(id)
	if err != nil || len(markdown) == 0 {
		return ""
	}
	content := string(markdown)
	runes := []rune(content)
	if int32(len(runes)) > maxChars {
		return string(runes[:maxChars])
	}
	return content
}

// validateSearchRequest validates the search request.
func (*Server) validateSearchRequest(req *apiv1.SearchContentRequest) error {
	if req.Query == "" {
		return status.Error(codes.InvalidArgument, "query cannot be empty")
	}
	return nil
}

// buildIncludeFilterSets builds sets of pages for each include filter key.
func (s *Server) buildIncludeFilterSets(filterKeys []string) []map[wikipage.PageIdentifier]bool {
	if len(filterKeys) == 0 {
		return nil
	}

	var filterSets []map[wikipage.PageIdentifier]bool
	for _, filterKey := range filterKeys {
		pageIDs := s.frontmatterIndexQueryer.QueryKeyExistence(wikipage.DottedKeyPath(filterKey))
		pageSet := make(map[wikipage.PageIdentifier]bool, len(pageIDs))
		for _, id := range pageIDs {
			pageSet[id] = true
		}
		filterSets = append(filterSets, pageSet)
	}
	return filterSets
}

// buildExcludedPagesSet builds the set of pages to exclude based on filter keys.
func (s *Server) buildExcludedPagesSet(filterKeys []string) map[wikipage.PageIdentifier]bool {
	if len(filterKeys) == 0 {
		return nil
	}

	excludedPages := make(map[wikipage.PageIdentifier]bool)
	for _, filterKey := range filterKeys {
		pageIDs := s.frontmatterIndexQueryer.QueryKeyExistence(wikipage.DottedKeyPath(filterKey))
		for _, id := range pageIDs {
			excludedPages[id] = true
		}
	}
	return excludedPages
}

// filterAndConvertResults filters search results and converts them to API format.
func (s *Server) filterAndConvertResults(
	searchResults []bleve.SearchResult,
	includeFilterSets []map[wikipage.PageIdentifier]bool,
	excludedPages map[wikipage.PageIdentifier]bool,
	fmKeysToReturn []string,
) ([]*apiv1.SearchResult, error) {
	var results []*apiv1.SearchResult
	for _, result := range searchResults {
		mungedIDStr, err := wikiidentifiers.MungeIdentifier(string(result.Identifier))
		if err != nil {
			return nil, fmt.Errorf("invalid identifier %q in search index: %w", result.Identifier, err)
		}
		mungedID := wikipage.PageIdentifier(mungedIDStr)

		if !matchesAllIncludeFilters(mungedID, includeFilterSets) {
			continue
		}
		if excludedPages[mungedID] {
			continue
		}

		apiResult, err := s.convertSearchResult(result, mungedID, fmKeysToReturn)
		if err != nil {
			return nil, err
		}
		results = append(results, apiResult)
	}
	return results, nil
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
func (s *Server) convertSearchResult(result bleve.SearchResult, mungedID wikipage.PageIdentifier, fmKeysToReturn []string) (*apiv1.SearchResult, error) {
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
			if value := s.frontmatterIndexQueryer.GetValue(mungedID, key); value != "" {
				apiResult.Frontmatter[key] = value
			}
		}
	}

	inventoryContext, err := s.buildInventoryContext(mungedID)
	if err != nil {
		return nil, fmt.Errorf("failed to build inventory context for %s: %w", mungedID, err)
	}
	apiResult.InventoryContext = inventoryContext

	return apiResult, nil
}

// buildInventoryContext builds inventory context for a search result if applicable.
// Returns nil only when the item is not inventory-related (no inventory.container in frontmatter).
func (s *Server) buildInventoryContext(itemID wikipage.PageIdentifier) (*apiv1.InventoryContext, error) {
	containerID := s.frontmatterIndexQueryer.GetValue(itemID, "inventory.container")
	if containerID == "" {
		return nil, nil
	}

	// Build the full path from root to immediate container
	path, err := s.buildContainerPath(containerID)
	if err != nil {
		return nil, err
	}

	return &apiv1.InventoryContext{
		IsInventoryRelated: true,
		Path:               path,
	}, nil
}

// buildContainerPath recursively builds the full container path from root to the given container.
func (s *Server) buildContainerPath(containerID string) ([]*apiv1.ContainerPathElement, error) {
	const maxDepth = 20 // Prevent infinite loops
	var reversed []*apiv1.ContainerPathElement
	visited := make(map[string]bool)

	currentID := containerID

	// Collect elements from immediate container up to root (in reverse order)
	for currentID != "" && len(reversed) < maxDepth {
		if visited[currentID] {
			// Circular reference detected, break
			break
		}
		visited[currentID] = true

		mungedIDStr, err := wikiidentifiers.MungeIdentifier(currentID)
		if err != nil {
			return nil, fmt.Errorf("invalid container identifier %q in path: %w", currentID, err)
		}
		mungedID := wikipage.PageIdentifier(mungedIDStr)
		title := s.frontmatterIndexQueryer.GetValue(mungedID, "title")

		reversed = append(reversed, &apiv1.ContainerPathElement{
			Identifier: currentID,
			Title:      title,
			// Depth will be set after we know the total path length
		})

		// Get the parent container
		currentID = s.frontmatterIndexQueryer.GetValue(mungedID, "inventory.container")
	}

	// Reverse to get root-first order, then assign depth values
	path := make([]*apiv1.ContainerPathElement, len(reversed))
	for i, el := range reversed {
		j := len(reversed) - 1 - i
		el.Depth = int32(j)
		path[j] = el
	}

	return path, nil
}

// buildPageText assembles the full wiki page text by prepending TOML frontmatter
// (enclosed in +++ delimiters) to the markdown body.
func buildPageText(frontmatter map[string]any, frontmatterToml []byte, markdown wikipage.Markdown) string {
	var b strings.Builder

	if len(frontmatter) > 0 {
		_, _ = b.WriteString("+++\n") // WriteString on strings.Builder never fails
		_, _ = b.Write(frontmatterToml) // Write on strings.Builder never fails
		if !bytes.HasSuffix(frontmatterToml, []byte("\n")) {
			_, _ = b.WriteString("\n") // WriteString on strings.Builder never fails
		}
		_, _ = b.WriteString("+++\n") // WriteString on strings.Builder never fails
	}

	_, _ = b.WriteString(string(markdown)) // WriteString on strings.Builder never fails

	return b.String()
}

// ReadPage implements the ReadPage RPC.
func (s *Server) ReadPage(_ context.Context, req *apiv1.ReadPageRequest) (*apiv1.ReadPageResponse, error) {
	// Read the page markdown and frontmatter
	_, markdown, err := s.pageReaderMutator.ReadMarkdown(wikipage.PageIdentifier(req.PageName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, pageNotFoundErrFmt, req.PageName)
		}
		return nil, status.Errorf(codes.Internal, "failed to read page: %v", err)
	}

	_, frontmatter, err := s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(req.PageName))
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, failedToReadFrontmatterErrFmt, err)
	}

	// Convert frontmatter to TOML
	frontmatterToml, err := toml.Marshal(frontmatter)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to marshal frontmatter: %v", err)
	}

	pageText := buildPageText(frontmatter, frontmatterToml, markdown)

	// Create a Page object and render it
	page := &wikipage.Page{
		Identifier: req.PageName,
		Text:       pageText,
	}

	// Render the page if rendering dependencies are available
	var renderedHTML string
	var renderedMarkdown string

	if s.markdownRenderer != nil && s.templateExecutor != nil {
		renderErr := page.Render(s.pageReaderMutator, s.markdownRenderer, s.templateExecutor, s.frontmatterIndexQueryer)
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
		VersionHash:             computeContentHash(markdown),
	}, nil
}

// RenderMarkdown implements the RenderMarkdown RPC.
// Renders arbitrary markdown content to HTML with chat-safe template macros.
func (s *Server) RenderMarkdown(ctx context.Context, req *apiv1.RenderMarkdownRequest) (*apiv1.RenderMarkdownResponse, error) {
	select {
	case <-ctx.Done():
		return nil, status.FromContextError(ctx.Err()).Err()
	default:
	}

	if req.Content == "" {
		return &apiv1.RenderMarkdownResponse{RenderedHtml: ""}, nil
	}

	if s.markdownRenderer == nil {
		return nil, status.Error(codes.FailedPrecondition, "markdown renderer is not available")
	}

	markdownBytes := []byte(req.Content)

	// Always execute chat-safe template macros, enriching with page frontmatter when available
	frontmatter := make(wikipage.FrontMatter)
	if req.Page != "" {
		_, pageFM, err := s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(req.Page))
		if err != nil && !os.IsNotExist(err) {
			return nil, status.Errorf(codes.Internal, "failed to read page frontmatter: %v", err)
		}
		if err == nil {
			frontmatter = pageFM
		}
	}

	chatExecutor := server.ChatTemplateExecutor{}
	expanded, err := chatExecutor.ExecuteTemplate(string(markdownBytes), frontmatter, s.pageReaderMutator, s.frontmatterIndexQueryer)
	if err != nil {
		// Template execution failed — render the raw markdown without macros
		s.logger.Warn("chat template execution failed, rendering raw markdown: %v", err)
	} else {
		markdownBytes = expanded
	}

	renderedHTML, err := wikipage.RenderMarkdownToHTML(markdownBytes, s.markdownRenderer)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to render markdown: %v", err)
	}

	return &apiv1.RenderMarkdownResponse{RenderedHtml: string(renderedHTML)}, nil
}

// GenerateIdentifier implements the GenerateIdentifier RPC.
// Converts text to wiki identifier format and checks if it's available.
func (s *Server) GenerateIdentifier(_ context.Context, req *apiv1.GenerateIdentifierRequest) (*apiv1.GenerateIdentifierResponse, error) {
	if req.Text == "" {
		return nil, status.Error(codes.InvalidArgument, "text is required")
	}

	// Generate the base identifier
	identifier, err := wikiidentifiers.MungeIdentifier(req.Text)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "cannot generate identifier from text: %v", err)
	}

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
	_, fm, err := s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(identifier))
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
	// Try suffixes _1, _2, _3, etc.
	for i := 1; i < maxUniqueIdentifierAttempts; i++ {
		candidate := fmt.Sprintf("%s_%d", baseIdentifier, i)

		_, _, err := s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(candidate))
		if err != nil {
			// Page doesn't exist, we found a unique identifier
			return candidate
		}
	}

	// Fallback: return with a high number
	return baseIdentifier + "_999"
}

// CreatePage implements the CreatePage RPC.
// Creates a new wiki page with optional template support.
func (s *Server) CreatePage(_ context.Context, req *apiv1.CreatePageRequest) (*apiv1.CreatePageResponse, error) {
	if req.PageName == "" {
		return nil, status.Error(codes.InvalidArgument, pageNameRequiredErr)
	}

	identifier, err := wikiidentifiers.MungeIdentifier(req.PageName)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid page name: %v", err)
	}

	_, existingFm, err := s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(identifier))
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "failed to check page existence: %v", err)
	}
	if existingFm != nil {
		return &apiv1.CreatePageResponse{
			Success: false,
			Error:   fmt.Sprintf("page already exists: %s", identifier),
		}, nil
	}

	fm, err := s.buildNewPageFrontmatter(identifier, req.Template, req.Frontmatter)
	if err != nil {
		return &apiv1.CreatePageResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	markdown := req.ContentMarkdown
	if markdown == "" {
		markdown = wikipage.DefaultPageTemplate
	}

	if err := templating.ValidateTemplate(string(markdown)); err != nil {
		return &apiv1.CreatePageResponse{
			Success: false,
			Error:   fmt.Sprintf(invalidTemplateErrFmt, err),
		}, nil
	}

	if err := s.pageReaderMutator.WriteFrontMatter(wikipage.PageIdentifier(identifier), wikipage.FrontMatter(fm)); err != nil {
		return nil, status.Errorf(codes.Internal, failedToWriteFrontmatterErrFmt, err)
	}

	if err := s.pageReaderMutator.WriteMarkdown(wikipage.PageIdentifier(identifier), wikipage.Markdown(markdown)); err != nil {
		return nil, status.Errorf(codes.Internal, failedToWriteMarkdownErrFmt, err)
	}

	return &apiv1.CreatePageResponse{
		Success: true,
	}, nil
}

// buildNewPageFrontmatter assembles frontmatter for a new page by merging
// template defaults with any explicitly provided frontmatter values.
func (s *Server) buildNewPageFrontmatter(identifier string, template *string, frontmatter *structpb.Struct) (map[string]any, error) {
	fm := map[string]any{identifierKey: identifier}

	if template != nil && *template != "" {
		templateFm, err := s.loadTemplateFrontmatter(*template)
		if err != nil {
			return nil, err
		}
		applyTemplateFrontmatter(fm, templateFm)
	}

	applyProvidedFrontmatter(fm, frontmatter)
	return fm, nil
}

// applyTemplateFrontmatter copies template frontmatter keys to dest,
// excluding the identifier and template keys.
func applyTemplateFrontmatter(dest, src map[string]any) {
	for k, v := range src {
		if k != identifierKey && k != "template" {
			dest[k] = v
		}
	}
}

// applyProvidedFrontmatter copies user-provided frontmatter keys to dest,
// excluding the identifier key.
func applyProvidedFrontmatter(dest map[string]any, frontmatter *structpb.Struct) {
	if frontmatter == nil {
		return
	}
	for k, v := range frontmatter.AsMap() {
		if k != identifierKey {
			dest[k] = v
		}
	}
}

// UpdatePageContent implements the UpdatePageContent RPC.
// Updates only the markdown content of an existing page, preserving its frontmatter.
// If old_content_markdown is provided, performs a find-and-replace: locates the old content
// within the page and substitutes it with new_content_markdown, leaving the rest intact.
// If expected_version_hash is provided, the write will be rejected if the current content
// has changed since the hash was computed (optimistic concurrency control).
// Empty content is rejected; use ClearPageContent to explicitly clear a page's content.
func (s *Server) UpdatePageContent(_ context.Context, req *apiv1.UpdatePageContentRequest) (*apiv1.UpdatePageContentResponse, error) {
	if req.PageName == "" {
		return nil, status.Error(codes.InvalidArgument, pageNameRequiredErr)
	}

	if strings.TrimSpace(req.NewContentMarkdown) == "" {
		return nil, status.Error(codes.InvalidArgument, "new_content_markdown cannot be empty; use ClearPageContent to explicitly clear page content")
	}

	if req.OldContentMarkdown != nil && strings.TrimSpace(*req.OldContentMarkdown) == "" {
		return nil, status.Error(codes.InvalidArgument, "old_content_markdown cannot be empty when provided")
	}

	// Read current content for: (1) page existence check, (2) version hash verification
	// if requested, and (3) rollback data if the post-write invariant check fails.
	_, originalMarkdown, err := s.pageReaderMutator.ReadMarkdown(wikipage.PageIdentifier(req.PageName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, pageNotFoundErrFmt, req.PageName)
		}
		return nil, status.Errorf(codes.Internal, "failed to read current content: %v", err)
	}

	// NOTE: There is a narrow TOCTOU race between this hash check and the subsequent
	// WriteMarkdown call. True atomicity would require file-level locking at the storage layer.
	if err := checkContentVersionHash(originalMarkdown, req.ExpectedVersionHash); err != nil {
		return nil, err
	}

	if err := templating.ValidateTemplate(req.NewContentMarkdown); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, invalidTemplateErrFmt, err)
	}

	// Compute the markdown to write: find-and-replace when old_content_markdown is set,
	// otherwise replace the entire page content.
	markdownToWrite, err := resolveMarkdownContent(originalMarkdown, req.OldContentMarkdown, req.NewContentMarkdown)
	if err != nil {
		return nil, err
	}

	if err := s.pageReaderMutator.WriteMarkdown(wikipage.PageIdentifier(req.PageName), markdownToWrite); err != nil {
		return nil, status.Errorf(codes.Internal, failedToWriteMarkdownErrFmt, err)
	}

	// Invariant check: read back the stored content to ensure the write did not blank the page.
	// If the content is missing or empty after a successful write, attempt to restore the
	// original content to prevent data loss.
	storedMarkdown, err := s.verifyStoredContent(wikipage.PageIdentifier(req.PageName), originalMarkdown)
	if err != nil {
		return nil, err
	}

	return &apiv1.UpdatePageContentResponse{
		Success:     true,
		VersionHash: computeContentHash(storedMarkdown),
	}, nil
}

// checkContentVersionHash verifies that the current page content matches the expected version hash.
// Returns an error if there is a version mismatch; returns nil if expectedHash is nil (no check requested).
func checkContentVersionHash(currentMarkdown wikipage.Markdown, expectedHash *string) error {
	if expectedHash == nil {
		return nil
	}

	currentHash := computeContentHash(currentMarkdown)
	if subtle.ConstantTimeCompare([]byte(currentHash), []byte(*expectedHash)) != 1 {
		return status.Error(codes.Aborted, "content version mismatch: page was modified since last read; re-read the page and retry")
	}

	return nil
}

// resolveMarkdownContent computes the markdown to write.
// When oldContentMarkdown is provided, performs a find-and-replace within originalMarkdown.
// Otherwise returns newContentMarkdown as the full replacement.
func resolveMarkdownContent(originalMarkdown wikipage.Markdown, oldContentMarkdown *string, newContentMarkdown string) (wikipage.Markdown, error) {
	if oldContentMarkdown == nil {
		return wikipage.Markdown(newContentMarkdown), nil
	}

	if !strings.Contains(string(originalMarkdown), *oldContentMarkdown) {
		return "", status.Error(codes.NotFound, "old_content_markdown not found in current page content")
	}

	return wikipage.Markdown(strings.Replace(string(originalMarkdown), *oldContentMarkdown, newContentMarkdown, 1)), nil
}

// verifyStoredContent reads back the page after a write to confirm the content is non-empty.
// If the stored content is empty or unreadable, it attempts to restore the original content.
func (s *Server) verifyStoredContent(pageID wikipage.PageIdentifier, originalMarkdown wikipage.Markdown) (wikipage.Markdown, error) {
	_, storedMarkdown, readBackErr := s.pageReaderMutator.ReadMarkdown(pageID)
	if readBackErr != nil || strings.TrimSpace(string(storedMarkdown)) == "" {
		_ = s.pageReaderMutator.WriteMarkdown(pageID, originalMarkdown)
		if readBackErr != nil {
			return "", status.Errorf(codes.Internal, "failed to verify stored content after write: %v", readBackErr)
		}
		return "", status.Error(codes.Internal, "invariant violation: write resulted in empty content; original content has been restored")
	}

	return storedMarkdown, nil
}

// ClearPageContent implements the ClearPageContent RPC.
// Explicitly clears the markdown content of a page, preserving its frontmatter.
// confirm_clear must be true to prevent accidental data loss.
func (s *Server) ClearPageContent(_ context.Context, req *apiv1.ClearPageContentRequest) (*apiv1.ClearPageContentResponse, error) {
	if req.PageName == "" {
		return nil, status.Error(codes.InvalidArgument, pageNameRequiredErr)
	}

	if !req.ConfirmClear {
		return nil, status.Error(codes.InvalidArgument, "confirm_clear must be true to clear page content")
	}

	// Verify the page exists
	_, _, err := s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(req.PageName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, pageNotFoundErrFmt, req.PageName)
		}
		return nil, status.Errorf(codes.Internal, failedToReadFrontmatterErrFmt, err)
	}

	if err := s.pageReaderMutator.WriteMarkdown(wikipage.PageIdentifier(req.PageName), ""); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to clear markdown: %v", err)
	}

	return &apiv1.ClearPageContentResponse{Success: true}, nil
}

// UpdateWholePage implements the UpdateWholePage RPC.
// Replaces the full content of an existing page, including its frontmatter.
// The new_whole_markdown field must contain the complete page text (frontmatter + markdown).
func (s *Server) UpdateWholePage(_ context.Context, req *apiv1.UpdateWholePageRequest) (*apiv1.UpdateWholePageResponse, error) {
	if req.PageName == "" {
		return nil, status.Error(codes.InvalidArgument, pageNameRequiredErr)
	}

	// Verify the page exists
	_, _, err := s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(req.PageName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, pageNotFoundErrFmt, req.PageName)
		}
		return nil, status.Errorf(codes.Internal, failedToReadFrontmatterErrFmt, err)
	}

	// Parse frontmatter and markdown from the combined content
	page := &wikipage.Page{
		Identifier: req.PageName,
		Text:       req.NewWholeMarkdown,
	}

	fm, err := page.GetFrontMatter()
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to parse frontmatter: %v", err)
	}

	md, err := page.GetMarkdown()
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to parse markdown: %v", err)
	}

	if err := templating.ValidateTemplate(string(md)); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, invalidTemplateErrFmt, err)
	}

	// Preserve the page identifier in frontmatter
	if fm == nil {
		fm = make(map[string]any)
	}
	fm[identifierKey] = req.PageName

	if err := s.pageReaderMutator.WriteMarkdown(wikipage.PageIdentifier(req.PageName), md); err != nil {
		return nil, status.Errorf(codes.Internal, failedToWriteMarkdownErrFmt, err)
	}

	if err := s.pageReaderMutator.WriteFrontMatter(wikipage.PageIdentifier(req.PageName), fm); err != nil {
		return nil, status.Errorf(codes.Internal, failedToWriteFrontmatterErrFmt, err)
	}

	return &apiv1.UpdateWholePageResponse{Success: true}, nil
}

// loadTemplateFrontmatter loads and validates a template page's frontmatter.
func (s *Server) loadTemplateFrontmatter(templateIdentifier string) (map[string]any, error) {
	_, fm, err := s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(templateIdentifier))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("template '%s' does not exist", templateIdentifier)
		}
		return nil, fmt.Errorf("failed to read template '%s': %w", templateIdentifier, err)
	}

	// Validate that the page is marked as a template
	if !isTemplatePage(fm) {
		return nil, fmt.Errorf("page '%s' is not a template (missing template: true)", templateIdentifier)
	}

	return fm, nil
}

// isTemplatePage checks if a frontmatter map indicates a template page.
func isTemplatePage(fm map[string]any) bool {
	templateVal, ok := fm["template"]
	if !ok {
		return false
	}

	switch v := templateVal.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true")
	case int64:
		// TOML parses integers as int64
		return v != 0
	case float64:
		// Handle floats for robustness
		return v != 0
	default:
		return false
	}
}

// ListTemplates implements the ListTemplates RPC.
// Returns all pages marked as templates (with template: true frontmatter).
func (s *Server) ListTemplates(_ context.Context, req *apiv1.ListTemplatesRequest) (*apiv1.ListTemplatesResponse, error) {
	// Build exclusion set
	excludeSet := make(map[string]bool)
	for _, id := range req.ExcludeIdentifiers {
		excludeSet[id] = true
	}

	// Query pages with template: true
	templatePages := s.frontmatterIndexQueryer.QueryExactMatch("template", "true")

	templates := make([]*apiv1.TemplateInfo, 0, len(templatePages))
	for _, pageID := range templatePages {
		// Skip excluded identifiers
		if excludeSet[string(pageID)] {
			continue
		}

		// Read frontmatter to get title and description
		_, fm, err := s.pageReaderMutator.ReadFrontMatter(pageID)
		if err != nil {
			// Skip pages that can't be read
			continue
		}

		template := &apiv1.TemplateInfo{
			Identifier: string(pageID),
		}

		// Get title
		if title, ok := fm["title"].(string); ok {
			template.Title = title
		}

		// Get description
		if desc, ok := fm["description"].(string); ok {
			template.Description = desc
		}

		templates = append(templates, template)
	}

	return &apiv1.ListTemplatesResponse{
		Templates: templates,
	}, nil
}

// WatchPage implements the WatchPage RPC for real-time page content change notifications.
// It streams the current version_hash of the page content when it changes.
func (s *Server) WatchPage(req *apiv1.WatchPageRequest, stream apiv1.PageManagementService_WatchPageServer) error {
	if req.PageName == "" {
		return status.Error(codes.InvalidArgument, pageNameRequiredErr)
	}

	pageID := wikipage.PageIdentifier(req.PageName)
	interval := resolveCheckInterval(req.GetCheckIntervalMs())

	// Read initial content hash and mod time
	hash, modTime, err := s.readPageHashAndModTime(pageID)
	if err != nil {
		if os.IsNotExist(err) {
			return status.Errorf(codes.NotFound, pageNotFoundErrFmt, req.PageName)
		}
		return status.Errorf(codes.Internal, "failed to read page: %v", err)
	}

	lastHash := hash

	// Send initial hash and mod time immediately
	if err := stream.Send(&apiv1.WatchPageResponse{
		VersionHash:  lastHash,
		LastModified: timestamppb.New(modTime),
	}); err != nil {
		return err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Stream updates when content changes
	for {
		select {
		case <-ticker.C:
			newHash, err := s.sendPageUpdateIfChanged(stream, pageID, lastHash)
			if err != nil {
				return err
			}
			lastHash = newHash

		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

// resolveCheckInterval returns the polling interval for WatchPage.
// Defaults to 1 second when not specified; enforces a 100ms minimum to protect the server.
func resolveCheckInterval(requestedIntervalMs int32) time.Duration {
	const defaultInterval = 1 * time.Second
	const minIntervalMs = 100

	interval := time.Duration(requestedIntervalMs) * time.Millisecond
	if interval == 0 {
		interval = defaultInterval
	}

	if interval < minIntervalMs*time.Millisecond {
		interval = minIntervalMs * time.Millisecond
	}

	return interval
}

// sendPageUpdateIfChanged checks whether the page content has changed since lastHash and,
// if so, sends a WatchPageResponse on the stream. Returns the current hash (updated or unchanged).
// On a transient read error it logs and returns the previous hash so the caller can retry.
func (s *Server) sendPageUpdateIfChanged(stream apiv1.PageManagementService_WatchPageServer, pageID wikipage.PageIdentifier, lastHash string) (string, error) {
	currentHash, currentModTime, err := s.readPageHashAndModTime(pageID)
	if err != nil {
		if os.IsNotExist(err) {
			return "", status.Errorf(codes.NotFound, "page deleted: %s", string(pageID))
		}
		s.logger.Warn("WatchPage: failed to read page content, continuing: %v", err)
		return lastHash, nil
	}

	if currentHash == lastHash {
		return lastHash, nil
	}

	if err := stream.Send(&apiv1.WatchPageResponse{
		VersionHash:  currentHash,
		LastModified: timestamppb.New(currentModTime),
	}); err != nil {
		return "", err
	}

	return currentHash, nil
}

// readPageHashAndModTime reads a page's content hash and file modification time.
func (s *Server) readPageHashAndModTime(pageID wikipage.PageIdentifier) (string, time.Time, error) {
	page, err := s.pageOpener.ReadPage(pageID)
	if err != nil {
		return "", time.Time{}, err
	}
	if page.IsNew() {
		return "", time.Time{}, os.ErrNotExist
	}
	markdown, err := page.GetMarkdown()
	if err != nil {
		return "", time.Time{}, err
	}
	return computeContentHash(wikipage.Markdown(markdown)), page.ModTime, nil
}

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
