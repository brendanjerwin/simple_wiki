package engine

import (
	"context"
	"fmt"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// Unbind removes a wiki checklist's binding to a remote list per
// ADR-0011. Strongly consistent under the per-checklist mutex.
//
// Algorithm (per MATRIX.md row 3 + ADR-0011 operation contracts):
//
//  1. Acquire the per-checklist mutex on (page, listName) via
//     LeaseTable.WithChecklistLock — serializes against concurrent
//     Bind / Unbind for the same checklist.
//  2. Inside the checklist mutex, acquire the per-profile lock via
//     BindingStore.WithProfileLock — serializes against concurrent
//     binding writes on the same profile page.
//  3. Inside both locks, call BindingStore.DeleteBinding. Per the
//     store's contract, DeleteBinding is a no-op when no matching
//     binding exists, so Unbind is idempotent.
//  4. On store success, release the lease via LeaseTable.Release.
//     On store failure, leave the lease untouched (the binding may
//     still be present on the profile, so the lease stays accurate).
//
// Note: Unbind does NOT delete the remote list (Tasks tasklist or
// Keep note). Those are user-managed in their respective backends —
// the wiki only manages the binding record on its own side.
func (e *Engine) Unbind(ctx context.Context, profileID wikipage.PageIdentifier, page, listName string) error {
	if err := e.lease.WaitReady(ctx); err != nil {
		return fmt.Errorf("await lease-table ready: %w", err)
	}

	checklistKey := connectors.ChecklistKey{Page: page, ListName: listName}
	kind := e.adapter.Kind()

	return e.lease.WithChecklistLock(checklistKey, func() error {
		profileLockErr := e.store.WithProfileLock(profileID, func() error {
			if err := e.store.DeleteBinding(profileID, kind, page, listName); err != nil {
				return fmt.Errorf("delete binding %s/%s for profile %s: %w",
					page, listName, profileID, err)
			}
			return nil
		})
		if profileLockErr != nil {
			return profileLockErr
		}
		e.lease.Release(checklistKey)
		e.logger.Info("connectors/engine: unbind kind=%s profile=%s page=%s list=%s",
			kind, string(profileID), page, listName)
		return nil
	})
}
