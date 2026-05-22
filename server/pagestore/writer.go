package pagestore

import "github.com/brendanjerwin/simple_wiki/wikipage"

// Writer is the write-only surface of the page store. Disjoint from Reader
// by design: the read-only Reader interface has no Write* methods, so a
// consumer holding only a Reader cannot accidentally save during a read.
//
// Side effects beyond bytes-on-disk (indexing, agent-schedule cron
// registration, search reindexing) are NOT the Writer's responsibility —
// those belong to the caller. Writer is the storage primitive.
type Writer interface {
	// WriteFrontMatter atomically reads the current markdown for the page
	// and writes back the markdown combined with fm under the page's lock.
	WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error

	// WriteMarkdown atomically reads the current frontmatter and writes
	// back the frontmatter combined with md under the page's lock.
	WriteMarkdown(id wikipage.PageIdentifier, md wikipage.Markdown) error

	// ModifyMarkdown atomically reads the markdown section, calls fn,
	// and writes the result back while preserving the existing frontmatter.
	// The full read-modify-write is held under the page's lock.
	ModifyMarkdown(id wikipage.PageIdentifier, fn func(wikipage.Markdown) (wikipage.Markdown, error)) error

	// SoftDeletePage moves the page's .md file to __deleted__/<timestamp>/.
	// Returns os.ErrNotExist if the file did not exist.
	SoftDeletePage(id wikipage.PageIdentifier) error
}
