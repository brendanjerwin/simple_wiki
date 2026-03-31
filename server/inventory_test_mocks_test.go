//revive:disable:dot-imports
package server

import (
	"os"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// mockPageReaderMutator implements InventoryNormalizationDependencies for testing.
// This is a consolidated mock that can be used across inventory-related test files.
type mockPageReaderMutator struct {
	// pages stores page data by identifier
	pages map[string]*mockPage

	// Call tracking
	readFrontMatterCalls []string
	writeFrontMatterCalls []string
	writeMarkdownCalls   []string
	deletedPages         []string

	// Injectable errors (override per-page errors)
	readFrontMatterErr  error
	writeFrontMatterErr error
	writeMarkdownErr    error
	deletePageErr       error
}

// mockPage represents a page's data and optional errors.
type mockPage struct {
	frontmatter map[string]any
	markdown    string
	readErr     error // per-page error for read operations
}

func newMockPageReaderMutator() *mockPageReaderMutator {
	return &mockPageReaderMutator{
		pages: make(map[string]*mockPage),
	}
}

// setPage adds or updates a page in the mock.
func (m *mockPageReaderMutator) setPage(id string, fm map[string]any, md string) {
	m.pages[id] = &mockPage{
		frontmatter: fm,
		markdown:    md,
	}
}

// setPageWithError adds a page that returns an error when read.
func (m *mockPageReaderMutator) setPageWithError(id string, err error) {
	m.pages[id] = &mockPage{
		readErr: err,
	}
}

// getMarkdown returns the markdown for a page (for test assertions).
func (m *mockPageReaderMutator) getMarkdown(id string) string {
	if page, ok := m.pages[id]; ok {
		return page.markdown
	}
	return ""
}

func (m *mockPageReaderMutator) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	m.readFrontMatterCalls = append(m.readFrontMatterCalls, string(id))

	// Global error takes precedence
	if m.readFrontMatterErr != nil {
		return "", nil, m.readFrontMatterErr
	}

	page, ok := m.pages[string(id)]
	if !ok {
		return "", nil, os.ErrNotExist
	}

	if page.readErr != nil {
		return "", nil, page.readErr
	}

	return id, page.frontmatter, nil
}

func (m *mockPageReaderMutator) ReadMarkdown(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	page, ok := m.pages[string(id)]
	if !ok {
		return "", "", os.ErrNotExist
	}

	if page.readErr != nil {
		return "", "", page.readErr
	}

	return id, wikipage.Markdown(page.markdown), nil
}

func (m *mockPageReaderMutator) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	m.writeFrontMatterCalls = append(m.writeFrontMatterCalls, string(id))

	if m.writeFrontMatterErr != nil {
		return m.writeFrontMatterErr
	}

	if m.pages[string(id)] == nil {
		m.pages[string(id)] = &mockPage{}
	}
	m.pages[string(id)].frontmatter = fm
	return nil
}

func (m *mockPageReaderMutator) WriteMarkdown(id wikipage.PageIdentifier, md wikipage.Markdown) error {
	m.writeMarkdownCalls = append(m.writeMarkdownCalls, string(id))

	if m.writeMarkdownErr != nil {
		return m.writeMarkdownErr
	}

	if m.pages[string(id)] == nil {
		m.pages[string(id)] = &mockPage{}
	}
	m.pages[string(id)].markdown = string(md)
	return nil
}

func (m *mockPageReaderMutator) DeletePage(id wikipage.PageIdentifier) error {
	m.deletedPages = append(m.deletedPages, string(id))

	if m.deletePageErr != nil {
		return m.deletePageErr
	}

	delete(m.pages, string(id))
	return nil
}

func (m *mockPageReaderMutator) ModifyMarkdown(id wikipage.PageIdentifier, modifier func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	var currentMD wikipage.Markdown
	if page, ok := m.pages[string(id)]; ok {
		currentMD = wikipage.Markdown(page.markdown)
	}

	newMD, err := modifier(currentMD)
	if err != nil {
		return err
	}

	if m.writeMarkdownErr != nil {
		return m.writeMarkdownErr
	}

	if m.pages[string(id)] == nil {
		m.pages[string(id)] = &mockPage{}
	}
	m.pages[string(id)].markdown = string(newMD)
	return nil
}

// getFrontmatter returns the frontmatter for a page (for test assertions).
func (m *mockPageReaderMutator) getFrontmatter(id string) map[string]any {
	if page, ok := m.pages[id]; ok {
		return page.frontmatter
	}
	return nil
}


// hasPage returns true if a page exists in the mock.
func (m *mockPageReaderMutator) hasPage(id string) bool {
	_, ok := m.pages[id]
	return ok
}
