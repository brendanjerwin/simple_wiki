package engine

import (
	"context"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// This file declares the wiki-side ports the engine consumes. Per the
// Go consumer-defined-interface idiom (and ADR-0012's "wiki-side ports
// stay consumer-defined" principle), these interfaces live in the
// engine package — the package that USES them — not in the package
// that implements them. Production wiring connects the wiki's
// ChecklistReadMutator / suppressor / etc. to these slots.

// ChecklistReader is the wiki-side port the engine uses to read the
// current state of a checklist (items + tombstones + events) for
// causal divergence classification, outbound diff, and parity test
// scenarios.
type ChecklistReader interface {
	// ListItems returns the current Checklist for (page, listName) —
	// the items[] array, the tombstones[] array, and the events[]
	// op-log per ADR-0015. Empty checklists return a non-nil
	// *apiv1.Checklist with zero-length slices; missing pages return
	// a non-nil error.
	ListItems(ctx context.Context, page, listName string) (*apiv1.Checklist, error)
}

// ChecklistMutator is the wiki-side port the engine uses to apply
// inbound remote changes to a checklist. Every mutation routes through
// the mutator's For-Sync methods (which suppress outbound notify
// events) so an inbound apply does not loop back as an outbound push
// trigger.
//
// All For-Sync methods append a self-source event to the op-log per
// ADR-0015 (src=connector:<kind>:apply). The engine does NOT emit
// op-log events directly — it calls the mutator and trusts the
// mutator to log.
type ChecklistMutator interface {
	AddItemForSync(ctx context.Context, page, listName, ownerEmail, text string, checked bool, tags []string, description string, position string, due *time.Time) (string, error)
	UpdateItemForSync(ctx context.Context, page, listName, ownerEmail, uid, text string, checked bool, tags []string, description string, due *time.Time) error
	DeleteItemForSync(ctx context.Context, page, listName, ownerEmail, uid string) error

	// AppendSyncEvent writes a self-source op-log event after a
	// successful outbound primitive (Insert/Patch/Delete) so the
	// causal classifier on the next inbound tick recognizes our own
	// write and does not re-apply the remote echo.
	AppendSyncEvent(ctx context.Context, page, listName, uid, op string) error
}

// SyncSuppressor is the wiki-side port that pauses the mutator's
// outbound-notify firing for a specific (profile, page, listName)
// triple while the engine's inbound apply pass runs. Without this,
// every inbound apply would re-trigger the debouncer.
type SyncSuppressor interface {
	Suppress(profileID wikipage.PageIdentifier, page, listName string)
	Unsuppress(profileID wikipage.PageIdentifier, page, listName string)
}

// Logger is the engine's structured-log surface. Methods are minimal —
// the production wiring uses the wiki's lumber-shaped logger; tests
// use a buffer-backed logger.
type Logger interface {
	Info(format string, args ...any)
	Warn(format string, args ...any)
	Error(format string, args ...any)
}

// Clock is the testable wall-clock seam. Production: time.Now-backed.
// Tests: sinon-style fake clock with Tick/AdvanceTo controls.
type Clock interface {
	Now() time.Time
}

// BindingStore is the per-profile binding persistence port. The engine
// reads/writes Bindings via this interface; the production
// implementation walks the profile's frontmatter under
// wiki.connectors.<kind>.bindings[]. The store is engine-owned (not
// per-adapter) — adapter state rides through it as the opaque
// AdapterState subtree on each Binding.
type BindingStore interface {
	// LoadBindings returns every Binding the profile owns for the
	// given connector kind. Empty when the profile has none.
	LoadBindings(profileID wikipage.PageIdentifier, kind connectors.ConnectorKind) ([]connectors.Binding, error)

	// FindBinding returns the Binding for (profileID, kind, page,
	// listName) or (Binding{}, false, nil) if none.
	FindBinding(profileID wikipage.PageIdentifier, kind connectors.ConnectorKind, page, listName string) (connectors.Binding, bool, error)

	// SaveBinding writes a Binding back to the profile, replacing any
	// existing entry with the same (page, listName) key. Idempotent.
	SaveBinding(profileID wikipage.PageIdentifier, kind connectors.ConnectorKind, binding connectors.Binding) error

	// DeleteBinding removes the Binding for (profileID, kind, page,
	// listName). No-op if none exists.
	DeleteBinding(profileID wikipage.PageIdentifier, kind connectors.ConnectorKind, page, listName string) error

	// WithProfileLock runs fn while holding the per-profile mutex.
	// Used by the engine's Bind / Unbind ceremony to enforce the
	// mutex+fan-out-re-read invariant (per ADR-0011).
	WithProfileLock(profileID wikipage.PageIdentifier, fn func() error) error

	// ListAllProfilesWithBindings returns every profile that has at
	// least one binding for the given kind. Used by the boot lease
	// rebuild fan-out scan.
	ListAllProfilesWithBindings(kind connectors.ConnectorKind) ([]wikipage.PageIdentifier, error)
}
