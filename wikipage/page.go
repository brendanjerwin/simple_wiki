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

// DefaultPageTemplate is the default markdown template added to new wiki pages.
// This displays the page title or identifier as an H1 heading.
const DefaultPageTemplate = "# {{or .Title .Identifier}}"

// Page represents a wiki page
type Page struct {
	Identifier             string
	Text                   string
	RenderedPage           []byte `json:"-"`
	RenderedMarkdown       []byte `json:"-"` // Template-expanded markdown before HTML conversion
	FrontmatterJSON        []byte `json:"-"`
	WasLoadedFromDisk      bool      `json:"-"`
	ModTime                time.Time `json:"-"`
}

// parse parses the page content into frontmatter and markdown
func (p *Page) parse() (FrontMatter, Markdown, error) {
	frontmatter := &map[string]any{}
	r := strings.NewReader(p.Text)

	// Try to parse with default delimiter which is ---
	mdContent, err := adrgfrontmatter.Parse(r, frontmatter)
	if err != nil && !errors.Is(err, io.EOF) {
		// Failed to parse, try swapping delimiters
		// This handles the case where content has TOML-like frontmatter with +++ delimiters
		// but the parser expects YAML-like frontmatter with --- delimiters
		swapped := strings.Replace(p.Text, "+++", "---", 2)
		mdContent, err = adrgfrontmatter.Parse(strings.NewReader(swapped), frontmatter)
		if err != nil && !errors.Is(err, io.EOF) {
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


// RenderingResult contains the results of rendering a page
type RenderingResult struct {
	HTML             []byte
	RenderedMarkdown []byte
	FrontmatterJSON  []byte
}

// ParseFrontmatterAndMarkdown parses a page's content string into frontmatter and markdown.
// It returns the markdown content, the parsed frontmatter map, and any error encountered.
func ParseFrontmatterAndMarkdown(content string) ([]byte, map[string]any, error) {
	matterMap := &map[string]any{}
	markdownBytes, err := adrgfrontmatter.Parse(strings.NewReader(content), &matterMap)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	// Ensure the frontmatter map is always non-nil
	if matterMap == nil || *matterMap == nil {
		empty := make(map[string]any)
		matterMap = &empty
	}

	return markdownBytes, *matterMap, nil
}

// ExecuteTemplatesOnMarkdown executes templates within markdown content using the provided
// frontmatter, page reader, template executor, and query interface.
// It returns the template-expanded markdown.
func ExecuteTemplatesOnMarkdown(markdown []byte, frontmatter map[string]any, reader PageReader, templateExecutor IExecuteTemplate, query IQueryFrontmatterIndex) ([]byte, error) {
	if templateExecutor == nil {
		return nil, errors.New("template executor is not initialized")
	}
	if query == nil {
		return nil, errors.New("frontmatter index queryer is not initialized")
	}
	if reader == nil {
		return nil, errors.New("page reader is not initialized")
	}

	expandedMarkdown, err := templateExecutor.ExecuteTemplate(string(markdown), frontmatter, reader, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute templates: %w", err)
	}
	return expandedMarkdown, nil
}

// RenderMarkdownToHTML converts markdown content to HTML using the provided renderer.
// It returns the HTML output.
func RenderMarkdownToHTML(markdown []byte, renderer IRenderMarkdownToHTML) ([]byte, error) {
	if renderer == nil {
		return nil, errors.New("renderer is not initialized")
	}

	html, err := renderer.Render(markdown)
	if err != nil {
		return nil, fmt.Errorf("failed to render markdown to HTML: %w", err)
	}
	return html, nil
}

// RenderPageWithTemplates renders a page by parsing frontmatter, executing templates on markdown,
// and converting the result to HTML. It composes ParseFrontmatterAndMarkdown, ExecuteTemplatesOnMarkdown,
// and RenderMarkdownToHTML to create a complete rendering pipeline.
// It returns both the HTML, the template-expanded markdown, and the frontmatter JSON.
func RenderPageWithTemplates(content string, reader PageReader, renderer IRenderMarkdownToHTML, templateExecutor IExecuteTemplate, query IQueryFrontmatterIndex) (RenderingResult, error) {
	var result RenderingResult

	// Validate required dependencies
	if reader == nil {
		return result, errors.New("page reader is not initialized")
	}
	if renderer == nil {
		return result, errors.New("renderer is not initialized")
	}
	if templateExecutor == nil {
		return result, errors.New("template executor is not initialized")
	}
	if query == nil {
		return result, errors.New("frontmatter index queryer is not initialized")
	}

	// Step 1: Parse frontmatter and markdown
	markdownBytes, frontmatter, err := ParseFrontmatterAndMarkdown(content)
	if err != nil {
		return result, err
	}

	// Serialize frontmatter to JSON
	result.FrontmatterJSON, err = json.Marshal(frontmatter)
	if err != nil {
		return result, fmt.Errorf("failed to marshal frontmatter to JSON: %w", err)
	}

	// Step 2: Execute templates on markdown
	expandedMarkdown, err := ExecuteTemplatesOnMarkdown(markdownBytes, frontmatter, reader, templateExecutor, query)
	if err != nil {
		return result, err
	}

	// Capture the template-expanded markdown before HTML conversion
	result.RenderedMarkdown = expandedMarkdown

	// Step 3: Render markdown to HTML
	result.HTML, err = RenderMarkdownToHTML(expandedMarkdown, renderer)
	if err != nil {
		return result, err
	}

	return result, nil
}

// Render renders the page content to HTML
func (p *Page) Render(reader PageReader, renderer IRenderMarkdownToHTML, templateExecutor IExecuteTemplate, query IQueryFrontmatterIndex) error {
	result, err := RenderPageWithTemplates(p.Text, reader, renderer, templateExecutor, query)
	if err != nil {
		return fmt.Errorf("error rendering page: %w", err)
	}
	p.RenderedPage = result.HTML
	p.RenderedMarkdown = result.RenderedMarkdown
	p.FrontmatterJSON = result.FrontmatterJSON
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