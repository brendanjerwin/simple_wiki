// Package canonicalize provides the format canonicalizer that rewrites
// wiki page frontmatter to its canonical on-disk form. Wraps the existing
// `migrations/lazy` applicator so the canonicalization logic stays single-
// source-of-truth while the structural refactor proceeds.
//
// Phase 4 wiring: Site's `pagestore.Store` and `CanonicalReader` are
// constructed with `NewFormatCanonicalizer()` instead of `NoopCanonicalizer`,
// flipping every read and write through the canonicalization chain.
//
// Phase 6 (future): consolidate the lazy/ implementation under this package
// (or vice versa) and delete the redundant layer.
package canonicalize

import (
	"github.com/brendanjerwin/simple_wiki/migrations/lazy"
)

// FormatCanonicalizer adapts the lazy migration applicator to the
// pagestore.FrontmatterCanonicalizer interface. The applicator's
// `ApplyMigrations(content) -> (content, error)` shape is exactly what
// canonicalization expects.
type FormatCanonicalizer struct {
	applicator lazy.FrontmatterMigrationApplicator
}

// NewFormatCanonicalizer constructs the canonicalizer with the default
// migration set (YAML→TOML, dot-notation, identifier munging, inventory-
// container munging, table spacing — in that order).
func NewFormatCanonicalizer() *FormatCanonicalizer {
	return &FormatCanonicalizer{applicator: lazy.NewApplicator()}
}

// Canonicalize applies the migration chain over content and returns the
// canonical result. Idempotent on canonical input (each migration's
// `AppliesTo` returns false on its own `Apply` output).
func (c *FormatCanonicalizer) Canonicalize(content []byte) ([]byte, error) {
	return c.applicator.ApplyMigrations(content)
}
