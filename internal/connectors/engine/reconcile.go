package engine

import (
	"context"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
)

// reconcile runs one inbound-then-outbound reconcile pass for the
// binding identified by key. The algorithm (per MATRIX.md row 1):
//
//  1. Load the binding via the BindingStore.
//  2. If paused, return early.
//  3. Per-binding rate-limit choke (5s post-success window).
//  4. Adapter.PullRemote(ctx, binding) → list of remote items + new
//     cursor (opaque per-adapter).
//  5. For each remote item, run the engine's classifier (classify.go)
//     to compute (wiki_diverged, remote_diverged) per uid.
//  6. Apply the 4-cell merge per ADR-0015:
//     - no-op   : neither side changed; skip
//     - push    : wiki diverged, remote unchanged → InsertRemote /
//       PatchRemote / DeleteRemote
//     - apply   : remote diverged, wiki unchanged → mutator
//       AddItemForSync / UpdateItemForSync / DeleteItemForSync
//       (wrapped in suppressor)
//     - conflict: both diverged → remote-wins (apply remote, defer
//       any pending wiki state to next tick's outbound)
//  7. On precondition failure during a push, route into
//     precondition_recovery.go.
//  8. After every successful push primitive, mutator.AppendSyncEvent.
//  9. Adapter.AdvanceCursor(binding, result) → updated binding.
//  10. Persist the binding via BindingStore.SaveBinding.
//
// Phase 2 status: stub. Phase 3 fills this in under TDD against
// FakeAdapter, then parity-tests it through every real adapter.
func (*Engine) reconcile(_ context.Context, _ connectors.SubscriptionKey) error {
	return ErrNotYetImplemented
}
