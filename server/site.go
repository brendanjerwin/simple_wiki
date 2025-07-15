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

	adrgFrontmatter "github.com/adrg/frontmatter"
	"github.com/brendanjerwin/simple_wiki/common"
	"github.com/brendanjerwin/simple_wiki/index"
	"github.com/brendanjerwin/simple_wiki/index/bleve"
	"github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/sec"
	"github.com/brendanjerwin/simple_wiki/utils"
	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/jcelliott/lumber"
	"github.com/pelletier/go-toml/v2"
	"github.com/schollz/versionedtext"
)

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
	MarkdownRenderer        utils.IRenderMarkdownToHtml
	IndexMaintainer         index.IMaintainIndex
	FrontmatterIndexQueryer frontmatter.IQueryFrontmatterIndex
	BleveIndexQueryer       bleve.IQueryBleveIndex
	saveMut                 sync.Mutex
}

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
	defer file.Close()

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

func (s *Site) InitializeIndexing() error {
	frontmatterIndex := frontmatter.NewFrontmatterIndex(s)
	bleveIndex, err := bleve.NewBleveIndex(s, frontmatterIndex)
	if err != nil {
		return err
	}
	multiMaintainer := index.NewMultiMaintainer(frontmatterIndex, bleveIndex)

	s.FrontmatterIndexQueryer = frontmatterIndex
	s.BleveIndexQueryer = bleveIndex
	s.IndexMaintainer = multiMaintainer

	files := s.DirectoryList()
	for _, file := range files {
		s.IndexMaintainer.AddPageToIndex(file.Name())
	}

	s.Logger.Info("Indexing complete. Added %v pages.", len(files))

	return nil
}

// --- Site methods moved from page.go ---

func (s *Site) readFileByIdentifier(identifier, extension string) (string, []byte, error) {
	// First try with the munged identifier
	munged_identifier := common.MungeIdentifier(identifier)
	b, err := os.ReadFile(path.Join(s.PathToData, utils.EncodeToBase32(strings.ToLower(munged_identifier))+"."+extension))
	if err == nil {
		return munged_identifier, b, nil
	}

	// Then try with the original identifier if that didn't work (older files)
	b, err = os.ReadFile(path.Join(s.PathToData, utils.EncodeToBase32(strings.ToLower(identifier))+"."+extension))
	if err == nil {
		return identifier, b, nil
	}

	return munged_identifier, nil, err
}

func (s *Site) Open(requested_identifier string) (p *Page) {
	// Create a new page object to be returned if no file is found.
	p = new(Page)
	p.Identifier = requested_identifier
	p.Site = s
	p.Text = versionedtext.NewVersionedText("")
	p.WasLoadedFromDisk = false

	identifier, bJSON, err := s.readFileByIdentifier(requested_identifier, "json")
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
		s.Logger.Error("Error reading page json for %s: %v", requested_identifier, err)
		return p // Return empty page object
	}

	// JSON file not found, try to load from .md file.
	identifier, mdBytes, err := s.readFileByIdentifier(requested_identifier, "md")
	if err != nil {
		return p // Return empty page object
	}

	p.Identifier = identifier
	p.Text = versionedtext.NewVersionedText(string(mdBytes))
	p.WasLoadedFromDisk = true
	return p
}

func (s *Site) OpenOrInit(requested_identifier string, req *http.Request) (p *Page) {
	p = s.Open(requested_identifier)
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
		p.Save()
	}
	p.Render()
	return p
}

type DirectoryEntry struct {
	Path       string
	Length     int
	Numchanges int
	LastEdited time.Time
}

func (d DirectoryEntry) LastEditTime() string {
	return d.LastEdited.Format("Mon Jan 2 15:04:05 MST 2006")
}

func (d DirectoryEntry) Name() string {
	return d.Path
}

func (d DirectoryEntry) Size() int64 {
	return int64(d.Length)
}

func (d DirectoryEntry) Mode() os.FileMode {
	return os.ModePerm
}

func (d DirectoryEntry) ModTime() time.Time {
	return d.LastEdited
}

func (d DirectoryEntry) IsDir() bool {
	return false
}

func (d DirectoryEntry) Sys() any {
	return nil
}

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

type UploadEntry struct {
	os.FileInfo
}

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

// --- PageReadWriter implementation ---

func combineFrontmatterAndMarkdown(fm common.FrontMatter, md common.Markdown) (string, error) {
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
		content.WriteString("+++\n")
		content.Write(fmBytes)
		if !bytes.HasSuffix(fmBytes, []byte("\n")) {
			content.WriteString("\n")
		}
		content.WriteString("+++\n")
	}
	content.WriteString(string(md))
	return content.String(), nil
}

func (s *Site) WriteFrontMatter(identifier common.PageIdentifier, fm common.FrontMatter) error {
	p := s.Open(string(identifier))

	// Use the PageReadWriter interface to get the current markdown content.
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

func lenientParse(content []byte, matter any) (body []byte, err error) {
	body, err = adrgFrontmatter.Parse(bytes.NewReader(content), matter)
	if err != nil {
		var tomlErr *toml.DecodeError
		// If it's a TOML parsing error and it has TOML delimiters, try to parse as YAML.
		// `adrg/frontmatter` does not export its YAML/TOML parsing errors, so we have
		// to rely on `go-toml`'s error type or string matching for the error.
		if (errors.As(err, &tomlErr) || strings.Contains(err.Error(), "bare keys cannot contain")) &&
			bytes.HasPrefix(content, []byte("+++")) {
			// Replace TOML delimiters with YAML and try again
			newContent := bytes.Replace(content, []byte("+++"), []byte("---"), 2)
			body, err = adrgFrontmatter.Parse(bytes.NewReader(newContent), matter)
			return
		}
	}
	return
}

func (s *Site) WriteMarkdown(identifier common.PageIdentifier, md common.Markdown) error {
	p := s.Open(string(identifier))

	// Use the PageReadWriter interface to get the current frontmatter.
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

func (s *Site) ReadFrontMatter(identifier common.PageIdentifier) (common.PageIdentifier, common.FrontMatter, error) {
	identifier, content, err := s.readFileByIdentifier(identifier, "md")
	if err != nil {
		return identifier, nil, err
	}

	var matter common.FrontMatter
	_, err = lenientParse(content, &matter)
	if err != nil {
		if strings.Contains(err.Error(), "format not found") {
			return identifier, make(common.FrontMatter), nil
		}
		return identifier, nil, err
	}

	if matter == nil {
		return identifier, make(common.FrontMatter), nil
	}

	return identifier, matter, nil
}

func (s *Site) ReadMarkdown(identifier common.PageIdentifier) (common.PageIdentifier, common.Markdown, error) {
	identifier, content, err := s.readFileByIdentifier(identifier, "md")
	if err != nil {
		return identifier, "", err
	}

	var dummy any
	body, err := lenientParse(content, &dummy)
	if err != nil {
		if strings.Contains(err.Error(), "format not found") {
			// No frontmatter found, the entire content is markdown.
			return identifier, common.Markdown(body), nil
		}
		return identifier, "", err // A real parsing error.
	}

	return identifier, common.Markdown(body), nil
}
