package engine

import (
	"context"
	"fmt"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/server/checklistmutator"
)

// Precondition recovery is the engine's response to an
// ErrorClassPreconditionFailed return from Adapter.PatchRemote. Per
// MATRIX.md row 6 (and the Tasks v2 implementation that becomes
// engine policy under strictest-behavior-wins), the recovery has
// three branches:
//
//   - 412_remote_deleted: Adapter.ReadRemoteByRef returns
//     RemoteItem{Deleted: true} (or the read failure classifies as
//     ErrorClassNotFound). The remote item was deleted between our
//     last observation and the patch attempt. Mirror the deletion
//     into the wiki via mutator.DeleteItemForSync (with suppressor)
//     and clean up the wiki-uid → remote-ref mapping in idMap.
//
//   - 412_remote_unchanged_repatch: Adapter.ReadRemoteByRef returns
//     a RemoteItem whose user-facing fields match the wiki's current
//     translation (Title/Notes/Status/Due via WikiToRemote). The 412
//     was phantom — etag desync, sequencing race, or server-side
//     internal bump. Re-PATCH using the freshly-read remote ref. The
//     user's wiki edit is the only real change.
//
//   - 412_remote_authoritative_apply: Adapter.ReadRemoteByRef returns
//     a RemoteItem whose fields differ from the wiki's translation.
//     The remote really did move under us. Translate via
//     Adapter.RemoteToWiki and apply via the mutator (wrapped in
//     suppressor). The next tick's reconcile re-evaluates the diff
//     against the now-authoritative wiki state.
//
// Keep's stage3 HTTP 500 on stale/missing baseVersion maps to
// ErrorClassPreconditionFailed via Keep's adapter.ClassifyError; the
// same 3 branches apply to Keep with no per-adapter override. This
// is the operationalization of "strictest-behavior-wins."
//
// Auth-failed during ReadRemoteByRef: returned as a wrapped error.
// The reconcile loop aborts the current tick; the next tick's
// PullRemote will hit the same auth failure and route through the
// engine's normal handleAuthFailure path (transition to paused,
// PausedReason=auth_failed). We deliberately do NOT inline the
// transition here so the recovery body stays focused on the 3
// branches and the auth-failed transition stays in one place.

// runPreconditionRecovery handles the 3-branch 412/precondition-failed
// recovery for a single uid. Caller (reconcile.go's pushOutbound) has
// already classified the patch error as ErrorClassPreconditionFailed
// and supplies the wiki item (so the recovery can re-PATCH without
// re-reading the wiki) plus the idMap (so branch A can remove the
// uid mapping in-place; branch C can update the ref if it changed).
//
// Returns nil on every successful branch (the recovery handled the
// failure; reconcile continues to the next item). Returns a wrapped
// non-nil error only when ReadRemoteByRef fails (transient or
// auth-failed) or the re-PATCH in branch B fails again — those
// abort the current tick so the next tick gets a fresh shot.
func (e *Engine) runPreconditionRecovery(
	ctx context.Context,
	binding connectors.Binding,
	ref connectors.RemoteRef,
	uid string,
	wikiItem connectors.WikiItem,
	idMap map[string]string,
	patchErr error,
) error {
	kind := e.adapter.Kind()

	// Tag any wiki write performed by the recovery branches
	// (412_remote_deleted / 412_remote_authoritative_apply) with the
	// push_recovery op so the engine's causal merge rule recognizes
	// the apply as the engine's own write rather than a user edit.
	ctx = checklistmutator.WithSource(ctx,
		checklistmutator.ConnectorSource(string(kind), "push_recovery"))

	readCtx, readCancel := e.withRPCDeadline(ctx)
	remote, readErr := e.adapter.ReadRemoteByRef(readCtx, binding, ref)
	readCancel()
	if readErr != nil {
		// Branch A is also reachable when ReadRemoteByRef itself
		// surfaces NotFound (e.g., Tasks's GET returns 404). Treat
		// the same as RemoteItem{Deleted: true}: mirror the delete
		// into the wiki and clean up the mapping.
		if e.adapter.ClassifyError(readErr) == connectors.ErrorClassNotFound {
			return e.precondRemoteDeleted(ctx, binding, ref, uid, idMap)
		}
		return fmt.Errorf("precondition_recovery: read remote by ref %s for profile %s: %w",
			string(ref), string(binding.ProfileID), readErr)
	}

	if remote.Deleted {
		return e.precondRemoteDeleted(ctx, binding, ref, uid, idMap)
	}

	// Refresh the per-item baseline (Tasks: item_etags; Keep:
	// item_mapping.base_version) from the freshly-read remote BEFORE
	// repatching. Without this, the re-PATCH carries the same stale
	// baseline that triggered the original 412/stage3-500, looping
	// forever (production regression 2026-05-06: Tasks tick froze for
	// 8h while every recovery repatch hit 412 on the same etag).
	binding = e.adapter.RefreshItemBaseline(binding, remote)

	// Wiki-wins on every non-deleted recovery. Two reasons per ADR-0015:
	//
	//   1. The patch path is gated on classification[uid].WikiDiverged
	//      (reconcile.go's outbound loop). Any call into this function
	//      already implies the wiki has unsynced user/cross-connector
	//      intent for this uid — the wiki side WANTS to push.
	//
	//   2. The legacy 3-branch recovery had a remote-authoritative-apply
	//      branch when remote-now != wiki-now. That branch CLOBBERED user
	//      wiki edits whenever a 412 fired without a real remote content
	//      change (e.g., Tasks bumping etag for a metadata-only update).
	//      Production regression 2026-05-06: a user check-off on the wiki
	//      was reverted by the recovery's authoritative-apply branch
	//      because the remote etag had bumped while content stayed false.
	//
	// True wiki-vs-remote conflicts (rare third-party concurrent edit)
	// surface on the next tick via PullRemote → applyInbound, which
	// honors ADR-0015's conflict-remote-wins via RemoteDiverged on the
	// item. The recovery's job is just to preserve user wiki intent past
	// stale-etag 412s; conflict resolution happens in the inbound path.
	freshRef := remote.Ref
	if freshRef == "" {
		freshRef = ref
	}
	return e.precondWikiWinsRepatch(ctx, binding, freshRef, uid, wikiItem, idMap, patchErr)
}

