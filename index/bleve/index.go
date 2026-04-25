// Package bleve provides a Bleve search index implementation.
package bleve

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/blevesearch/bleve"
	// Register the keyword analyzer (used for the tags field below).
	_ "github.com/blevesearch/bleve/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/search"
	"github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/internal/hashtags"
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

// BleveIndexQueryer defines the interface for querying the Bleve index.
type BleveIndexQueryer interface {
	Query(query string) ([]SearchResult, error)
}

// tagsField is the document field used to store extracted page-level
// hashtags. The keyword analyzer matches whole tags (so `#home-lab` is a
// single token, not split into `home`+`lab`).
const tagsField = "tags"

// NewIndex creates a new BleveIndex.
func NewIndex(pageReader wikipage.PageReader, frontmatterQueryer frontmatter.IQueryFrontmatterIndex) (*Index, error) {
	mapping := bleve.NewIndexMapping()
	mapping.DefaultAnalyzer = "en"

	// Treat the `tags` field as keyword (whole-string match) rather than
	// analyzing it through the default English analyzer.
	tagsFieldMapping := bleve.NewTextFieldMapping()
	tagsFieldMapping.Analyzer = "keyword"
	mapping.DefaultMapping.AddFieldMappingsAt(tagsField, tagsFieldMapping)

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
	mungedIdentifier, err := wikiidentifiers.MungeIdentifier(string(requestedIdentifier))
	if err != nil {
		return fmt.Errorf("invalid identifier %q: %w", requestedIdentifier, err)
	}
	identifier, markdown, err := b.pageReader.ReadMarkdown(requestedIdentifier)
	if err != nil {
		return fmt.Errorf("bleve indexer failed to read markdown for page %q: %w", requestedIdentifier, err)
	}

	_, pageFrontmatter, err := b.pageReader.ReadFrontMatter(identifier)
	if err != nil {
		return fmt.Errorf("bleve indexer failed to read frontmatter for page %q: %w", requestedIdentifier, err)
	}

	renderedBytes, err := templating.ExecuteTemplate(string(markdown), pageFrontmatter, b.pageReader, b.frontmatterQueryer)
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
	pageFrontmatter[tagsField] = hashtags.Extract(string(markdown))

	b.mu.Lock()
	defer b.mu.Unlock()

	_ = b.index.Delete(string(identifier))
	_ = b.index.Delete(string(requestedIdentifier))
	_ = b.index.Delete(mungedIdentifier)

	err = b.index.Index(string(identifier), pageFrontmatter)
	if err != nil {
		return fmt.Errorf("bleve indexer failed to index page %q: %w", requestedIdentifier, err)
	}

	return nil
}

// RemovePageFromIndex removes a page from the Bleve index.
func (b *Index) RemovePageFromIndex(identifier wikipage.PageIdentifier) error {
	// Munge identifier for consistent lookup; if munging fails, use original
	mungedIdentifier := identifier
	if munged, err := wikiidentifiers.MungeIdentifier(string(identifier)); err == nil {
		mungedIdentifier = wikipage.PageIdentifier(munged)
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	// Try to delete all possible variations of the identifier to ensure complete removal.
	// Unlike AddPageToIndex where deletion is background cleanup, RemovePageFromIndex's
	// primary purpose is deletion, so we return any errors encountered.
	err1 := b.index.Delete(string(identifier))
	err2 := b.index.Delete(string(mungedIdentifier))

	return errors.Join(err1, err2)
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

	// Defensive sanitization: remove any residual invalid UTF-8 bytes from source content.
	if !utf8.ValidString(fragment) {
		fragment = strings.ToValidUTF8(fragment, "")
	}
	
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

	// Defensive check: ensure start <= end (can happen with stale index data)
	if fragmentStart > fragmentEnd {
		fragmentStart = fragmentEnd
	}

	// Align to rune boundaries to prevent splitting multi-byte UTF-8 characters.
	// In valid UTF-8, a rune uses at most 4 bytes, so these loops advance/retreat
	// at most 3 steps (past continuation bytes). For invalid UTF-8 the loops are
	// still bounded by the [fragmentStart, fragmentEnd) range; any remaining invalid
	// bytes are removed by the ToValidUTF8 call in extractFragmentFromLocations.
	// Move fragmentStart forward past any continuation bytes.
	for fragmentStart < fragmentEnd && !utf8.RuneStart(contentText[fragmentStart]) {
		fragmentStart++
	}
	// Move fragmentEnd backward past any continuation bytes (only when not at end of string).
	for fragmentEnd > fragmentStart && fragmentEnd < len(contentText) && !utf8.RuneStart(contentText[fragmentEnd]) {
		fragmentEnd--
	}

	return fragmentStart, fragmentEnd
}


// Query searches the Bleve index.
func (b *Index) Query(query string) ([]SearchResult, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Exact match on title (boosted highest)
	titleQuery := bleve.NewMatchQuery(query)
	titleQuery.SetField("title")
	titleQuery.SetBoost(2.0)

	// Prefix match on title (for partial matches like "Container" → "ContainerFoo")
	titlePrefixQuery := bleve.NewPrefixQuery(strings.ToLower(query))
	titlePrefixQuery.SetField("title")
	titlePrefixQuery.SetBoost(1.5)

	// Match query on content (safer than QueryStringQuery which interprets special chars like /)
	contentQuery := bleve.NewMatchQuery(query)
	contentQuery.SetField(contentField)

	q := bleve.NewDisjunctionQuery(titleQuery, titlePrefixQuery, contentQuery)

	searchReq := bleve.NewSearchRequest(q)
	searchReq.Highlight = bleve.NewHighlight()
	searchReq.IncludeLocations = true
	searchReq.Fields = []string{contentField}  // Include content field to get original text
	bleveResults, err := b.index.Search(searchReq)
	if err != nil {
		return nil, fmt.Errorf("bleve search for query %q: %w", query, err)
	}

	var results []SearchResult
	for _, hit := range bleveResults.Hits {
		result := SearchResult{
			Identifier: wikipage.PageIdentifier(hit.ID),
			Title:      b.frontmatterQueryer.GetValue(wikipage.PageIdentifier(hit.ID), "title"),
		}

		if result.Title == "" {
			result.Title = string(result.Identifier)
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
