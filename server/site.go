package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/brendanjerwin/simple_wiki/index"
	"github.com/brendanjerwin/simple_wiki/index/bleve"
	"github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/rollingmigrations"
	"github.com/brendanjerwin/simple_wiki/sec"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/jcelliott/lumber"
	"github.com/pelletier/go-toml/v2"
	"github.com/schollz/versionedtext"
)

// IRenderMarkdownToHTML is an interface that abstracts the rendering process
type IRenderMarkdownToHTML interface {
	Render(input []byte) ([]byte, error)
}

// Site represents the wiki site.
type Site struct {
	PathToData              string
	CSS                     []byte
	DefaultPage             string
	DefaultPassword         string
	Debounce                int
	SessionStore            cookie.Store
	SecretCode              string
	Fileuploads             bool
	MaxUploadSize           uint
	MaxDocumentSize         uint // in runes; about a 10mb limit by default
	Logger                  *lumber.ConsoleLogger
	MarkdownRenderer        IRenderMarkdownToHTML
	IndexMaintainer         index.IMaintainIndex
	BackgroundIndexer       *index.BackgroundIndexingCoordinator
	FrontmatterIndexQueryer frontmatter.IQueryFrontmatterIndex
	BleveIndexQueryer       bleve.IQueryBleveIndex
	MigrationApplicator     rollingmigrations.FrontmatterMigrationApplicator
	saveMut                 sync.Mutex
}

const (
	tomlDelimiter = "+++\n"
	mdExtension   = "md"
)

func (s *Site) defaultLock() string {
	if s.DefaultPassword == "" {
		return ""
	}
	return sec.HashPassword(s.DefaultPassword)
}

func (s *Site) sniffContentType(name string) (string, error) {
	file, err := os.Open(path.Join(s.PathToData, name))
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	// The mimetype library reads up to 3072 bytes by default.
	buffer := make([]byte, 3072)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", err
	}

	// Use mimetype library to detect content type. It is more accurate and
	// can detect charsets.
	mtype := mimetype.Detect(buffer[:n])
	return mtype.String(), nil
}

// InitializeIndexing initializes the site's indexes.
func (s *Site) InitializeIndexing() error {
	frontmatterIndex := frontmatter.NewIndex(s)
	bleveIndex, err := bleve.NewIndex(s, frontmatterIndex)
	if err != nil {
		return err
	}
	multiMaintainer := index.NewMultiMaintainer(frontmatterIndex, bleveIndex)

	s.FrontmatterIndexQueryer = frontmatterIndex
	s.BleveIndexQueryer = bleveIndex
	s.IndexMaintainer = multiMaintainer

	// Create background indexing coordinator
	s.BackgroundIndexer = index.NewBackgroundIndexingCoordinatorWithCPUWorkers(multiMaintainer, s.Logger)

	// Get all files that need to be indexed
	files := s.DirectoryList()
	if len(files) == 0 {
		s.Logger.Info("No pages found to index.")
		return nil
	}

	// Convert files to page identifiers
	pageIdentifiers := make([]string, len(files))
	for i, file := range files {
		pageIdentifiers[i] = file.Name()
	}

	// Start background indexing
	err = s.BackgroundIndexer.StartBackground(pageIdentifiers)
	if err != nil {
		s.Logger.Error("Failed to start background indexing: %v", err)
		// Fall back to synchronous processing
		return s.initializeIndexingSynchronously(files)
	}

	s.Logger.Info("Background indexing started for %d pages. Application is ready.", len(files))
	return nil
}

// initializeIndexingSynchronously provides fallback synchronous indexing
func (s *Site) initializeIndexingSynchronously(files []os.FileInfo) error {
	s.Logger.Info("Starting synchronous indexing fallback...")
	
	for _, file := range files {
		if err := s.IndexMaintainer.AddPageToIndex(file.Name()); err != nil {
			// Check for application setup errors that should prevent startup
			var configErr *ConfigurationError
			if errors.As(err, &configErr) {
				s.Logger.Error("Application configuration error during initialization: %v", err)
				return fmt.Errorf("failed to initialize due to configuration error: %w", err)
			}
			// Log individual page errors but continue with other pages
			s.Logger.Error("Failed to add page '%s' to index during initialization: %v", file.Name(), err)
		}
	}

	s.Logger.Info("Synchronous indexing complete. Added %v pages.", len(files))
	return nil
}

// --- Site methods moved from page.go ---

