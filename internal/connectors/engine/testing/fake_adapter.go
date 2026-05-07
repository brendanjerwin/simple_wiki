// Package testing provides an in-memory FakeAdapter for engine-level
// unit tests. Per ADR-0015's "engine-level rule tests don't depend on
// either Keep or Tasks fakes" directive, this fake is the single test
// double the engine package's *_test.go files use to exercise reconcile,
// bind, unbind, force-resync, resume, precondition-recovery, and
// dead-letter behavior in isolation.
//
// The fake is deliberately programmable: tests configure each method's
// return values via fields on the struct, and inspect call counts /
// arguments via parallel "Recorded*" slices. This is the same pattern
// the existing per-connector test fakes use; reviewers familiar with
// google_tasks/sync/fakes_for_test.go will find this shape comfortable.
package testing

import (
	"context"
	"errors"
	"sync"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// FakeAdapter implements connectors.BackendAdapter for engine tests.
// All methods are programmable via the matching Set* configuration
// or the per-method response slices; all calls are recorded for
// after-the-fact assertion.
//
// Usage:
//
//	fa := &FakeAdapter{ConnectorKind: connectors.ConnectorKindGoogleTasks}
//	fa.SetPullRemoteResponse(connectors.RemotePullResult{...}, nil)
//	engine, _ := engine.NewEngine(fa, leaseTable, ...)
//	_ = engine.Sync(ctx, key)
//	require.Equal(t, 1, len(fa.RecordedPullRemote))
type FakeAdapter struct {
	mu sync.Mutex

	// ConnectorKind is the kind this fake reports via Kind().
	ConnectorKind connectors.ConnectorKind

	// SubtasksSupport is what SupportsSubtasks() returns.
	SubtasksSupport bool

	// Per-method programmable responses. When the slice has entries,
	// each call shifts the front; when empty, the per-method default
	// (zero-value + nil error) is returned.
	pullRemoteResponses           []pullRemoteResponse
	insertRemoteResponses         []refResponse
	patchRemoteResponses          []refResponse
	deleteRemoteResponses         []errResponse
	remoteToWikiResponses         []wikiItemResponse
	wikiToRemoteResponses         []remoteItemResponse
	advanceCursorResponses        []connectors.Binding
	syncCollectionStateResponses  []syncCollectionStateResponse
	seedBindingStateResponses     []adapterStateResponse
	validateRemoteBindingResponses []errResponse
	rebuildAdapterStateResponses  []adapterStateResponse
	listRemoteCollectionsResponses []listCollectionsResponse
	titleSyncResponses            []titleResponse
	encodeAdapterStateResponses   []encodeResponse
	decodeAdapterStateResponses   []adapterStateResponse
	readRemoteByRefResponses      []remoteItemResponse
	classifyErrorResponses        []connectors.ErrorClass

	// Recorded calls (parallel slices, ordered by call sequence).
	RecordedPullRemote            []recordedPullRemote
	RecordedInsertRemote          []recordedInsertRemote
	RecordedPatchRemote           []recordedPatchRemote
	RecordedDeleteRemote          []recordedDeleteRemote
	RecordedRemoteToWiki          []connectors.RemoteItem
	RecordedWikiToRemote          []connectors.WikiItem
	RecordedAdvanceCursor         []recordedAdvanceCursor
	RecordedSeedBindingState      []recordedSeedBindingState
	RecordedValidateRemoteBinding []recordedValidateRemoteBinding
	RecordedRebuildAdapterState   []connectors.Binding
	RecordedListRemoteCollections []wikipage.PageIdentifier
	RecordedFetchRemoteListTitle  []recordedTitleSync
	RecordedEncodeAdapterState    []connectors.AdapterState
	RecordedDecodeAdapterState    []map[string]any
	RecordedReadRemoteByRef       []recordedReadRemoteByRef
	RecordedClassifyError         []error
	RecordedRefreshItemBaseline   []recordedRefreshItemBaseline
	RecordedSyncCollectionState   []recordedSyncCollectionState
}

type pullRemoteResponse struct {
	Result connectors.RemotePullResult
	Err    error
}

type refResponse struct {
	Ref connectors.RemoteRef
	Err error
}

type errResponse struct {
	Err error
}

type wikiItemResponse struct {
	Item connectors.WikiItem
	Err  error
}

type remoteItemResponse struct {
	Item connectors.RemoteItem
	Err  error
}

type adapterStateResponse struct {
	State connectors.AdapterState
	Err   error
}

type listCollectionsResponse struct {
	Collections []connectors.RemoteCollection
	Err         error
}

type titleResponse struct {
	Title string
	OK    bool
	Err   error
}

type encodeResponse struct {
	Raw map[string]any
	Err error
}

type recordedPullRemote struct {
	Binding connectors.Binding
}

type recordedInsertRemote struct {
	Binding connectors.Binding
	Item    connectors.WikiItem
}

type recordedPatchRemote struct {
	Binding connectors.Binding
	Ref     connectors.RemoteRef
	Item    connectors.WikiItem
}

type recordedDeleteRemote struct {
	Binding connectors.Binding
	Ref     connectors.RemoteRef
}

type recordedAdvanceCursor struct {
	Binding connectors.Binding
	Result  connectors.RemotePullResult
}

type recordedSeedBindingState struct {
	ProfileID    wikipage.PageIdentifier
	RemoteHandle string
}

type recordedValidateRemoteBinding struct {
	ProfileID    wikipage.PageIdentifier
	RemoteHandle string
}

type recordedTitleSync struct {
	ProfileID    wikipage.PageIdentifier
	RemoteHandle string
}

type recordedReadRemoteByRef struct {
	Binding connectors.Binding
	Ref     connectors.RemoteRef
}

// Kind implements connectors.BackendAdapter.
func (f *FakeAdapter) Kind() connectors.ConnectorKind {
	return f.ConnectorKind
}

// SupportsSubtasks implements connectors.BackendAdapter.
func (f *FakeAdapter) SupportsSubtasks() bool {
	return f.SubtasksSupport
}

// PullRemote implements connectors.BackendAdapter.
func (f *FakeAdapter) PullRemote(_ context.Context, binding connectors.Binding) (connectors.RemotePullResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RecordedPullRemote = append(f.RecordedPullRemote, recordedPullRemote{Binding: binding})
	if len(f.pullRemoteResponses) == 0 {
		return connectors.RemotePullResult{}, nil
	}
	r := f.pullRemoteResponses[0]
	f.pullRemoteResponses = f.pullRemoteResponses[1:]
	return r.Result, r.Err
}

// SetPullRemoteResponse queues a response for the next PullRemote call.
func (f *FakeAdapter) SetPullRemoteResponse(result connectors.RemotePullResult, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pullRemoteResponses = append(f.pullRemoteResponses, pullRemoteResponse{Result: result, Err: err})
}

// InsertRemote implements connectors.BackendAdapter.
func (f *FakeAdapter) InsertRemote(_ context.Context, binding connectors.Binding, item connectors.WikiItem) (connectors.RemoteRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RecordedInsertRemote = append(f.RecordedInsertRemote, recordedInsertRemote{Binding: binding, Item: item})
	if len(f.insertRemoteResponses) == 0 {
		return "", nil
	}
	r := f.insertRemoteResponses[0]
	f.insertRemoteResponses = f.insertRemoteResponses[1:]
	return r.Ref, r.Err
}

// SetInsertRemoteResponse queues a response for the next InsertRemote call.
func (f *FakeAdapter) SetInsertRemoteResponse(ref connectors.RemoteRef, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.insertRemoteResponses = append(f.insertRemoteResponses, refResponse{Ref: ref, Err: err})
}

// PatchRemote implements connectors.BackendAdapter.
func (f *FakeAdapter) PatchRemote(_ context.Context, binding connectors.Binding, ref connectors.RemoteRef, item connectors.WikiItem) (connectors.RemoteRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RecordedPatchRemote = append(f.RecordedPatchRemote, recordedPatchRemote{Binding: binding, Ref: ref, Item: item})
	if len(f.patchRemoteResponses) == 0 {
		return ref, nil
	}
	r := f.patchRemoteResponses[0]
	f.patchRemoteResponses = f.patchRemoteResponses[1:]
	return r.Ref, r.Err
}

// SetPatchRemoteResponse queues a response for the next PatchRemote call.
func (f *FakeAdapter) SetPatchRemoteResponse(ref connectors.RemoteRef, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.patchRemoteResponses = append(f.patchRemoteResponses, refResponse{Ref: ref, Err: err})
}

// DeleteRemote implements connectors.BackendAdapter.
func (f *FakeAdapter) DeleteRemote(_ context.Context, binding connectors.Binding, ref connectors.RemoteRef) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RecordedDeleteRemote = append(f.RecordedDeleteRemote, recordedDeleteRemote{Binding: binding, Ref: ref})
	if len(f.deleteRemoteResponses) == 0 {
		return nil
	}
	r := f.deleteRemoteResponses[0]
	f.deleteRemoteResponses = f.deleteRemoteResponses[1:]
	return r.Err
}

