package wikipage

import (
	"errors"
	"time"
)

// ErrPageRestoreConflict means a trashed page cannot be restored because a
// live page already exists at the target identifier.
var ErrPageRestoreConflict = errors.New("page restore conflict")

// PageIdentifier is the unique identifier for a page.
type PageIdentifier string

// String returns the string representation of the PageIdentifier.
func (p PageIdentifier) String() string { return string(p) }

// Markdown is the content of a page in Markdown format.
type Markdown string

// String returns the string representation of the Markdown.
func (m Markdown) String() string { return string(m) }

// FrontMatter is the frontmatter of a page.
type FrontMatter map[string]any

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

// PageDeleter is an interface for deleting pages.
type PageDeleter interface {
	DeletePage(identifier PageIdentifier) error
}

// PageTrashDeleter records the actor responsible for moving a page to trash.
type PageTrashDeleter interface {
	DeletePageBy(identifier PageIdentifier, deletedBy string) error
}

// TrashEntry describes a page that has been moved out of normal wiki storage
// and can still be restored or purged.
type TrashEntry struct {
	TrashID    string
	Identifier PageIdentifier
	Title      string
	DeletedAt  time.Time
	DeletedBy  string
	PurgesAt   time.Time
}

// PageTrashReader lists pages currently held in trash.
type PageTrashReader interface {
	ListTrash() ([]TrashEntry, error)
}

// PageTrashRestorer restores a trashed page to normal wiki storage.
type PageTrashRestorer interface {
	RestorePage(trashID string) error
}

// PageTrashPurger permanently removes trashed pages.
type PageTrashPurger interface {
	PurgePage(trashID string) error
	EmptyTrash() (int, error)
}

// PageOpener is an interface for opening pages as full Page objects.
type PageOpener interface {
	ReadPage(identifier PageIdentifier) (*Page, error)
}

// PageModifier provides atomic read-modify-write semantics for page content.
// Implementations must hold a write lock for the duration of the modifier call
// to prevent TOCTOU races between concurrent writers.
type PageModifier interface {
	// ModifyMarkdown atomically reads the markdown section, calls modifier with it,
	// and writes the result back (preserving the existing frontmatter).
	// The entire read-modify-write cycle is held under a write lock.
	// If modifier returns an error, the page is not written.
	ModifyMarkdown(identifier PageIdentifier, modifier func(Markdown) (Markdown, error)) error
}

// PageReaderMutator is an interface that combines PageReader, PageWriter, PageDeleter, and PageModifier.
type PageReaderMutator interface {
	PageReader
	PageWriter
	PageDeleter
	PageModifier
}
