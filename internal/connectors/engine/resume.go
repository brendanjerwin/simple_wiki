package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// resumeFullResyncHorizon is the duration past which a paused binding's
// adapter-side bookkeeping is considered too stale to safely resume in
// place. When Resume detects pause >= this horizon, it routes to the
// force-full-resync path (RebuildAdapterState) rather than flipping the
// binding back to active with the old AdapterState. Mirrors the wiki's
// tombstone GC default and Tasks's existing implementation.
const resumeFullResyncHorizon = 7 * 24 * time.Hour

// lookupPausedReason reports the binding's pause state for status display.
//
// Per MATRIX.md row 5, this unifies Tasks's explicit PausedReason field
// with Keep's auth-disconnect implicit pause; both flow through this
// method now. Tasks already had an explicit field; Keep gains it as
// part of the engine extraction (the field is set by applyPausedTransition
// and cleared by Resume / runForceFullResync).
//
// Errors from FindBinding return ("", false). This is a status-display
// path (called via the gRPC GetStatus handler), not an authoritative
// read — surfacing a transient store error here would disrupt the UI
// for what is effectively a "we don't know" condition.
func (e *Engine) lookupPausedReason(key connectors.BindingKey) (string, bool) {
	profileID := wikipage.PageIdentifier(key.ProfileID)
	kind := e.adapter.Kind()

	binding, found, err := e.store.FindBinding(profileID, kind, key.Page, key.ListName)
	if err != nil {
		return "", false
	}
	if !found {
		return "", false
	}
	if binding.State != connectors.BindingStatePaused {
		return "", false
	}
	return binding.PausedReason, true
}

// TransitionToPaused is the exported wrapper around applyPausedTransition
// for callers outside the engine package (Phase 4-3: the
// FrontmatterCredentialStore.ClearCredentials path needs to mark every
// active binding as paused on Disconnect). Forwards directly to the
// internal helper, which holds the per-checklist mutex + per-profile
// lock and writes the binding's State / PausedReason / PausedAt.
//
// kind defaults to e.adapter.Kind() — the per-engine instance dictates
// which connector kind this method writes for, so callers don't need
// to thread that through.
func (e *Engine) TransitionToPaused(profileID wikipage.PageIdentifier, page, listName, reason string) error {
	return e.applyPausedTransition(profileID, e.adapter.Kind(), page, listName, reason)
}

// applyPausedTransition writes the binding into the paused state with the
// supplied reason, preserving cursor (LastSyncedSeq) and AdapterState
// so a within-horizon Resume can flip it back without losing progress.
//
// Called by reconcile.go's outbound paths when adapter.ClassifyError
// returns ErrorClassAuthFailed (per MATRIX.md row 5). Per ADR-0011,
// write lifecycle paths hold both the per-checklist mutex (via
// LeaseTable.WithChecklistLock) and the per-profile lock (via
// BindingStore.WithProfileLock).
//
// Algorithm:
//
//  1. Acquire the per-checklist mutex on (page, listName).
//  2. Inside it, acquire the per-profile lock.
//  3. Find the binding; if absent, return ErrBindingNotFound.
//  4. Set State=Paused, PausedReason=reason, PausedAt=clock.Now().
//     LastSyncedSeq, RemoteHandle, BoundAt, AdapterState are
//     intentionally preserved — a within-horizon Resume reuses them.
//  5. SaveBinding.
//  6. Log a structured event line.
func (e *Engine) applyPausedTransition(
	profileID wikipage.PageIdentifier,
	kind connectors.ConnectorKind,
	page, listName, reason string,
) error {
	checklistKey := connectors.ChecklistKey{Page: page, ListName: listName}

	transitionErr := e.lease.WithChecklistLock(checklistKey, func() error {
		return e.store.WithProfileLock(profileID, func() error {
			binding, found, err := e.store.FindBinding(profileID, kind, page, listName)
			if err != nil {
				return fmt.Errorf("find binding %s/%s for profile %s: %w",
					page, listName, profileID, err)
			}
			if !found {
				return fmt.Errorf("%w: kind=%s profile=%s page=%s list=%s",
					ErrBindingNotFound, kind, profileID, page, listName)
			}

			paused := binding
			paused.State = connectors.BindingStatePaused
			paused.PausedReason = reason
			paused.PausedAt = e.clock.Now()
			// LastSyncedSeq, AdapterState, RemoteHandle, BoundAt are
			// intentionally preserved so a within-horizon Resume does
			// not lose progress.

			if err := e.store.SaveBinding(profileID, kind, paused); err != nil {
				return fmt.Errorf("save paused binding %s/%s for profile %s: %w",
					page, listName, profileID, err)
			}
			return nil
		})
	})
	if transitionErr != nil {
		return transitionErr
	}

	e.logger.Info("connectors/engine: transition_to_paused kind=%s profile=%s page=%s list=%s reason=%s",
		kind, string(profileID), page, listName, reason)
	return nil
}

