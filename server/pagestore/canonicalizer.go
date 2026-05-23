package pagestore

// FrontmatterCanonicalizer rewrites frontmatter to its canonical on-disk
// form. The pure interface — no I/O — so the same implementation can be
// composed onto both the read path (in-memory transform via CanonicalReader)
// and the write path (Store.writeRawTextLocked runs the canonicalizer
// before persisting; see Store.SetCanonicalizer).
//
// The migrations/canonicalize package provides the production implementation
// (FormatCanonicalizer); tests typically use NoopCanonicalizer or a small
// fake.
type FrontmatterCanonicalizer interface {
	Canonicalize(content []byte) ([]byte, error)
}

// NoopCanonicalizer returns input unchanged. Used as the default in Store
// (until SetCanonicalizer swaps in a real canonicalizer) and as a stand-in
// in tests that don't exercise canonicalization.
type NoopCanonicalizer struct{}

// Canonicalize returns the input unchanged.
func (NoopCanonicalizer) Canonicalize(content []byte) ([]byte, error) {
	return content, nil
}