// SetDeleteRemoteResponse queues a response for the next DeleteRemote call.
func (f *FakeAdapter) SetDeleteRemoteResponse(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteRemoteResponses = append(f.deleteRemoteResponses, errResponse{Err: err})
}

// RemoteToWiki implements connectors.BackendAdapter.
func (f *FakeAdapter) RemoteToWiki(remote connectors.RemoteItem) (connectors.WikiItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RecordedRemoteToWiki = append(f.RecordedRemoteToWiki, remote)
	if len(f.remoteToWikiResponses) == 0 {
		// Default: passthrough — title→text, notes→description, status string ignored.
		return connectors.WikiItem{Text: remote.Title, Description: remote.Notes}, nil
	}
	r := f.remoteToWikiResponses[0]
	f.remoteToWikiResponses = f.remoteToWikiResponses[1:]
	return r.Item, r.Err
}

// SetRemoteToWikiResponse queues a response for the next RemoteToWiki call.
func (f *FakeAdapter) SetRemoteToWikiResponse(item connectors.WikiItem, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.remoteToWikiResponses = append(f.remoteToWikiResponses, wikiItemResponse{Item: item, Err: err})
}

// WikiToRemote implements connectors.BackendAdapter.
func (f *FakeAdapter) WikiToRemote(wiki connectors.WikiItem) (connectors.RemoteItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RecordedWikiToRemote = append(f.RecordedWikiToRemote, wiki)
	if len(f.wikiToRemoteResponses) == 0 {
		return connectors.RemoteItem{Title: wiki.Text, Notes: wiki.Description}, nil
	}
	r := f.wikiToRemoteResponses[0]
	f.wikiToRemoteResponses = f.wikiToRemoteResponses[1:]
	return r.Item, r.Err
}

