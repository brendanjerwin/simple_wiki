package engine

// SyncDebouncer batches checklist mutations into a single Sync call
// per (profileID, page, listName) inside a debounce window, and
// suppresses the next tick if it would fire inside a post-success
// choke window of a successful Sync.
//
// Per MATRIX.md row 9: both pre-extraction connectors used a 1.5s
// debounce window; Tasks added a 5s post-success choke. The engine
// adopts both uniformly under strictest-behavior-wins (Keep gains
// the choke).
//
// One SyncDebouncer is shared across all engines (all adapter kinds).
// The mutator notifies the debouncer on every wiki-side mutation; the
// debouncer routes to the engine that currently owns the (page,
// listName) lease.
//
// Phase 2 status: skeleton. Phase 3 wires the timer loop, the
// debounceWindow + postSyncChokeWindow constants, and the engine
// lookup table. Tests use sinon-style fake-clock control.
type SyncDebouncer struct{}

// NewSyncDebouncer constructs the debouncer.
//
// Phase 2 status: stub. Phase 3 takes the engine lookup function +
// the clock + the logger as constructor params.
func NewSyncDebouncer() *SyncDebouncer {
	return &SyncDebouncer{}
}

// OnChecklistMutated is invoked by the wiki-side checklist mutator on
// every successful mutation. The debouncer schedules a Sync after
// debounceWindow unless one is already scheduled.
//
// Phase 2 status: stub.
func (*SyncDebouncer) OnChecklistMutated(_, _ string) {
	// Phase 3 implementation.
}
