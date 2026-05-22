package pagestore

import (
	"fmt"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// CanonicalReader wraps a Reader and canonicalizes returned page bytes
// in-memory. The on-disk file is never modified by this decorator — the
// transform is a pure read-side projection. Compare with
// CanonicalizingWriter (write-side).
//
// Composition: typical wiring is `CanonicalReader{canonicalizer, *Store}`,
// so readers always see canonical bytes regardless of disk state. Once
// Phase 4's eager backfill catches up, on-disk and in-memory representations
// converge; in the interim window the decorator bridges the gap.
type CanonicalReader struct {
	canonicalizer FrontmatterCanonicalizer
	inner         Reader
}

// NewCanonicalReader composes a Reader with a canonicalizer. If
// canonicalizer is nil, NoopCanonicalizer is used so the wiring is always
// safe to construct without optionality at the call site.
func NewCanonicalReader(canonicalizer FrontmatterCanonicalizer, inner Reader) *CanonicalReader {
	if canonicalizer == nil {
		canonicalizer = NoopCanonicalizer{}
	}
	return &CanonicalReader{canonicalizer: canonicalizer, inner: inner}
}

// ReadPage delegates to the inner Reader, then runs the canonicalizer over
// the returned bytes. The on-disk file is never mutated — the canonical
// bytes only flow back to the caller.
func (r *CanonicalReader) ReadPage(id wikipage.PageIdentifier) (*wikipage.Page, error) {
	p, err := r.inner.ReadPage(id)
	if err != nil {
		return nil, err
	}
	if p == nil || !p.WasLoadedFromDisk {
		// New / nonexistent page — nothing to canonicalize.
		return p, nil
	}
	canonical, cErr := r.canonicalizer.Canonicalize([]byte(p.Text))
	if cErr != nil {
		return nil, fmt.Errorf("canonicalize page %s: %w", p.Identifier, cErr)
	}
	p.Text = string(canonical)
	return p, nil
}

// ReadFrontMatter delegates to the inner Reader through ReadPage so the
// canonicalizer runs on the bytes before the frontmatter is parsed.
func (r *CanonicalReader) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	p, err := r.ReadPage(id)
	if err != nil {
		return id, nil, err
	}
	if p.IsNew() {
		// File didn't exist; the contract is to return os.ErrNotExist —
		// match *Store.ReadFrontMatter's shape exactly.
		return r.inner.ReadFrontMatter(id)
	}
	fm, fmErr := p.GetFrontMatter()
	if fmErr != nil {
		return wikipage.PageIdentifier(p.Identifier), nil, fmt.Errorf("parse frontmatter for %s: %w", p.Identifier, fmErr)
	}
	return wikipage.PageIdentifier(p.Identifier), fm, nil
}
