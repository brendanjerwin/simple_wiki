// Package etag derives the ETag, CTag, and sync-token values the CalDAV
// bridge serves on per-item and per-collection responses.
//
// Per-item ETag: weak, derived from item.UpdatedAt formatted as
// RFC3339Nano. Weak (W/"...") because we hash a logical timestamp, not
// a byte-exact body — clients that re-serialize before comparing
// (Apple Reminders has been observed doing this) accept weak ETags
// without the strong-ETag byte-equality requirement.
//
// Collection CTag: a non-standard but widely-honored extension (Apple
// CalDAV-CTag draft) that lets clients skip a full PROPFIND when the
// CTag has not changed. Derived from checklist.UpdatedAt.
//
// Collection sync-token (RFC 6578): URI form of checklist.SyncToken.
// The wiki uses a stable URN-shaped prefix so sync tokens are
// recognizable as ours and round-trippable through ParseSyncToken.
package etag

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
)

// syncURIPrefix is the URI prefix on every emitted sync-token. RFC
// 6578 mandates the token be a URI; we use a wiki-specific URN-shaped
// prefix so the suffix is unambiguously ours. Kept unexported because
// callers should round-trip through CollectionSyncToken / ParseSyncToken
// rather than building or parsing tokens by hand.
const syncURIPrefix = "http://simple-wiki.local/ns/sync/"

// syncTokenBase / syncTokenBitSize are the numeric base and bit width
// of the integer suffix on a sync-token URI (decimal int64).
const (
	syncTokenBase    = 10
	syncTokenBitSize = 64
)

// ItemETag returns the per-item ETag a CalDAV client should compare on
// If-Match preconditions. Returns the empty string for nil items so
// callers can branch on "no item" without panicking.
func ItemETag(item *apiv1.ChecklistItem) string {
	if item == nil || item.UpdatedAt == nil {
		return ""
	}
	return fmt.Sprintf("W/%q", item.UpdatedAt.AsTime().Format(time.RFC3339Nano))
}

// CollectionCTag returns the collection-level CTag value clients use
// to skip full PROPFINDs. Derived from the checklist's collection-
// level updated_at, which the mutator bumps on every successful
// mutation across any item in the list.
func CollectionCTag(checklist *apiv1.Checklist) string {
	if checklist == nil || checklist.UpdatedAt == nil {
		return ""
	}
	return strconv.Quote(checklist.UpdatedAt.AsTime().Format(time.RFC3339Nano))
}

// CollectionSyncToken returns the RFC 6578 sync-token value for the
// checklist. The token is an opaque URI; clients pass it back on the
// next sync-collection REPORT so the server can return only changes
// since that point.
func CollectionSyncToken(checklist *apiv1.Checklist) string {
	if checklist == nil {
		return ""
	}
	return syncURIPrefix + strconv.FormatInt(checklist.SyncToken, syncTokenBase)
}

// ParseSyncToken extracts the integer counter from a sync-token URI
// previously emitted by CollectionSyncToken. Empty token (initial
// sync) returns 0 with no error. Malformed tokens return an error so
// the report handler can respond with a sync-collection 403/410 per
// RFC 6578 §3.2 to ask the client for a fresh full sync.
func ParseSyncToken(token string) (int64, error) {
	if token == "" {
		return 0, nil
	}
	if !strings.HasPrefix(token, syncURIPrefix) {
		return 0, fmt.Errorf("etag: sync-token %q does not have prefix %q", token, syncURIPrefix)
	}
	suffix := strings.TrimPrefix(token, syncURIPrefix)
	n, err := strconv.ParseInt(suffix, syncTokenBase, syncTokenBitSize)
	if err != nil {
		return 0, fmt.Errorf("etag: sync-token %q has non-integer suffix: %w", token, err)
	}
	return n, nil
}
