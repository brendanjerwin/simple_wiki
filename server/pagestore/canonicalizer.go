package pagestore

// FrontmatterCanonicalizer rewrites frontmatter to its canonical on-disk
// form. The pure interface — no I/O — so the same implementation can be
// composed onto both the read path (in-memory transform via CanonicalReader)
// and the write path (canonicalize-before-persist via CanonicalizingWriter).
//
// Phase 3 ships the interface plus a no-op implementation; Phase 4 adds
// the format-migration implementation under migrations/canonicalize/ and
// flips the wiring.
type FrontmatterCanonicalizer interface {
	Canonicalize(content []byte) ([]byte, error)
}

// NoopCanonicalizer returns input unchanged. The default wiring during
// Phase 3 — Site uses CanonicalReader(NoopCanonicalizer{}, store) and
// CanonicalizingWriter(NoopCanonicalizer{}, store) so the decorator
// machinery exists at runtime without changing observable behavior.
type NoopCanonicalizer struct{}

// Canonicalize returns the input unchanged.
func (NoopCanonicalizer) Canonicalize(content []byte) ([]byte, error) {
	return content, nil
}
