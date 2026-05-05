package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/server/checklistmutator"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// rateLimitChoke is the per-binding cooldown between successful
// reconciles. Mirrors Tasks's existing 5s post-success choke (see
// google_tasks/sync/connector.go's rateLimitChokeSeconds). The
// strictest-behavior-wins rule promotes the choke to engine policy so
// every adapter inherits it on collapse: the unified scheduler tick
// reaches every binding, and back-to-back wiki edits within the
// debouncer's 1.5s window collapse into one sync — but human-paced
// edits arriving 1.6s–4.9s apart would each trigger their own sync
// without the choke. The name drops Tasks's "Seconds" suffix because
// the engine's value is a time.Duration, and ST1011 (Go style) bans
// unit suffixes on duration constants.
const rateLimitChoke = 5 * time.Second

// pausedReasonAuthFailed is the canonical PausedReason the engine
// stamps when ClassifyError returns ErrorClassAuthFailed. Both Tasks
// and Keep currently surface the same string in the UI's paused-badge
// tooltip; promoting it to a constant keeps the rename surface small
// (Phase 6).
const pausedReasonAuthFailed = "auth_failed"

// adapterStateItemIDMapKey is the well-known AdapterState subtree key
// the engine reads/writes to track wiki-uid → remote-ref mappings. The
// engine does NOT inspect the values' types beyond the documented
// shape: map[string]string (wiki uid → opaque ref string). Both
// adapters use this convention; future adapters MUST honor it for the
// engine's diff logic to work. Documented in ADR-0015's "AdapterState
// schema" section.
const adapterStateItemIDMapKey = "item_id_map"

// reconcile runs one inbound-then-outbound reconcile pass for the
// binding identified by key. The algorithm (per MATRIX.md row 1 +
// ADR-0015's causal merge rule):
//
//  1. Wait for the lease-table boot rebuild gate.
//  2. Find the binding. Missing → return nil (boot race; not an error).
//  3. Skip if paused (engine writes to that binding only via Resume /
//     ForceFullResync paths).
//  4. Per-binding 5s rate-limit choke against LastSuccessfulSyncAt.
//  5. Read the wiki checklist + classify divergence per ADR-0015.
//  6. Pull remote. On Truncated, delegate to runForceFullResync. On
//     auth-failed, transition to paused with PausedReason=auth_failed.
//  7. Apply inbound items inside Suppressor.Suppress/Unsuppress. The
//     classifier-skip-on-divergence rule preserves pending wiki edits.
//  8. Push outbound (insert / patch / delete) by diffing the wiki
//     items against the AdapterState's item_id_map. Append a
//     self-source op-log event after every successful primitive so
//     the next tick's classifier recognizes our own writes.
//  9. AdvanceCursor (opaque per-adapter mutation of the binding).
//  10. Re-read the op-log to advance LastSyncedSeq past self-events.
//  11. Stamp LastSuccessfulSyncAt = clock.Now().
//  12. SaveBinding under WithProfileLock.
//
// AdapterState mutation strategy: the engine reads/writes the
// well-known `item_id_map` subtree (uid → ref string) to drive the
// outbound diff. Other adapter-state fields ride through opaquely;
// the adapter codec round-trips them. This is documented in ADR-0015.
//
//revive:disable-next-line:cyclomatic,cognitive-complexity,function-length
func (e *Engine) reconcile(ctx context.Context, key connectors.BindingKey) error {
	if err := e.lease.WaitReady(ctx); err != nil {
		return fmt.Errorf("await lease-table ready: %w", err)
	}

	profileID := wikipage.PageIdentifier(key.ProfileID)
	kind := e.adapter.Kind()

	binding, found, err := e.store.FindBinding(profileID, kind, key.Page, key.ListName)
	if err != nil {
		return fmt.Errorf("find binding %s/%s for profile %s: %w",
			key.Page, key.ListName, profileID, err)
	}
	if !found {
		// Boot race: the scheduler tick can fire before the lease
		// rebuild + binding load complete, or after an unbind. Not an
		// error — there is nothing to sync.
		return nil
	}

	if binding.IsPaused() {
		return nil
	}

	now := e.clock.Now()
	if !binding.LastSuccessfulSyncAt.IsZero() && now.Sub(binding.LastSuccessfulSyncAt) < rateLimitChoke {
		return nil
	}

	classification := e.classifyDivergence(ctx, binding)

	pullResult, pullErr := e.adapter.PullRemote(ctx, binding)
	if pullErr != nil {
		switch e.adapter.ClassifyError(pullErr) {
		case connectors.ErrorClassAuthFailed:
			return e.handleAuthFailure(profileID, kind, key.Page, key.ListName)
		case connectors.ErrorClassRateLimited:
			e.logger.Info("connectors/engine: pull_rate_limited kind=%s profile=%s page=%s list=%s",
				kind, string(profileID), key.Page, key.ListName)
			return nil
		case connectors.ErrorClassNone, connectors.ErrorClassTransient,
			connectors.ErrorClassRetryable, connectors.ErrorClassFatal,
			connectors.ErrorClassPreconditionFailed, connectors.ErrorClassNotFound:
			fallthrough
		default:
			return fmt.Errorf("pull remote %s/%s for profile %s: %w",
				key.Page, key.ListName, profileID, pullErr)
		}
	}

	if pullResult.Truncated {
		e.logger.Info("connectors/engine: pull_truncated kind=%s profile=%s page=%s list=%s — delegating to force_full_resync",
			kind, string(profileID), key.Page, key.ListName)
		return e.runForceFullResync(ctx, key)
	}

	idMap := readItemIDMap(binding.AdapterState)

	if applyErr := e.applyInbound(ctx, binding, pullResult.Items, classification, idMap); applyErr != nil {
		return applyErr
	}

	updatedBinding, outboundAuth, outboundErr := e.pushOutbound(ctx, binding, idMap)
	if outboundErr != nil {
		return outboundErr
	}
	if outboundAuth {
		// Auth failure during outbound: transition the binding to
		// paused. Engine-internal happy path returns nil (paused is a
		// steady-state condition).
		return e.handleAuthFailure(profileID, kind, key.Page, key.ListName)
	}
	binding = updatedBinding

	binding = writeItemIDMap(binding, idMap)

	binding = e.adapter.AdvanceCursor(binding, pullResult)

	binding.LastSyncedSeq = e.advanceLastSyncedSeq(ctx, binding, kind)
	binding.LastSuccessfulSyncAt = now

	if saveErr := e.store.WithProfileLock(profileID, func() error {
		if err := e.store.SaveBinding(profileID, kind, binding); err != nil {
			return fmt.Errorf("save binding %s/%s for profile %s: %w",
				key.Page, key.ListName, profileID, err)
		}
		return nil
	}); saveErr != nil {
		return saveErr
	}

	e.logger.Info("connectors/engine: reconcile_ok kind=%s profile=%s page=%s list=%s last_synced_seq=%d",
		kind, string(profileID), key.Page, key.ListName, binding.LastSyncedSeq)
	return nil
}

