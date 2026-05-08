package engine

import (
	"context"
	"time"
)

// PerRPCDeadline is the per-call timeout the engine applies before
// invoking any I/O-bearing BackendAdapter primitive (PullRemote,
// InsertRemote, PatchRemote, DeleteRemote, ReadRemoteByRef,
// RefreshItemBaseline-with-context, SyncCollectionState,
// RebuildAdapterState, SeedBindingState, ValidateRemoteBinding,
// FetchRemoteListTitle, ListRemoteCollections).
//
// Rationale (panel review round 3, Bailis): a vendor-side hang on a
// single RPC must not hold the per-checklist lease indefinitely. The
// 30s tick budget leaves headroom for one retry within the same tick
// after a 15s deadline-exceeded fail-fast. See rules §11.9.
//
// Exposed as `var` (not `const`) so adapter-level integration tests
// can shorten it; the engine's call sites read it through
// withRPCDeadline so any override applies uniformly.
var PerRPCDeadline = 15 * time.Second

// withRPCDeadline derives a per-RPC context from the caller's ctx,
// applying PerRPCDeadline. The returned cancel MUST be deferred at
// the call site so resources are released after the primitive call
// returns or the deadline fires.
//
// Engine call sites use this helper exclusively; primitive calls
// that bypass it are forbidden by the convention enforced via
// opengrep (see .semgrep/rules.yml). This is the "Option A"
// invariant-by-construction Bailis specified in the round-3 review:
// the deadline is a property of the engine→adapter boundary, not a
// per-adapter responsibility.
func (e *Engine) withRPCDeadline(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, PerRPCDeadline)
}
