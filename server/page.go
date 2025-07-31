package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	adrgfrontmatter "github.com/adrg/frontmatter"
	indexfrontmatter "github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/templating"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/schollz/versionedtext"
)

const nanosecondsPerSecond = 1000000000

// Page is the basic struct
type Page struct {
	Site *Site `json:"-"`

	Identifier         string
	Text               versionedtext.VersionedText
	Meta               string
	RenderedPage       []byte `json:"-"`
	IsLocked           bool
	PassphraseToUnlock string
	UnlockedFor        string
	FrontmatterJSON    []byte `json:"-"`
	WasLoadedFromDisk  bool   `json:"-"`
}

// LastEditTime returns the last edit time of the page.
func (p Page) LastEditTime() time.Time {
	return time.Unix(p.LastEditUnixTime(), 0)
}

// LastEditUnixTime returns the last edit time of the page in Unix nanoseconds.
func (p Page) LastEditUnixTime() int64 {
	return p.Text.LastEditTime() / nanosecondsPerSecond
}

func (p *Page) parse() (wikipage.FrontMatter, wikipage.Markdown, error) {
	text := p.Text.GetCurrent()
	reader := strings.NewReader(text)

	var fm wikipage.FrontMatter
	md, err := adrgfrontmatter.Parse(reader, &fm) // Auto-detect
	if err != nil {
		// Check if it was a TOML parsing error. This can happen if fences are '+++' but content is YAML-like.
		// We can't consistently rely on the specific error type due to versioning issues, so we check the message.
		if strings.Contains(err.Error(), "bare keys cannot contain") {
			p.Site.Logger.Trace("TOML-like parse failed for %s, retrying with fences swapped to YAML. Error: %v", p.Identifier, err)
			// Reset reader and read all content
			_, seekErr := reader.Seek(0, io.SeekStart)
			if seekErr != nil {
				return nil, "", fmt.Errorf("failed to seek for parse retry: %w", seekErr)
			}
			contentBytes, readErr := io.ReadAll(reader)
			if readErr != nil {
				return nil, "", fmt.Errorf("failed to read content for parse retry: %w", readErr)
			}
			// Swap fences and retry parsing. Replace only the first two occurrences.
			swappedContent := strings.Replace(string(contentBytes), "+++", "---", 2)
			md, err = adrgfrontmatter.Parse(strings.NewReader(swappedContent), &fm)
		}
	}

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			// This isn't an error, it just means there's no frontmatter.
			return make(wikipage.FrontMatter), wikipage.Markdown(text), nil
		}
		// This wrapping is needed for the test to pass.
		return nil, "", fmt.Errorf("failed to unmarshal frontmatter for %s: %w", p.Identifier, err)
	}

	if fm == nil {
		fm = make(wikipage.FrontMatter)
	}

	return fm, wikipage.Markdown(md), nil
}

// DecodeFileName decodes a filename from base32.
func DecodeFileName(s string) string {
	s2, _ := base32tools.DecodeFromBase32(strings.Split(s, ".")[0])
	return s2
}

// Update overwrites the page's content with newText, saves the change, and re-renders the page.
func (p *Page) Update(newText string) error {
	return p.updateWithMigrations(newText)
}

// updateWithoutMigrations provides internal update mechanism that skips migrations
func (p *Page) updateWithoutMigrations(newText string) error {
	// Update the versioned text
	p.Text.Update(newText)

	// Render the new page
	p.Render()

	return p.Save()
}

// updateWithMigrations applies migrations and updates the page
func (p *Page) updateWithMigrations(newText string) error {
	// Apply migrations to fix user mistakes in real-time
	migratedContent, err := p.applyMigrations([]byte(newText))
	if err != nil {
		return fmt.Errorf("failed to apply migrations during save: %w", err)
	}

	// If migration changed the content, use the migrated version
	if string(migratedContent) != newText {
		newText = string(migratedContent)
	}

	// Update the versioned text
	p.Text.Update(newText)

	// Render the new page
	p.Render()

	return p.Save()
}

// applyMigrations applies frontmatter migrations to content and auto-saves if successful
func (p *Page) applyMigrations(content []byte) ([]byte, error) {
	if p == nil || p.Site == nil {
		return nil, errors.New("page or site is nil")
	}

	// Error if no migration applicator is configured - this is an application setup mistake
	if p.Site.MigrationApplicator == nil {
		return nil, errors.New("migration applicator not configured: this is an application setup mistake")
	}

	migratedContent, err := p.Site.MigrationApplicator.ApplyMigrations(content)
	if err != nil {
		// Log migration failure but continue with original content
		p.Site.Logger.Warn("Migration failed, using original content: %v", err)
		return content, nil
	}

	// If migration was applied, save the migrated content using normal page saving mechanism
	// This ensures the migration appears in the page history like any other change
	if !bytes.Equal(content, migratedContent) {
		// Use updateWithoutMigrations to prevent recursive migration calls
		if saveErr := p.updateWithoutMigrations(string(migratedContent)); saveErr != nil {
			p.Site.Logger.Warn("Failed to save migrated content for %s: %v", p.Identifier, saveErr)
		} else {
			p.Site.Logger.Info("Successfully migrated and saved frontmatter for page: %s", p.Identifier)
		}
	}

	return migratedContent, nil
}