// SetWikiToRemoteResponse queues a response for the next WikiToRemote call.
func (f *FakeAdapter) SetWikiToRemoteResponse(item connectors.RemoteItem, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.wikiToRemoteResponses = append(f.wikiToRemoteResponses, remoteItemResponse{Item: item, Err: err})
}

// AdvanceCursor implements connectors.BackendAdapter.
func (f *FakeAdapter) AdvanceCursor(binding connectors.Binding, result connectors.RemotePullResult) connectors.Binding {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RecordedAdvanceCursor = append(f.RecordedAdvanceCursor, recordedAdvanceCursor{Binding: binding, Result: result})
	if len(f.advanceCursorResponses) == 0 {
		// Default: pass binding through untouched. Tests that care
		// about cursor advance configure responses explicitly.
		return binding
	}
	r := f.advanceCursorResponses[0]
	f.advanceCursorResponses = f.advanceCursorResponses[1:]
	return r
}

// SetAdvanceCursorResponse queues a response for the next AdvanceCursor call.
func (f *FakeAdapter) SetAdvanceCursorResponse(binding connectors.Binding) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.advanceCursorResponses = append(f.advanceCursorResponses, binding)
}

// recordedRefreshItemBaseline captures one RefreshItemBaseline call.
type recordedRefreshItemBaseline struct {
	Binding connectors.Binding
	Remote  connectors.RemoteItem
}

// recordedSyncCollectionState captures one SyncCollectionState call.
type recordedSyncCollectionState struct {
	Binding connectors.Binding
	Items   []connectors.WikiItem
}

// SyncCollectionState implements connectors.BackendAdapter. Default
// behavior: pass binding through untouched while recording the call.
func (f *FakeAdapter) SyncCollectionState(_ context.Context, binding connectors.Binding, items []connectors.WikiItem) (connectors.Binding, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RecordedSyncCollectionState = append(f.RecordedSyncCollectionState, recordedSyncCollectionState{
		Binding: binding,
		Items:   append([]connectors.WikiItem(nil), items...),
	})
	if len(f.syncCollectionStateResponses) > 0 {
		r := f.syncCollectionStateResponses[0]
		f.syncCollectionStateResponses = f.syncCollectionStateResponses[1:]
		return r.Binding, r.Err
	}
	return binding, nil
}

type syncCollectionStateResponse struct {
	Binding connectors.Binding
	Err     error
}

// SetSyncCollectionStateResponse queues a response for the next
// SyncCollectionState call.
func (f *FakeAdapter) SetSyncCollectionStateResponse(binding connectors.Binding, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.syncCollectionStateResponses = append(f.syncCollectionStateResponses, syncCollectionStateResponse{Binding: binding, Err: err})
}

