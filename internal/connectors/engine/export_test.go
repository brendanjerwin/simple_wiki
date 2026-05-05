package engine

import (
	"context"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// TransitionToPausedForTest exposes the engine's transitionToPaused
// helper for engine_test package tests. The helper is engine-internal
// because production callers live in reconcile.go's outbound paths
// (auth-failed branch); the test seam exists so Phase 3e can drive it
// in isolation under TDD without first wiring reconcile.go.
//
// Safe to remove once reconcile.go's auth-failed branch is wired and
// transitionToPaused has organic call-site coverage. Until then this
// keeps the lock dance + binding mutation logic testable on its own.
func (e *Engine) TransitionToPausedForTest(
	profileID wikipage.PageIdentifier,
	kind connectors.ConnectorKind,
	page, listName, reason string,
) error {
	return e.transitionToPaused(profileID, kind, page, listName, reason)
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