// ReadFrontMatter reads the frontmatter for this page, applying migrations as needed.
func (p *Page) ReadFrontMatter() (wikipage.FrontMatter, error) {
	if p == nil || p.Site == nil {
		return nil, errors.New("page or site is nil")
	}

	_, content, err := p.Site.readFileByIdentifier(wikipage.PageIdentifier(p.Identifier), mdExtension)
	if err != nil {
		return nil, err
	}

	// Apply migrations to the content
	migratedContent, err := p.applyMigrations(content)
	if err != nil {
		return nil, err
	}

	var matter wikipage.FrontMatter
	_, err = p.Site.lenientParse(migratedContent, &matter)
	if err != nil {
		if strings.Contains(err.Error(), "format not found") {
			return make(wikipage.FrontMatter), nil
		}
		return nil, err
	}

	if matter == nil {
		return make(wikipage.FrontMatter), nil
	}

	return matter, nil
}

// ReadMarkdown reads the markdown content for this page, applying migrations as needed.
func (p *Page) ReadMarkdown() (wikipage.Markdown, error) {
	if p == nil || p.Site == nil {
		return "", errors.New("page or site is nil")
	}

	_, content, err := p.Site.readFileByIdentifier(wikipage.PageIdentifier(p.Identifier), mdExtension)
	if err != nil {
		return "", err
	}

	// Apply migrations to the content
	migratedContent, err := p.applyMigrations(content)
	if err != nil {
		return "", err
	}

	var dummy any
	body, err := p.Site.lenientParse(migratedContent, &dummy)
	if err != nil {
		if strings.Contains(err.Error(), "format not found") {
			// No frontmatter found, the entire content is markdown.
			return wikipage.Markdown(body), nil
		}
		return "", err // A real parsing error.
	}

	return wikipage.Markdown(body), nil
}

func markdownToHTMLAndJSONFrontmatter(s string, site wikipage.PageReader, renderer IRenderMarkdownToHTML, query indexfrontmatter.IQueryFrontmatterIndex) (html []byte, matter []byte, err error) {
	var markdownBytes []byte

	matterMap := &map[string]any{}
	markdownBytes, err = adrgfrontmatter.Parse(strings.NewReader(s), &matterMap)
	if err != nil {
		return []byte(err.Error()), nil, err
	}
	matter, _ = json.Marshal(matterMap)

	markdownBytes, err = templating.ExecuteTemplate(string(markdownBytes), *matterMap, site, query)
	if err != nil {
		return []byte(err.Error()), nil, err
	}

	html, err = renderer.Render(markdownBytes)
	if err != nil {
		return nil, nil, err
	}

	return html, matter, nil
}

// Render renders the page content to HTML and extracts frontmatter.
func (p *Page) Render() {
	var err error
	p.RenderedPage, p.FrontmatterJSON, err = markdownToHTMLAndJSONFrontmatter(p.Text.GetCurrent(), p.Site, p.Site.MarkdownRenderer, p.Site.FrontmatterIndexQueryer)
	if err != nil {
		p.Site.Logger.Error("Error rendering page: %v", err)
		p.RenderedPage = []byte(err.Error())
	}
}

// Save saves the page to disk.
func (p *Page) Save() error {
	p.Site.saveMut.Lock()
	defer p.Site.saveMut.Unlock()
	bJSON, err := json.MarshalIndent(p, "", " ")
	if err != nil {
		return err
	}

	err = os.WriteFile(path.Join(p.Site.PathToData, base32tools.EncodeToBase32(strings.ToLower(p.Identifier))+".json"), bJSON, 0644)
	if err != nil {
		return err
	}

	// Write the current Markdown
	err = os.WriteFile(path.Join(p.Site.PathToData, base32tools.EncodeToBase32(strings.ToLower(p.Identifier))+".md"), []byte(p.Text.CurrentText), 0644)
	if err != nil {
		return err
	}

	_ = p.Site.IndexMaintainer.AddPageToIndex(p.Identifier)

	return nil
}

// IsNew returns true if the page has not been loaded from disk.
func (p *Page) IsNew() bool {
	return !p.WasLoadedFromDisk
}

// Erase deletes the page from disk.
func (p *Page) Erase() error {
	p.Site.Logger.Trace("Erasing %s", p.Identifier)
	_ = p.Site.IndexMaintainer.RemovePageFromIndex(p.Identifier)
	err := os.Remove(path.Join(p.Site.PathToData, base32tools.EncodeToBase32(strings.ToLower(p.Identifier))+".json"))
	if err != nil {
		return err
	}
	return os.Remove(path.Join(p.Site.PathToData, base32tools.EncodeToBase32(strings.ToLower(p.Identifier))+".md"))
}
