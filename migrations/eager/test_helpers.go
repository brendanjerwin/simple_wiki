package eager

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/schollz/versionedtext"
)

// MockMigrationDeps provides a simple mock implementation for testing
type MockMigrationDeps struct {
	dataDir string
	pages   map[string]*wikipage.Page
	mu      sync.RWMutex
}

func NewMockMigrationDeps(dataDir string) *MockMigrationDeps {
	return &MockMigrationDeps{
		dataDir: dataDir,
		pages:   make(map[string]*wikipage.Page),
	}
}

func (m *MockMigrationDeps) ReadPage(identifier wikipage.PageIdentifier) (*wikipage.Page, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if page, exists := m.pages[string(identifier)]; exists {
		return page, nil
	}
	
	// Return empty page for non-existing pages
	return &wikipage.Page{
		Identifier:        string(identifier),
		Text:              versionedtext.NewVersionedText(""),
		WasLoadedFromDisk: false,
	}, nil
}

func (m *MockMigrationDeps) ReadFrontMatter(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	page, err := m.ReadPage(identifier)
	if err != nil {
		return identifier, nil, err
	}
	
	if page.IsNew() {
		return identifier, nil, os.ErrNotExist
	}
	
	fm, err := page.GetFrontMatter()
	return identifier, fm, err
}

func (m *MockMigrationDeps) ReadMarkdown(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	page, err := m.ReadPage(identifier)
	if err != nil {
		return identifier, "", err
	}
	
	if page.IsNew() {
		return identifier, "", os.ErrNotExist
	}
	
	md, err := page.GetMarkdown()
	return identifier, md, err
}

func (m *MockMigrationDeps) WriteFrontMatter(identifier wikipage.PageIdentifier, _ wikipage.FrontMatter) error {
	// Simple implementation for testing
	return m.UpdatePageContent(identifier, "# Mock content")
}

func (m *MockMigrationDeps) WriteMarkdown(identifier wikipage.PageIdentifier, md wikipage.Markdown) error {
	return m.UpdatePageContent(identifier, string(md))
}

func (m *MockMigrationDeps) UpdatePageContent(identifier wikipage.PageIdentifier, newText string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	page := &wikipage.Page{
		Identifier:        string(identifier),
		Text:              versionedtext.NewVersionedText(newText),
		WasLoadedFromDisk: true,
	}
	
	m.pages[string(identifier)] = page
	return nil
}

func (m *MockMigrationDeps) DeletePage(identifier wikipage.PageIdentifier) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Remove from memory
	delete(m.pages, string(identifier))
	
	// Try to remove files from disk if they exist
	base32Name := base32tools.EncodeToBase32(string(identifier))
	jsonPath := filepath.Join(m.dataDir, base32Name+".json")
	mdPath := filepath.Join(m.dataDir, base32Name+".md")
	
	_ = os.Remove(jsonPath) // Ignore errors
	_ = os.Remove(mdPath)   // Ignore errors
	
	return nil
}