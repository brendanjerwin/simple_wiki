package pagestore

import "github.com/brendanjerwin/simple_wiki/wikipage"

// Reader is the read-only surface of the page store. Consumers that only
// need to read pages should depend on this interface, not on *Store or on
// Site — the compiler will then refuse to let them call any Write* method
// (because Reader has none).
//
// This split exists because the production incident this package was
// extracted to fix was a Read* method calling a Write* method (
// `Site.ReadPage` → `applyMigrationsForPage` → `savePage` under a global
// lock). Making the asymmetry a compile error rather than a lint rule is
// the structural barrier that prevents the bug class from returning.
type Reader interface {
	// ReadPage opens a page by its identifier. Returns a non-nil *Page
	// even when the file does not exist (with WasLoadedFromDisk=false).
	ReadPage(id wikipage.PageIdentifier) (*wikipage.Page, error)

	// ReadFrontMatter reads the frontmatter for a page. Returns the
	// actual identifier the file was found under (munged or raw),
	// the parsed frontmatter, and any error. os.ErrNotExist is returned
	// when the page does not exist.
	ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error)
}