// precondWikiWinsRepatch re-PATCHes using the freshly-read remote ref.
// Used by all non-deleted recovery branches: the patch path is gated on
// WikiDiverged, so the wiki has the authoritative user intent. If the
// re-PATCH fails again, return the wrapped error so the next tick
// retries — the recovery never loops on its own.
func (e *Engine) precondWikiWinsRepatch(
	ctx context.Context,
	binding connectors.Binding,
	freshRef connectors.RemoteRef,
	uid string,
	wikiItem connectors.WikiItem,
	idMap map[string]string,
	patchErr error,
) error {
	kind := e.adapter.Kind()

	repatchCtx, repatchCancel := e.withRPCDeadline(ctx)
	_, repatchErr := e.adapter.PatchRemote(repatchCtx, binding, freshRef, wikiItem)
	repatchCancel()
	if repatchErr != nil {
		e.logger.Info("connectors/engine: precondition_recovery_wiki_wins_repatch_failed kind=%s profile=%s page=%s list=%s uid=%s ref=%s err=%v original_err=%v",
			kind, string(binding.ProfileID), binding.Page, binding.ListName, uid, string(freshRef), repatchErr, patchErr)
		return fmt.Errorf("precondition_recovery: re-patch %s for profile %s: %w",
			uid, string(binding.ProfileID), repatchErr)
	}

	idMap[uid] = string(freshRef)

	// Tag the recovery's repatch as a self-write so advanceLastSyncedSeq
	// can advance past it AND past any user/cross-connector events
	// covered by this push. Without this AppendSyncEvent the cursor
	// stays pinned at the user event forever — every subsequent tick
	// re-classifies the user event as WikiDiverged and re-attempts the
	// patch. Production observation 2026-05-06: cursor stuck at
	// last_synced_seq=45 across 8 ticks while etag drifted between
	// adapter_state and the remote.
	if appendErr := e.mutator.AppendSyncEvent(ctx, binding.Page, binding.ListName, uid, "outbound_patched"); appendErr != nil {
		e.logger.Info("connectors/engine: append_sync_event_failed page=%s list=%s uid=%s op=outbound_patched err=%v",
			binding.Page, binding.ListName, uid, appendErr)
	}

	e.logger.Info("connectors/engine: precondition_recovery_wiki_wins_repatch kind=%s profile=%s page=%s list=%s uid=%s ref=%s",
		kind, string(binding.ProfileID), binding.Page, binding.ListName, uid, string(freshRef))
	return nil
}

// precondRemoteDeleted handles branch A: the remote item is gone
// (either Deleted=true on read, or NotFound classified). Mirrors the
// deletion to the wiki under suppressor, removes the uid from the
// caller's idMap, and emits a distinct log event.
func (e *Engine) precondRemoteDeleted(
	ctx context.Context,
	binding connectors.Binding,
	ref connectors.RemoteRef,
	uid string,
	idMap map[string]string,
) error {
	kind := e.adapter.Kind()
	e.logger.Info("connectors/engine: precondition_recovery_remote_deleted kind=%s profile=%s page=%s list=%s uid=%s ref=%s",
		kind, string(binding.ProfileID), binding.Page, binding.ListName, uid, string(ref))

	e.suppressor.Suppress(binding.ProfileID, binding.Page, binding.ListName)
	defer e.suppressor.Unsuppress(binding.ProfileID, binding.Page, binding.ListName)

	if err := e.mutator.DeleteItemForSync(ctx, binding.Page, binding.ListName, "", uid); err != nil {
		return fmt.Errorf("precondition_recovery: delete wiki item %s on profile %s: %w",
			uid, string(binding.ProfileID), err)
	}
	delete(idMap, uid)
	return nil
}

