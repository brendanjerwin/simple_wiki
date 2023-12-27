package bleve

import (
	bleveActual "github.com/blevesearch/bleve"
	"github.com/brendanjerwin/simple_wiki/common"
	"github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/templating"
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
	frontmatter["content"] = string(renderedBytes)

	b.index.Delete(identifier)
	b.index.Delete(requested_identifier)
	b.index.Delete(munged_identifier)

	return b.index.Index(identifier, frontmatter)
}

func (b *BleveIndex) RemovePageFromIndex(identifier common.PageIdentifier) error {
	identifier = common.MungeIdentifier(identifier)
	return b.index.Delete(identifier)
}

func (b *BleveIndex) Query(query string) ([]SearchResult, error) {
	q := bleveActual.NewQueryStringQuery(query)
	search := bleveActual.NewSearchRequest(q)
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
			result.Fragment = hit.Fragments["content"][0]
		}
		results = append(results, result)
	}

	return results, nil
}

type SearchResult struct {
	Identifier common.PageIdentifier
	Title      string
	Fragment   string
}
