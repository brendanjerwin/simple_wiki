package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"github.com/brendanjerwin/simple_wiki/common"
	"github.com/brendanjerwin/simple_wiki/utils"
	"github.com/schollz/versionedtext"
	"gopkg.in/yaml.v2"
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

	// check for frontmatter
	parts := bytes.SplitN([]byte(text), []byte("---\n"), 3)
	if len(parts) < 3 {
		return make(common.FrontMatter), common.Markdown(text), nil
	}

	var fm common.FrontMatter
	markdown := common.Markdown(parts[2])
	if err := yaml.Unmarshal(parts[1], &fm); err != nil {
		return nil, markdown, fmt.Errorf("failed to unmarshal frontmatter for %s: %v", p.Identifier, err)
	}

	return fm, markdown, nil
}

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
