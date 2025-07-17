package server

import (
	"github.com/brendanjerwin/simple_wiki/index"
	"github.com/brendanjerwin/simple_wiki/index/bleve"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// MockIndexMaintainer is a mock implementation of index.IMaintainIndex for testing.
type MockIndexMaintainer struct {
	AddPageToIndexCalledWith      wikipage.PageIdentifier
	RemovePageFromIndexCalledWith wikipage.PageIdentifier
}

func (m *MockIndexMaintainer) AddPageToIndex(identifier wikipage.PageIdentifier) error {
	m.AddPageToIndexCalledWith = identifier
	return nil
}

func (m *MockIndexMaintainer) RemovePageFromIndex(identifier wikipage.PageIdentifier) error {
	m.RemovePageFromIndexCalledWith = identifier
	return nil
}

func (m *MockIndexMaintainer) Name() string {
	return "mock"
}

var _ index.IMaintainIndex = (*MockIndexMaintainer)(nil)

// mockFrontmatterIndexQueryer is a mock implementation of the FrontmatterIndexQueryer for testing.
type mockFrontmatterIndexQueryer struct {
	data                  map[string]map[string]string
	QueryExactMatchFunc   func(string, string) []string
	QueryPrefixMatchFunc  func(string, string) []string
	QueryKeyExistenceFunc func(string) []string
}

func (m *mockFrontmatterIndexQueryer) GetValue(id, key string) string {
	if page, ok := m.data[id]; ok {
		return page[key]
	}
	return ""
}

func (m *mockFrontmatterIndexQueryer) QueryExactMatch(key, value string) []string {
	if m.QueryExactMatchFunc != nil {
		return m.QueryExactMatchFunc(key, value)
	}
	return nil
}

func (m *mockFrontmatterIndexQueryer) QueryPrefixMatch(key, prefix string) []string {
	if m.QueryPrefixMatchFunc != nil {
		return m.QueryPrefixMatchFunc(key, prefix)
	}
	return nil
}

func (m *mockFrontmatterIndexQueryer) QueryKeyExistence(key string) []string {
	if m.QueryKeyExistenceFunc != nil {
		return m.QueryKeyExistenceFunc(key)
	}
	return nil
}

type mockBleveIndexQueryer struct {
	QueryFunc func(query string) ([]bleve.SearchResult, error)
}

func (m *mockBleveIndexQueryer) Query(query string) ([]bleve.SearchResult, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(query)
	}
	return nil, nil
}
