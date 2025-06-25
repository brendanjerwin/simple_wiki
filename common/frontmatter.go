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
