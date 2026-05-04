package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// ErrBindingNotFound is returned by ForceFullResync (and any other
// engine lifecycle method that operates on an existing binding) when
// the addressed (profileID, kind, page, listName) tuple has no record
// in the BindingStore. Callers at the gRPC boundary surface this as
// NotFound.
var ErrBindingNotFound = errors.New("connectors/engine: binding not found")

// runForceFullResync rebuilds an existing binding's AdapterState from
// scratch and clears any pause / rate-limit state so the next
// scheduler tick processes the binding immediately. Per MATRIX.md row
// 4 + ADR-0015's force-resync entry point.
//
// Algorithm:
//
//  1. Wait for the lease-table boot rebuild so any read-then-act
//     sequencing matches the rest of the lifecycle.
//  2. Look up the binding via BindingStore.FindBinding. Missing →
//     ErrBindingNotFound.
//  3. Acquire the per-checklist mutex via LeaseTable.WithChecklistLock
//     — serializes against concurrent Bind / Unbind / ForceFullResync
//     for the same checklist.
//  4. Inside the checklist mutex, call Adapter.RebuildAdapterState to
//     get a fresh AdapterState. Tasks does a full-list re-pull and
//     text-match seed; Keep does a full Changes call. The engine
//     never inspects the result.
//  5. Acquire the per-profile lock via BindingStore.WithProfileLock.
//  6. Inside both locks, replace AdapterState, reset State to Active,
//     clear PausedReason, PausedAt, and LastSuccessfulSyncAt (so the
//     next tick is unrate-limited; matches Tasks's existing behavior
//     in lifecycle.go:370-411). LastSyncedSeq is preserved — the
//     op-log is the wiki's authority and the resync is about
//     remote-side bookkeeping, not the wiki's causal cursor.
//  7. SaveBinding.
//  8. Release locks (defers).
//
// Used by:
//   - The cursor-truncation recovery path (adapter signals via
//     RemotePullResult.Truncated=true).
//   - The pause-resume horizon path (resume.go) when pause >= 7 days.
//   - An operator-triggered admin RPC.
func (e *Engine) runForceFullResync(ctx context.Context, key connectors.SubscriptionKey) error {
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
		return fmt.Errorf("%w: kind=%s profile=%s page=%s list=%s",
			ErrBindingNotFound, kind, profileID, key.Page, key.ListName)
	}

	checklistKey := connectors.ChecklistKey{Page: key.Page, ListName: key.ListName}

	resyncErr := e.lease.WithChecklistLock(checklistKey, func() error {
		newState, rebuildErr := e.adapter.RebuildAdapterState(ctx, binding)
		if rebuildErr != nil {
			return fmt.Errorf("rebuild adapter state for %s/%s on profile %s: %w",
				key.Page, key.ListName, profileID, rebuildErr)
		}

		return e.store.WithProfileLock(profileID, func() error {
			rebuilt := binding
			rebuilt.AdapterState = newState
			rebuilt.State = connectors.BindingStateActive
			rebuilt.PausedReason = ""
			rebuilt.PausedAt = time.Time{}
			rebuilt.LastSuccessfulSyncAt = time.Time{}
			// LastSyncedSeq is intentionally preserved — the op-log is
			// the wiki's authority and the resync only refreshes
			// adapter-side bookkeeping.

			if err := e.store.SaveBinding(profileID, kind, rebuilt); err != nil {
				return fmt.Errorf("save binding %s/%s for profile %s: %w",
					key.Page, key.ListName, profileID, err)
			}
			return nil
		})
	})
	if resyncErr != nil {
		return resyncErr
	}

	e.logger.Info("connectors/engine: force_full_resync kind=%s profile=%s page=%s list=%s",
		kind, string(profileID), key.Page, key.ListName)
	return nil
}
