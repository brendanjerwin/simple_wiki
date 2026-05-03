// Package sync owns the wiki-side of the Google Tasks bridge:
// per-user connector state on profile pages, sync orchestration,
// outbound debouncing, and persistence of subscriptions.
//
// Translation between Tasks task shapes and wiki ChecklistItems lives
// in the sibling translator package; the wire-protocol port (OAuth +
// REST) lives in the sibling gateway package. This package's only
// reason to depend on either is to compose the orchestration loop.
//
// Vocabulary follows plan §"Decisions locked" #4:
//
//   - Subscription = the persisted record on the user profile
//     (wiki.connectors.google_tasks.subscriptions[]).
//   - Lease = the in-memory exclusive-ownership claim, owned by
//     internal/connectors.LeaseTable.
//   - ItemMapping = per-item id-map entry (just a Tasks task id,
//     keyed by wiki uid).
package sync

import "time"

// SubscriptionState is the lifecycle state of a Subscription. Active
// subscriptions sync on every tick; paused ones return early from
// Sync (cursor frozen, item_id_map preserved). Resume on reconnect
// flips active.
type SubscriptionState string

const (
	// SubscriptionStateActive is the steady-state value: the
	// scheduler ticks Sync and changes flow both directions.
	SubscriptionStateActive SubscriptionState = "active"

	// SubscriptionStatePaused indicates auth_failed (or operator
	// action). Cursor is frozen, item_id_map preserved; reconnect
	// transitions back to active. See plan §"Pause / resume horizon".
	SubscriptionStatePaused SubscriptionState = "paused"
)

// PausedReasonAuthFailed is the canonical reason string written into
// PausedReason when invalid_grant retry-once exhausts. Surfaced to
// the UI's paused-badge tooltip.
const PausedReasonAuthFailed = "auth_failed"

// Subscription is one user's link from a wiki checklist (page +
// list_name) to a Google Tasks tasklist in their account. The
// authoritative copy lives on the user's profile page under
// wiki.connectors.google_tasks.subscriptions[].
//
// ItemIDMap maps wiki item UID → Google Tasks task id. Built at the
// subscribe ceremony by text-matching existing tasks (see plan
// §"Identifiers — Match-by-text on initial seed") and updated on
// every successful sync.
//
// LastUpdatedMin is the inbound poll cursor. Populated from Google's
// Task.updated field — never the wiki clock — so the cursor remains
// authoritative across process restarts. Apply-then-advance: the
// orchestrator persists changes BEFORE advancing the cursor, so a
// crash mid-sync re-fetches the same window on the next tick
// (idempotent under the wiki:uid marker contract).
//
// State + PausedReason + PausedAt encode the pause-horizon. Pause
// freezes LastUpdatedMin in place; resume after <7d does an
// incremental fetch from the frozen cursor; resume after ≥7d forces
// a full resync per the plan.
//
// ItemEtags caches Tasks etags from the last fetched state per task
// id. Used as If-Match on outbound patches; on 412 the orchestrator
// pulls fresh and retries once. Maintained by the inbound apply path.
//
// SyncedItems is the per-item sync state, keyed by wiki uid. Carries
// both the LAST SUCCESSFULLY PUSHED state (Synced*) and the wiki state
// observed at the END of the last tick (LastObservedWiki*). The
// outbound diff loop uses these to skip patches when nothing changed
// in the wiki since the last push — without this, every tick would
// patch every item and overwrite phone-side changes that happened
// in between ticks. Mirrors Keep's per-item ItemMapping pattern.
type Subscription struct {
	// Page is the wiki page identifier carrying the checklist.
	Page string
	// ListName is the named checklist on Page (the <name> in
	// checklists.<name>).
	ListName string
	// RemoteListID is the Google Tasks tasklist id this subscription
	// is bound to. Stable across the subscription's lifetime; a
	// re-subscribe is required to retarget.
	RemoteListID string
	// RemoteListTitle is the friendly display name of the tasklist
	// at subscribe time. Refreshed on each tick when the gateway
	// returns a different title for the same id (the operator
	// renamed it in Tasks).
	RemoteListTitle string
	// ItemIDMap maps wiki uid → Google Tasks task id.
	ItemIDMap map[string]string
	// ItemEtags caches the most recent etag observed per task id
	// (NOT keyed by wiki uid — multiple wiki uids might transiently
	// resolve to the same task id during marker-loss recovery, but
	// the etag is still per-task). Used as If-Match on outbound
	// patches.
	ItemEtags map[string]string
	// LastUpdatedMin is the inbound-poll cursor. The next ListTasks
	// call uses this as updatedMin. Apply-then-advance: never
	// advanced before changes are persisted. Zero value = full
	// initial pull on next sync.
	LastUpdatedMin time.Time
	// LastSuccessfulSyncAt is the wall-clock time of the last
	// successful Sync call (used to enforce the per-connector
	// rate-limit choke; the orchestrator skips a tick if this is
	// within rateLimitChokeSeconds of now).
	LastSuccessfulSyncAt time.Time
	// State is active or paused. Paused subscriptions return early
	// from Sync.
	State SubscriptionState
	// PausedReason is the short, user-facing reason a subscription
	// is paused. Empty when active. Surfaced to the paused-badge.
	PausedReason string
	// PausedAt is the wall-clock time the subscription transitioned
	// to paused. Used by the resume horizon: <7d → incremental,
	// ≥7d → force full resync.
	PausedAt time.Time
	// SubscribedAt is the wall-clock time the subscription was
	// established. Informational only (rendered by the UI).
	SubscribedAt time.Time
	// SyncedItems is the per-item sync state keyed by wiki uid. Each
	// entry records what was last successfully pushed to Google
	// (Synced*) and what wiki state was observed at the end of the
	// last tick (LastObservedWiki*). The outbound diff loop reads
	// this map to decide whether a wiki item changed since the last
	// push; if it didn't, we skip the patch — preventing every-tick
	// overwrites of phone-side edits.
	//
	// Always populated alongside ItemIDMap: when a uid is added to
	// ItemIDMap, a corresponding SyncedItems entry is created; when
	// it's removed, the SyncedItems entry is removed too.
	SyncedItems map[string]ItemSyncState
	// LastSyncedSeq is this binding's cursor in the per-checklist
	// op-log (ADR-0015). Each successful round-trip advances it past
	// every self-event written this tick; subsequent inbound applies
	// classify divergence by scanning events with seq > LastSyncedSeq.
	//
	// Zero on freshly-created or pre-deploy subscriptions; the
	// codec's backfillBaselineEvents synthesizes a baseline event
	// per item, and the first tick advances cursor past those.
	LastSyncedSeq int64
}