// getFilePathsForIdentifier returns the munged and original file paths for an identifier
func (s *Site) getFilePathsForIdentifier(identifier, extension string) (mungedPath, originalPath, actualIdentifier string) {
	mungedIdentifier := wikiidentifiers.MungeIdentifier(identifier)
	mungedPath = path.Join(s.PathToData, base32tools.EncodeToBase32(strings.ToLower(mungedIdentifier))+"."+extension)
	originalPath = path.Join(s.PathToData, base32tools.EncodeToBase32(strings.ToLower(identifier))+"."+extension)
	actualIdentifier = mungedIdentifier
	return mungedPath, originalPath, actualIdentifier
}

func (s *Site) readFileByIdentifier(identifier, extension string) (string, []byte, error) {
	mungedPath, originalPath, mungedIdentifier := s.getFilePathsForIdentifier(identifier, extension)

	// First try with the munged identifier
	b, err := os.ReadFile(mungedPath)
	if err == nil {
		return mungedIdentifier, b, nil
	}

	// Then try with the original identifier if that didn't work (older files)
	b, err = os.ReadFile(originalPath)
	if err == nil {
		return identifier, b, nil
	}

	return mungedIdentifier, nil, err
}

// Open opens a page by its identifier.
func (s *Site) Open(requestedIdentifier string) (p *Page) {
	// Create a new page object to be returned if no file is found.
	p = new(Page)
	p.Identifier = requestedIdentifier
	p.Site = s
	p.Text = versionedtext.NewVersionedText("")
	p.WasLoadedFromDisk = false

	identifier, bJSON, err := s.readFileByIdentifier(requestedIdentifier, "json")
	if err == nil {
		// Found JSON file, decode it.
		// The previous code `json.Unmarshal(bJSON, &p)` was incorrect. It replaces the pointer p,
		// wiping out the p.Site assignment. The correct way is to unmarshal into the struct pointed to by p.
		if errJSON := json.Unmarshal(bJSON, p); errJSON != nil {
			s.Logger.Error("Failed to unmarshal page %s: %v", identifier, errJSON)
		} else {
			p.WasLoadedFromDisk = true
		}
		return p
	}

	if !os.IsNotExist(err) {
		s.Logger.Error("Error reading page json for %s: %v", requestedIdentifier, err)
		return p // Return empty page object
	}

	// JSON file not found, try to load from .md file.
	identifier, mdBytes, err := s.readFileByIdentifier(requestedIdentifier, mdExtension)
	if err != nil {
		return p // Return empty page object
	}

	p.Identifier = identifier
	p.Text = versionedtext.NewVersionedText(string(mdBytes))
	p.WasLoadedFromDisk = true
	return p
}

// OpenOrInit opens a page or initializes a new one if it doesn't exist.
// Returns an error if page initialization fails to save.
func (s *Site) OpenOrInit(requestedIdentifier string, req *http.Request) (*Page, error) {
	p := s.Open(requestedIdentifier)
	if p.IsNew() {
		prams := req.URL.Query()
		initialText := "identifier = \"" + p.Identifier + "\"\n"
		tmpl := prams.Get("tmpl")
		for pram, vals := range prams {
			if len(vals) > 1 {
				initialText += pram + " = [ \"" + strings.Join(vals, "\", \"") + "\"]\n"
			} else if len(vals) == 1 {
				initialText += pram + " = \"" + vals[0] + "\"\n"
			}
		}

		if tmpl == "inv_item" {
			initialText += `

[inventory]
items = [

]

`
		}

		if initialText != "" {
			initialText = "+++\n" + initialText + "+++\n"
		}

		initialText += "\n# {{or .Title .Identifier}}" + "\n"

		if tmpl == "inv_item" {
			initialText += `
### Goes in: {{LinkTo .Inventory.Container }}

{{if IsContainer .Identifier }}
## Contents
{{ ShowInventoryContentsOf .Identifier }}
{{ end }}
`
		}

		p.Text = versionedtext.NewVersionedText(initialText)
		p.Render()
		if err := p.Save(); err != nil {
			s.Logger.Error("Failed to save new page '%s': %v", p.Identifier, err)
			return nil, fmt.Errorf("failed to save new page '%s': %w", p.Identifier, err)
		}
	}
	p.Render()
	return p, nil
}

// DirectoryEntry represents an entry in the wiki directory.
type DirectoryEntry struct {
	Path       string
	Length     int
	Numchanges int
	LastEdited time.Time
}

// LastEditTime returns the last edit time of the directory entry.
func (d DirectoryEntry) LastEditTime() string {
	return d.LastEdited.Format("Mon Jan 2 15:04:05 MST 2006")
}

// Name returns the name of the directory entry.
func (d DirectoryEntry) Name() string {
	return d.Path
}

// Size returns the size of the directory entry.
func (d DirectoryEntry) Size() int64 {
	return int64(d.Length)
}

// Mode returns the file mode of the directory entry.
func (DirectoryEntry) Mode() os.FileMode {
	return os.ModePerm
}

