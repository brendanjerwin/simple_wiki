package connectors

import (
	"context"
	"time"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// BackendAdapter is the contract every connector's per-tick implementation
// MUST honor. The SyncEngine (internal/connectors/engine) calls these
// primitives; the adapter provides only wire-protocol verbs, translation,
// capability bits, and error classification — nothing about the merge
// algorithm, the bind ceremony, the cursor advance, or the precondition
// recovery. All of those live in the engine and are shared across every
// backend.
//
// Per ADR-0015 + the audited MATRIX.md (internal/connectors/MATRIX.md):
// each connector type asserts compliance via:
//
//	var _ connectors.BackendAdapter = (*Connector)(nil)
//
// Adding a method to BackendAdapter is a compile error for every backend
// that doesn't implement it. That is the whole point — the user's
// directive (2026-05-04, "complete abstraction; I don't want to re-litigate
// edge conditions") is operationalized by making missing behavior a
// compile error, not a runtime parity gap.
//
// The Phase 1 contract here is grown from MATRIX.md's audit. Every method
// corresponds to one or more rows in the matrix. Adapter-specific
// primitives (Tasks's PatchTask wire shape; Keep's Changes call
// shape) stay inside each adapter's gateway/ + translator/ packages —
// only the shapes here cross the engine boundary.
type BackendAdapter interface {
	// Identity. Used by metrics, structured logs, lease attribution,
	// op-log self-source markers (`connector:<kind>:apply`).
	Kind() ConnectorKind

	// Per-tick primitives (MATRIX row 1).

	// PullRemote fetches every remote item the engine should consider
	// this tick. The adapter walks pagination internally (the engine
	// never sees a partial pull). NewCursor is opaque per-adapter —
	// the engine stores it on the binding and passes it back next
	// tick via the adapter's AdvanceCursor method. Truncated=true
	// signals that the remote refused the cursor (e.g., expired) and
	// the engine should run ForceFullResync.
	PullRemote(ctx context.Context, binding Binding) (RemotePullResult, error)

	// InsertRemote pushes a fresh wiki item to the remote backend.
	// Returns the new RemoteRef (Tasks task id; Keep server id) so
	// the engine can record the wiki-uid → remote-ref mapping in
	// AdapterState.
	InsertRemote(ctx context.Context, binding Binding, item WikiItem) (RemoteRef, error)

	// PatchRemote pushes an update to an existing remote item using
	// the prior RemoteRef. The adapter is responsible for backend-
	// specific optimistic-concurrency tokens (Tasks: If-Match etag;
	// Keep: baseVersion). On a precondition failure, the adapter
	// returns an error that ClassifyError maps to PreconditionFailed,
	// and the engine runs the 3-branch recovery path (MATRIX row 6).
	PatchRemote(ctx context.Context, binding Binding, ref RemoteRef, item WikiItem) (RemoteRef, error)

	// DeleteRemote removes a remote item. Idempotent per-backend
	// semantics (Tasks: 404 is a no-op; Keep: Trashed flag).
	DeleteRemote(ctx context.Context, binding Binding, ref RemoteRef) error

	// SyncCollectionState reconciles per-collection (per-binding)
	// remote state from the supplied wiki items, after the engine has
	// pushed all per-item changes for the tick. Adapter-specific:
	//
	//   - Keep: extract hashtags from items, mint Keep labels for any
	//     unmapped names, update the LIST node's labelIDs assignment
	//     in a single Changes request (legacy keepsync's
	//     SyncToKeep label push, restored from the Phase 5-A port).
	//
	//   - Tasks: no-op (Tasks lacks user-defined labels at the list
	//     level; tags live in the title and round-trip per-item).
	//
	// Called once per reconcile tick by the engine after the per-item
	// outbound loop. Returns the (possibly updated) binding; the engine
	// persists. Errors are treated like a per-item Retryable failure —
	// they don't abort the rest of the tick's progress.
	SyncCollectionState(ctx context.Context, binding Binding, items []WikiItem) (Binding, error)

	// Translation (MATRIX rows 1, 13).

	// RemoteToWiki converts the normalized RemoteItem shape into a
	// WikiItem the engine can apply via the wiki's checklistmutator.
	// Translator-internal concerns (e.g., Tasks's wiki:uid marker
	// extraction; Keep's label resolution) are wholly inside the
	// per-adapter translator package — the engine sees only the
	// resolved WikiItem with a populated UID.
	RemoteToWiki(remote RemoteItem) (WikiItem, error)

	// WikiToRemote converts a wiki item into the normalized RemoteItem
	// shape the adapter's outbound primitives consume. Translator-
	// internal stamping (marker bytes, sort encoding) happens here.
	WikiToRemote(wiki WikiItem) (RemoteItem, error)

	// Cursor advance (MATRIX row 16). Engine call site is uniform;
	// the body is per-adapter (Keep: store the server's cursor token
	// as-is; Tasks: max(updated) - safety_buffer). Returns the
	// updated binding; the engine persists.
	AdvanceCursor(binding Binding, result RemotePullResult) Binding

	// RefreshItemBaseline updates the adapter's stored per-item
	// concurrency baseline (Tasks: item_etags[ref]; Keep:
	// item_mapping[ref].base_version) from a freshly-read RemoteItem.
	// Called by the engine's precondition_recovery path after a
	// successful ReadRemoteByRef so the subsequent re-PATCH carries the
	// fresh baseline instead of the stale one that triggered the 412 in
	// the first place. Without this refresh the recovery loops on 412
	// forever (production regression 2026-05-06: Tasks tick froze for
	// 8h while every recovery repatch hit 412 on the same stale etag).
	// Returns the updated binding; the engine persists.
	RefreshItemBaseline(binding Binding, remote RemoteItem) Binding

	// Bind ceremony (MATRIX row 2).

	// SeedBindingState produces the initial AdapterState for a fresh
	// binding. The engine calls this inside the bind mutex AFTER
	// ValidateRemoteBinding passes; the wikiItems argument is the
	// current wiki checklist's items so the adapter can pre-populate
	// item_id_map by matching wiki uids to remote items. Adapters
	// with native wiki-uid markers (Tasks: `wiki:<uid>` Notes) ignore
	// wikiItems — the marker drives alignment. Adapters without a
	// marker (Keep) text-match wikiItems against the pulled remote
	// items to populate item_id_map at bind time, closing the
	// duplicate-Insert hazard observed 2026-05-08 (production
	// regression).
	SeedBindingState(ctx context.Context, profileID wikipage.PageIdentifier, remoteHandle string, wikiItems []WikiItem) (AdapterState, error)

	// ValidateRemoteBinding checks per-adapter pre-conditions before
	// the engine writes a new binding to the profile. Tasks rejects
	// lists that already contain parent-child subtasks
	// (ErrTasksListHasSubtasks); Keep validates the note exists and
	// is a checklist note. Returns nil to proceed; non-nil aborts
	// the bind ceremony and surfaces the error to the user.
	ValidateRemoteBinding(ctx context.Context, profileID wikipage.PageIdentifier, remoteHandle string) error

	// Force full resync (MATRIX row 4).

	// RebuildAdapterState is invoked by the engine when a binding
	// needs its AdapterState reconstructed from scratch — either
	// because the cursor was rejected, or because pause exceeded the
	// 7-day horizon, or because an operator triggered the admin RPC.
	// The implementation pulls the full remote state and produces a
	// fresh AdapterState (text-match, marker-recovery, etc.).
	RebuildAdapterState(ctx context.Context, binding Binding) (AdapterState, error)

	// Title sync (MATRIX row 11; existed in v1).

	// FetchRemoteListTitle reports the current display title of the
	// remote list/note bound to the given remote handle. Returns
	// (title, true, nil) on success, ("", false, nil) on transient
	// failure or list-not-found (caller preserves prior title), and
	// (_, _, non-nil err) on auth/permission failures the engine
	// should surface.
	FetchRemoteListTitle(ctx context.Context, profileID wikipage.PageIdentifier, remoteHandle string) (string, bool, error)

	// List candidate collections for the bind UI (MATRIX row 19).

	// ListRemoteCollections returns every remote list/note the
	// authenticated profile owns, so the bind UI can present a picker.
	// CollectionCapabilities lets the UI gray out lists that violate
	// adapter constraints (e.g., Tasks lists with subtasks).
	ListRemoteCollections(ctx context.Context, profileID wikipage.PageIdentifier) ([]RemoteCollection, error)

	// Adapter-state codec (MATRIX row 10). Engine treats AdapterState
	// as an opaque map[string]any sealed envelope: the engine moves
	// it through the binding's TOML row but never inspects fields.
	// Adapters encode/decode their own structured state.

	EncodeAdapterState(state AdapterState) (map[string]any, error)
	DecodeAdapterState(raw map[string]any) (AdapterState, error)

	// Capability bits (MATRIX row 12).

	// SupportsSubtasks reports whether the backend has a parent-child
	// item hierarchy. The engine consults this during bind to refuse
	// hierarchies on backends that can't represent them, and during
	// inbound apply to flatten silently when subtasks appear post-bind.
	SupportsSubtasks() bool

	// Read remote by ref (MATRIX row 6, post-412 pull).

	// ReadRemoteByRef pulls a single remote item by its handle. Used
	// by the engine's precondition_recovery path to decide whether
	// the remote is gone (mirror delete to wiki), unchanged from the
	// last synced baseline (refresh etag and re-PATCH), or moved
	// (apply remote-authoritative). On a true 404, return
	// (RemoteItem{Deleted: true}, nil) — not an error.
	ReadRemoteByRef(ctx context.Context, binding Binding, ref RemoteRef) (RemoteItem, error)

	// Error classification (MATRIX rows 5, 6, 7).

	// ClassifyError translates a vendor-specific error from any
	// adapter primitive into the engine's ErrorClass vocabulary.
	// The engine routes recovery by class: PreconditionFailed runs
	// the 3-branch recovery; AuthFailed transitions the binding to
	// paused with PausedReason=auth_failed; Retryable increments
	// PushFailureCount; Fatal dead-letters the item immediately;
	// RateLimited backs off the 30s tick for this binding.
	ClassifyError(err error) ErrorClass
}

// Binding is the engine-owned, cross-connector record that ties a wiki
// checklist (page, list_name) on a profile to a remote list. Replaces
// each connector's per-package Subscription struct (those collapse onto
// AdapterState in Phase 4/5).
//
// Engine fields (LastSyncedSeq, State, PausedReason, etc.) are owned by
// the engine and serialized at top level on the binding's TOML row.
// AdapterState is the opaque per-adapter subtree — the engine moves it
// through the row but never inspects it.
type Binding struct {
	// Identity (the aggregate root per ADR-0011).
	ProfileID    wikipage.PageIdentifier
	Page         string
	ListName     string
	RemoteHandle string

	// Display.
	RemoteListTitle string

	// Engine-owned cursor.
	LastSyncedSeq int64

	// Engine-owned lifecycle state.
	State        BindingState
	PausedReason string
	PausedAt     time.Time
	BoundAt      time.Time

	// Engine-owned per-binding scheduling.
	LastSuccessfulSyncAt time.Time

	// Adapter-opaque state subtree.
	AdapterState AdapterState
}

// BindingState is the engine-owned lifecycle state of a binding.
type BindingState string

const (
	// BindingStateActive is the steady-state. The engine ticks this
	// binding every 30s.
	BindingStateActive BindingState = "active"

	// BindingStatePaused means the engine skips this binding's tick
	// until a transition restores it. PausedReason carries the
	// human-readable why.
	BindingStatePaused BindingState = "paused"
)

// IsPaused reports whether the binding is in the paused state.
func (b Binding) IsPaused() bool { return b.State == BindingStatePaused }

// AdapterState is the per-adapter opaque blob persisted on each binding.
// The engine treats it as a sealed envelope: passes it back to the
// adapter on every primitive call; never inspects fields.
type AdapterState map[string]any

// RemoteRef is an opaque handle to a remote item (Tasks task id;
// Keep server id). The engine treats it as an identifier — never a
// URL, never a path, never inspected.
type RemoteRef string

// RemoteItem is the normalized shape the engine sees post-PullRemote
// (and pre-RemoteToWiki). Translator-internal stamping (markers,
// labels) is invisible at this layer.
type RemoteItem struct {
	Ref      RemoteRef
	Etag     string // empty if the backend has no per-item etag concept
	Title    string
	Notes    string
	Status   string // backend-specific; translator normalizes to checked
	Due      time.Time
	Updated  time.Time
	Deleted  bool
	Position string         // backend-specific ordering hint; translator decides
	Vendor   map[string]any // adapter-internal extra fields, opaque to engine

	// RemoteDiverged is true when the adapter's PullRemote detected that
	// the remote item has genuinely changed since the engine last applied
	// it (i.e., the stored per-item etag or BaseVersion differs from the
	// incoming value). Used by the engine's applyInbound to implement
	// ADR-0015's 4-cell merge rule:
	//
	//   wd ∧ ¬rd → push-wiki  (skip inbound; outbound will push the wiki edit)
	//   wd ∧  rd → conflict-remote-wins (apply remote despite wiki divergence)
	//
	// Adapters MUST set this to true whenever the remote content
	// differs from the last-persisted snapshot. Adapters with server-
	// issued cursors (Keep) may set it unconditionally for all returned
	// items; adapters with safety-buffer cursors (Tasks) MUST compare
	// the incoming etag against the stored AdapterState etag.
	// False when the stored snapshot is absent (first ever pull of an
	// item — treat as not diverged so the inbound apply proceeds normally).
	RemoteDiverged bool
}

// WikiItem is the normalized post-translation shape the engine apply
// path consumes. Mirrors apiv1.ChecklistItem at the engine boundary.
type WikiItem struct {
	UID         string
	Text        string
	Checked     bool
	Tags        []string
	Description string
	Due         time.Time
	SortOrder   int64
}

// RemotePullResult is the output of PullRemote, normalized across
// adapters. NewCursor is opaque (Tasks: time.Time; Keep: string token).
type RemotePullResult struct {
	Items     []RemoteItem
	NewCursor any
	Truncated bool
}

// RemoteCollection is a candidate remote list for the bind UI picker.
type RemoteCollection struct {
	Handle       string
	Title        string
	Capabilities CollectionCapabilities
}

// CollectionCapabilities reports per-collection capability flags so the
// UI can gate selection (e.g., gray out Tasks lists with subtasks).
type CollectionCapabilities struct {
	HasSubtasks bool
}

// ErrorClass is the engine's vocabulary for what the adapter saw on a
// failed primitive. The engine routes recovery by class; adapters
// translate vendor-specific errors via ClassifyError.
type ErrorClass int

const (
	// ErrorClassNone is the zero value — no error / not classified.
	ErrorClassNone ErrorClass = iota

	// ErrorClassTransient is "the network glitched; try again next tick."
	// Engine logs and continues.
	ErrorClassTransient

	// ErrorClassRetryable is "this item failed; bump PushFailureCount,
	// schedule NextAttemptAt with exponential backoff, eventually
	// dead-letter at the threshold." Engine bookkeeping.
	ErrorClassRetryable

	// ErrorClassFatal is "this item will never succeed as-is; dead-letter
	// immediately and surface in the macro UI."
	ErrorClassFatal

	// ErrorClassAuthFailed is "credentials are no longer usable." Engine
	// transitions the binding to paused with PausedReason=auth_failed.
	ErrorClassAuthFailed

	// ErrorClassPreconditionFailed is "the optimistic-concurrency token
	// was stale" (Tasks 412; Keep stage3-500 on bad baseVersion). Engine
	// runs the 3-branch precondition recovery (remote-deleted /
	// remote-unchanged-repatch / remote-authoritative-apply).
	ErrorClassPreconditionFailed

	// ErrorClassRateLimited is "back off the 30s tick for this binding
	// per the Retry-After hint." Engine schedules.
	ErrorClassRateLimited

	// ErrorClassNotFound is "the remote item is gone." Engine mirrors
	// the deletion into the wiki via the mutator.
	ErrorClassNotFound
)