// ItemSyncState is the per-item sync record for one wiki uid. It
// carries:
//
//   - SyncedTitle, SyncedNotes, SyncedStatus, SyncedDue: the
//     authoritative "what we last pushed to Google" snapshot. Set on
//     every successful insert/patch. The outbound diff compares
//     current wiki fields to these values; equal → skip the patch.
//
//   - LastObservedWikiTitle, LastObservedWikiNotes,
//     LastObservedWikiStatus, LastObservedWikiDue: the wiki content
//     observed at the END of the last tick. Used on the next tick to
//     detect "wiki changed since last observation" — in concert with
//     Synced* this is how we tell apart "the user edited the wiki"
//     from "no local change."
//
// All fields can be empty/zero on first-tick or freshly-bound items;
// the outbound diff treats an empty SyncedTitle as "never pushed →
// must push" (the insert-first-time path). The TaskID lives in the
// parallel ItemIDMap.
type ItemSyncState struct {
	SyncedTitle  string
	SyncedNotes  string
	SyncedStatus string
	SyncedDue    time.Time

	LastObservedWikiTitle  string
	LastObservedWikiNotes  string
	LastObservedWikiStatus string
	LastObservedWikiDue    time.Time
}

// IsPaused reports whether the subscription is in the paused state.
// Convenience accessor used by Sync's early-return and by
// PausedReason on the Connector.
func (s Subscription) IsPaused() bool {
	return s.State == SubscriptionStatePaused
}

// ConnectorState is the per-user Google Tasks connector configuration
// stored on the profile page under wiki.connectors.google_tasks.*.
//
// The RefreshToken is plaintext per ADR-0014 — the trust perimeter is
// the Tailnet, not the filesystem. Encryption-at-rest is deliberately
// out of scope.
type ConnectorState struct {
	// Email is the Google account email for this connection.
	Email string
	// RefreshToken is the OAuth refresh token (plaintext per
	// ADR-0014). Empty after Disconnect; rotated atomically by the
	// gateway's RefreshClient.
	RefreshToken string
	// ConnectedAt is when the user completed the OAuth flow.
	ConnectedAt time.Time
	// LastVerifiedAt is when we last successfully exchanged the
	// refresh token for an access token.
	LastVerifiedAt time.Time
	// Subscriptions is the per-user list of Tasks-bound checklists.
	Subscriptions []Subscription
}

// IsConfigured reports whether the connector has a refresh token
// (i.e., the user has completed the connect flow). LoadState's zero
// return reads as "not configured" via this check.
func (s ConnectorState) IsConfigured() bool {
	return s.RefreshToken != ""
}
