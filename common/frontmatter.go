package common

type FrontMatter = map[string]interface{}
type IReadPages interface {
	ReadFrontMatter(identifier string) (FrontMatter, error)
	ReadMarkdown(identifier string) (string, error)
}
