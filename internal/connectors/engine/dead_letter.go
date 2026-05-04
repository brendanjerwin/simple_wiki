package engine

import (
	"github.com/brendanjerwin/simple_wiki/internal/connectors"
)

// Dead-letter retry is the engine's response to per-item push failures
// the adapter has classified as ErrorClassRetryable. Per MATRIX.md
// row 7 (and Keep's existing implementation that becomes engine
// policy under strictest-behavior-wins), the engine tracks per-item:
//
//   - PushFailureCount: incremented on every Retryable failure;
//     reset to zero on the first Success.
//   - NextAttemptAt: wall-clock floor for the next retry; computed
//     via exponential backoff from PushFailureCount.
//
// Items with PushFailureCount >= deadLetterThreshold (10, matching
// Keep's existing default) are dead-lettered: the engine emits a
// metric counter and SKIPS the outbound push for that item until
// the failure count is cleared by user intervention (typically a
// wiki-side edit that re-resets the item to a known state, or an
// admin RPC).
//
// The state lives in the binding's AdapterState via the adapter's
// EncodeAdapterState codec — the engine writes per-item failure
// bookkeeping into a known map[string]any subtree the adapter agrees
// to round-trip. (The exact subtree key is part of the adapter's
// codec contract; engine doesn't inspect or assume the shape.)
//
// Phase 3a status: stub (recordPushFailure logs and returns). Phase
// 3g wires the per-item bookkeeping into the outbound push loop in
// reconcile.go.
//
// Tasks (which previously had no dead-letter mechanism) gains this
// behavior on collapse — every adapter inherits it via the engine.

// recordPushFailure is the Phase 3a stub for the engine's dead-letter
// bookkeeping. It is invoked from reconcile.go's outbound push path
// when an adapter primitive returns an error classified as
// ErrorClassRetryable. Phase 3g wires the PushFailureCount /
// NextAttemptAt bookkeeping; until then, the stub logs the failure
// and returns. Per Tasks's pre-engine behavior (every tick re-attempts
// the item), reconcile continues to the next item rather than
// aborting the whole sync — the Phase 3g implementation will refine
// to apply the threshold-based skip.
func (e *Engine) recordPushFailure(
	binding connectors.Binding,
	uid string,
	op string,
	pushErr error,
) {
	e.logger.Info("connectors/engine: record_push_failure_pending kind=%s profile=%s page=%s list=%s uid=%s op=%s err=%v",
		e.adapter.Kind(), string(binding.ProfileID), binding.Page, binding.ListName, uid, op, pushErr)
}
