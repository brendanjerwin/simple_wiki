package engine

import "github.com/brendanjerwin/simple_wiki/internal/connectors"

// lookupPausedReason returns the binding's pause state. Per MATRIX.md
// row 5, this unifies Tasks's explicit PausedReason field with Keep's
// auth-disconnect implicit pause; both flow through this method now.
//
// Phase 2 status: stub. Phase 3 implementation reads the binding via
// store.FindBinding, returns (binding.PausedReason, true) when state
// is BindingStatePaused.
//
// Phase 3 also adds the following helpers in this file:
//
//   - runResume(ctx, key): user-driven reconnect path; if pause exceeds
//     the 7-day horizon, calls Adapter.RebuildAdapterState.
//   - transitionToPaused(key, reason): writes the binding as paused,
//     preserving cursor + AdapterState so a within-horizon Resume
//     doesn't lose progress.
//   - resumeFullResyncHorizon constant (7d, mirroring tombstone GC).
//
// Those are added under TDD when their callers (engine.Resume RPC
// and the auth-failed transition in reconcile.go) land.
func (*Engine) lookupPausedReason(_ connectors.SubscriptionKey) (string, bool) {
	return "", false
}
