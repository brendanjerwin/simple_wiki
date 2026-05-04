package engine

import (
	"context"
	"fmt"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
)

// Precondition recovery is the engine's response to an
// ErrorClassPreconditionFailed return from Adapter.PatchRemote. Per
// MATRIX.md row 6 (and the Tasks v2 implementation that becomes
// engine policy under strictest-behavior-wins), the recovery has
// three branches:
//
//   - 412_remote_deleted: Adapter.ReadRemoteByRef returns
//     RemoteItem{Deleted: true} or NotFound. The remote item was
//     deleted between our last observation and the patch attempt.
//     Mirror the deletion into the wiki via mutator.DeleteItemForSync
//     (with suppressor); clean up id_map and etag entries in
//     AdapterState.
//
//   - 412_remote_unchanged_repatch: Adapter.ReadRemoteByRef returns
//     a RemoteItem whose fields match the last successfully-pushed
//     SyncedItems baseline (or the equivalent in AdapterState). The
//     412 was phantom — etag desync, sequencing race, server-side
//     internal bump. Refresh the etag from the read response and
//     re-PATCH with fresh ItemEtags. The user's wiki edit wins.
//
//   - 412_remote_authoritative_apply: Adapter.ReadRemoteByRef returns
//     a RemoteItem whose fields differ from our baseline. The remote
//     really did move under us. Translate the remote via
//     Adapter.RemoteToWiki and apply via the mutator (with
//     suppressor). The next tick's reconcile re-evaluates the diff
//     against the now-authoritative wiki state.
//
// Phase 3a status: stub. Phase 3f wires the 3-branch body. Phase 3a
// invokes this stub from reconcile.go's outbound push path so the
// failure mode is observable in logs and propagates through the
// outbound loop, even though the recovery itself is not yet
// implemented.
//
// Keep's stage3 HTTP 500 on stale/missing baseVersion maps to
// ErrorClassPreconditionFailed via Keep's adapter.ClassifyError;
// the same 3 branches apply to Keep with no per-adapter override.
// This is the operationalization of "strictest-behavior-wins."

// runPreconditionRecovery is the Phase 3a stub for the engine's
// 412/precondition-failed recovery. It is invoked from reconcile.go's
// outbound push path when Adapter.PatchRemote returns an error
// classified as ErrorClassPreconditionFailed. Phase 3f wires the real
// 3-branch recovery (remote-deleted / remote-unchanged-repatch /
// remote-authoritative-apply); until then, the stub returns the
// original error wrapped with "precondition_recovery_pending" so the
// pending recovery is visible in structured logs.
func (e *Engine) runPreconditionRecovery(
	_ context.Context,
	binding connectors.Binding,
	ref connectors.RemoteRef,
	uid string,
	patchErr error,
) error {
	e.logger.Info("connectors/engine: precondition_recovery_pending kind=%s profile=%s page=%s list=%s uid=%s ref=%s err=%v",
		e.adapter.Kind(), string(binding.ProfileID), binding.Page, binding.ListName, uid, string(ref), patchErr)
	return fmt.Errorf("precondition_recovery_pending: %w", patchErr)
}