// ModTime returns the modification time of the directory entry.
func (d DirectoryEntry) ModTime() time.Time {
	return d.LastEdited
}

// IsDir returns true if the directory entry is a directory.
func (DirectoryEntry) IsDir() bool {
	return false
}

// Sys returns the underlying data source of the directory entry.
func (DirectoryEntry) Sys() any {
	return nil
}

// DirectoryList returns a list of all wiki pages in the data directory.
func (s *Site) DirectoryList() []os.FileInfo {
	files, _ := os.ReadDir(s.PathToData)
	entries := make([]os.FileInfo, len(files))
	found := 0
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".json") {
			name := DecodeFileName(f.Name())
			p := s.Open(name)
			entries[found] = DirectoryEntry{
				Path:       name,
				Length:     len(p.Text.GetCurrent()),
				Numchanges: p.Text.NumEdits(),
				LastEdited: time.Unix(p.Text.LastEditTime()/1000000000, 0),
			}
			found = found + 1
		}
	}
	entries = entries[:found]
	sort.Slice(entries, func(i, j int) bool { return entries[i].ModTime().Before(entries[j].ModTime()) })
	return entries
}

// UploadEntry represents an uploaded file entry.
type UploadEntry struct {
	os.FileInfo
}

// UploadList returns a list of all uploaded files in the data directory.
func (s *Site) UploadList() ([]os.FileInfo, error) {
	paths, err := filepath.Glob(path.Join(s.PathToData, "sha256*"))
	if err != nil {
		return nil, err
	}
	result := make([]os.FileInfo, len(paths))
	for i := range paths {
		result[i], err = os.Stat(paths[i])
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

// --- PageReaderMutator implementation ---

func writeFrontmatterToBuffer(content *bytes.Buffer, fmBytes []byte) error {
	if _, err := content.WriteString(tomlDelimiter); err != nil {
		return err
	}
	if _, err := content.Write(fmBytes); err != nil {
		return err
	}
	if !bytes.HasSuffix(fmBytes, []byte("\n")) {
		if _, err := content.WriteString("\n"); err != nil {
			return err
		}
	}
	if _, err := content.WriteString(tomlDelimiter); err != nil {
		return err
	}
	return nil
}

func combineFrontmatterAndMarkdown(fm wikipage.FrontMatter, md wikipage.Markdown) (string, error) {
	fmBytes, err := toml.Marshal(fm)
	if err != nil {
		return "", fmt.Errorf("failed to marshal frontmatter: %v", err)
	}

	// If there's no content, no need to write anything.
	if len(fm) == 0 && len(md) == 0 {
		return "", nil
	}

	var content bytes.Buffer
	if len(fm) > 0 {
		if err := writeFrontmatterToBuffer(&content, fmBytes); err != nil {
			return "", err
		}
	}
	if _, err := content.WriteString(string(md)); err != nil {
		return "", err
	}
	return content.String(), nil
}

// WriteFrontMatter writes the frontmatter for a page.
func (s *Site) WriteFrontMatter(identifier wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	p := s.Open(string(identifier))

	// Use the PageReaderMutator interface to get the current markdown content.
	_, md, err := s.ReadMarkdown(identifier)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not read markdown to write frontmatter: %w", err)
	}

	newText, err := combineFrontmatterAndMarkdown(fm, md)
	if err != nil {
		return err
	}

	// Use Update to correctly save history to .json and current version to .md
	return p.Update(newText)
}


// WriteMarkdown writes the markdown content for a page.
func (s *Site) WriteMarkdown(identifier wikipage.PageIdentifier, md wikipage.Markdown) error {
	p := s.Open(string(identifier))

	// Use the PageReaderMutator interface to get the current frontmatter.
	_, fm, err := s.ReadFrontMatter(identifier)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not read frontmatter to write markdown: %w", err)
	}

	newText, err := combineFrontmatterAndMarkdown(fm, md)
	if err != nil {
		return err
	}

	// Use Update to correctly save history to .json and current version to .md
	return p.Update(newText)
}

// ReadFrontMatter reads the frontmatter for a page.
func (s *Site) ReadFrontMatter(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	page := s.Open(string(identifier))
	matter, err := page.ReadFrontMatter()
	if err != nil {
		return identifier, nil, err
	}
	return identifier, matter, nil
}

// ReadMarkdown reads the markdown content for a page.
func (s *Site) ReadMarkdown(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	page := s.Open(string(identifier))
	markdown, err := page.ReadMarkdown()
	if err != nil {
		return identifier, "", err
	}
	return identifier, markdown, nil
}

// DeletePage deletes a page from disk.
func (s *Site) DeletePage(identifier wikipage.PageIdentifier) error {
	p := s.Open(string(identifier))
	return p.Erase()
}