// RefreshItemBaseline implements connectors.BackendAdapter. Default
// behavior: pass binding through untouched while recording the call.
// Tests that care about the baseline-refresh side-effect can inspect
// RecordedRefreshItemBaseline.
func (f *FakeAdapter) RefreshItemBaseline(binding connectors.Binding, remote connectors.RemoteItem) connectors.Binding {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RecordedRefreshItemBaseline = append(f.RecordedRefreshItemBaseline, recordedRefreshItemBaseline{Binding: binding, Remote: remote})
	return binding
}

// SeedBindingState implements connectors.BackendAdapter.
func (f *FakeAdapter) SeedBindingState(_ context.Context, profileID wikipage.PageIdentifier, remoteHandle string) (connectors.AdapterState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RecordedSeedBindingState = append(f.RecordedSeedBindingState, recordedSeedBindingState{ProfileID: profileID, RemoteHandle: remoteHandle})
	if len(f.seedBindingStateResponses) == 0 {
		return connectors.AdapterState{}, nil
	}
	r := f.seedBindingStateResponses[0]
	f.seedBindingStateResponses = f.seedBindingStateResponses[1:]
	return r.State, r.Err
}

// SetSeedBindingStateResponse queues a response for the next SeedBindingState call.
func (f *FakeAdapter) SetSeedBindingStateResponse(state connectors.AdapterState, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seedBindingStateResponses = append(f.seedBindingStateResponses, adapterStateResponse{State: state, Err: err})
}

// ValidateRemoteBinding implements connectors.BackendAdapter.
func (f *FakeAdapter) ValidateRemoteBinding(_ context.Context, profileID wikipage.PageIdentifier, remoteHandle string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RecordedValidateRemoteBinding = append(f.RecordedValidateRemoteBinding, recordedValidateRemoteBinding{ProfileID: profileID, RemoteHandle: remoteHandle})
	if len(f.validateRemoteBindingResponses) == 0 {
		return nil
	}
	r := f.validateRemoteBindingResponses[0]
	f.validateRemoteBindingResponses = f.validateRemoteBindingResponses[1:]
	return r.Err
}

// SetValidateRemoteBindingResponse queues a response for the next call.
func (f *FakeAdapter) SetValidateRemoteBindingResponse(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.validateRemoteBindingResponses = append(f.validateRemoteBindingResponses, errResponse{Err: err})
}

// RebuildAdapterState implements connectors.BackendAdapter.
func (f *FakeAdapter) RebuildAdapterState(_ context.Context, binding connectors.Binding) (connectors.AdapterState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RecordedRebuildAdapterState = append(f.RecordedRebuildAdapterState, binding)
	if len(f.rebuildAdapterStateResponses) == 0 {
		return connectors.AdapterState{}, nil
	}
	r := f.rebuildAdapterStateResponses[0]
	f.rebuildAdapterStateResponses = f.rebuildAdapterStateResponses[1:]
	return r.State, r.Err
}

// SetRebuildAdapterStateResponse queues a response for the next call.
func (f *FakeAdapter) SetRebuildAdapterStateResponse(state connectors.AdapterState, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rebuildAdapterStateResponses = append(f.rebuildAdapterStateResponses, adapterStateResponse{State: state, Err: err})
}

// ListRemoteCollections implements connectors.BackendAdapter.
func (f *FakeAdapter) ListRemoteCollections(_ context.Context, profileID wikipage.PageIdentifier) ([]connectors.RemoteCollection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RecordedListRemoteCollections = append(f.RecordedListRemoteCollections, profileID)
	if len(f.listRemoteCollectionsResponses) == 0 {
		return nil, nil
	}
	r := f.listRemoteCollectionsResponses[0]
	f.listRemoteCollectionsResponses = f.listRemoteCollectionsResponses[1:]
	return r.Collections, r.Err
}

// SetListRemoteCollectionsResponse queues a response for the next call.
func (f *FakeAdapter) SetListRemoteCollectionsResponse(cols []connectors.RemoteCollection, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listRemoteCollectionsResponses = append(f.listRemoteCollectionsResponses, listCollectionsResponse{Collections: cols, Err: err})
}

// FetchRemoteListTitle implements connectors.BackendAdapter.
func (f *FakeAdapter) FetchRemoteListTitle(_ context.Context, profileID wikipage.PageIdentifier, remoteHandle string) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RecordedFetchRemoteListTitle = append(f.RecordedFetchRemoteListTitle, recordedTitleSync{ProfileID: profileID, RemoteHandle: remoteHandle})
	if len(f.titleSyncResponses) == 0 {
		return "", false, nil
	}
	r := f.titleSyncResponses[0]
	f.titleSyncResponses = f.titleSyncResponses[1:]
	return r.Title, r.OK, r.Err
}

