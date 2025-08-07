// Package bleve provides a Bleve search index implementation.
package bleve

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/search"
	"github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/templating"
	"github.com/brendanjerwin/simple_wiki/utils/goldmarkrenderer"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/k3a/html2text"
)

// Index is a Bleve search index implementation.
type Index struct {
	index              bleve.Index
	pageReader         wikipage.PageReader
	frontmatterQueryer frontmatter.IQueryFrontmatterIndex
	mu                 sync.RWMutex // Protects concurrent access to bleve operations
}

// IQueryBleveIndex defines the interface for querying the Bleve index.
type IQueryBleveIndex interface {
	Query(query string) ([]SearchResult, error)
}

// NewIndex creates a new BleveIndex.
func NewIndex(pageReader wikipage.PageReader, frontmatterQueryer frontmatter.IQueryFrontmatterIndex) (*Index, error) {
	mapping := bleve.NewIndexMapping()
	mapping.DefaultAnalyzer = "en"
	index, err := bleve.NewMemOnly(mapping)
	if err != nil {
		return nil, err
	}

	return &Index{
		index:              index,
		pageReader:         pageReader,
		frontmatterQueryer: frontmatterQueryer,
	}, nil
}


var (
	linkRemoval          = regexp.MustCompile(`<.*?>`)
	repeatedNewlineRegex = regexp.MustCompile(`\s*\n\s*\n\s*\n(\s*\n)*`)
)

// AddPageToIndex adds a page to the Bleve index.
func (b *Index) AddPageToIndex(requestedIdentifier wikipage.PageIdentifier) error {
	mungedIdentifier := wikiidentifiers.MungeIdentifier(requestedIdentifier)
	identifier, markdown, err := b.pageReader.ReadMarkdown(requestedIdentifier)
	if err != nil {
		return fmt.Errorf("bleve indexer failed to read markdown for page %q: %w", requestedIdentifier, err)
	}

	_, pageFrontmatter, err := b.pageReader.ReadFrontMatter(identifier)
	if err != nil {
		return fmt.Errorf("bleve indexer failed to read frontmatter for page %q: %w", requestedIdentifier, err)
	}

	renderedBytes, err := templating.ExecuteTemplate(markdown, pageFrontmatter, b.pageReader, b.frontmatterQueryer)
	if err != nil {
		return fmt.Errorf("bleve indexer failed to execute template for page %q: %w", requestedIdentifier, err)
	}
	markdownRenderer := goldmarkrenderer.GoldmarkRenderer{}
	htmlBytes, err := markdownRenderer.Render(renderedBytes)
	var content string
	if err != nil {
		content = string(renderedBytes)
	} else {
		content = html2text.HTML2TextWithOptions(string(htmlBytes), html2text.WithLinksInnerText(), html2text.WithUnixLineBreaks())
		content = linkRemoval.ReplaceAllString(content, "")
		content = strings.TrimSpace(content)
		content = repeatedNewlineRegex.ReplaceAllString(content, "\n\n")
	}

	pageFrontmatter["content"] = content

	b.mu.Lock()
	defer b.mu.Unlock()

	_ = b.index.Delete(identifier)
	_ = b.index.Delete(requestedIdentifier)
	_ = b.index.Delete(mungedIdentifier)

	err = b.index.Index(identifier, pageFrontmatter)
	if err != nil {
		return fmt.Errorf("bleve indexer failed to index page %q: %w", requestedIdentifier, err)
	}

	return nil
}

// RemovePageFromIndex removes a page from the Bleve index.
func (b *Index) RemovePageFromIndex(identifier wikipage.PageIdentifier) error {
	identifier = wikiidentifiers.MungeIdentifier(identifier)
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.index.Delete(identifier)
}

const (
	// Fragment length for search result snippets
	maxFragmentLength = 200
	contextPadding    = 50
	contentField      = "content"
)

