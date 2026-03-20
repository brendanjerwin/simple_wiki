package server

import "github.com/brendanjerwin/simple_wiki/wikipage"

// mockFrontmatterIndexQueryer is a mock implementation of the FrontmatterIndexQueryer for testing.
type mockFrontmatterIndexQueryer struct {
	data                  map[string]map[string]string
	QueryExactMatchFunc   func(string, string) []wikipage.PageIdentifier
	QueryPrefixMatchFunc  func(string, string) []wikipage.PageIdentifier
	QueryKeyExistenceFunc func(string) []wikipage.PageIdentifier
}

func (m *mockFrontmatterIndexQueryer) GetValue(id wikipage.PageIdentifier, key string) string {
	if page, ok := m.data[string(id)]; ok {
		return page[key]
	}
	return ""
}

func (m *mockFrontmatterIndexQueryer) QueryExactMatch(key, value string) []wikipage.PageIdentifier {
	if m.QueryExactMatchFunc != nil {
		return m.QueryExactMatchFunc(key, value)
	}
	return nil
}

func (m *mockFrontmatterIndexQueryer) QueryPrefixMatch(key, prefix string) []wikipage.PageIdentifier {
	if m.QueryPrefixMatchFunc != nil {
		return m.QueryPrefixMatchFunc(key, prefix)
	}
	return nil
}

func (m *mockFrontmatterIndexQueryer) QueryKeyExistence(key string) []wikipage.PageIdentifier {
	if m.QueryKeyExistenceFunc != nil {
		return m.QueryKeyExistenceFunc(key)
	}
	return nil
}

func (*mockFrontmatterIndexQueryer) QueryExactMatchSortedBy(_, _, _ string, _ bool, _ int) []wikipage.PageIdentifier {
	return nil
}
