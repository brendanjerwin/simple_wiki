package server

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"

	adrgFrontmatter "github.com/adrg/frontmatter"
	"github.com/brendanjerwin/simple_wiki/common"
	"github.com/brendanjerwin/simple_wiki/utils"
	"github.com/schollz/versionedtext"
)

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
	FrontmatterJson    []byte `json:"-"`
	WasLoadedFromDisk  bool   `json:"-"`
}

func (p Page) LastEditTime() time.Time {
	return time.Unix(p.LastEditUnixTime(), 0)
}

func (p Page) LastEditUnixTime() int64 {
	return p.Text.LastEditTime() / 1000000000
}

func (p *Page) parse() (common.FrontMatter, common.Markdown, error) {
	text := p.Text.GetCurrent()
	reader := strings.NewReader(text)

	var fm common.FrontMatter
	md, err := adrgFrontmatter.Parse(reader, &fm) // Auto-detect
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
			md, err = adrgFrontmatter.Parse(strings.NewReader(swappedContent), &fm)
		}
	}

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			// This isn't an error, it just means there's no frontmatter.
			return make(common.FrontMatter), common.Markdown(text), nil
		}
		// This wrapping is needed for the test to pass.
		return nil, "", fmt.Errorf("failed to unmarshal frontmatter for %s: %w", p.Identifier, err)
	}

	if fm == nil {
		fm = make(common.FrontMatter)
	}

	return fm, common.Markdown(md), nil
}

// DecodeFileName decodes a base32-encoded filename and returns the original page name.
// It extracts the filename part before the extension and decodes it from base32.
func DecodeFileName(s string) string {
	s2, _ := utils.DecodeFromBase32(strings.Split(s, ".")[0])
	return s2
}

// Update overwrites the page's content with newText, saves the change, and re-renders the page.
func (p *Page) Update(newText string) error {
	// Update the versioned text
	p.Text.Update(newText)

	// Render the new page
	p.Render()

	return p.Save()
}

func (p *Page) Render() {
	var err error
	p.RenderedPage, p.FrontmatterJson, err = utils.MarkdownToHtmlAndJsonFrontmatter(p.Text.GetCurrent(), true, p.Site, p.Site.MarkdownRenderer, p.Site.FrontmatterIndexQueryer)
	if err != nil {
		p.Site.Logger.Error("Error rendering page: %v", err)
		p.RenderedPage = []byte(err.Error())
	}
}

func (p *Page) Save() error {
	p.Site.saveMut.Lock()
	defer p.Site.saveMut.Unlock()
	bJSON, err := json.MarshalIndent(p, "", " ")
	if err != nil {
		return err
	}

	err = os.WriteFile(path.Join(p.Site.PathToData, utils.EncodeToBase32(strings.ToLower(p.Identifier))+".json"), bJSON, 0644)
	if err != nil {
		return err
	}

	// Write the current Markdown
	err = os.WriteFile(path.Join(p.Site.PathToData, utils.EncodeToBase32(strings.ToLower(p.Identifier))+".md"), []byte(p.Text.CurrentText), 0644)
	if err != nil {
		return err
	}

	p.Site.IndexMaintainer.AddPageToIndex(p.Identifier)

	return nil
}

func (p *Page) IsNew() bool {
	return !p.WasLoadedFromDisk
}

func (p *Page) Erase() error {
	p.Site.Logger.Trace("Erasing %s", p.Identifier)
	p.Site.IndexMaintainer.RemovePageFromIndex(p.Identifier)
	err := os.Remove(path.Join(p.Site.PathToData, utils.EncodeToBase32(strings.ToLower(p.Identifier))+".json"))
	if err != nil {
		return err
	}
	return os.Remove(path.Join(p.Site.PathToData, utils.EncodeToBase32(strings.ToLower(p.Identifier))+".md"))
}
