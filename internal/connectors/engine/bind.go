package engine

import (
	"context"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// Bind wires a wiki checklist (page, listName) to a remote list per
// ADR-0011's ChecklistBinding aggregate. The ceremony enforces the
// strongly-consistent single-Binding invariant via mutex + fan-out
// re-read across all profiles before the lease take.
//
// Algorithm (per MATRIX.md row 2 + ADR-0011 operation contracts):
//
//  1. Acquire the per-checklist mutex on (page, listName) via
//     LeaseTable.AcquireBindMutex (blocks concurrent Bind calls
//     for the same checklist).
//  2. Adapter.ValidateRemoteBinding(ctx, profileID, remoteHandle) —
//     adapter-specific pre-conditions (e.g., Tasks rejects lists
//     with subtasks via ErrTasksListHasSubtasks).
//  3. Fan-out scan: BindingStore.ListAllProfilesWithBindings(any
//     kind) — verify no other profile / connector already owns this
//     (page, listName). If one does, return ErrAlreadyBoundForChecklist.
//     This is the linearizability guarantee.
//  4. Adapter.SeedBindingState(ctx, profileID, remoteHandle) —
//     adapter produces the initial AdapterState (Tasks: text-match
//     seed for existing list items; Keep: clone the wiki list onto
//     a new Keep note and record per-item ServerIDs).
//  5. Acquire BindingStore.WithProfileLock; write the new Binding
//     under wiki.connectors.<kind>.bindings[].
//  6. LeaseTable.TakeLease — record the (page, listName) → (kind,
//     profileID) mapping in the in-memory cache.
//  7. Release the per-checklist mutex.
//
// Phase 2 status: stub.
func (*Engine) Bind(_ context.Context, _ wikipage.PageIdentifier, _, _, _ string) (connectors.Binding, error) {
	return connectors.Binding{}, ErrNotYetImplemented
}
