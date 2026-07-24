package v1

import (
	"context"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// PageHistoryReader is the contract the PageHistoryService handlers use to
// read and restore page version history. The concrete type is *pagestore.Store.
type PageHistoryReader interface {
	ListVersions(identifier wikipage.PageIdentifier) ([]PageVersionMetadata, error)
	ReadVersion(identifier wikipage.PageIdentifier, versionID string) (string, error)
	RestoreVersion(identifier wikipage.PageIdentifier, versionID string, identity wikipage.Identity) error
	DiffVersions(identifier wikipage.PageIdentifier, oldID, newID string) (string, error)
}

// PageVersionMetadata is the metadata for a single version, as returned by
// the pagestore.HistoryReader interface. Re-declared here to avoid a direct
// dependency on the pagestore package.
type PageVersionMetadata struct {
	VersionID string
	CreatedAt time.Time
	Author    string
	IsAgent   bool
	Source    string
	SHA256    string
	ByteSize  int64
}

// HistorySearchFilter holds the filter parameters for a global history search.
type HistorySearchFilter struct {
	Query          string
	PageNameFilter string
	AuthorFilter   string
	From           time.Time
	To             time.Time
}

// HistorySearchResult is a single result from a history search.
type HistorySearchResult struct {
	PageName string
	Version  PageVersionMetadata
	Snippet  string
}

// HistorySearcher is the contract for searching page history.
type HistorySearcher interface {
	SearchPageHistory(identifier wikipage.PageIdentifier, query string) ([]HistorySearchResult, error)
	SearchHistory(filter HistorySearchFilter) ([]HistorySearchResult, error)
}

// ListPageVersions implements the PageHistoryService RPC.
func (s *Server) ListPageVersions(_ context.Context, req *apiv1.ListPageVersionsRequest) (*apiv1.ListPageVersionsResponse, error) {
	if req.GetPageName() == "" {
		return nil, status.Error(codes.InvalidArgument, "page_name is required")
	}

	if s.historyReader == nil {
		return nil, status.Error(codes.Unavailable, "history reader not configured")
	}

	versions, err := s.historyReader.ListVersions(wikipage.PageIdentifier(req.GetPageName()))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list versions for %s: %v", req.GetPageName(), err)
	}

	limit := int(req.GetLimit())
	resp := &apiv1.ListPageVersionsResponse{}
	for i, v := range versions {
		if limit > 0 && i >= limit {
			break
		}
		resp.Versions = append(resp.Versions, convertVersionMetadata(v))
	}

	return resp, nil
}

// ReadPageVersion implements the PageHistoryService RPC.
func (s *Server) ReadPageVersion(_ context.Context, req *apiv1.ReadPageVersionRequest) (*apiv1.ReadPageVersionResponse, error) {
	if req.GetPageName() == "" {
		return nil, status.Error(codes.InvalidArgument, "page_name is required")
	}
	if req.GetVersionId() == "" {
		return nil, status.Error(codes.InvalidArgument, "version_id is required")
	}

	if s.historyReader == nil {
		return nil, status.Error(codes.Unavailable, "history reader not configured")
	}

	content, err := s.historyReader.ReadVersion(wikipage.PageIdentifier(req.GetPageName()), req.GetVersionId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to read version %s for %s: %v", req.GetVersionId(), req.GetPageName(), err)
	}

	return &apiv1.ReadPageVersionResponse{Content: content}, nil
}

// RestorePageVersion implements the PageHistoryService RPC.
func (s *Server) RestorePageVersion(ctx context.Context, req *apiv1.RestorePageVersionRequest) (*apiv1.RestorePageVersionResponse, error) {
	if req.GetPageName() == "" {
		return nil, status.Error(codes.InvalidArgument, "page_name is required")
	}
	if req.GetVersionId() == "" {
		return nil, status.Error(codes.InvalidArgument, "version_id is required")
	}

	if s.historyReader == nil {
		return nil, status.Error(codes.Unavailable, "history reader not configured")
	}

	identity := tailscale.IdentityFromContext(ctx)
	if err := s.historyReader.RestoreVersion(wikipage.PageIdentifier(req.GetPageName()), req.GetVersionId(), identity); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to restore version %s for %s: %v", req.GetVersionId(), req.GetPageName(), err)
	}

	return &apiv1.RestorePageVersionResponse{}, nil
}