// classifyDivergence reads the wiki checklist's op-log and runs the
// engine's causal classifier (ADR-0015). Returns nil on read error —
// "no classification info" is interpreted as "don't skip any apply,"
// matching Tasks's existing legacy behavior.
func (e *Engine) classifyDivergence(ctx context.Context, binding connectors.Binding) map[string]EventClassification {
	cl, err := e.checklist.ListItems(ctx, binding.Page, binding.ListName)
	if err != nil || cl == nil {
		return nil
	}
	return Classify(cl, BindingCursor{
		Page:          binding.Page,
		ListName:      binding.ListName,
		LastSyncedSeq: binding.LastSyncedSeq,
	}, string(e.adapter.Kind()))
}

// applyInbound walks the remote items and applies non-deleted, non-
// diverged items to the wiki via the mutator (wrapped in suppressor).
// Deleted items mirror through the mutator's DeleteItemForSync. The
// suppressor pause runs across the whole loop so a post-mutator
// debouncer fire doesn't loop back as an outbound trigger.
//
//revive:disable-next-line:cyclomatic,cognitive-complexity
func (e *Engine) applyInbound(
	ctx context.Context,
	binding connectors.Binding,
	items []connectors.RemoteItem,
	classification map[string]EventClassification,
	idMap map[string]string,
) error {
	if len(items) == 0 {
		return nil
	}

	// Tag every event the apply pass writes with the engine's self-
	// source so the classifier on the next tick recognizes our own
	// writes per ADR-0015.
	ctx = checklistmutator.WithSource(ctx,
		checklistmutator.ConnectorSource(string(e.adapter.Kind()), "apply"))

	e.suppressor.Suppress(binding.ProfileID, binding.Page, binding.ListName)
	defer e.suppressor.Unsuppress(binding.ProfileID, binding.Page, binding.ListName)

	// Reverse map: remote ref → wiki uid (used for deleted-item lookup).
	refToUID := make(map[string]string, len(idMap))
	for uid, ref := range idMap {
		refToUID[ref] = uid
	}

	for _, remoteItem := range items {
		if remoteItem.Deleted {
			uid, hasUID := refToUID[string(remoteItem.Ref)]
			if !hasUID {
				continue
			}
			if delErr := e.mutator.DeleteItemForSync(ctx, binding.Page, binding.ListName, "", uid); delErr != nil {
				return fmt.Errorf("delete wiki item %s on profile %s: %w",
					uid, binding.ProfileID, delErr)
			}
			delete(idMap, uid)
			delete(refToUID, string(remoteItem.Ref))
			continue
		}

		wikiItem, txErr := e.adapter.RemoteToWiki(remoteItem)
		if txErr != nil {
			return fmt.Errorf("translate remote %s on profile %s: %w",
				remoteItem.Ref, binding.ProfileID, txErr)
		}

		uid := wikiItem.UID
		if uid == "" {
			uid = refToUID[string(remoteItem.Ref)]
		}

		if uid != "" {
			if cls, has := classification[uid]; has && cls.WikiDiverged {
				e.logger.Info("connectors/engine: wiki_diverged_skipped_inbound kind=%s profile=%s page=%s list=%s uid=%s ref=%s latest_src=%s",
					e.adapter.Kind(), string(binding.ProfileID), binding.Page, binding.ListName, uid, string(remoteItem.Ref), cls.LatestEventSource)
				continue
			}
			if updErr := e.mutator.UpdateItemForSync(ctx, binding.Page, binding.ListName, "", uid,
				wikiItem.Text, wikiItem.Checked, wikiItem.Tags, wikiItem.Description); updErr != nil {
				return fmt.Errorf("update wiki item %s on profile %s: %w",
					uid, binding.ProfileID, updErr)
			}
			idMap[uid] = string(remoteItem.Ref)
			refToUID[string(remoteItem.Ref)] = uid
			continue
		}

		newUID, addErr := e.mutator.AddItemForSync(ctx, binding.Page, binding.ListName, "",
			wikiItem.Text, wikiItem.Checked, wikiItem.Tags, wikiItem.Description, remoteItem.Position)
		if addErr != nil {
			return fmt.Errorf("add wiki item from remote %s on profile %s: %w",
				remoteItem.Ref, binding.ProfileID, addErr)
		}
		if newUID != "" {
			idMap[newUID] = string(remoteItem.Ref)
			refToUID[string(remoteItem.Ref)] = newUID
		}
	}
	return nil
}

