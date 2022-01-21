package server

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

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

func (s *Site) Open(name string) (p *Page) {
	p = new(Page)
	p.Site = s
	p.Identifier = name
	p.Text = versionedtext.NewVersionedText("")
	p.Render()
	bJSON, err := ioutil.ReadFile(path.Join(s.PathToData, encodeToBase32(strings.ToLower(name))+".json"))
	if err != nil {
		return
	}
	err = json.Unmarshal(bJSON, &p)
	if err != nil {
		p = new(Page)
	}
	return p
}

func (s *Site) OpenOrInit(identifier string, req *http.Request) (p *Page) {
	bJSON, err := ioutil.ReadFile(path.Join(s.PathToData, encodeToBase32(strings.ToLower(identifier))+".json"))
	if err != nil {
		p = new(Page)
		p.Site = s
		p.Identifier = identifier

		prams := req.URL.Query()
		initialText := "identifier = \"" + identifier + "\"\n"
		title := prams.Get("title")
		for pram, vals := range prams {
			if len(vals) > 1 {
				initialText += pram + " = [ \"" + strings.Join(vals, "\", \"") + "\"]\n"
			} else if len(vals) == 1 {
				initialText += pram + " = \"" + vals[0] + "\"\n"
			}
		}

		if initialText != "" {
			initialText = "+++\n" + initialText + "+++\n"
		}

		if title != "" {
			initialText += "\n# " + title + "\n"
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
	files, _ := ioutil.ReadDir(s.PathToData)
	entries := make([]os.FileInfo, len(files))
	found := -1
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".json") {
			name := DecodeFileName(f.Name())
			p := s.Open(name)
			found = found + 1
			entries[found] = DirectoryEntry{
				Path:       name,
				Length:     len(p.Text.GetCurrent()),
				Numchanges: p.Text.NumEdits(),
				LastEdited: time.Unix(p.Text.LastEditTime()/1000000000, 0),
			}
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
	s2, _ := decodeFromBase32(strings.Split(s, ".")[0])
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

var rBracketPage = regexp.MustCompile(`\[\[(.*?)\]\]`)

func (p *Page) Render() {
	// Convert [[page]] to [page](/page/view)
	currentText := p.Text.GetCurrent()
	for _, s := range rBracketPage.FindAllString(currentText, -1) {
		currentText = strings.Replace(currentText, s, "["+s[2:len(s)-2]+"](/"+s[2:len(s)-2]+"/view)", 1)
	}
	p.Text.Update(currentText)
	p.RenderedPage, p.FrontmatterJson = MarkdownToHtmlAndJsonFrontmatter(p.Text.GetCurrent(), true)
}

func (p *Page) Save() error {
	p.Site.saveMut.Lock()
	defer p.Site.saveMut.Unlock()
	bJSON, err := json.MarshalIndent(p, "", " ")
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path.Join(p.Site.PathToData, encodeToBase32(strings.ToLower(p.Identifier))+".json"), bJSON, 0644)
	if err != nil {
		return err
	}

	// Write the current Markdown
	return ioutil.WriteFile(path.Join(p.Site.PathToData, encodeToBase32(strings.ToLower(p.Identifier))+".md"), []byte(p.Text.CurrentText), 0644)
}

func (p *Page) IsNew() bool {
	return !exists(path.Join(p.Site.PathToData, encodeToBase32(strings.ToLower(p.Identifier))+".json"))
}

func (p *Page) Erase() error {
	p.Site.Logger.Trace("Erasing " + p.Identifier)

	err := os.Remove(path.Join(p.Site.PathToData, encodeToBase32(strings.ToLower(p.Identifier))+".json"))
	if err != nil {
		return err
	}
	return os.Remove(path.Join(p.Site.PathToData, encodeToBase32(strings.ToLower(p.Identifier))+".md"))
}
