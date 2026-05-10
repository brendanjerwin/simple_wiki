// "Last-observed" wiki state translation. The sync layer records the
// wiki content as observed at the END of each tick so the NEXT tick
// can detect "user re-edited locally since the last push attempt" —
// when current wiki_fp differs from this triple, the connector resets
// PushFailureCount to 0 (the obvious user-fix path after a dead-letter).
//
// The fingerprint computation itself is pure — the orchestrator side
// just walks the resulting map and writes the values into its
// per-item state. Captured at end-of-tick (not start) so intra-tick
// wiki edits aren't missed.

package translator

import (
	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
)

// LastObservedWikiFingerprints returns the wiki fingerprint for each
// currently-paired item, keyed by wiki UID.
//
// Inputs:
//   - pairedUIDs: the set of wiki UIDs the binding currently knows
//     about (typically the keys of binding.ItemIDMap). Only items in
//     this set are processed; unpaired wiki items are ignored — they
//     have no per-item state to stamp.
//   - checklist: the wiki checklist whose items will be fingerprinted.
//
// Output: map[uid]Fingerprint. UIDs not present in the checklist are
// omitted (a paired item that's been deleted wiki-side leaves no
// fingerprint to stamp; the caller's id_map cleanup handles that
// separately on inbound apply).
//
// Pure: no I/O, no clock; just walks the checklist and computes
// fingerprints via FingerprintWiki.
func LastObservedWikiFingerprints(pairedUIDs map[string]struct{}, checklist *apiv1.Checklist) map[string]Fingerprint {
	if checklist == nil {
		return map[string]Fingerprint{}
	}
	wikiByUID := make(map[string]*apiv1.ChecklistItem, len(checklist.GetItems()))
	for _, it := range checklist.GetItems() {
		if it.GetUid() == "" {
			continue
		}
		wikiByUID[it.GetUid()] = it
	}
	out := make(map[string]Fingerprint, len(pairedUIDs))
	for uid := range pairedUIDs {
		item, ok := wikiByUID[uid]
		if !ok {
			continue
		}
		out[uid] = FingerprintWiki(item)
	}
	return out
}