// pushOutbound diffs the wiki checklist against the AdapterState's
// item_id_map and pushes the delta to the remote backend via the
// adapter's primitives. Returns:
//
//   - the (possibly updated) binding,
//   - authFailed=true if the outbound encountered an auth failure
//     (caller transitions to paused),
//   - any non-recoverable error that aborts the sync.
//
// Per Tasks's existing behavior (single-item retryable errors don't
// abort the whole sync), retryable errors are routed to
// recordPushFailure and the loop continues to the next item.
//
//revive:disable-next-line:cyclomatic,cognitive-complexity,function-length
func (e *Engine) pushOutbound(
	ctx context.Context,
	binding connectors.Binding,
	idMap map[string]string,
) (connectors.Binding, bool, error) {
	// Tag self-events emitted from the outbound path so the engine's
	// causal merge rule sees them as our own writes per ADR-0015.
	ctx = checklistmutator.WithSource(ctx,
		checklistmutator.ConnectorSource(string(e.adapter.Kind()), "outbound_push"))

	cl, readErr := e.checklist.ListItems(ctx, binding.Page, binding.ListName)
	if readErr != nil {
		return binding, false, fmt.Errorf("read wiki checklist %s/%s: %w",
			binding.Page, binding.ListName, readErr)
	}

	currentUIDs := map[string]*apiv1.ChecklistItem{}
	if cl != nil {
		for _, it := range cl.GetItems() {
			if it.GetUid() != "" {
				currentUIDs[it.GetUid()] = it
			}
		}
	}

	// Inserts / patches.
	for uid, item := range currentUIDs {
		wikiItem := connectors.WikiItem{
			UID:         uid,
			Text:        item.GetText(),
			Checked:     item.GetChecked(),
			Tags:        item.GetTags(),
			Description: item.GetDescription(),
			SortOrder:   item.GetSortOrder(),
		}
		_, txErr := e.adapter.WikiToRemote(wikiItem)
		if txErr != nil {
			return binding, false, fmt.Errorf("translate wiki item %s for profile %s: %w",
				uid, binding.ProfileID, txErr)
		}

		ref, alreadyBound := idMap[uid]
		if !alreadyBound {
			if skip, reason := e.shouldSkipPush(binding, uid); skip {
				e.logger.Info("connectors/engine: outbound_push_skipped kind=%s profile=%s page=%s list=%s uid=%s op=outbound_inserted reason=%s",
					e.adapter.Kind(), string(binding.ProfileID), binding.Page, binding.ListName, uid, reason)
				continue
			}
			newRef, insErr := e.adapter.InsertRemote(ctx, binding, wikiItem)
			if insErr != nil {
				switch e.adapter.ClassifyError(insErr) {
				case connectors.ErrorClassAuthFailed:
					return binding, true, nil
				case connectors.ErrorClassRetryable:
					binding = e.recordPushFailure(binding, uid, "outbound_inserted", insErr)
					continue
				case connectors.ErrorClassNone, connectors.ErrorClassTransient,
					connectors.ErrorClassFatal, connectors.ErrorClassPreconditionFailed,
					connectors.ErrorClassRateLimited, connectors.ErrorClassNotFound:
					fallthrough
				default:
					return binding, false, fmt.Errorf("insert remote item %s for profile %s: %w",
						uid, binding.ProfileID, insErr)
				}
			}
			idMap[uid] = string(newRef)
			binding = e.recordPushSuccess(binding, uid)
			if appendErr := e.mutator.AppendSyncEvent(ctx, binding.Page, binding.ListName, uid, "outbound_inserted"); appendErr != nil {
				e.logger.Info("connectors/engine: append_sync_event_failed page=%s list=%s uid=%s op=outbound_inserted err=%v",
					binding.Page, binding.ListName, uid, appendErr)
			}
			continue
		}

		if skip, reason := e.shouldSkipPush(binding, uid); skip {
			e.logger.Info("connectors/engine: outbound_push_skipped kind=%s profile=%s page=%s list=%s uid=%s op=outbound_patched reason=%s",
				e.adapter.Kind(), string(binding.ProfileID), binding.Page, binding.ListName, uid, reason)
			continue
		}
		_, patchErr := e.adapter.PatchRemote(ctx, binding, connectors.RemoteRef(ref), wikiItem)
		if patchErr != nil {
			switch e.adapter.ClassifyError(patchErr) {
			case connectors.ErrorClassAuthFailed:
				return binding, true, nil
			case connectors.ErrorClassPreconditionFailed:
				if recErr := e.runPreconditionRecovery(ctx, binding, connectors.RemoteRef(ref), uid, wikiItem, idMap, patchErr); recErr != nil {
					return binding, false, recErr
				}
				continue
			case connectors.ErrorClassRetryable:
				binding = e.recordPushFailure(binding, uid, "outbound_patched", patchErr)
				continue
			case connectors.ErrorClassNone, connectors.ErrorClassTransient,
				connectors.ErrorClassFatal, connectors.ErrorClassRateLimited,
				connectors.ErrorClassNotFound:
				fallthrough
			default:
				return binding, false, fmt.Errorf("patch remote item %s for profile %s: %w",
					uid, binding.ProfileID, patchErr)
			}
		}
		binding = e.recordPushSuccess(binding, uid)
		if appendErr := e.mutator.AppendSyncEvent(ctx, binding.Page, binding.ListName, uid, "outbound_patched"); appendErr != nil {
			e.logger.Info("connectors/engine: append_sync_event_failed page=%s list=%s uid=%s op=outbound_patched err=%v",
				binding.Page, binding.ListName, uid, appendErr)
		}
	}

	// Deletes: for every uid in idMap that is no longer in currentUIDs.
	for uid, ref := range idMap {
		if _, stillThere := currentUIDs[uid]; stillThere {
			continue
		}
		if skip, reason := e.shouldSkipPush(binding, uid); skip {
			e.logger.Info("connectors/engine: outbound_push_skipped kind=%s profile=%s page=%s list=%s uid=%s op=outbound_deleted reason=%s",
				e.adapter.Kind(), string(binding.ProfileID), binding.Page, binding.ListName, uid, reason)
			continue
		}
		if delErr := e.adapter.DeleteRemote(ctx, binding, connectors.RemoteRef(ref)); delErr != nil {
			switch e.adapter.ClassifyError(delErr) {
			case connectors.ErrorClassAuthFailed:
				return binding, true, nil
			case connectors.ErrorClassRetryable:
				binding = e.recordPushFailure(binding, uid, "outbound_deleted", delErr)
				continue
			case connectors.ErrorClassNone, connectors.ErrorClassTransient,
				connectors.ErrorClassFatal, connectors.ErrorClassPreconditionFailed,
				connectors.ErrorClassRateLimited, connectors.ErrorClassNotFound:
				fallthrough
			default:
				return binding, false, fmt.Errorf("delete remote item %s for profile %s: %w",
					uid, binding.ProfileID, delErr)
			}
		}
		delete(idMap, uid)
		binding = e.recordPushSuccess(binding, uid)
		if appendErr := e.mutator.AppendSyncEvent(ctx, binding.Page, binding.ListName, uid, "outbound_deleted"); appendErr != nil {
			e.logger.Info("connectors/engine: append_sync_event_failed page=%s list=%s uid=%s op=outbound_deleted err=%v",
				binding.Page, binding.ListName, uid, appendErr)
		}
	}
	return binding, false, nil
}

