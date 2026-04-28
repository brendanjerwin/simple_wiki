package checklistmutator

import (
	"context"

	"github.com/brendanjerwin/simple_wiki/tailscale"
)

// SyncIdentity is the synthetic identity used by the Keep sync engine
// when applying inbound changes. Built via NewAgentIdentity so it
// satisfies tailscale.IdentityValue without us reimplementing every
// method. Flagged IsAgent so wiki-managed completed_by/automated
// fields render with the agent suffix. We DON'T notify on these calls
// — the SyncDebouncer is suppressed for the duration of inbound apply
// — so the sync agent identity never reaches
// Subscriber.OnChecklistMutated.
var SyncIdentity tailscale.IdentityValue = tailscale.NewAgentIdentity(
	"system:keep-sync",
	"Keep Sync (system)",
	"keep-sync",
)

// AddItemForSync is the Keep-sync entry point for "add this Keep
// item to the wiki." Calls AddItem with the synthetic identity, then
// returns the new wiki uid so the caller can update its id_map.
//
// SortValueHint is forwarded as args.SortOrder when non-zero —
// preserves Keep's relative ordering when seeding fresh items.
func (m *Mutator) AddItemForSync(ctx context.Context, page, listName, text string, checked bool, tags []string, description, sortValueHint string) (string, error) {
	args := AddItemArgs{
		Text: text,
		Tags: tags,
	}
	if description != "" {
		d := description
		args.Description = &d
	}
	if sortValueHint != "" {
		// Best-effort numeric parse — caller passes a Keep SortValue
		// string; treat unparseable as "no hint" rather than failing.
		if n := parseSortHint(sortValueHint); n != 0 {
			args.SortOrder = &n
		}
	}
	item, _, err := m.AddItem(ctx, page, listName, args, SyncIdentity)
	if err != nil {
		return "", err
	}
	if checked {
		// Toggle to set checked=true; the toggle path stamps
		// completed_at correctly.
		if _, _, err := m.ToggleItem(ctx, page, listName, item.GetUid(), nil, SyncIdentity); err != nil {
			return item.GetUid(), err
		}
	}
	return item.GetUid(), nil
}

// UpdateItemForSync is the Keep-sync entry point for "Keep changed
// this item; mirror it on the wiki side." Replaces text/tags/
// description and reconciles checked.
func (m *Mutator) UpdateItemForSync(ctx context.Context, page, listName, uid, text string, checked bool, tags []string, description string) error {
	args := UpdateItemArgs{
		Text:           &text,
		Tags:           tags,
		TagsSet:        true,
		DescriptionSet: true,
	}
	if description != "" {
		d := description
		args.Description = &d
	}
	current, _, err := m.UpdateItem(ctx, page, listName, uid, args, nil, SyncIdentity)
	if err != nil {
		return err
	}
	if current.GetChecked() != checked {
		if _, _, err := m.ToggleItem(ctx, page, listName, uid, nil, SyncIdentity); err != nil {
			return err
		}
	}
	return nil
}

// DeleteItemForSync is the Keep-sync entry point for "Keep trashed
// this item; remove it from the wiki side too."
func (m *Mutator) DeleteItemForSync(ctx context.Context, page, listName, uid string) error {
	_, err := m.DeleteItem(ctx, page, listName, uid, nil, SyncIdentity)
	return err
}

// parseSortHint parses Keep's SortValue (numeric string, sometimes
// float-style). Returns 0 on any parse failure — caller treats 0 as
// "no hint, append to end."
func parseSortHint(s string) int64 {
	if s == "" {
		return 0
	}
	// Tiny inline parser; avoids an extra strconv import here while
	// the parsing is shared with bridge/mapping.go's parseSortValue.
	var n int64
	for i, r := range s {
		if i == 0 && r == '-' {
			continue
		}
		if r < '0' || r > '9' {
			return 0 // float / non-numeric: skip the hint
		}
	}
	for _, r := range s {
		if r == '-' {
			n = -n
			continue
		}
		n = n*10 + int64(r-'0')
	}
	return n
}
