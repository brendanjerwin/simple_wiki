package common

type PageIdentifier = string
type Markdown = string
type FrontMatter = map[string]interface{}

type IReadPages interface {
	ReadFrontMatter(requested_identifier string) (PageIdentifier, FrontMatter, error)
	ReadMarkdown(requested_identifier string) (PageIdentifier, Markdown, error)
}
