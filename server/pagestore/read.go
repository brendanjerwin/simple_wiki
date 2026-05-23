package pagestore

import (
	"fmt"
	"os"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// mdExtension is the on-disk file extension for wiki pages.
const mdExtension = "md"

// ReadPage opens a page by its identifier. Returns a non-nil *Page even
// when the file does not exist (WasLoadedFromDisk=false in that case).
//
// This is the PURE read path — no migration side effects, no save-on-read.
// The lazy-migration save-on-read that caused the production incident has
// been moved out of the storage layer; canonicalization is now a Phase 3
// decorator (CanonicalReader) that wraps a bare *Store.
func (s *Store) ReadPage(requestedIdentifier wikipage.PageIdentifier) (*wikipage.Page, error) {
	identifierStr := string(requestedIdentifier)
	p := new(wikipage.Page)
	p.Identifier = identifierStr
	p.Text = ""
	p.WasLoadedFromDisk = false

	identifier, mdBytes, err := s.readFileByIdentifier(identifierStr, mdExtension)
	if err != nil {
		// File not found — return empty page (normal for new pages).
		return p, nil
	}

	p.Identifier = identifier

	mungedPath, originalPath, _ := s.getFilePaths(identifier, mdExtension)
	if stat, statErr := os.Stat(mungedPath); statErr == nil {
		p.ModTime = stat.ModTime()
	} else if stat, statErr := os.Stat(originalPath); statErr == nil {
		p.ModTime = stat.ModTime()
	}
	// Both stat attempts may fail (race with delete), but ModTime is non-
	// critical for page loading. Zero time is acceptable.

	p.Text = string(mdBytes)
	p.WasLoadedFromDisk = true
	return p, nil
}

// ReadFrontMatter reads the frontmatter for a page. Returns the actual
// identifier the file was found under (munged or raw), the parsed
// frontmatter, and any error. os.ErrNotExist is returned when the page
// does not exist; other errors propagate as-is.
func (s *Store) ReadFrontMatter(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	page, err := s.ReadPage(identifier)
	if err != nil {
		return identifier, nil, fmt.Errorf("failed to open page %s: %w", identifier, err)
	}
	if page.IsNew() {
		return identifier, nil, os.ErrNotExist
	}
	fm, err := page.GetFrontMatter()
	if err != nil {
		return wikipage.PageIdentifier(page.Identifier), nil, fmt.Errorf("failed to parse frontmatter for %s: %w", page.Identifier, err)
	}
	return wikipage.PageIdentifier(page.Identifier), fm, nil
}
