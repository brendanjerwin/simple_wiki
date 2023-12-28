package bleve

import (
	"regexp"
	"strings"

	bleveActual "github.com/blevesearch/bleve"
	"github.com/brendanjerwin/simple_wiki/common"
	"github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/templating"
	"github.com/brendanjerwin/simple_wiki/utils"
	"github.com/k3a/html2text"
)

type BleveIndex struct {
	index              bleveActual.Index
	pageReader         common.IReadPages
	frontmatterQueryer frontmatter.IQueryFrontmatterIndex
}

type IQueryBleveIndex interface {
	Query(query string) ([]SearchResult, error)
}

func NewBleveIndex(pageReader common.IReadPages, frontmatterQueryer frontmatter.IQueryFrontmatterIndex) (*BleveIndex, error) {
	mapping := bleveActual.NewIndexMapping()
	index, err := bleveActual.NewMemOnly(mapping)
	if err != nil {
		return nil, err
	}

	return &BleveIndex{
		index:              index,
		pageReader:         pageReader,
		frontmatterQueryer: frontmatterQueryer,
	}, nil
}

var linkRemoval = regexp.MustCompile(`<.*?>`)
var repeatedNewlineRegex = regexp.MustCompile(`\s*\n\s*\n\s*\n(\s*\n)*`)

func (b *BleveIndex) AddPageToIndex(requested_identifier common.PageIdentifier) error {
	munged_identifier := common.MungeIdentifier(requested_identifier)
	identifier, markdown, err := b.pageReader.ReadMarkdown(requested_identifier)
	if err != nil {
		return err
	}

	_, frontmatter, err := b.pageReader.ReadFrontMatter(identifier)
	if err != nil {
		return err
	}
	renderedBytes, err := templating.ExecuteTemplate(markdown, frontmatter, b.pageReader, b.frontmatterQueryer)
	if err != nil {
		return err
	}
	markdownRenderer := utils.GoldmarkRenderer{}
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

	frontmatter["content"] = content

	b.index.Delete(identifier)
	b.index.Delete(requested_identifier)
	b.index.Delete(munged_identifier)

	return b.index.Index(identifier, frontmatter)
}

func (b *BleveIndex) RemovePageFromIndex(identifier common.PageIdentifier) error {
	identifier = common.MungeIdentifier(identifier)
	return b.index.Delete(identifier)
}

var newlineRegex = regexp.MustCompile("\n")

func (b *BleveIndex) Query(query string) ([]SearchResult, error) {
	titleQuery := bleveActual.NewMatchQuery(query)
	titleQuery.SetField("title")
	titleQuery.SetBoost(2.0)

	overallQuery := bleveActual.NewQueryStringQuery(query)

	q := bleveActual.NewDisjunctionQuery(titleQuery, overallQuery)

	search := bleveActual.NewSearchRequest(q)
	search.Highlight = bleveActual.NewHighlight()
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

type SearchResult struct {
	Identifier   common.PageIdentifier
	Title        string
	FragmentHTML string
}
