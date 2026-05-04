package engine

import (
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
