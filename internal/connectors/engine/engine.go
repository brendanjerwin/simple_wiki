// Package engine owns the cross-connector synchronization algorithm.
// Per ADR-0015 (audited 2026-05-04 refinement) and ADR-0012 (audited
// 2026-05-04 refinement), the engine is the single owner of:
//
//   - Per-tick reconcile (pull → classify → decide → push → cursor advance)
//   - Bind / Unbind ceremony (mutex + fan-out re-read + profile write +
//     lease take, per ADR-0011)
//   - ForceFullResync
//   - Pause / Resume (with the 7-day horizon → reseed)
//   - Precondition recovery (3-branch: remote-deleted /
//     remote-unchanged-repatch / remote-authoritative-apply)
//   - Dead-letter retry (PushFailureCount + NextAttemptAt + threshold)
//   - Sync debouncer (1.5s window + 5s post-success choke)
//   - Binding store (per-profile mutex + TOML serialization)
//   - Tombstone GC interaction
//
// Per-adapter primitives live behind the BackendAdapter interface in
// internal/connectors/adapter.go. The audited contract is documented
// per-row in internal/connectors/MATRIX.md.
//
// This file holds the Engine struct, its constructor, and its
// satisfaction of the cross-connector dispatch interface
// (connectors.Connector). The body of each lifecycle method is split
// into its own file (reconcile.go, bind.go, unbind.go, etc.) for
// reviewer ergonomics.
//
// Phase 2 status (skeleton): the methods on this Engine compile but
// are not wired through. Phase 3 fills each one in under strict TDD
// (red engine_test.go using FakeAdapter → green implementation →
// refactor → parity test against every real adapter).
package engine

import (
	"context"
	"errors"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
)

// ErrNotYetImplemented is the sentinel an engine method returns when
// Phase 3 has not yet wired its behavior. Lets dependent code compile
// against the Engine surface (and tests assert that unwired paths
// fail loudly) without resorting to panics in library code.
//
// Each Phase 3 todo replaces ErrNotYetImplemented with the real
// implementation under TDD discipline; this sentinel disappears
// from the engine package by the time Phase 3 completes.
var ErrNotYetImplemented = errors.New("connectors/engine: not yet implemented (Phase 3 in progress)")

// Engine wraps a BackendAdapter and produces a connectors.Connector that
// the shared SyncScheduler / gRPC ConnectorService / LeaseTable use as
// the dispatch shape. One Engine per adapter; the shared scheduler
// fans out to all registered Engines on each 30s tick.
//
// Construction: see NewEngine. All Engine fields are wired up at
// construction time and never mutated thereafter — the Engine is
// safe for concurrent use across the scheduler tick, mutator-driven
// debouncer fires, and gRPC handler calls.
type Engine struct {
	adapter   connectors.BackendAdapter
	lease     *connectors.LeaseTable
	checklist ChecklistReader
	mutator   ChecklistMutator
	suppressor SyncSuppressor
	logger    Logger
	clock     Clock
	store     BindingStore
}

// NewEngine wires the production dependencies for one connector kind.
// Returns an Engine that satisfies connectors.Connector and exposes
// the lifecycle methods (Bind, Unbind, ForceFullResync, Resume,
// FetchRemoteListTitle, ListRemoteCollections).
//
// Every parameter is required; a nil collaborator is a wiring bug at
// startup and returns an error rather than crashing later.
func NewEngine(
	adapter connectors.BackendAdapter,
	lease *connectors.LeaseTable,
	checklist ChecklistReader,
	mutator ChecklistMutator,
	suppressor SyncSuppressor,
	logger Logger,
	clock Clock,
	store BindingStore,
) (*Engine, error) {
	if adapter == nil {
		return nil, errors.New("connectors/engine: adapter must not be nil")
	}
	if lease == nil {
		return nil, errors.New("connectors/engine: leaseTable must not be nil")
	}
	if checklist == nil {
		return nil, errors.New("connectors/engine: checklistReader must not be nil")
	}
	if mutator == nil {
		return nil, errors.New("connectors/engine: checklistMutator must not be nil")
	}
	if suppressor == nil {
		return nil, errors.New("connectors/engine: suppressor must not be nil")
	}
	if logger == nil {
		return nil, errors.New("connectors/engine: logger must not be nil")
	}
	if clock == nil {
		return nil, errors.New("connectors/engine: clock must not be nil")
	}
	if store == nil {
		return nil, errors.New("connectors/engine: bindingStore must not be nil")
	}
	return &Engine{
		adapter:   adapter,
		lease:     lease,
		checklist: checklist,
		mutator:   mutator,
		suppressor: suppressor,
		logger:    logger,
		clock:     clock,
		store:     store,
	}, nil
}

// Kind returns the connector kind of the wrapped adapter. Implements
// connectors.Connector.Kind.
func (e *Engine) Kind() connectors.ConnectorKind {
	return e.adapter.Kind()
}

// Sync runs one reconcile pass for the given binding. Implements
// connectors.Connector.Sync. Body lives in reconcile.go.
func (e *Engine) Sync(ctx context.Context, key connectors.SubscriptionKey) error {
	return e.reconcile(ctx, key)
}

// PausedReason reports the binding's pause state. Implements
// connectors.Connector.PausedReason. Body in resume.go (the file
// that owns pause/resume state machinery).
func (e *Engine) PausedReason(key connectors.SubscriptionKey) (string, bool) {
	return e.lookupPausedReason(key)
}

// ForceFullResync triggers a one-shot full re-fetch on the next Sync.
// Implements connectors.Connector.ForceFullResync. Body in
// force_resync.go.
func (e *Engine) ForceFullResync(ctx context.Context, key connectors.SubscriptionKey) error {
	return e.runForceFullResync(ctx, key)
}

// Compile-time check: *Engine satisfies the cross-connector dispatch
// shape for any adapter. This is the structural guarantee that the
// scheduler / RPC service / lease table can drive an Engine without
// knowing which backend is wrapped.
var _ connectors.Connector = (*Engine)(nil)
