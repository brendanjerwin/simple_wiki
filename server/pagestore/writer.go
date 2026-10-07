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
	// The identity parameter is used for history attribution.
	WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter, identity wikipage.Identity) error

	// WriteMarkdown atomically reads the current frontmatter and writes
	// back the frontmatter combined with md under the page's lock.
	// The identity parameter is used for history attribution.
	WriteMarkdown(id wikipage.PageIdentifier, md wikipage.Markdown, identity wikipage.Identity) error

	// ModifyMarkdown atomically reads the markdown section, calls fn,
	// and writes the result back while preserving the existing frontmatter.
	// The full read-modify-write is held under the page's lock.
	// The identity parameter is used for history attribution.
	ModifyMarkdown(id wikipage.PageIdentifier, fn func(wikipage.Markdown) (wikipage.Markdown, error), identity wikipage.Identity) error

	// SoftDeletePage moves the page's .md file to trash.
	// Returns os.ErrNotExist if the file did not exist.
	// The identity parameter is used for history attribution.
	SoftDeletePage(id wikipage.PageIdentifier, identity wikipage.Identity) error
	SoftDeletePageBy(id wikipage.PageIdentifier, deletedBy string, identity wikipage.Identity) error
	ListTrash() ([]wikipage.TrashEntry, error)
	RestorePage(trashID string) error
	PurgePage(trashID string) error
	EmptyTrash() (int, error)
}
