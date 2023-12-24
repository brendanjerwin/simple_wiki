package common

type FrontMatter = map[string]interface{}
type IReadPages interface {
	ReadFrontMatter(requested_identifier string) (string, FrontMatter, error)
	ReadMarkdown(requested_identifier string) (string, string, error)
}