// advanceLastSyncedSeq returns the new cursor value for binding. Per
// ADR-0015: cursor advances ONLY past our own writes (events whose
// src starts with `connector:<kind>:`), NOT to MaxSeq, so any user
// or cross-connector event that interleaved with our work stays
// visible to next tick's classify.
func (e *Engine) advanceLastSyncedSeq(ctx context.Context, binding connectors.Binding, kind connectors.ConnectorKind) int64 {
	cl, err := e.checklist.ListItems(ctx, binding.Page, binding.ListName)
	if err != nil || cl == nil {
		return binding.LastSyncedSeq
	}
	selfPrefix := SourcePrefixForKind(string(kind))
	maxSelfSeq := binding.LastSyncedSeq
	for _, ev := range cl.GetEvents() {
		if ev == nil || !strings.HasPrefix(ev.GetSrc(), selfPrefix) {
			continue
		}
		if ev.GetSeq() > maxSelfSeq {
			maxSelfSeq = ev.GetSeq()
		}
	}
	return maxSelfSeq
}

// handleAuthFailure transitions the binding to paused with
// PausedReason=auth_failed. Returns nil because paused is a
// steady-state condition (the next user-initiated Resume / Reconnect
// is the recovery path), not a sync error to bubble up.
func (e *Engine) handleAuthFailure(profileID wikipage.PageIdentifier, kind connectors.ConnectorKind, page, listName string) error {
	if err := e.applyPausedTransition(profileID, kind, page, listName, pausedReasonAuthFailed); err != nil {
		// If the transition itself fails (e.g., the binding vanished
		// between FindBinding and SaveBinding), treat ErrBindingNotFound
		// as a benign race; everything else is a real error.
		if errors.Is(err, ErrBindingNotFound) {
			return nil
		}
		return err
	}
	return nil
}

