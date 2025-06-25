package common

type (
	PageIdentifier = string
	Markdown       = string
	FrontMatter    = map[string]any
)

type PageReader interface {
	ReadFrontMatter(requestedIdentifier PageIdentifier) (PageIdentifier, FrontMatter, error)
	ReadMarkdown(requestedIdentifier PageIdentifier) (PageIdentifier, Markdown, error)
}

type PageWriter interface {
	WriteFrontMatter(identifier PageIdentifier, fm FrontMatter) error
	WriteMarkdown(identifier PageIdentifier, md Markdown) error
}

type PageReadWriter interface {
	PageReader
	PageWriter
}
