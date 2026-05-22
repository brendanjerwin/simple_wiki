// Package pagestore is the disk-backed storage primitive for wiki pages.
// *Store owns: raw byte reads, raw byte writes, per-page locks, and the
// on-disk file layout (base32-encoded identifiers, munged path resolution,
// soft-delete via __deleted__/ rename). It owns nothing else — no
// indexing, no migration, no rendering, no caching. Decorators
// (CanonicalizingWriter, CanonicalReader) in this package compose policy
// onto the storage primitive without entangling it.
//
// The Reader / Writer interface split (reader.go, writer.go) is the
// load-bearing structural change: consumers that should only read depend
// on Reader; the type system then refuses to let them call Write*.
package pagestore

import (
	"strings"

	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
)

// CanonicalLockKey returns the canonical lock key for a page identifier.
// All callers of Store.lockPage funnel through this so the same logical
// page resolves to the same *sync.Mutex regardless of id spelling (raw
// vs. munged, mixed case). Without this, a user write and an eager
// backfill write to the same page would key on different mutex values
// and a torn-write window would open.
//
// MungeIdentifier is documented as idempotent and produces a URL-safe
// form; we additionally lowercase to match the on-disk path computation
// in getFilePathsForIdentifier (which lowercases before base32-encoding).
//
// If MungeIdentifier returns an error (malformed input), we fall back to
// the raw lowercased identifier. The fallback is safe because every
// caller passing the same malformed input will resolve to the same key
// — drift only happens across spellings of the same page, not across
// error paths.
//
// Exported so eager-backfill jobs and other in-package consumers can
// reason about lock keys when needed; the helper is the only sanctioned
// way to derive one.
func CanonicalLockKey(id string) string {
	munged, err := wikiidentifiers.MungeIdentifier(id)
	if err != nil {
		return strings.ToLower(id)
	}
	return strings.ToLower(munged)
}
