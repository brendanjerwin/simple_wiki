package engine

import (
	"context"
	"fmt"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// FetchRemoteListTitle returns the current display title of the remote
// list bound to the given handle. Per MATRIX.md row 11, this is a
// thin pass-through to the adapter — the engine's only role is to
// observe the result and update binding.RemoteListTitle on the next
// SaveBinding (via reconcile.go's tail).
func (e *Engine) FetchRemoteListTitle(ctx context.Context, profileID wikipage.PageIdentifier, remoteHandle string) (string, bool, error) {
	return e.adapter.FetchRemoteListTitle(ctx, profileID, remoteHandle)
}

// ListRemoteCollections enumerates every candidate remote list/note the
// authenticated profile owns, so the bind UI can present a picker. Per
// MATRIX.md row 19, this is a thin pass-through to the adapter — the
// engine's role is purely to expose the adapter primitive on the same
// dispatch surface every connector reaches via the gRPC ConnectorService.
//
// CollectionCapabilities lets the UI gate selection (e.g., gray out
// Tasks lists with subtasks). The engine does not filter — the UI
// decides.
func (e *Engine) ListRemoteCollections(ctx context.Context, profileID wikipage.PageIdentifier) ([]connectors.RemoteCollection, error) {
	cols, err := e.adapter.ListRemoteCollections(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("list remote collections for kind=%s profile=%s: %w",
			e.adapter.Kind(), profileID, err)
	}
	return cols, nil
}
