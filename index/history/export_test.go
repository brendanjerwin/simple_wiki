package history

import (
	"github.com/blevesearch/bleve"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// QueryRawForTest runs an arbitrary bleve query string against the underlying
// index and returns the matching document IDs. Used by tests that need to
// inspect specific fields (e.g. `version_id:<id>`) without going through the
// public search methods.
func (i *Index) QueryRawForTest(queryStr string) ([]wikipage.PageIdentifier, error) {
	q := bleve.NewQueryStringQuery(queryStr)
	req := bleve.NewSearchRequest(q)
	res, err := i.index.Search(req)
	if err != nil {
		return nil, err
	}
	ids := make([]wikipage.PageIdentifier, 0, len(res.Hits))
	for _, hit := range res.Hits {
		ids = append(ids, wikipage.PageIdentifier(hit.ID))
	}
	return ids, nil
}