// extractFragmentFromLocations creates a text fragment with highlights using Bleve's structured location data
func (b *Index) extractFragmentFromLocations(contentText string, locations search.FieldTermLocationMap) (string, []HighlightSpan) {
	if locations == nil || locations[contentField] == nil {
		// No locations available, return empty fragment
		return "", nil
	}

	contentLocations := locations[contentField]
	
	// Collect all term locations and sort by position
	var allLocations []*search.Location
	for _, termLocations := range contentLocations {
		for _, location := range termLocations {
			allLocations = append(allLocations, location)
		}
	}
	
	if len(allLocations) == 0 {
		return "", nil
	}

	// Sort locations by byte start position
	sort.Slice(allLocations, func(i, j int) bool {
		return allLocations[i].Start < allLocations[j].Start
	})

	// Find the best fragment window around the matches
	fragmentStart, fragmentEnd := b.calculateFragmentWindow(contentText, allLocations)
	
	// Extract the fragment text
	fragment := contentText[fragmentStart:fragmentEnd]
	
	// Convert absolute byte positions to relative positions within the fragment
	var highlights []HighlightSpan
	for _, location := range allLocations {
		if location.Start >= uint64(fragmentStart) && location.End <= uint64(fragmentEnd) {
			highlights = append(highlights, HighlightSpan{
				Start: int32(location.Start) - int32(fragmentStart),
				End:   int32(location.End) - int32(fragmentStart),
			})
		}
	}
	
	return fragment, highlights
}

// calculateFragmentWindow determines the best window of text to show for search results
func (*Index) calculateFragmentWindow(contentText string, locations []*search.Location) (start int, end int) {
	if len(locations) == 0 {
		return 0, min(len(contentText), maxFragmentLength)
	}

	// Find the first and last match positions
	firstMatch := locations[0].Start
	lastMatch := locations[len(locations)-1].End

	// Try to center the fragment around all matches
	matchSpan := int(lastMatch - firstMatch)
	totalNeeded := matchSpan + 2*contextPadding

	var fragmentStart, fragmentEnd int

	if totalNeeded <= maxFragmentLength {
		// All matches fit with context, center them
		center := int(firstMatch + lastMatch) / 2
		fragmentStart = max(0, center-maxFragmentLength/2)
		fragmentEnd = min(len(contentText), fragmentStart+maxFragmentLength)
		
		// Adjust start if we hit the end
		if fragmentEnd-fragmentStart < maxFragmentLength {
			fragmentStart = max(0, fragmentEnd-maxFragmentLength)
		}
	} else {
		// Matches span too wide, focus on first match with some context
		fragmentStart = max(0, int(firstMatch)-contextPadding)
		fragmentEnd = min(len(contentText), fragmentStart+maxFragmentLength)
	}

	return fragmentStart, fragmentEnd
}


// Query searches the Bleve index.
func (b *Index) Query(query string) ([]SearchResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	titleQuery := bleve.NewMatchQuery(query)
	titleQuery.SetField("title")
	titleQuery.SetBoost(2.0)

	overallQuery := bleve.NewQueryStringQuery(query)

	q := bleve.NewDisjunctionQuery(titleQuery, overallQuery)

	searchReq := bleve.NewSearchRequest(q)
	searchReq.Highlight = bleve.NewHighlight()
	searchReq.IncludeLocations = true
	searchReq.Fields = []string{contentField}  // Include content field to get original text
	bleveResults, err := b.index.Search(searchReq)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	for _, hit := range bleveResults.Hits {
		result := SearchResult{
			Identifier: hit.ID,
			Title:      b.frontmatterQueryer.GetValue(hit.ID, "title"),
		}

		if result.Title == "" {
			result.Title = result.Identifier
		}

		// Get fragment and highlights from the structured location data
		if hit.Fields != nil && hit.Fields[contentField] != nil {
			if contentText, ok := hit.Fields[contentField].(string); ok {
				result.Fragment, result.Highlights = b.extractFragmentFromLocations(contentText, hit.Locations)
			}
		}

		results = append(results, result)
	}

	return results, nil
}

// HighlightSpan represents a text span that should be highlighted in search results.
type HighlightSpan struct {
	Start int32
	End   int32
}

// SearchResult represents a search result from the Bleve index.
type SearchResult struct {
	Identifier wikipage.PageIdentifier
	Title      string
	Fragment   string
	Highlights []HighlightSpan
}
