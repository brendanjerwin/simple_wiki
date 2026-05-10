package engine

import (
	"context"
	"time"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// TransitionToPausedForTest exposes the engine's applyPausedTransition
// helper for engine_test package tests. The helper is engine-internal
// because production callers live in reconcile.go's outbound paths
// (auth-failed branch); the test seam exists so Phase 3e can drive it
// in isolation under TDD without first wiring reconcile.go.
//
// Note: distinct from the exported Engine.TransitionToPaused (which
// takes only profileID/page/listName/reason and infers kind from the
// adapter). This test seam takes an explicit kind so legacy multi-
// adapter test scenarios can drive it.
func (e *Engine) TransitionToPausedForTest(
	profileID wikipage.PageIdentifier,
	kind connectors.ConnectorKind,
	page, listName, reason string,
) error {
	return e.applyPausedTransition(profileID, kind, page, listName, reason)
}

// RunPreconditionRecoveryForTest exposes the engine's runPrecondition
// Recovery helper for engine_test package tests. The helper is
// engine-internal because production callers live in reconcile.go's
// outbound push path (the 412/precondition-failed branch in pushOutbound);
// the test seam exists so Phase 3f can drive each of the three
// recovery branches in isolation under TDD without staging an
// end-to-end Sync that simultaneously exercises pull, classify,
// inbound apply, and the rest of the outbound diff.
//
// Mirrors the production signature exactly. The idMap parameter is
// the caller-owned wiki-uid → remote-ref map the recovery mutates
// in-place (delete on branch A, set on branches B and C); tests
// inspect it after the call to verify the mutation.
func (e *Engine) RunPreconditionRecoveryForTest(
	ctx context.Context,
	binding connectors.Binding,
	ref connectors.RemoteRef,
	uid string,
	wikiItem connectors.WikiItem,
	idMap map[string]string,
	patchErr error,
) error {
	return e.runPreconditionRecovery(ctx, binding, ref, uid, wikiItem, idMap, patchErr)
}

// RecordPushFailureForTest exposes the engine's recordPushFailure helper
// for engine_test package tests. Production callers live in
// reconcile.go's outbound push loop (the Retryable branches of Insert/
// Patch/Delete); the test seam exists so Phase 3g can exercise the
// per-uid PushFailureCount + NextAttemptAt bookkeeping in isolation
// without staging a full Sync.
func (e *Engine) RecordPushFailureForTest(
	binding connectors.Binding,
	uid string,
	op string,
	pushErr error,
) connectors.Binding {
	return e.recordPushFailure(binding, uid, op, pushErr)
}

// RecordPushSuccessForTest exposes the engine's recordPushSuccess helper
// for engine_test package tests. Production callers live in reconcile.go
// after each successful Insert/Patch/Delete; the test seam exists so
// Phase 3g can verify the failure record is cleared in isolation.
func (e *Engine) RecordPushSuccessForTest(
	binding connectors.Binding,
	uid string,
) connectors.Binding {
	return e.recordPushSuccess(binding, uid)
}

// ShouldSkipPushForTest exposes the engine's shouldSkipPush gate for
// engine_test package tests. Production callers live in reconcile.go
// before each Adapter.{Insert,Patch,Delete}Remote primitive; the test
// seam exists so Phase 3g can verify the dead-letter and backoff gates
// in isolation.
func (e *Engine) ShouldSkipPushForTest(
	binding connectors.Binding,
	uid string,
) (bool, string) {
	return e.shouldSkipPush(binding, uid)
}

// DeadLetterThresholdForTest exposes the engine's deadLetterThreshold
// constant so tests can drive the threshold-breach scenario without
// hardcoding the magic number 10 (and so a future change to the
// constant ripples through the test suite by simply rerunning).
const DeadLetterThresholdForTest = deadLetterThreshold

// PushFailureBackoffForTest exposes the engine's pushFailureBackoff
// formula so tests can compute expected NextAttemptAt values without
// duplicating the formula.
func PushFailureBackoffForTest(n int) time.Duration {
	return pushFailureBackoff(n)
}
