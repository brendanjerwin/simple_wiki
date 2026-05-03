// Label-side translations. The wiki side carries plain string tags
// (page-level frontmatter tags + inline #hashtag markers); Keep side
// carries label entities with stable MainIDs assigned to LIST nodes
// via labelIds. This file owns the named transformations between the
// two — the call site in sync stays a thin orchestrator.

package translator

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/gateway"
)

// MergeKeepLabels reconciles the host page's tags against the user's
// existing Keep labels. Returns:
//   - labelPush: Label CRUD entries for tags that don't have a Keep
//     label yet (one fresh MainID per missing tag).
//   - listLabelIDs: the set of Keep label MainIDs that should be
//     assigned to the bound LIST after the push.
//
// Lookup precedence:
//  1. persistedLabelIDs (the binding's persistent name → MainID map):
//     primary source. Survives incremental pulls that return no labels
//     at all, so the connector stops emitting fresh label CRUD entries
//     every tick for labels Keep already has.
//  2. existingLabels (from this pull's Labels slice): secondary source.
//     Picks up labels that Keep returned this tick but the binding hasn't
//     observed yet — typically the very first sync after Bind, or after
//     a forced full resync.
//
// Tombstoned labels in existingLabels (Deleted != zero) are skipped
// — we re-create rather than reviving.
//
// Pure: no I/O, no clock state held internally; "now" is a parameter
// so tests can pass deterministic timestamps and mint deterministic
// MainIDs (modulo the random suffix).
func MergeKeepLabels(tags []string, persistedLabelIDs map[string]string, existingLabels []gateway.LabelEntry, now time.Time) (labelPush []gateway.LabelEntry, listLabelIDs []string, err error) {
	if len(tags) == 0 {
		return nil, nil, nil
	}
	// Lookup is keyed by lowercased name so a wiki tag like `household`
	// resolves to a Keep label whose canonical name is "Household". The
	// PERSISTED map (binding.LabelIDs) preserves Keep's canonical case
	// — see the post-pull persistence loop in SyncToKeep — but the
	// LOOKUP normalizes both sides; otherwise every tick mints a fresh
	// MainID and emits a duplicate-label CRUD entry.
	byName := make(map[string]string, len(persistedLabelIDs)+len(existingLabels))
	// Primary: persisted FKs from prior pulls. Survives incremental
	// pulls that don't echo labels back.
	for name, mainID := range persistedLabelIDs {
		if mainID == "" {
			continue
		}
		byName[strings.ToLower(name)] = mainID
	}
	// Secondary: this pull's labels. Overlay so a label whose MainID
	// has changed (rare but possible — Keep can re-issue) gets
	// absorbed; the post-pull update in SyncToKeep will then write
	// the new value back into the persisted map.
	for _, l := range existingLabels {
		if !l.Deleted.IsZero() {
			continue
		}
		if l.MainID == "" {
			continue
		}
		byName[strings.ToLower(l.Name)] = l.MainID
	}
	listLabelIDs = make([]string, 0, len(tags))
	for _, tag := range tags {
		key := strings.ToLower(tag)
		if existingID, ok := byName[key]; ok {
			listLabelIDs = append(listLabelIDs, existingID)
			continue
		}
		newID, idErr := GenerateLabelMainID(now, len(labelPush))
		if idErr != nil {
			return nil, nil, idErr
		}
		labelPush = append(labelPush, gateway.LabelEntry{
			MainID:  newID,
			Name:    tag,
			Created: now,
			Updated: now,
		})
		listLabelIDs = append(listLabelIDs, newID)
		// Cache the just-created mapping so a duplicate tag in the
		// input list reuses the same MainID rather than creating
		// another (Keep tolerates duplicates but we'd rather not).
		byName[key] = newID
	}
	return labelPush, listLabelIDs, nil
}

// GenerateLabelMainID makes a Keep-style label MainID. Same shape as a
// Node ID ("ms-hex.16-hex"); gkeepapi node.py:1077-1085 (Label._gen_id).
// The idx parameter bumps the millisecond component so callers minting
// multiple labels in the same instant don't collide.
func GenerateLabelMainID(now time.Time, idx int) (string, error) {
	var entropy [8]byte
	if _, err := io.ReadFull(rand.Reader, entropy[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x.%016x", now.UnixMilli()+int64(idx), binary.BigEndian.Uint64(entropy[:])), nil
}
