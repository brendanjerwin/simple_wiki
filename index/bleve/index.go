// Package bleve provides a Bleve search index implementation.
package bleve

import (
	"regexp"
	"strings"

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
		return err
	}

	_, pageFrontmatter, err := b.pageReader.ReadFrontMatter(identifier)
	if err != nil {
		return err
	}
	renderedBytes, err := templating.ExecuteTemplate(markdown, pageFrontmatter, b.pageReader, b.frontmatterQueryer)
	if err != nil {
		return err
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

	_ = b.index.Delete(identifier)
	_ = b.index.Delete(requestedIdentifier)
	_ = b.index.Delete(mungedIdentifier)

	return b.index.Index(identifier, pageFrontmatter)
}

// RemovePageFromIndex removes a page from the Bleve index.
func (b *Index) RemovePageFromIndex(identifier wikipage.PageIdentifier) error {
	identifier = wikiidentifiers.MungeIdentifier(identifier)
	return b.index.Delete(identifier)
}

var newlineRegex = regexp.MustCompile("\n")

// Query searches the Bleve index.
func (b *Index) Query(query string) ([]SearchResult, error) {
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

		if hit.Fragments != nil && hit.Fragments["content"] != nil {
			result.FragmentHTML = hit.Fragments["content"][0]
			result.FragmentHTML = newlineRegex.ReplaceAllString(result.FragmentHTML, "<br>")
		}
		results = append(results, result)
	}

	return results, nil
}

// SearchResult represents a search result from the Bleve index.
type SearchResult struct {
	Identifier   wikipage.PageIdentifier
	Title        string
	FragmentHTML string
}