// readItemIDMap pulls the well-known item_id_map subtree out of an
// AdapterState. Returns a fresh, non-nil map even when the AdapterState
// has no entry, so the caller can mutate without nil-checks.
//
// Per ADR-0015's AdapterState schema, the engine reads/writes
// item_id_map as map[string]string (wiki uid → opaque ref string). The
// adapter codec preserves it across encode/decode round-trips.
//
// On TOML round-trips, the underlying type may surface as
// `map[string]any` even though every value is a string (TOML decodes
// don't preserve the originally-typed inner map). The defensive
// conversion handles both shapes.
func readItemIDMap(state connectors.AdapterState) map[string]string {
	out := map[string]string{}
	if state == nil {
		return out
	}
	raw, ok := state[adapterStateItemIDMapKey]
	if !ok || raw == nil {
		return out
	}
	switch m := raw.(type) {
	case map[string]string:
		for k, v := range m {
			out[k] = v
		}
	case map[string]any:
		for k, v := range m {
			if s, isString := v.(string); isString {
				out[k] = s
			}
		}
	}
	return out
}

// writeItemIDMap writes the supplied uid → ref map back into the
// binding's AdapterState under the well-known item_id_map key.
// Returns the updated binding (caller stores). If the map is empty,
// the key is preserved with an empty map so downstream consumers
// (RebuildAdapterState, EncodeAdapterState) don't have to treat
// presence/absence as different states.
func writeItemIDMap(binding connectors.Binding, idMap map[string]string) connectors.Binding {
	if binding.AdapterState == nil {
		binding.AdapterState = connectors.AdapterState{}
	}
	out := make(map[string]string, len(idMap))
	for k, v := range idMap {
		out[k] = v
	}
	binding.AdapterState[adapterStateItemIDMapKey] = out
	return binding
}
