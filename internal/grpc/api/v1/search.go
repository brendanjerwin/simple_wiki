package v1

import (
	"context"
	"fmt"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/index/bleve"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
