package checklistmutator

import (
	"context"

	"github.com/brendanjerwin/simple_wiki/tailscale"
)

// syncIdentityFor builds the per-call sync identity used when applying
// inbound connector changes. The change originated from a human
// action — the subscription owner toggling/editing the item in the
// remote (Keep, Tasks, …) — even though the cron tick is what
// actually replays it on the wiki side. So we treat the apply as the
// user's own action: NewIdentity (not NewAgentIdentity), so the
// wiki's checklist UI surfaces "Done by alice@gmail.com · Nm ago"
// rather than collapsing to "Done by an agent." The cron is
// transport, not actor.
//
// Empty ownerEmail falls back to a generic "system:connector-sync"
// loginName. The previous fallback was "system:keep-sync", which
// misattributed Tasks-side writes to Keep when the Tasks connector's
// state was missing the email field — masking the real offender
// during cross-connector debugging. Production should never hit the
// empty-ownerEmail path because every Subscribe requires a connected
// account; the fallback exists only to keep the function total.
func syncIdentityFor(ownerEmail string) tailscale.IdentityValue {
	if ownerEmail == "" {
		return tailscale.NewIdentity("system:connector-sync", "Connector Sync (system)", "connector-sync")
	}
	return tailscale.NewIdentity(ownerEmail, ownerEmail, "connector-sync")
}

// resolveSyncSource returns the explicit Source override the caller
// threaded via WithSource(ctx, …), falling back to a generic
// connector-apply attribution when the caller didn't override. Per
// ADR-0015: connector packages should always set the override so the
// engine's causal merge rule sees the right kind. The fallback
// preserves correctness (the event is at least tagged "not user")
// when a legacy caller hasn't been updated yet.
func resolveSyncSource(ctx context.Context) Source {
	if s, ok := sourceFromContext(ctx); ok {
		return s
	}
	return ConnectorSource("unknown", "apply")
}

// AddItemForSync is the connector-sync entry point for "add this
// remote item to the wiki." Attributes the add to the binding owner's
// email via ownerEmail (used for completed_by). The event-log entry's
// src is taken from WithSource(ctx, …) — set at the top of the
// connector's inbound apply pass.
//
// SortValueHint is forwarded as args.SortOrder when non-zero —
// preserves the remote's relative ordering when seeding fresh items.
//
// `checked` is a value, not a control flag — it's the remote's
// reported checkbox state for the new item.
//
//revive:disable-next-line:flag-parameter
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
	source := resolveSyncSource(ctx)
	item, _, err := m.addItemImpl(ctx, page, listName, args, identity, source)
	if err != nil {
		return "", err
	}
	if checked {
		// Toggle to set checked=true; the toggle path stamps
		// completed_at correctly.
		if _, _, err := m.toggleItemImpl(ctx, page, listName, item.GetUid(), nil, identity, source); err != nil {
			return item.GetUid(), err
		}
	}
	return item.GetUid(), nil
}

// UpdateItemForSync is the connector-sync entry point for "remote
// changed this item; mirror it on the wiki side." Replaces text/tags/
// description and reconciles checked. completed_by goes to ownerEmail;
// the event-log entry's src is taken from WithSource(ctx, …).
//
// `checked` is a value, not a control flag — it's the new desired
// checkbox state from the remote. The connector passes whatever the
// remote reports (checked or unchecked) and this function applies it.
//
//revive:disable-next-line:flag-parameter
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
	source := resolveSyncSource(ctx)
	current, _, err := m.updateItemImpl(ctx, page, listName, uid, args, nil, identity, source)
	if err != nil {
		return err
	}
	if current.GetChecked() != checked {
		if _, _, err := m.toggleItemImpl(ctx, page, listName, uid, nil, identity, source); err != nil {
			return err
		}
	}
	return nil
}

// DeleteItemForSync is the connector-sync entry point for "remote
// trashed this item; remove it from the wiki side too." Attribution
// (for audit logging) goes to ownerEmail; event-log src is taken from
// WithSource(ctx, …).
func (m *Mutator) DeleteItemForSync(ctx context.Context, page, listName, ownerEmail, uid string) error {
	source := resolveSyncSource(ctx)
	_, err := m.deleteItemImpl(ctx, page, listName, uid, nil, syncIdentityFor(ownerEmail), source)
	return err
}

// decimalRadix is the multiplicative base used by parseSortHint to
// shift accumulated digits left by one position.
const decimalRadix = 10

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
		n = n*decimalRadix + int64(r-'0')
	}
	return n
}