// SetFetchRemoteListTitleResponse queues a response for the next call.
func (f *FakeAdapter) SetFetchRemoteListTitleResponse(title string, ok bool, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.titleSyncResponses = append(f.titleSyncResponses, titleResponse{Title: title, OK: ok, Err: err})
}

// EncodeAdapterState implements connectors.BackendAdapter.
func (f *FakeAdapter) EncodeAdapterState(state connectors.AdapterState) (map[string]any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RecordedEncodeAdapterState = append(f.RecordedEncodeAdapterState, state)
	if len(f.encodeAdapterStateResponses) == 0 {
		// Default: cast through (AdapterState is map[string]any).
		return map[string]any(state), nil
	}
	r := f.encodeAdapterStateResponses[0]
	f.encodeAdapterStateResponses = f.encodeAdapterStateResponses[1:]
	return r.Raw, r.Err
}

// SetEncodeAdapterStateResponse queues a response for the next call.
func (f *FakeAdapter) SetEncodeAdapterStateResponse(raw map[string]any, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.encodeAdapterStateResponses = append(f.encodeAdapterStateResponses, encodeResponse{Raw: raw, Err: err})
}

// DecodeAdapterState implements connectors.BackendAdapter.
func (f *FakeAdapter) DecodeAdapterState(raw map[string]any) (connectors.AdapterState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RecordedDecodeAdapterState = append(f.RecordedDecodeAdapterState, raw)
	if len(f.decodeAdapterStateResponses) == 0 {
		return connectors.AdapterState(raw), nil
	}
	r := f.decodeAdapterStateResponses[0]
	f.decodeAdapterStateResponses = f.decodeAdapterStateResponses[1:]
	return r.State, r.Err
}

// SetDecodeAdapterStateResponse queues a response for the next call.
func (f *FakeAdapter) SetDecodeAdapterStateResponse(state connectors.AdapterState, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.decodeAdapterStateResponses = append(f.decodeAdapterStateResponses, adapterStateResponse{State: state, Err: err})
}

// ReadRemoteByRef implements connectors.BackendAdapter.
func (f *FakeAdapter) ReadRemoteByRef(_ context.Context, binding connectors.Binding, ref connectors.RemoteRef) (connectors.RemoteItem, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RecordedReadRemoteByRef = append(f.RecordedReadRemoteByRef, recordedReadRemoteByRef{Binding: binding, Ref: ref})
	if len(f.readRemoteByRefResponses) == 0 {
		return connectors.RemoteItem{Ref: ref}, nil
	}
	r := f.readRemoteByRefResponses[0]
	f.readRemoteByRefResponses = f.readRemoteByRefResponses[1:]
	return r.Item, r.Err
}

// SetReadRemoteByRefResponse queues a response for the next call.
func (f *FakeAdapter) SetReadRemoteByRefResponse(item connectors.RemoteItem, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.readRemoteByRefResponses = append(f.readRemoteByRefResponses, remoteItemResponse{Item: item, Err: err})
}

// ClassifyError implements connectors.BackendAdapter.
//
// Default behavior: nil error → ErrorClassNone; any error → looks up
// the next queued response, falling back to ErrorClassTransient if no
// response is queued. Tests configure specific classifications via
// SetClassifyErrorResponse for the errors they care about.
func (f *FakeAdapter) ClassifyError(err error) connectors.ErrorClass {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RecordedClassifyError = append(f.RecordedClassifyError, err)
	if err == nil {
		return connectors.ErrorClassNone
	}
	if len(f.classifyErrorResponses) == 0 {
		return connectors.ErrorClassTransient
	}
	c := f.classifyErrorResponses[0]
	f.classifyErrorResponses = f.classifyErrorResponses[1:]
	return c
}

// SetClassifyErrorResponse queues a classification for the next ClassifyError call.
func (f *FakeAdapter) SetClassifyErrorResponse(class connectors.ErrorClass) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.classifyErrorResponses = append(f.classifyErrorResponses, class)
}

// Compile-time assertion: FakeAdapter satisfies the full
// BackendAdapter contract. If Phase 1's adapter.go grows a method,
// this assertion fails and the test build breaks until the fake is
// extended — same compile-error guarantee that protects production
// adapters.
var _ connectors.BackendAdapter = (*FakeAdapter)(nil)

// Convenience constructor errors used by tests.
var (
	// ErrFakeConfigured is a sentinel some tests use when programming
	// the fake to return "the call happened but the adapter said no."
	ErrFakeConfigured = errors.New("fake adapter: programmed error")
)
