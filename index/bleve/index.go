// Package bleve provides a Bleve search index implementation.
package bleve

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/blevesearch/bleve"
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

var (
	newlineRegex = regexp.MustCompile("\n")
	highlightRegex = regexp.MustCompile(`<mark>([^<]*)</mark>`)
)

// extractFragmentAndHighlights converts a Bleve HTML fragment with <mark> tags
// to plain text with highlight position spans.
func extractFragmentAndHighlights(fragmentHTML string) (string, []HighlightSpan) {
	// Replace newlines with spaces for consistent fragment display
	cleanHTML := newlineRegex.ReplaceAllString(fragmentHTML, " ")
	
	var highlights []HighlightSpan
	var plainText strings.Builder
	var currentPos int
	
	// Find all <mark> tags and their positions
	matches := highlightRegex.FindAllStringSubmatchIndex(cleanHTML, -1)
	
	for _, match := range matches {
		// match[0], match[1] = start and end of entire match including <mark> tags
		// match[2], match[3] = start and end of captured group (text inside <mark>)
		
		// Add text before the <mark> tag
		if currentPos < match[0] {
			plainText.WriteString(cleanHTML[currentPos:match[0]])
		}
		
		// Record where the highlighted text starts in the plain text
		highlightStart := int32(plainText.Len())
		
		// Add the highlighted text (without <mark> tags)
		highlightedText := cleanHTML[match[2]:match[3]]
		plainText.WriteString(highlightedText)
		
		// Record where the highlighted text ends
		highlightEnd := int32(plainText.Len())
		
		// Add the highlight span
		highlights = append(highlights, HighlightSpan{
			Start: highlightStart,
			End:   highlightEnd,
		})
		
		// Move past the closing </mark> tag
		currentPos = match[1]
	}
	
	// Add any remaining text after the last highlight
	if currentPos < len(cleanHTML) {
		plainText.WriteString(cleanHTML[currentPos:])
	}
	
	return plainText.String(), highlights
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

	search := bleve.NewSearchRequest(q)
	search.Highlight = bleve.NewHighlight()
	bleveResults, err := b.index.Search(search)
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

		// Get the fragment text
		if hit.Fragments != nil && hit.Fragments["content"] != nil && len(hit.Fragments["content"]) > 0 {
			// Use the fragment text from Bleve (which contains <mark> tags for highlights)
			fragmentHTML := hit.Fragments["content"][0]
			// Extract plain text and highlight positions from the HTML fragment
			result.Fragment, result.Highlights = extractFragmentAndHighlights(fragmentHTML)
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
