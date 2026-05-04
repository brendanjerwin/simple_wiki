package engine

import (
	"context"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// FetchRemoteListTitle returns the current display title of the remote
// list bound to the given handle. Per MATRIX.md row 11, this is a
// thin pass-through to the adapter — the engine's only role is to
// observe the result and update binding.RemoteListTitle on the next
// SaveBinding (via reconcile.go's tail).
//
// Phase 2 status: pass-through stub. Phase 3 wires the post-tick
// title-sync update into reconcile.go.
func (e *Engine) FetchRemoteListTitle(ctx context.Context, profileID wikipage.PageIdentifier, remoteHandle string) (string, bool, error) {
	return e.adapter.FetchRemoteListTitle(ctx, profileID, remoteHandle)
}