// Resume is the user-driven reconnect path. The engine flips a paused
// binding back to active; if pause exceeded the 7-day horizon, the
// engine routes to runForceFullResync instead so adapter bookkeeping
// is rebuilt from scratch (per MATRIX.md row 5 + Tasks's existing
// 7-day horizon).
//
// Algorithm:
//
//  1. WaitReady on the lease table (boot rebuild gate).
//  2. Find the binding; if absent, return ErrBindingNotFound.
//  3. If the binding is already active, return nil — Resume is
//     idempotent on an active binding (matching Tasks's behavior).
//  4. Compute pauseDuration = clock.Now().Sub(binding.PausedAt).
//     If pauseDuration >= resumeFullResyncHorizon, dispatch to
//     runForceFullResync (which re-runs adapter.RebuildAdapterState
//     under both locks and clears all pause state).
//  5. Otherwise, in-place transition: WithChecklistLock →
//     WithProfileLock → set State=Active, clear PausedReason and
//     PausedAt, SaveBinding. LastSyncedSeq and AdapterState are
//     preserved — within-horizon resume should not lose progress.
//  6. Log a structured event line.
func (e *Engine) Resume(ctx context.Context, profileID wikipage.PageIdentifier, page, listName string) error {
	if err := e.lease.WaitReady(ctx); err != nil {
		return fmt.Errorf("await lease-table ready: %w", err)
	}

	kind := e.adapter.Kind()

	binding, found, err := e.store.FindBinding(profileID, kind, page, listName)
	if err != nil {
		return fmt.Errorf("find binding %s/%s for profile %s: %w",
			page, listName, profileID, err)
	}
	if !found {
		return fmt.Errorf("%w: kind=%s profile=%s page=%s list=%s",
			ErrBindingNotFound, kind, profileID, page, listName)
	}

	if binding.State != connectors.BindingStatePaused {
		// Already active — Resume is idempotent.
		return nil
	}

	pauseDuration := e.clock.Now().Sub(binding.PausedAt)
	if pauseDuration >= resumeFullResyncHorizon {
		key := connectors.BindingKey{
			ProfileID: string(profileID),
			Page:      page,
			ListName:  listName,
		}
		if err := e.runForceFullResync(ctx, key); err != nil {
			return err
		}
		e.logger.Info("connectors/engine: resume_via_full_resync kind=%s profile=%s page=%s list=%s pause_seconds=%.0f",
			kind, string(profileID), page, listName, pauseDuration.Seconds())
		return nil
	}

	checklistKey := connectors.ChecklistKey{Page: page, ListName: listName}
	resumeErr := e.lease.WithChecklistLock(checklistKey, func() error {
		return e.store.WithProfileLock(profileID, func() error {
			active := binding
			active.State = connectors.BindingStateActive
			active.PausedReason = ""
			active.PausedAt = time.Time{}
			// LastSyncedSeq and AdapterState are intentionally preserved —
			// within-horizon resume reuses progress.

			if err := e.store.SaveBinding(profileID, kind, active); err != nil {
				return fmt.Errorf("save resumed binding %s/%s for profile %s: %w",
					page, listName, profileID, err)
			}
			return nil
		})
	})
	if resumeErr != nil {
		return resumeErr
	}

	e.logger.Info("connectors/engine: resume_in_place kind=%s profile=%s page=%s list=%s pause_seconds=%.0f",
		kind, string(profileID), page, listName, pauseDuration.Seconds())
	return nil
}
