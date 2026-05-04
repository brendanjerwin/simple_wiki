package engine

import (
	"context"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// Unbind removes a wiki checklist's binding to a remote list per
// ADR-0011. Strongly consistent under the per-checklist mutex.
//
// Algorithm (per MATRIX.md row 3 + ADR-0011 operation contracts):
//
//  1. Acquire the per-checklist mutex on (page, listName).
//  2. Acquire BindingStore.WithProfileLock; remove the Binding entry
//     under wiki.connectors.<kind>.bindings[].
//  3. LeaseTable.ReleaseLease — drop the (page, listName) entry.
//  4. Release the per-checklist mutex.
//
// Note: Unbind does NOT delete the remote list (Tasks tasklist or
// Keep note). Those are user-managed in their respective backends —
// the wiki only manages the binding record on its own side.
//
// Phase 2 status: stub.
func (*Engine) Unbind(_ context.Context, _ wikipage.PageIdentifier, _, _ string) error {
	return ErrNotYetImplemented
}
