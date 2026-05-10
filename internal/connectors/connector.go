// Package connectors holds the cross-connector abstractions shared
// between Google Keep, Google Tasks, and (future) iCloud Reminders
// bridges. The shapes here are deliberately minimal: a `Connector`
// dispatch interface, a `LeaseTable` for per-checklist exclusivity,
// a `SyncScheduler` that fires the unified 30s tick, and typed event
// constants for structured logs/metrics.
//
// Wiki-side ports (ChecklistReader, ChecklistMutator, SyncSuppressor,
// JobEnqueuer) are intentionally NOT defined here — per Go idiom they
// stay in their consumer packages. This package only contains the
// abstractions every backend has to satisfy.
package connectors

import "context"

// ConnectorKind identifies a backend connector. Stable across releases —
// stored values land in user profile frontmatter and structured logs,
// so adding a new kind is fine but renaming or removing one is a
// migration.
type ConnectorKind string

// Known connector kinds. The ICloudReminders kind is reserved for the
// next bridge after Google Tasks; it appears here so the dispatcher
// can treat the set as closed.
const (
	// ConnectorKindGoogleKeep identifies the Google Keep bridge.
	ConnectorKindGoogleKeep ConnectorKind = "google_keep"

	// ConnectorKindGoogleTasks identifies the Google Tasks bridge.
	ConnectorKindGoogleTasks ConnectorKind = "google_tasks"
)

// BindingKey is the (profile, page, list) tuple that identifies
// a single checklist's binding to a remote list. The wiki-side
// fields (Page, ListName) are owned by the wiki's checklist namespace;
// ProfileID is the per-user wiki profile page id that holds the
// connector state.
//
// Per the plan, the unit of consistency is the (page, list_name)
// aggregate root — multiple users on the same wiki can each look up
// who currently owns a checklist via LeaseTable.LookupOwner without
// needing to know that user's profile id ahead of time. The ProfileID
// rides on the BindingKey because the connector still needs it
// to read/write per-user state during Sync.
type BindingKey struct {
	ProfileID string
	Page      string
	ListName  string
}

// Connector is the dispatch shape every backend implements. The
// SyncScheduler walks the registered Connectors via this interface;
// gRPC handlers route per-kind operations through it; `LeaseTable`
// records which Connector currently owns a checklist.
//
// Implementations MUST be safe for concurrent use — the scheduler
// dispatches Sync calls without serializing per-connector.
type Connector interface {
	// Kind returns the connector's stable identifier. Used by the
	// scheduler to attribute metrics and by the lease table to
	// answer "which backend currently owns this checklist?"
	Kind() ConnectorKind

	// Sync runs one reconcile pass for the given binding.
	// Returns an error if the run failed in a way the caller should
	// know about; the per-connector rate-limit / pause-state
	// "skip-this-tick" logic is handled INSIDE Sync (returning nil
	// is the right answer when a binding is paused).
	Sync(ctx context.Context, key BindingKey) error

	// PausedReason reports whether the given binding is paused
	// and, if so, a human-readable reason. Returns ("", false) when
	// the binding is healthy. The reason string is surfaced to
	// the UI's paused-badge tooltip — keep it short and actionable.
	PausedReason(key BindingKey) (string, bool)

	// ForceFullResync marks the given binding for a one-shot
	// full re-fetch on the next Sync. Used by the cursor-truncation
	// recovery path and by an operator-triggered admin RPC.
	ForceFullResync(ctx context.Context, key BindingKey) error
}
