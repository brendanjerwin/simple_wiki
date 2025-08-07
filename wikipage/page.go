package wikipage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	adrgfrontmatter "github.com/adrg/frontmatter"
)

// Page represents a wiki page
type Page struct {
	Identifier         string
	Text               string
	RenderedPage       []byte `json:"-"`
	FrontmatterJSON    []byte `json:"-"`
	WasLoadedFromDisk  bool   `json:"-"`
	ModTime            time.Time `json:"-"`
}

// parse parses the page content into frontmatter and markdown
func (p *Page) parse() (FrontMatter, Markdown, error) {
	frontmatter := &map[string]any{}
	r := strings.NewReader(p.Text)

	// Try to parse with default delimiter which is ---
	mdContent, err := adrgfrontmatter.Parse(r, frontmatter)
	if err != nil && err != io.EOF {
		// Failed to parse, try swapping delimiters
		// This handles the case where content has TOML-like frontmatter with +++ delimiters
		// but the parser expects YAML-like frontmatter with --- delimiters
		swapped := strings.Replace(p.Text, "+++", "---", 2)
		mdContent, err = adrgfrontmatter.Parse(strings.NewReader(swapped), frontmatter)
		if err != nil && err != io.EOF {
			// Neither delimiter worked, return the error
			return nil, "", fmt.Errorf("failed to parse frontmatter: %w", err)
		}
	}

	// Ensure the frontmatter map is always non-nil
	if frontmatter == nil || *frontmatter == nil {
		empty := make(map[string]any)
		frontmatter = &empty
	}

	return *frontmatter, string(mdContent), nil
}

// GetFrontMatter returns the parsed frontmatter from the page
func (p *Page) GetFrontMatter() (FrontMatter, error) {
	fm, _, err := p.parse()
	return fm, err
}

// GetMarkdown returns the markdown content of the page
func (p *Page) GetMarkdown() (Markdown, error) {
	_, md, err := p.parse()
	return md, err
}

// IRenderMarkdownToHTML is an interface that abstracts the rendering process
type IRenderMarkdownToHTML interface {
	Render(input []byte) ([]byte, error)
}

// IExecuteTemplate is an interface that abstracts template execution
type IExecuteTemplate interface {
	ExecuteTemplate(templateString string, fm FrontMatter, reader PageReader, query IQueryFrontmatterIndex) ([]byte, error)
}

// DottedKeyPath represents a dot-separated path to a frontmatter key.
type DottedKeyPath = string

// Value represents a frontmatter value.
type Value = string

// IQueryFrontmatterIndex defines the interface for querying the frontmatter index.
type IQueryFrontmatterIndex interface {
	QueryExactMatch(dottedKeyPath DottedKeyPath, value Value) []PageIdentifier
	QueryKeyExistence(dottedKeyPath DottedKeyPath) []PageIdentifier
	QueryPrefixMatch(dottedKeyPath DottedKeyPath, valuePrefix string) []PageIdentifier
	GetValue(identifier PageIdentifier, dottedKeyPath DottedKeyPath) Value
}


// markdownToHTMLAndJSONFrontmatter converts markdown to HTML and extracts frontmatter as JSON
func markdownToHTMLAndJSONFrontmatter(s string, reader PageReader, renderer IRenderMarkdownToHTML, templateExecutor IExecuteTemplate, query IQueryFrontmatterIndex) (html []byte, matter []byte, err error) {
	var markdownBytes []byte

	if renderer == nil {
		return nil, nil, errors.New("renderer is not initialized")
	}

	matterMap := &map[string]any{}
	markdownBytes, err = adrgfrontmatter.Parse(strings.NewReader(s), &matterMap)
	if err != nil {
		return []byte(err.Error()), nil, err
	}
	matter, _ = json.Marshal(matterMap)

	markdownBytes, err = templateExecutor.ExecuteTemplate(string(markdownBytes), *matterMap, reader, query)
	if err != nil {
		return []byte(err.Error()), nil, err
	}

	html, err = renderer.Render(markdownBytes)
	if err != nil {
		return nil, nil, err
	}

	return html, matter, nil
}

// Render renders the page content to HTML
func (p *Page) Render(reader PageReader, renderer IRenderMarkdownToHTML, templateExecutor IExecuteTemplate, query IQueryFrontmatterIndex) error {
	var err error
	p.RenderedPage, p.FrontmatterJSON, err = markdownToHTMLAndJSONFrontmatter(p.Text, reader, renderer, templateExecutor, query)
	if err != nil {
		p.RenderedPage = []byte(err.Error())
		return fmt.Errorf("error rendering page: %w", err)
	}
	return nil
}

// IsNew returns true if the page has not been loaded from disk
func (p *Page) IsNew() bool {
	return !p.WasLoadedFromDisk
}

// IsModifiedSince returns true if the page has been modified since the given timestamp
func (p *Page) IsModifiedSince(timestamp int64) bool {
	return p.ModTime.Unix() > timestamp
}