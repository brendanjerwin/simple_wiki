package sync

import (
	"context"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// This file holds placeholder implementations of the BackendAdapter
// methods that were grown by Phase 1 of the SyncEngine extraction.
// They exist ONLY to keep this package compiling (and its tests
// passing) during Phases 2 and 3, when the engine is being built and
// tested against an in-memory FakeAdapter — the real Keep adapter is
// not yet wired through these methods.
//
// INVARIANT: none of these methods are reachable at runtime today.
// The SyncEngine that would call them is still under construction in
// internal/connectors/engine; this connector still serves traffic via
// the legacy SyncToKeep / Bind / Unbind paths in connector.go. If any
// of these stubs panic, that means the engine started routing through
// a Connector before its Phase 5 collapse landed — a wiring bug.
//
// Phase 5 collapse: this file is DELETED. Real implementations move to
// internal/connectors/google_keep/adapter.go. Until then, every method
// here is `panic("INVARIANT: ...")` — the explicit "not yet wired"
// signal makes the staging contract clear to readers and surfaces
// any premature engine routing instantly.

const stubInvariant = "INVARIANT: google_keep.Connector BackendAdapter stub called before Phase 5 collapse — engine is not yet routing through this adapter"

// PullRemote is a Phase 1 stub. See the file comment for the invariant.
func (*Connector) PullRemote(_ context.Context, _ connectors.Binding) (connectors.RemotePullResult, error) {
	panic(stubInvariant)
}

// InsertRemote is a Phase 1 stub. See the file comment for the invariant.
func (*Connector) InsertRemote(_ context.Context, _ connectors.Binding, _ connectors.WikiItem) (connectors.RemoteRef, error) {
	panic(stubInvariant)
}

// PatchRemote is a Phase 1 stub. See the file comment for the invariant.
func (*Connector) PatchRemote(_ context.Context, _ connectors.Binding, _ connectors.RemoteRef, _ connectors.WikiItem) (connectors.RemoteRef, error) {
	panic(stubInvariant)
}

// DeleteRemote is a Phase 1 stub. See the file comment for the invariant.
func (*Connector) DeleteRemote(_ context.Context, _ connectors.Binding, _ connectors.RemoteRef) error {
	panic(stubInvariant)
}

// RemoteToWiki is a Phase 1 stub. See the file comment for the invariant.
func (*Connector) RemoteToWiki(_ connectors.RemoteItem) (connectors.WikiItem, error) {
	panic(stubInvariant)
}

// WikiToRemote is a Phase 1 stub. See the file comment for the invariant.
func (*Connector) WikiToRemote(_ connectors.WikiItem) (connectors.RemoteItem, error) {
	panic(stubInvariant)
}

// AdvanceCursor is a Phase 1 stub. See the file comment for the invariant.
func (*Connector) AdvanceCursor(_ connectors.Binding, _ connectors.RemotePullResult) connectors.Binding {
	panic(stubInvariant)
}

// SeedBindingState is a Phase 1 stub. See the file comment for the invariant.
func (*Connector) SeedBindingState(_ context.Context, _ wikipage.PageIdentifier, _ string) (connectors.AdapterState, error) {
	panic(stubInvariant)
}

// ValidateRemoteBinding is a Phase 1 stub. See the file comment for the invariant.
func (*Connector) ValidateRemoteBinding(_ context.Context, _ wikipage.PageIdentifier, _ string) error {
	panic(stubInvariant)
}

// RebuildAdapterState is a Phase 1 stub. See the file comment for the invariant.
func (*Connector) RebuildAdapterState(_ context.Context, _ connectors.Binding) (connectors.AdapterState, error) {
	panic(stubInvariant)
}

// ListRemoteCollections is a Phase 1 stub. See the file comment for the invariant.
func (*Connector) ListRemoteCollections(_ context.Context, _ wikipage.PageIdentifier) ([]connectors.RemoteCollection, error) {
	panic(stubInvariant)
}

// EncodeAdapterState is a Phase 1 stub. See the file comment for the invariant.
func (*Connector) EncodeAdapterState(_ connectors.AdapterState) (map[string]any, error) {
	panic(stubInvariant)
}

// DecodeAdapterState is a Phase 1 stub. See the file comment for the invariant.
func (*Connector) DecodeAdapterState(_ map[string]any) (connectors.AdapterState, error) {
	panic(stubInvariant)
}

// SupportsSubtasks is a Phase 1 stub. Keep doesn't have a parent-child
// hierarchy concept; the post-Phase-5 implementation will return false.
// Returning the correct steady-state value here (rather than panicking)
// is harmless because the engine doesn't yet route through this method
// either — Phase 1's invariant remains "engine isn't routing yet."
func (*Connector) SupportsSubtasks() bool {
	return false
}

// ReadRemoteByRef is a Phase 1 stub. See the file comment for the invariant.
func (*Connector) ReadRemoteByRef(_ context.Context, _ connectors.Binding, _ connectors.RemoteRef) (connectors.RemoteItem, error) {
	panic(stubInvariant)
}

// ClassifyError is a Phase 1 stub. See the file comment for the invariant.
func (*Connector) ClassifyError(_ error) connectors.ErrorClass {
	panic(stubInvariant)
}
