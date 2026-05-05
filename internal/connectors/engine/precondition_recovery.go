package engine

import (
	"context"
	"fmt"
	"time"

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

	remote, readErr := e.adapter.ReadRemoteByRef(ctx, binding, ref)
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

	// Branch B vs C: compare the read remote to the wiki's current
	// translation. If user-facing fields match, the 412 was phantom
	// (re-PATCH branch). Otherwise the remote is authoritative
	// (apply branch).
	wikiAsRemote, txErr := e.adapter.WikiToRemote(wikiItem)
	if txErr != nil {
		return fmt.Errorf("precondition_recovery: translate wiki item %s for profile %s: %w",
			uid, string(binding.ProfileID), txErr)
	}

	if remoteFieldsMatch(remote, wikiAsRemote) {
		return e.precondRemoteUnchangedRepatch(ctx, binding, remote, uid, wikiItem, idMap, patchErr)
	}

	return e.precondRemoteAuthoritativeApply(ctx, binding, remote, uid, idMap)
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

// precondRemoteUnchangedRepatch handles branch B: the read remote's
// user-facing fields match the wiki's current translation, so the
// 412 was phantom (etag desync, sequencing race, server-side bump).
// Re-PATCH using the freshly-read remote ref. If the re-PATCH fails
// again, return the wrapped error so the next tick retries — the
// recovery never loops on its own.
func (e *Engine) precondRemoteUnchangedRepatch(
	ctx context.Context,
	binding connectors.Binding,
	remote connectors.RemoteItem,
	uid string,
	wikiItem connectors.WikiItem,
	idMap map[string]string,
	patchErr error,
) error {
	kind := e.adapter.Kind()
	freshRef := remote.Ref

	_, repatchErr := e.adapter.PatchRemote(ctx, binding, freshRef, wikiItem)
	if repatchErr != nil {
		e.logger.Info("connectors/engine: precondition_recovery_remote_unchanged_repatch_failed kind=%s profile=%s page=%s list=%s uid=%s ref=%s err=%v original_err=%v",
			kind, string(binding.ProfileID), binding.Page, binding.ListName, uid, string(freshRef), repatchErr, patchErr)
		return fmt.Errorf("precondition_recovery: re-patch %s for profile %s: %w",
			uid, string(binding.ProfileID), repatchErr)
	}

	idMap[uid] = string(freshRef)
	e.logger.Info("connectors/engine: precondition_recovery_remote_unchanged_repatch_success kind=%s profile=%s page=%s list=%s uid=%s ref=%s",
		kind, string(binding.ProfileID), binding.Page, binding.ListName, uid, string(freshRef))
	return nil
}

// precondRemoteAuthoritativeApply handles branch C: the read remote
// genuinely diverges from the wiki's translation, so the remote is
// authoritative. Translate via RemoteToWiki and apply via the mutator
// under suppressor. Updates the idMap if the remote ref changed
// (uncommon but defensive: some adapters re-key on re-read).
func (e *Engine) precondRemoteAuthoritativeApply(
	ctx context.Context,
	binding connectors.Binding,
	remote connectors.RemoteItem,
	uid string,
	idMap map[string]string,
) error {
	kind := e.adapter.Kind()
	wikiItem, txErr := e.adapter.RemoteToWiki(remote)
	if txErr != nil {
		return fmt.Errorf("precondition_recovery: translate remote ref %s on profile %s: %w",
			string(remote.Ref), string(binding.ProfileID), txErr)
	}

	resolvedUID := wikiItem.UID
	if resolvedUID == "" {
		resolvedUID = uid
	}

	e.suppressor.Suppress(binding.ProfileID, binding.Page, binding.ListName)
	defer e.suppressor.Unsuppress(binding.ProfileID, binding.Page, binding.ListName)

	if _, alreadyMapped := idMap[resolvedUID]; alreadyMapped {
		if err := e.mutator.UpdateItemForSync(ctx, binding.Page, binding.ListName, "", resolvedUID,
			wikiItem.Text, wikiItem.Checked, wikiItem.Tags, wikiItem.Description); err != nil {
			return fmt.Errorf("precondition_recovery: update wiki item %s on profile %s: %w",
				resolvedUID, string(binding.ProfileID), err)
		}
	} else {
		newUID, addErr := e.mutator.AddItemForSync(ctx, binding.Page, binding.ListName, "",
			wikiItem.Text, wikiItem.Checked, wikiItem.Tags, wikiItem.Description, remote.Position)
		if addErr != nil {
			return fmt.Errorf("precondition_recovery: add wiki item from remote ref %s on profile %s: %w",
				string(remote.Ref), string(binding.ProfileID), addErr)
		}
		if newUID != "" {
			resolvedUID = newUID
		}
	}

	idMap[resolvedUID] = string(remote.Ref)
	e.logger.Info("connectors/engine: precondition_recovery_remote_authoritative_apply kind=%s profile=%s page=%s list=%s uid=%s ref=%s",
		kind, string(binding.ProfileID), binding.Page, binding.ListName, resolvedUID, string(remote.Ref))
	return nil
}

// remoteFieldsMatch reports whether two RemoteItems have equal
// user-facing content fields (Title, Notes, Status, Due). Etag, Ref,
// Updated, Deleted, Position, and Vendor are intentionally ignored:
//
//   - Etag/Updated change on every server-side bump (the very
//     phenomenon branch B exists to recover from).
//   - Ref is opaque and mostly the input itself.
//   - Position is backend-specific ordering noise.
//   - Vendor is adapter-internal.
//
// Due is compared at date-only resolution. Some backends (Google
// Tasks) store Due as date-only and zero out the time-of-day on
// every wire round-trip; comparing at full-timestamp resolution
// would cause a permanent mismatch and force every 412 into the
// authoritative-apply branch incorrectly.
func remoteFieldsMatch(a, b connectors.RemoteItem) bool {
	if a.Title != b.Title {
		return false
	}
	if a.Notes != b.Notes {
		return false
	}
	if a.Status != b.Status {
		return false
	}
	if !sameDueDate(a.Due, b.Due) {
		return false
	}
	return true
}

// sameDueDate reports whether two Due timestamps refer to the same
// calendar date in UTC, honoring the date-only resolution rationale
// in remoteFieldsMatch's docstring.
func sameDueDate(a, b time.Time) bool {
	if a.IsZero() != b.IsZero() {
		return false
	}
	if a.IsZero() {
		return true
	}
	ay, am, ad := a.UTC().Date()
	by, bm, bd := b.UTC().Date()
	return ay == by && am == bm && ad == bd
}
