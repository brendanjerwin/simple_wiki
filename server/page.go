package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/adrg/frontmatter"
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
}

func (p Page) LastEditTime() time.Time {
	return time.Unix(p.LastEditUnixTime(), 0)
}

func (p Page) LastEditUnixTime() int64 {
	return p.Text.LastEditTime() / 1000000000
}

func (s *Site) ReadFrontMatter(requested_identifier string) (string, common.FrontMatter, error) {
	identifier, content, err := s.readFileByIdentifier(requested_identifier, "md")
	if err != nil {
		return identifier, nil, err
	}

	matter := &map[string]interface{}{}
	_, err = frontmatter.Parse(bytes.NewReader(content), &matter)
	if err != nil {
		return identifier, nil, err
	}

	return identifier, *matter, nil
}

func (s *Site) ReadMarkdown(requested_identifier string) (string, string, error) {
	identifier, content, err := s.readFileByIdentifier(requested_identifier, "md")
	if err != nil {
		return identifier, "", err
	}

	matter := &common.FrontMatter{}
	markdownBytes, err := frontmatter.Parse(bytes.NewReader(content), &matter)
	if err != nil {
		return identifier, "", err
	}

	return identifier, string(markdownBytes), nil
}

func (s *Site) Open(requested_identifier string) (p *Page) {
	identifier, bJSON, err := s.readFileByIdentifier(requested_identifier, "json")
	if err != nil {
		return
	}
	p = new(Page)
	p.Site = s
	p.Identifier = identifier
	p.Text = versionedtext.NewVersionedText("")
	err = json.Unmarshal(bJSON, &p)
	if err != nil {
		p = new(Page)
	}
	return p
}

func (s *Site) OpenOrInit(requested_identifier string, req *http.Request) (p *Page) {
	identifier, bJSON, err := s.readFileByIdentifier(requested_identifier, "json")
	if err != nil {
		p = new(Page)
		p.Site = s
		p.Identifier = identifier

		prams := req.URL.Query()
		initialText := "identifier = \"" + identifier + "\"\n"
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
		return p
	}
	err = json.Unmarshal(bJSON, &p)
	if err != nil {
		panic(err)
	}
	p.Site = s

	p.Render()

	return p
}

func (s *Site) readFileByIdentifier(identifier, extension string) (string, []byte, error) {

	//First try with the munged identifier
	munged_identifier := common.MungeIdentifier(identifier)
	bJSON, err := os.ReadFile(path.Join(s.PathToData, utils.EncodeToBase32(strings.ToLower(munged_identifier))+"."+extension))
	if err == nil {
		return munged_identifier, bJSON, nil
	}

	//Then try with the original identifier if that didn't work (older files)
	bJSON, err = os.ReadFile(path.Join(s.PathToData, utils.EncodeToBase32(strings.ToLower(identifier))+"."+extension))
	if err == nil {
		return identifier, bJSON, nil
	}

	return munged_identifier, nil, err
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

func (d DirectoryEntry) Sys() interface{} {
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
	sort.Slice(entries, func(i, j int) bool { return entries[i].ModTime().After(entries[j].ModTime()) })
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

func DecodeFileName(s string) string {
	s2, _ := utils.DecodeFromBase32(strings.Split(s, ".")[0])
	return s2
}

// Update cleans the text and updates the versioned text
// and generates a new render
func (p *Page) Update(newText string) error {
	// Trim space from end
	newText = strings.TrimRight(newText, "\n\t ")

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
		p.Site.Logger.Error(err.Error())
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
	return !utils.Exists(path.Join(p.Site.PathToData, utils.EncodeToBase32(strings.ToLower(p.Identifier))+".json"))
}

func (p *Page) Erase() error {
	p.Site.Logger.Trace("Erasing " + p.Identifier)

	err := os.Remove(path.Join(p.Site.PathToData, utils.EncodeToBase32(strings.ToLower(p.Identifier))+".json"))
	if err != nil {
		return err
	}
	return os.Remove(path.Join(p.Site.PathToData, utils.EncodeToBase32(strings.ToLower(p.Identifier))+".md"))
}
