package eager

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

const testFileTimestamp = 1609459200 // 2021-01-01 Unix timestamp

// MockMigrationDeps provides a simple mock implementation for testing migrations
type MockMigrationDeps struct {
	dataDir              string
	pages                map[string]*wikipage.Page
	mu                   sync.RWMutex
	readPageErr          error // injectable error for ReadPage
	deletePageErr        error // injectable error for DeletePage
	writeFrontMatterErr  error // injectable error for WriteFrontMatter
	writeMarkdownErr     error // injectable error for WriteMarkdown
}

func NewMockMigrationDeps(dataDir string) *MockMigrationDeps {
	return &MockMigrationDeps{
		dataDir: dataDir,
		pages:   make(map[string]*wikipage.Page),
	}
}

// SetReadPageError sets an error to return from ReadPage
func (m *MockMigrationDeps) SetReadPageError(err error) {
	m.readPageErr = err
}

// SetDeletePageError sets an error to return from DeletePage
func (m *MockMigrationDeps) SetDeletePageError(err error) {
	m.deletePageErr = err
}

// SetWriteFrontMatterError sets an error to return from WriteFrontMatter
func (m *MockMigrationDeps) SetWriteFrontMatterError(err error) {
	m.writeFrontMatterErr = err
}

// SetWriteMarkdownError sets an error to return from WriteMarkdown
func (m *MockMigrationDeps) SetWriteMarkdownError(err error) {
	m.writeMarkdownErr = err
}

