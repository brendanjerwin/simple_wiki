package index

import (
	"github.com/blevesearch/bleve"
	"github.com/brendanjerwin/simple_wiki/common"
)

type BleveIndex struct {
	index      bleve.Index
	pageReader common.IReadPages
}

type IQueryBleveIndex interface {
	Query(query string) ([]SearchResult, error)
}

func NewBleveIndex(pageReader common.IReadPages) (*BleveIndex, error) {
	mapping := bleve.NewIndexMapping()
	index, err := bleve.NewMemOnly(mapping)
	if err != nil {
		return nil, err
	}

	return &BleveIndex{
		index:      index,
		pageReader: pageReader,
	}, nil
}

func (b *BleveIndex) AddPageToIndex(requested_identifier common.PageIdentifier) error {
	munged_identifier := common.MungeIdentifier(requested_identifier)
	identifier, markdown, err := b.pageReader.ReadMarkdown(requested_identifier)
	if err != nil {
		return err
	}

	_, page, err := b.pageReader.ReadFrontMatter(identifier)
	if err != nil {
		return err
	}
	page["content"] = markdown

	b.index.Delete(identifier)
	b.index.Delete(requested_identifier)
	b.index.Delete(munged_identifier)

	return b.index.Index(identifier, page)
}

func (b *BleveIndex) RemovePageFromIndex(identifier common.PageIdentifier) error {
	identifier = common.MungeIdentifier(identifier)
	return b.index.Delete(identifier)
}

func (b *BleveIndex) Query(query string) ([]SearchResult, error) {
	q := bleve.NewQueryStringQuery(query)
	search := bleve.NewSearchRequest(q)
	bleveResults, err := b.index.Search(search)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	for _, hit := range bleveResults.Hits {
		result := SearchResult{
			Identifier: hit.ID,
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
	Fragment   string
}
