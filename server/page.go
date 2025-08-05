package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	adrgfrontmatter "github.com/adrg/frontmatter"
	indexfrontmatter "github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/templating"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/schollz/versionedtext"
)

// ConfigurationError represents an application setup/configuration error
type ConfigurationError struct {
	Component string
	Err       error
}

func (e *ConfigurationError) Error() string {
	return fmt.Sprintf("configuration error in %s: %v", e.Component, e.Err)
}

func (e *ConfigurationError) Unwrap() error {
	return e.Err
}

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

	// If migration was applied, save the migrated content without triggering rendering
	// This ensures the migration appears in the page history like any other change
	if !bytes.Equal(content, migratedContent) {
		// Update the versioned text with migrated content
		p.Text.Update(string(migratedContent))
		
		// Save without indexing to prevent circular references
		if saveErr := p.Site.savePageWithoutIndexing(p); saveErr != nil {
			p.Site.Logger.Warn("Failed to save migrated content for %s: %v", p.Identifier, saveErr)
		} else {
			p.Site.Logger.Info("Successfully migrated and saved frontmatter for page: %s", p.Identifier)
		}
	}

	return migratedContent, nil
}

// GetFrontMatter returns the frontmatter for this page from already-loaded content.
func (p *Page) GetFrontMatter() (wikipage.FrontMatter, error) {
	if p == nil {
		return nil, errors.New("page is nil")
	}

	fm, _, err := p.parse()
	return fm, err
}

// GetMarkdown returns the markdown content for this page from already-loaded content.
func (p *Page) GetMarkdown() (wikipage.Markdown, error) {
	if p == nil {
		return "", errors.New("page is nil")
	}

	_, md, err := p.parse()
	return md, err
}

func markdownToHTMLAndJSONFrontmatter(s string, site wikipage.PageReader, renderer IRenderMarkdownToHTML, query indexfrontmatter.IQueryFrontmatterIndex) (html []byte, matter []byte, err error) {
	var markdownBytes []byte

	if renderer == nil {
		return nil, nil, &ConfigurationError{
			Component: "MarkdownRenderer",
			Err:       errors.New("renderer is not initialized"),
		}
	}

	matterMap := &map[string]any{}
	markdownBytes, err = adrgfrontmatter.Parse(strings.NewReader(s), &matterMap)
	if err != nil {
		return []byte(err.Error()), nil, err
	}
	matter, _ = json.Marshal(matterMap)

	markdownBytes, err = templating.ExecuteTemplateForServer(string(markdownBytes), *matterMap, site, query)
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

// IsNew returns true if the page has not been loaded from disk.
func (p *Page) IsNew() bool {
	return !p.WasLoadedFromDisk
}
