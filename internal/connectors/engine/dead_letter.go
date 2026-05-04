package engine

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
// Phase 2 status: stub. Phase 3 wires the bookkeeping into the
// outbound push loop in reconcile.go.
//
// Tasks (which previously had no dead-letter mechanism) gains this
// behavior on collapse — every adapter inherits it via the engine.
