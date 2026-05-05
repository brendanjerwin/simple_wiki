package engine

import (
	"context"
	"fmt"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
)

// runInsertRecovery is the engine's response to an
// ErrorClassPreconditionFailed return from Adapter.InsertRemote. Insert
// has no RemoteRef (the engine doesn't know what to read by), so the
// 3-branch precondition_recovery used for Patch doesn't apply. Instead,
// the recovery is to call Adapter.RebuildAdapterState — full re-pull
// of the remote view — so the next tick has a correct uid → ref
// mapping (which routes the next attempt through Patch with a fresh
// baseline rather than another doomed Insert).
//
// Used by Keep specifically: stage3 HTTP 500 "Unknown Error" on a
// stale TargetVersion (cursor) or on an Insert that collides with a
// pre-existing item surfaces as ErrProtocolDrift via the gateway,
// which Keep's ClassifyError maps to ErrorClassPreconditionFailed.
// MATRIX row 6 + strictest-behavior-wins per ADR-0015.
//
// The caller (pushOutbound) bails out of the outbound loop after this
// returns, so the rebuilt AdapterState is persisted by reconcile's
// post-pushOutbound save. Items not yet processed in the loop will be
// re-processed on the next tick — no data loss, just a partial tick.
//
// Returns the binding with rebuilt AdapterState; mutates idMap in
// place to mirror the rebuilt state's item_id_map (so any subsequent
// caller that reads it sees the same view).
func (e *Engine) runInsertRecovery(
	ctx context.Context,
	binding connectors.Binding,
	uid string,
	idMap map[string]string,
) (connectors.Binding, error) {
	rebuilt, err := e.adapter.RebuildAdapterState(ctx, binding)
	if err != nil {
		return binding, fmt.Errorf("insert_recovery: rebuild adapter state for %s on profile %s: %w",
			uid, string(binding.ProfileID), err)
	}

	binding.AdapterState = rebuilt

	// Mirror the rebuilt state's item_id_map into the caller's idMap so
	// the post-loop writeItemIDMap doesn't clobber our work with a
	// stale snapshot.
	for k := range idMap {
		delete(idMap, k)
	}
	for k, v := range readItemIDMap(rebuilt) {
		idMap[k] = v
	}

	e.logger.Info("connectors/engine: insert_recovery_rebuilt kind=%s profile=%s page=%s list=%s uid=%s",
		e.adapter.Kind(), string(binding.ProfileID), binding.Page, binding.ListName, uid)

	return binding, nil
}
