package engine

import (
	"context"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
)

// runForceFullResync triggers a one-shot full re-fetch on the next Sync.
// Per MATRIX.md row 4 + ADR-0015's force-resync entry point:
//
//  1. Load the binding via the BindingStore.
//  2. Acquire the per-profile lock (engine-owned; not adapter's).
//  3. Call Adapter.RebuildAdapterState(ctx, binding) — adapter pulls
//     the full remote state and produces a fresh AdapterState (text-
//     match seed, marker-recovery, etc.).
//  4. Replace the binding's AdapterState; reset State to Active and
//     PausedReason to empty.
//  5. Save via BindingStore.
//
// Used by:
//   - The cursor-truncation recovery path (adapter signals via
//     RemotePullResult.Truncated=true).
//   - The pause-resume horizon path (resume.go) when pause >= 7 days.
//   - An operator-triggered admin RPC.
//
// Phase 2 status: stub.
func (*Engine) runForceFullResync(_ context.Context, _ connectors.SubscriptionKey) error {
	return ErrNotYetImplemented
}
