package checklistmutator

import (
	"context"

	"github.com/brendanjerwin/simple_wiki/tailscale"
)

// syncIdentityFor builds the per-call sync identity used when applying
// inbound Keep changes. The change originated from a human action — the
// binding owner toggling/editing the item in Keep — even though the
// cron tick is what actually replays it on the wiki side. So we treat
// the apply as the user's own action: NewIdentity (not NewAgentIdentity),
// so the wiki's checklist UI surfaces "Done by alice@gmail.com · Nm ago"
// rather than collapsing to "Done by an agent". The cron is transport,
// not actor.
//
// Empty ownerEmail falls back to a system loginName so the call still
// has a stable string for downstream rendering. Production should never
// hit that path because every binding requires a connected account.
func syncIdentityFor(ownerEmail string) tailscale.IdentityValue {
	if ownerEmail == "" {
		return tailscale.NewIdentity("system:keep-sync", "Keep Sync (system)", "keep-sync")
	}
	return tailscale.NewIdentity(ownerEmail, ownerEmail, "keep-sync")
}

// AddItemForSync is the Keep-sync entry point for "add this Keep
// item to the wiki." Attributes the add to the binding owner's email
// via ownerEmail (see syncIdentityFor).
//
// SortValueHint is forwarded as args.SortOrder when non-zero —
// preserves Keep's relative ordering when seeding fresh items.
func (m *Mutator) AddItemForSync(ctx context.Context, page, listName, ownerEmail, text string, checked bool, tags []string, description, sortValueHint string) (string, error) {
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
	identity := syncIdentityFor(ownerEmail)
	item, _, err := m.AddItem(ctx, page, listName, args, identity)
	if err != nil {
		return "", err
	}
	if checked {
		// Toggle to set checked=true; the toggle path stamps
		// completed_at correctly.
		if _, _, err := m.ToggleItem(ctx, page, listName, item.GetUid(), nil, identity); err != nil {
			return item.GetUid(), err
		}
	}
	return item.GetUid(), nil
}

// UpdateItemForSync is the Keep-sync entry point for "Keep changed
// this item; mirror it on the wiki side." Replaces text/tags/
// description and reconciles checked. Attribution goes to ownerEmail.
func (m *Mutator) UpdateItemForSync(ctx context.Context, page, listName, ownerEmail, uid, text string, checked bool, tags []string, description string) error {
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
	identity := syncIdentityFor(ownerEmail)
	current, _, err := m.UpdateItem(ctx, page, listName, uid, args, nil, identity)
	if err != nil {
		return err
	}
	if current.GetChecked() != checked {
		if _, _, err := m.ToggleItem(ctx, page, listName, uid, nil, identity); err != nil {
			return err
		}
	}
	return nil
}

// DeleteItemForSync is the Keep-sync entry point for "Keep trashed
// this item; remove it from the wiki side too." Attribution (for
// audit logging) goes to ownerEmail.
func (m *Mutator) DeleteItemForSync(ctx context.Context, page, listName, ownerEmail, uid string) error {
	_, err := m.DeleteItem(ctx, page, listName, uid, nil, syncIdentityFor(ownerEmail))
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
