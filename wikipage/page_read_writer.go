package wikipage

// PageIdentifier is the unique identifier for a page.
type PageIdentifier = string

// Markdown is the content of a page in Markdown format.
type Markdown = string

// FrontMatter is the frontmatter of a page.
type FrontMatter = map[string]any

// PageReader is an interface for reading page content.
type PageReader interface {
	ReadFrontMatter(requestedIdentifier PageIdentifier) (PageIdentifier, FrontMatter, error)
	ReadMarkdown(requestedIdentifier PageIdentifier) (PageIdentifier, Markdown, error)
}

// PageWriter is an interface for writing page content.
type PageWriter interface {
	WriteFrontMatter(identifier PageIdentifier, fm FrontMatter) error
	WriteMarkdown(identifier PageIdentifier, md Markdown) error
}

// PageReadWriter is an interface that combines PageReader and PageWriter.
type PageReadWriter interface {
	PageReader
	PageWriter
}
