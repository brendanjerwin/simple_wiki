// Package engine owns the connector-agnostic merge rule that consumes
// the per-checklist operation log defined by ADR-0015.
//
// The merge rule replaces the value-fingerprint baselines that lived
// on each subscription (Keep's Synced{Text,Checked,SortValue},
// Tasks's SyncedItems map). Per-binding state shrinks to a single
// LastSyncedSeq cursor; per-checklist state grows by one event row
// per mutation. The engine reads events with seq > LastSyncedSeq and
// classifies divergence causally — by who wrote, not by current value.
//
// This file is the minimum-viable surface (15d in the Phase 15 plan):
// the divergence classifier that Keep and Tasks call before applying
// an inbound update. The full engine extraction (Sync owning the
// algorithm; adapters owning only primitives) lands as a follow-up
// once both backends are using Classify.
package engine

import (
	"strings"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
)

// BindingCursor identifies a binding's position in a checklist's
// op-log. Each successful round-trip advances LastSyncedSeq to the
// max(seq) consumed; subsequent ticks scan forward from there.
type BindingCursor struct {
	Page          string
	ListName      string
	LastSyncedSeq int64
}

// EventClassification is the engine's per-item ruling on whether the
// wiki has diverged from the connector's last-synced baseline.
//
// WikiDiverged is true when AT LEAST ONE event with seq > cursor and
// src ≠ "connector:<myKind>:*" exists for this uid. That event is
// either:
//
//   - a user edit (src=user:…)
//   - a different connector's apply (src=connector:<other>:…)
//   - a migration backfill (src=migration:…)
//
// In any of those cases, the wiki holds local state the connector
// has not yet pushed. Applying a remote update would clobber that
// pending change. The connector's inbound apply path MUST skip the
// apply for items whose classification reports WikiDiverged.
//
// Self-writes (src=connector:<myKind>:*) do NOT count as divergence:
// they are this connector's own previous applies, returned by the
// cursor-safety-buffer's idempotent re-fetch, and re-applying them
// would clobber any subsequent user edit. The engine treats them as
// "already applied; nothing new on the wiki side."
type EventClassification struct {
	UID                string
	WikiDiverged       bool
	LatestEventSeq     int64
	LatestEventSource  string // diagnostic; the src string of the deciding event.
}

// SourcePrefixForKind builds the prefix `connector:<kind>:` used to
// identify a connector's own writes in event logs. Adapters pass
// their kind here when calling Classify.
func SourcePrefixForKind(kind string) string {
	return "connector:" + kind + ":"
}

// isDivergentSource reports whether a single event's source string
// represents a divergence relative to the calling connector. Only
// user edits and OTHER connectors' applies count as divergence.
//
// Self-writes (this connector's own apply / push_recovery) and the
// migration:initial_baseline events are NOT divergence — they are
// the synced baseline (or our own previous applies returned by the
// cursor-safety-buffer's idempotent re-fetch). Per ADR-0015's 4-cell
// table.
//
// Unknown sources fall through as divergent — fail-safe ("treat as
// possibly user-edited") rather than silently losing a pending edit.
func isDivergentSource(src, selfPrefix string) bool {
	switch {
	case strings.HasPrefix(src, selfPrefix):
		return false
	case strings.HasPrefix(src, "migration:"):
		return false
	case strings.HasPrefix(src, "user:"):
		return true
	case strings.HasPrefix(src, "connector:"):
		// Different connector than this one — divergence.
		return true
	default:
		// Unknown source (system: or future additions): treat as
		// divergent. Conservative classification preserves user edits
		// at the cost of the occasional unnecessary deferred apply.
		return true
	}
}

// Classify walks the checklist's events with seq > cursor.LastSyncedSeq
// and computes per-uid divergence relative to the calling connector
// (myKind). Returns a map keyed by uid; uids with no events since the
// cursor are absent from the result (nothing to decide).
//
// The classifier is pure: it reads the checklist + cursor, returns a
// map. No I/O. No mutation. Adapters call it once per tick, before
// the inbound apply loop, and consult the result per remote item.
func Classify(checklist *apiv1.Checklist, cursor BindingCursor, myKind string) map[string]EventClassification {
	out := map[string]EventClassification{}
	if checklist == nil {
		return out
	}
	selfPrefix := SourcePrefixForKind(myKind)
	for _, ev := range checklist.GetEvents() {
		if ev == nil || ev.GetUid() == "" {
			continue
		}
		if ev.GetSeq() <= cursor.LastSyncedSeq {
			continue
		}
		uid := ev.GetUid()
		divergent := isDivergentSource(ev.GetSrc(), selfPrefix)
		prev, exists := out[uid]
		// Divergence is sticky: once any divergent event lands since
		// the cursor, subsequent self-writes don't clear the bit.
		newDiverged := prev.WikiDiverged || divergent
		latestSeq := prev.LatestEventSeq
		latestSrc := prev.LatestEventSource
		if !exists || ev.GetSeq() > latestSeq {
			latestSeq = ev.GetSeq()
			latestSrc = ev.GetSrc()
		}
		out[uid] = EventClassification{
			UID:               uid,
			WikiDiverged:      newDiverged,
			LatestEventSeq:    latestSeq,
			LatestEventSource: latestSrc,
		}
	}
	return out
}
