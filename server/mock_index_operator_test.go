package server

import (
	"sync"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// MockIndexOperator is a test implementation of index.IndexOperator.
type MockIndexOperator struct {
	mu                            sync.Mutex
	AddPageToIndexFunc            func(identifier wikipage.PageIdentifier) error
	RemovePageFromIndexFunc       func(identifier wikipage.PageIdentifier) error
	AddPageToIndexCalledWith      []wikipage.PageIdentifier
	RemovePageFromIndexCalledWith []wikipage.PageIdentifier
}

func (m *MockIndexOperator) AddPageToIndex(identifier wikipage.PageIdentifier) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.AddPageToIndexCalledWith = append(m.AddPageToIndexCalledWith, identifier)
	if m.AddPageToIndexFunc != nil {
		return m.AddPageToIndexFunc(identifier)
	}
	return nil
}

func (m *MockIndexOperator) RemovePageFromIndex(identifier wikipage.PageIdentifier) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.RemovePageFromIndexCalledWith = append(m.RemovePageFromIndexCalledWith, identifier)
	if m.RemovePageFromIndexFunc != nil {
		return m.RemovePageFromIndexFunc(identifier)
	}
	return nil
}

// LastAddPageCall returns the last identifier passed to AddPageToIndex
func (m *MockIndexOperator) LastAddPageCall() wikipage.PageIdentifier {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if len(m.AddPageToIndexCalledWith) == 0 {
		return ""
	}
	return m.AddPageToIndexCalledWith[len(m.AddPageToIndexCalledWith)-1]
}

// LastRemovePageCall returns the last identifier passed to RemovePageFromIndex
func (m *MockIndexOperator) LastRemovePageCall() wikipage.PageIdentifier {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if len(m.RemovePageFromIndexCalledWith) == 0 {
		return ""
	}
	return m.RemovePageFromIndexCalledWith[len(m.RemovePageFromIndexCalledWith)-1]
}