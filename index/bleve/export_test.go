package bleve

import (
	"github.com/blevesearch/bleve"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// Export private methods for testing
// This pattern is acceptable because test-only exports in _test.go files are a common Go practice

var (
	// CalculateFragmentWindowForTest provides test access to the private calculateFragmentWindow method
	CalculateFragmentWindowForTest = (*Index).calculateFragmentWindow
	// ExtractFragmentFromLocationsForTest provides test access to the private extractFragmentFromLocations method
	ExtractFragmentFromLocationsForTest = (*Index).extractFragmentFromLocations
)

// QueryRawForTest runs an arbitrary bleve query string against the underlying
// index and returns the matching page identifiers. Used by tests that need
// to inspect specific fields (e.g. `tags:foo`) without going through the
// public Query method's title/content scoring.
func (b *Index) QueryRawForTest(queryStr string) ([]wikipage.PageIdentifier, error) {
	q := bleve.NewQueryStringQuery(queryStr)
	req := bleve.NewSearchRequest(q)
	res, err := b.index.Search(req)
	if err != nil {
		return nil, err
	}
	ids := make([]wikipage.PageIdentifier, 0, len(res.Hits))
	for _, hit := range res.Hits {
		ids = append(ids, wikipage.PageIdentifier(hit.ID))
	}
	return ids, nil
}