// runDeletePreconditionRecovery handles a `PreconditionFailed` error
// returned by DeleteRemote. The wiki side has already removed the item;
// the engine is trying to mirror that delete to the remote but the
// vendor rejected the call (e.g., Keep stage3 HTTP 500 from a stale
// baseline). Strategy:
//
//   - ReadRemoteByRef. If the remote returns NotFound or Deleted=true,
//     the delete is already complete on the vendor side. Idempotent
//     success: clear idMap, emit outbound_deleted, advance cursor next
//     tick.
//   - If the remote is still alive, refresh the per-item baseline (so
//     the next tick's DELETE has a fresh BaseVersion / etag), record
//     a push failure (Retryable backoff per §11.7), and continue. The
//     next tick will retry; the failed retry will dead-letter at
//     count >= 10 if the remote keeps refusing.
//
// Production motivation (round-3 deploy 2026-05-08): user reported
// "wiki side delete didn't propagate to keep side." Root cause: the
// DELETE branch in pushOutbound had no PreconditionFailed handler;
// stage3-500 protocol-drift errors fell into `default:` and aborted
// the entire tick, blocking every other item in the binding. This
// helper restores per-item handling parity with the PATCH branch.
func (e *Engine) runDeletePreconditionRecovery(
	ctx context.Context,
	binding connectors.Binding,
	ref connectors.RemoteRef,
	uid string,
	idMap map[string]string,
	originalErr error,
) (connectors.Binding, error) {
	kind := e.adapter.Kind()
	e.logger.Info("connectors/engine: delete_precondition_recovery_start kind=%s profile=%s page=%s list=%s uid=%s ref=%s err=%v",
		kind, string(binding.ProfileID), binding.Page, binding.ListName, uid, string(ref), originalErr)

	readCtx, readCancel := e.withRPCDeadline(ctx)
	remote, readErr := e.adapter.ReadRemoteByRef(readCtx, binding, ref)
	readCancel()

	switch {
	case readErr != nil && e.adapter.ClassifyError(readErr) == connectors.ErrorClassNotFound:
		// Remote already gone. Vendor returned 404. Idempotent success.
		return e.deletePrecondRemoteAlreadyGone(ctx, binding, ref, uid, idMap), nil
	case readErr != nil:
		return binding, fmt.Errorf("delete_precondition_recovery: read remote ref %s for uid %s on profile %s: %w",
			ref, uid, binding.ProfileID, readErr)
	case remote.Deleted:
		// Remote returns the item as a tombstone (Keep pattern).
		// Idempotent success.
		return e.deletePrecondRemoteAlreadyGone(ctx, binding, ref, uid, idMap), nil
	default:
		// Remote is still alive. Refresh baseline; defer the retry to
		// the next tick via the Retryable backoff path.
		binding = e.adapter.RefreshItemBaseline(binding, remote)
		binding = e.recordPushFailure(binding, uid, "outbound_deleted", originalErr)
		e.logger.Info("connectors/engine: delete_precondition_recovery_deferred kind=%s profile=%s page=%s list=%s uid=%s ref=%s",
			kind, string(binding.ProfileID), binding.Page, binding.ListName, uid, string(ref))
		return binding, nil
	}
}

// deletePrecondRemoteAlreadyGone records the idempotent-success path
// of runDeletePreconditionRecovery: the remote already has no item to
// delete, so we clear idMap, mark the push as successful, and emit
// `outbound_deleted` so the cursor advances on the next tick.
func (e *Engine) deletePrecondRemoteAlreadyGone(
	ctx context.Context,
	binding connectors.Binding,
	ref connectors.RemoteRef,
	uid string,
	idMap map[string]string,
) connectors.Binding {
	delete(idMap, uid)
	binding = e.recordPushSuccess(binding, uid)
	if appendErr := e.mutator.AppendSyncEvent(ctx, binding.Page, binding.ListName, uid, "outbound_deleted"); appendErr != nil {
		e.logger.Info("connectors/engine: append_sync_event_failed page=%s list=%s uid=%s op=outbound_deleted err=%v",
			binding.Page, binding.ListName, uid, appendErr)
	}
	e.logger.Info("connectors/engine: delete_precondition_recovery_remote_gone kind=%s profile=%s page=%s list=%s uid=%s ref=%s",
		e.adapter.Kind(), string(binding.ProfileID), binding.Page, binding.ListName, uid, string(ref))
	return binding
}