// DiffPageVersions implements the PageHistoryService RPC.
func (s *Server) DiffPageVersions(_ context.Context, req *apiv1.DiffPageVersionsRequest) (*apiv1.DiffPageVersionsResponse, error) {
	if req.GetPageName() == "" {
		return nil, status.Error(codes.InvalidArgument, "page_name is required")
	}
	if req.GetOldVersionId() == "" {
		return nil, status.Error(codes.InvalidArgument, "old_version_id is required")
	}
	if req.GetNewVersionId() == "" {
		return nil, status.Error(codes.InvalidArgument, "new_version_id is required")
	}

	if s.historyReader == nil {
		return nil, status.Error(codes.Unavailable, "history reader not configured")
	}

	diff, err := s.historyReader.DiffVersions(wikipage.PageIdentifier(req.GetPageName()), req.GetOldVersionId(), req.GetNewVersionId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to diff versions for %s: %v", req.GetPageName(), err)
	}

	return &apiv1.DiffPageVersionsResponse{Diff: diff}, nil
}

// SearchPageHistory implements the PageHistoryService RPC.
func (s *Server) SearchPageHistory(_ context.Context, req *apiv1.SearchPageHistoryRequest) (*apiv1.SearchPageHistoryResponse, error) {
	if req.GetPageName() == "" {
		return nil, status.Error(codes.InvalidArgument, "page_name is required")
	}
	if req.GetQuery() == "" {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}

	if s.historySearcher == nil {
		return nil, status.Error(codes.Unavailable, "history search not configured")
	}

	results, err := s.historySearcher.SearchPageHistory(wikipage.PageIdentifier(req.GetPageName()), req.GetQuery())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to search page history for %s: %v", req.GetPageName(), err)
	}

	resp := &apiv1.SearchPageHistoryResponse{}
	for _, r := range results {
		resp.Results = append(resp.Results, convertHistorySearchResult(r))
	}

	return resp, nil
}

// SearchHistory implements the PageHistoryService RPC.
func (s *Server) SearchHistory(_ context.Context, req *apiv1.SearchHistoryRequest) (*apiv1.SearchHistoryResponse, error) {
	if req.GetQuery() == "" {
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}

	if s.historySearcher == nil {
		return nil, status.Error(codes.Unavailable, "history search not configured")
	}

	filter := HistorySearchFilter{
		Query:          req.GetQuery(),
		PageNameFilter: req.GetPageNameFilter(),
		AuthorFilter:   req.GetAuthorFilter(),
	}

	if req.From != nil {
		filter.From = req.From.AsTime()
	}
	if req.To != nil {
		filter.To = req.To.AsTime()
	}

	results, err := s.historySearcher.SearchHistory(filter)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to search history: %v", err)
	}

	resp := &apiv1.SearchHistoryResponse{}
	for _, r := range results {
		resp.Results = append(resp.Results, convertHistorySearchResult(r))
	}

	return resp, nil
}

// convertVersionMetadata converts the internal PageVersionMetadata to the proto type.
func convertVersionMetadata(v PageVersionMetadata) *apiv1.PageVersion {
	return &apiv1.PageVersion{
		VersionId: v.VersionID,
		CreatedAt: timestamppb.New(v.CreatedAt),
		Author:    v.Author,
		IsAgent:   v.IsAgent,
		Source:    v.Source,
		Sha256:    v.SHA256,
		ByteSize:  v.ByteSize,
	}
}

// convertHistorySearchResult converts the internal HistorySearchResult to the proto type.
func convertHistorySearchResult(r HistorySearchResult) *apiv1.HistorySearchResult {
	return &apiv1.HistorySearchResult{
		PageName: r.PageName,
		Version:  convertVersionMetadata(r.Version),
		Snippet:  r.Snippet,
	}
}

// WithHistoryReader wires the page history reader into the server.
func (s *Server) WithHistoryReader(hr PageHistoryReader) *Server {
	s.historyReader = hr
	return s
}

// WithHistorySearcher wires the page history searcher into the server.
func (s *Server) WithHistorySearcher(hs HistorySearcher) *Server {
	s.historySearcher = hs
	return s
}