func (m *MockMigrationDeps) ReadPage(identifier wikipage.PageIdentifier) (*wikipage.Page, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.readPageErr != nil {
		return nil, m.readPageErr
	}

	if page, exists := m.pages[string(identifier)]; exists {
		return page, nil
	}

	// Return empty page for non-existing pages
	return &wikipage.Page{
		Identifier:        string(identifier),
		Text:              "",
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
	if m.writeFrontMatterErr != nil {
		return m.writeFrontMatterErr
	}
	// Simple implementation for testing
	return m.UpdatePageContent(identifier, "# Mock content")
}

func (m *MockMigrationDeps) WriteMarkdown(identifier wikipage.PageIdentifier, md wikipage.Markdown) error {
	if m.writeMarkdownErr != nil {
		return m.writeMarkdownErr
	}
	return m.UpdatePageContent(identifier, string(md))
}

func (m *MockMigrationDeps) UpdatePageContent(identifier wikipage.PageIdentifier, newText string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	page := &wikipage.Page{
		Identifier:        string(identifier),
		Text:              newText,
		WasLoadedFromDisk: true,
	}
	
	m.pages[string(identifier)] = page
	return nil
}

func (m *MockMigrationDeps) DeletePage(identifier wikipage.PageIdentifier) error {
	if m.deletePageErr != nil {
		return m.deletePageErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove from memory
	delete(m.pages, string(identifier))

	// Try to remove MD file from disk if it exists
	base32Name := base32tools.EncodeToBase32(string(identifier))
	mdPath := filepath.Join(m.dataDir, base32Name+".md")
	_ = os.Remove(mdPath) // Ignore errors

	return nil
}

// CreatePascalCasePage creates PascalCase pages directly on filesystem for testing
// It creates an MD file with TOML frontmatter containing the identifier
func CreatePascalCasePage(dir, identifier, content string) {
	// Create MD file with TOML frontmatter
	mdPath := filepath.Join(dir, base32tools.EncodeToBase32(strings.ToLower(identifier))+".md")

	// Build page with frontmatter containing the identifier
	fullContent := "+++\nidentifier = '" + identifier + "'\n+++\n\n" + content
	_ = os.WriteFile(mdPath, []byte(fullContent), 0644)
}

// CreateTestFile creates test files with consistent timestamps for migration testing
func CreateTestFile(dir, filename, content string) {
	filePath := filepath.Join(dir, filename)
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		panic(err)
	}
	// Set a consistent timestamp for testing
	timestamp := time.Unix(testFileTimestamp, 0)
	if err := os.Chtimes(filePath, timestamp, timestamp); err != nil {
		panic(err)
	}
}

// CreateMDFileWithoutFrontmatter creates an MD file without any frontmatter
// The identifier should be derived from the filename
func CreateMDFileWithoutFrontmatter(dir, identifier, content string) {
	mdPath := filepath.Join(dir, base32tools.EncodeToBase32(strings.ToLower(identifier))+".md")
	// Write content without frontmatter
	if err := os.WriteFile(mdPath, []byte(content), 0644); err != nil {
		panic(err)
	}
}

// CreateMDFileWithFrontmatterNoIdentifier creates an MD file with TOML frontmatter but no identifier field
func CreateMDFileWithFrontmatterNoIdentifier(dir, identifier, frontmatter, content string) {
	mdPath := filepath.Join(dir, base32tools.EncodeToBase32(strings.ToLower(identifier))+".md")
	// Build page with frontmatter but no identifier field
	fullContent := "+++\n" + frontmatter + "\n+++\n\n" + content
	if err := os.WriteFile(mdPath, []byte(fullContent), 0644); err != nil {
		panic(err)
	}
}

// CreateMDFileWithInvalidIdentifier creates an MD file with a specific identifier that may be invalid
func CreateMDFileWithInvalidIdentifier(dir, filename, identifier string) {
	mdPath := filepath.Join(dir, base32tools.EncodeToBase32(strings.ToLower(filename))+".md")
	// Build page with frontmatter containing the invalid identifier
	fullContent := "+++\nidentifier = '" + identifier + "'\n+++\n\n# Content"
	if err := os.WriteFile(mdPath, []byte(fullContent), 0644); err != nil {
		panic(err)
	}
}

// CreateMDFileWithMalformedFrontmatter creates an MD file with malformed TOML frontmatter
// (has opening +++ but not properly closed)
func CreateMDFileWithMalformedFrontmatter(dir, filename string) {
	mdPath := filepath.Join(dir, base32tools.EncodeToBase32(strings.ToLower(filename))+".md")
	// Malformed: has +++ but not properly closed (only 2 parts when split)
	fullContent := "+++\nidentifier = 'test'\n# Content without closing +++"
	if err := os.WriteFile(mdPath, []byte(fullContent), 0644); err != nil {
		panic(err)
	}
}

// CreateMDFileWithUnparseableTOML creates an MD file with invalid TOML syntax
func CreateMDFileWithUnparseableTOML(dir, filename string) {
	mdPath := filepath.Join(dir, base32tools.EncodeToBase32(strings.ToLower(filename))+".md")
	// Invalid TOML: unclosed string
	fullContent := "+++\nidentifier = 'unclosed\n+++\n\n# Content"
	if err := os.WriteFile(mdPath, []byte(fullContent), 0644); err != nil {
		panic(err)
	}
}

// MockDataDirScanner implements DataDirScanner for testing
type MockDataDirScanner struct {
	files     map[string][]byte // filename -> content
	dirExists bool
	readError error // optional: inject read errors
	listError error // optional: inject list errors
}

// NewMockDataDirScanner creates a new MockDataDirScanner with an empty filesystem
func NewMockDataDirScanner() *MockDataDirScanner {
	return &MockDataDirScanner{
		files:     make(map[string][]byte),
		dirExists: true,
	}
}

// DataDirExists returns whether the mock directory exists
func (m *MockDataDirScanner) DataDirExists() bool {
	return m.dirExists
}

// ListMDFiles returns all .md files in the mock filesystem
func (m *MockDataDirScanner) ListMDFiles() ([]string, error) {
	if m.listError != nil {
		return nil, m.listError
	}
	var files []string
	for name := range m.files {
		if strings.HasSuffix(name, ".md") {
			files = append(files, name)
		}
	}
	return files, nil
}

// ReadMDFile reads a file from the mock filesystem
func (m *MockDataDirScanner) ReadMDFile(filename string) ([]byte, error) {
	if m.readError != nil {
		return nil, m.readError
	}
	content, exists := m.files[filename]
	if !exists {
		return nil, os.ErrNotExist
	}
	return content, nil
}

// MDFileExistsByBase32Name checks if an MD file exists by base32 name
func (m *MockDataDirScanner) MDFileExistsByBase32Name(base32Name string) bool {
	_, exists := m.files[base32Name+".md"]
	return exists
}

// AddFile adds a file to the mock filesystem
func (m *MockDataDirScanner) AddFile(filename string, content []byte) {
	m.files[filename] = content
}

// AddPascalCasePage adds a PascalCase page with TOML frontmatter to the mock
func (m *MockDataDirScanner) AddPascalCasePage(identifier, markdownContent string) {
	base32Name := base32tools.EncodeToBase32(strings.ToLower(identifier))
	fullContent := "+++\nidentifier = '" + identifier + "'\n+++\n\n" + markdownContent
	m.files[base32Name+".md"] = []byte(fullContent)
}

// SetDirExists sets whether the mock directory exists
func (m *MockDataDirScanner) SetDirExists(exists bool) {
	m.dirExists = exists
}

// SetReadError sets an error to return from ReadMDFile
func (m *MockDataDirScanner) SetReadError(err error) {
	m.readError = err
}

// SetListError sets an error to return from ListMDFiles
func (m *MockDataDirScanner) SetListError(err error) {
	m.listError = err
}