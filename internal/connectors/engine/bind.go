package engine

import (
	"context"
	"errors"
	"fmt"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// ErrAlreadyBoundForChecklist is returned by Bind when the requested
// (page, listName) is already bound — by the same profile, a different
// profile, or even a different connector kind. Per ADR-0011 the
// aggregate root is (page, listName), and the at-most-one-Binding
// invariant is global across all profiles and kinds. Callers at the
// gRPC boundary surface this as AlreadyExists with the current owner
// described in the message.
var ErrAlreadyBoundForChecklist = errors.New("connectors/engine: checklist already bound")

// Bind wires a wiki checklist (page, listName) to a remote list per
// ADR-0011's ChecklistBinding aggregate. The ceremony enforces the
// strongly-consistent single-Binding invariant via mutex + fan-out
// re-read across all profiles before the lease take.
//
// Algorithm (per MATRIX.md row 2 + ADR-0011 operation contracts):
//
//  1. Wait for the lease-table boot rebuild (LeaseTable.WaitReady) so
//     the in-memory cache reflects every persisted binding before any
//     read-then-act check.
//  2. Acquire the per-checklist mutex on (page, listName) via
//     LeaseTable.WithChecklistLock — serializes against concurrent
//     Bind / Unbind for the same checklist.
//  3. Inside the checklist mutex, call Adapter.ValidateRemoteBinding for
//     adapter-specific pre-conditions (e.g., Tasks rejects lists with
//     subtasks via ErrTasksListHasSubtasks).
//  4. Inside the checklist mutex, fan-out re-read the lease table for
//     the (page, listName) tuple. The lease table is rebuilt from
//     profiles at boot, so an unowned key here means "no profile,
//     no kind" already binds it. If owned, return
//     ErrAlreadyBoundForChecklist — the linearizability guarantee.
//  5. Inside the checklist mutex, call Adapter.SeedBindingState for the
//     initial AdapterState (Tasks: text-match seed; Keep: clone the
//     wiki list onto a new note and record per-item ServerIDs).
//  6. Acquire BindingStore.WithProfileLock; write the new Binding.
//  7. After the store write succeeds, take the lease — the in-memory
//     cache now matches the durable record. On store failure, leave
//     the lease untouched: the binding was never persisted.
//  8. Release locks (defers).
func (e *Engine) Bind(
	ctx context.Context,
	profileID wikipage.PageIdentifier,
	page, listName, remoteHandle string,
) (connectors.Binding, error) {
	if err := e.lease.WaitReady(ctx); err != nil {
		return connectors.Binding{}, fmt.Errorf("await lease-table ready: %w", err)
	}

	checklistKey := connectors.ChecklistKey{Page: page, ListName: listName}
	kind := e.adapter.Kind()

	var result connectors.Binding
	bindErr := e.lease.WithChecklistLock(checklistKey, func() error {
		validCtx, validCancel := e.withRPCDeadline(ctx)
		validErr := e.adapter.ValidateRemoteBinding(validCtx, profileID, remoteHandle)
		validCancel()
		if validErr != nil {
			return fmt.Errorf("validate remote binding %s for profile %s: %w",
				remoteHandle, profileID, validErr)
		}

		if existing, ok := e.lease.LookupOwner(checklistKey); ok {
			return fmt.Errorf("%w: %s/%s held by %s/%s",
				ErrAlreadyBoundForChecklist, page, listName,
				existing.Kind, existing.ProfileID)
		}

		// Read the wiki checklist's current items so the adapter can
		// pre-populate item_id_map by matching wiki uids to remote
		// items at the seed step. Architectural fix 2026-05-08: Keep
		// has no native wiki-uid marker, so without this the first
		// reconcile after Bind would see every Keep item as
		// "unknown" and either duplicate the wiki state (legacy) or
        // fall through to the engine's applyInbound dedup-by-text
        // (which works but is downstream of the gap).
		wikiItems := readWikiItemsForBindSeed(ctx, e.checklist, page, listName)

		seedCtx, seedCancel := e.withRPCDeadline(ctx)
		adapterState, err := e.adapter.SeedBindingState(seedCtx, profileID, remoteHandle, wikiItems)
		seedCancel()
		if err != nil {
			return fmt.Errorf("seed adapter state for %s on profile %s: %w",
				remoteHandle, profileID, err)
		}

		// Populate RemoteListTitle at bind time so the UI shows a
		// human-readable name instead of the opaque RemoteHandle
		// (Keep ServerID, Tasks tasklist ID). User-reported
		// 2026-05-09: KeepConnect was rendering the ServerID UID
		// because RemoteListTitle was never populated. Errors are
		// silenced — title is best-effort, the binding still works
		// and the next refresh will retry.
		titleCtx, titleCancel := e.withRPCDeadline(ctx)
		title, titleOK, _ := e.adapter.FetchRemoteListTitle(titleCtx, profileID, remoteHandle)
		titleCancel()
		var remoteListTitle string
		if titleOK {
			remoteListTitle = title
		}

		newBinding := connectors.Binding{
			ProfileID:       profileID,
			Page:            page,
			ListName:        listName,
			RemoteHandle:    remoteHandle,
			RemoteListTitle: remoteListTitle,
			LastSyncedSeq:   0,
			State:           connectors.BindingStateActive,
			BoundAt:         e.clock.Now(),
			AdapterState:    adapterState,
		}

		profileLockErr := e.store.WithProfileLock(profileID, func() error {
			if err := e.store.SaveBinding(profileID, kind, newBinding); err != nil {
				return fmt.Errorf("save binding %s/%s for profile %s: %w",
					page, listName, profileID, err)
			}
			return nil
		})
		if profileLockErr != nil {
			return profileLockErr
		}

		// Take the lease only after the durable write succeeded — the
		// in-memory cache must not advertise a binding the profile
		// page does not actually carry.
		if err := e.lease.Take(checklistKey, connectors.LeaseOwner{
			Kind: kind, ProfileID: string(profileID),
		}); err != nil {
			return fmt.Errorf("take lease %s/%s for %s/%s: %w",
				page, listName, kind, profileID, err)
		}

		result = newBinding
		return nil
	})
	if bindErr != nil {
		return connectors.Binding{}, bindErr
	}

	e.logger.Info("connectors/engine: bind kind=%s profile=%s page=%s list=%s remote=%s",
		kind, string(profileID), page, listName, remoteHandle)
	return result, nil
}

// readWikiItemsForBindSeed reads the wiki checklist's current items
// and returns them as []connectors.WikiItem for the adapter's
// SeedBindingState pass. Errors are silenced — a missing item list
// at bind time just disables bind-time alignment; the engine's
// applyInbound dedup-by-text remains as the safety net.
func readWikiItemsForBindSeed(ctx context.Context, reader ChecklistReader, page, listName string) []connectors.WikiItem {
	if reader == nil {
		return nil
	}
	cl, err := reader.ListItems(ctx, page, listName)
	if err != nil || cl == nil {
		return nil
	}
	out := make([]connectors.WikiItem, 0, len(cl.GetItems()))
	for _, it := range cl.GetItems() {
		uid := it.GetUid()
		if uid == "" {
			continue
		}
		out = append(out, connectors.WikiItem{
			UID:         uid,
			Text:        it.GetText(),
			Checked:     it.GetChecked(),
			Tags:        it.GetTags(),
			Description: it.GetDescription(),
			SortOrder:   it.GetSortOrder(),
		})
	}
	return out
}
