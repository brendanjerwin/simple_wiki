package core

type (
	// PageIdentifier is a string that uniquely identifies a page.
	PageIdentifier = string
	// Markdown is the markdown content of a page.
	Markdown = string
	// FrontMatter is the frontmatter of a page.
	FrontMatter = map[string]any
)

// SearchResult represents a search result from the Bleve index.
type SearchResult struct {
	Identifier   PageIdentifier
	Title        string
	FragmentHTML string
}
