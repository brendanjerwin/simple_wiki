package testing

import (
	"sync"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// FakeBindingStore implements engine.BindingStore for engine-level
// unit tests. Same programmable + recorded-call shape as FakeAdapter:
// configure responses with Set* methods; inspect calls via parallel
// Recorded* slices.
//
// The fake preserves per-profile-lock-then-write ordering: callers
// invoking WithProfileLock(fn) get fn executed serially per profile,
// matching the production BindingStore's contract. Tests asserting the
// "DeleteBinding runs inside WithProfileLock" invariant rely on this.
type FakeBindingStore struct {
	mu sync.Mutex

	// Per-profile lock map. Lazily created on first WithProfileLock for
	// each profile; matches the production store's per-profile mutex.
	profileLocks map[wikipage.PageIdentifier]*sync.Mutex

	// Programmable error responses. When set, the next call to the
	// matching method returns the error (and the recorded-call slice
	// still records the attempt). Cleared after one consumption to
	// match FakeAdapter's queue semantics.
	loadBindingsErrors                []error
	findBindingResponses              []findBindingResponse
	saveBindingErrors                 []error
	deleteBindingErrors               []error
	withProfileLockErrors             []error
	listAllProfilesWithBindingsResps  []listProfilesResponse

	// Recorded calls (parallel slices, ordered by call sequence).
	RecordedLoadBindings                  []recordedLoadBindings
	RecordedFindBinding                   []recordedFindBinding
	RecordedSaveBinding                   []recordedSaveBinding
	RecordedDeleteBinding                 []recordedDeleteBinding
	RecordedWithProfileLock               []wikipage.PageIdentifier
	RecordedListAllProfilesWithBindings   []connectors.ConnectorKind

	// CallOrder records the names of methods called, in order. Tests
	// asserting lock-acquire-before-mutation use this to verify, e.g.,
	// "WithProfileLock fired before DeleteBinding".
	CallOrder []string

	// In-memory binding storage so the fake can answer FindBinding /
	// LoadBindings consistently across a single test scenario without
	// every test re-programming responses.
	bindings map[bindingKey]connectors.Binding
}

type bindingKey struct {
	ProfileID wikipage.PageIdentifier
	Kind      connectors.ConnectorKind
	Page      string
	ListName  string
}

type findBindingResponse struct {
	Binding connectors.Binding
	Found   bool
	Err     error
}

type listProfilesResponse struct {
	Profiles []wikipage.PageIdentifier
	Err      error
}

type recordedLoadBindings struct {
	ProfileID wikipage.PageIdentifier
	Kind      connectors.ConnectorKind
}

type recordedFindBinding struct {
	ProfileID wikipage.PageIdentifier
	Kind      connectors.ConnectorKind
	Page      string
	ListName  string
}

type recordedSaveBinding struct {
	ProfileID wikipage.PageIdentifier
	Kind      connectors.ConnectorKind
	Binding   connectors.Binding
}

type recordedDeleteBinding struct {
	ProfileID wikipage.PageIdentifier
	Kind      connectors.ConnectorKind
	Page      string
	ListName  string
}

// NewFakeBindingStore constructs an empty FakeBindingStore.
func NewFakeBindingStore() *FakeBindingStore {
	return &FakeBindingStore{
		profileLocks: map[wikipage.PageIdentifier]*sync.Mutex{},
		bindings:     map[bindingKey]connectors.Binding{},
	}
}

// SeedBinding installs a binding in the fake's in-memory map so that
// subsequent FindBinding/LoadBindings calls return it. Used by tests
// that want to exercise the "binding exists" branch.
func (f *FakeBindingStore) SeedBinding(b connectors.Binding, kind connectors.ConnectorKind) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bindings[bindingKey{ProfileID: b.ProfileID, Kind: kind, Page: b.Page, ListName: b.ListName}] = b
}

// LoadBindings implements engine.BindingStore.
func (f *FakeBindingStore) LoadBindings(profileID wikipage.PageIdentifier, kind connectors.ConnectorKind) ([]connectors.Binding, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RecordedLoadBindings = append(f.RecordedLoadBindings, recordedLoadBindings{ProfileID: profileID, Kind: kind})
	f.CallOrder = append(f.CallOrder, "LoadBindings")
	if len(f.loadBindingsErrors) > 0 {
		err := f.loadBindingsErrors[0]
		f.loadBindingsErrors = f.loadBindingsErrors[1:]
		if err != nil {
			return nil, err
		}
	}
	var out []connectors.Binding
	for k, b := range f.bindings {
		if k.ProfileID == profileID && k.Kind == kind {
			out = append(out, b)
		}
	}
	return out, nil
}

// SetLoadBindingsError queues an error for the next LoadBindings call.
func (f *FakeBindingStore) SetLoadBindingsError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.loadBindingsErrors = append(f.loadBindingsErrors, err)
}

// FindBinding implements engine.BindingStore.
func (f *FakeBindingStore) FindBinding(profileID wikipage.PageIdentifier, kind connectors.ConnectorKind, page, listName string) (connectors.Binding, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RecordedFindBinding = append(f.RecordedFindBinding, recordedFindBinding{
		ProfileID: profileID, Kind: kind, Page: page, ListName: listName,
	})
	f.CallOrder = append(f.CallOrder, "FindBinding")
	if len(f.findBindingResponses) > 0 {
		r := f.findBindingResponses[0]
		f.findBindingResponses = f.findBindingResponses[1:]
		return r.Binding, r.Found, r.Err
	}
	b, ok := f.bindings[bindingKey{ProfileID: profileID, Kind: kind, Page: page, ListName: listName}]
	return b, ok, nil
}

// SetFindBindingResponse queues a response for the next FindBinding call.
func (f *FakeBindingStore) SetFindBindingResponse(b connectors.Binding, found bool, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.findBindingResponses = append(f.findBindingResponses, findBindingResponse{Binding: b, Found: found, Err: err})
}

// SaveBinding implements engine.BindingStore.
func (f *FakeBindingStore) SaveBinding(profileID wikipage.PageIdentifier, kind connectors.ConnectorKind, binding connectors.Binding) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RecordedSaveBinding = append(f.RecordedSaveBinding, recordedSaveBinding{
		ProfileID: profileID, Kind: kind, Binding: binding,
	})
	f.CallOrder = append(f.CallOrder, "SaveBinding")
	if len(f.saveBindingErrors) > 0 {
		err := f.saveBindingErrors[0]
		f.saveBindingErrors = f.saveBindingErrors[1:]
		if err != nil {
			return err
		}
	}
	f.bindings[bindingKey{ProfileID: profileID, Kind: kind, Page: binding.Page, ListName: binding.ListName}] = binding
	return nil
}

// SetSaveBindingError queues an error for the next SaveBinding call.
func (f *FakeBindingStore) SetSaveBindingError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saveBindingErrors = append(f.saveBindingErrors, err)
}

// DeleteBinding implements engine.BindingStore. Per the contract, it is
// a no-op when the binding does not exist.
func (f *FakeBindingStore) DeleteBinding(profileID wikipage.PageIdentifier, kind connectors.ConnectorKind, page, listName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RecordedDeleteBinding = append(f.RecordedDeleteBinding, recordedDeleteBinding{
		ProfileID: profileID, Kind: kind, Page: page, ListName: listName,
	})
	f.CallOrder = append(f.CallOrder, "DeleteBinding")
	if len(f.deleteBindingErrors) > 0 {
		err := f.deleteBindingErrors[0]
		f.deleteBindingErrors = f.deleteBindingErrors[1:]
		if err != nil {
			return err
		}
	}
	delete(f.bindings, bindingKey{ProfileID: profileID, Kind: kind, Page: page, ListName: listName})
	return nil
}

// SetDeleteBindingError queues an error for the next DeleteBinding call.
func (f *FakeBindingStore) SetDeleteBindingError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleteBindingErrors = append(f.deleteBindingErrors, err)
}

// WithProfileLock implements engine.BindingStore. Acquires the
// per-profile mutex, runs fn, and releases. If a programmable error is
// queued, it is returned BEFORE fn is invoked (matching production:
// failure to acquire the lock means the critical section never runs).
func (f *FakeBindingStore) WithProfileLock(profileID wikipage.PageIdentifier, fn func() error) error {
	f.mu.Lock()
	f.RecordedWithProfileLock = append(f.RecordedWithProfileLock, profileID)
	f.CallOrder = append(f.CallOrder, "WithProfileLock")
	var queuedErr error
	if len(f.withProfileLockErrors) > 0 {
		queuedErr = f.withProfileLockErrors[0]
		f.withProfileLockErrors = f.withProfileLockErrors[1:]
	}
	mu, ok := f.profileLocks[profileID]
	if !ok {
		mu = &sync.Mutex{}
		f.profileLocks[profileID] = mu
	}
	f.mu.Unlock()

	if queuedErr != nil {
		return queuedErr
	}
	mu.Lock()
	defer mu.Unlock()
	return fn()
}

// SetWithProfileLockError queues an error for the next WithProfileLock
// call (returned before fn is invoked).
func (f *FakeBindingStore) SetWithProfileLockError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.withProfileLockErrors = append(f.withProfileLockErrors, err)
}

// ListAllProfilesWithBindings implements engine.BindingStore.
func (f *FakeBindingStore) ListAllProfilesWithBindings(kind connectors.ConnectorKind) ([]wikipage.PageIdentifier, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.RecordedListAllProfilesWithBindings = append(f.RecordedListAllProfilesWithBindings, kind)
	f.CallOrder = append(f.CallOrder, "ListAllProfilesWithBindings")
	if len(f.listAllProfilesWithBindingsResps) > 0 {
		r := f.listAllProfilesWithBindingsResps[0]
		f.listAllProfilesWithBindingsResps = f.listAllProfilesWithBindingsResps[1:]
		return r.Profiles, r.Err
	}
	seen := map[wikipage.PageIdentifier]struct{}{}
	var out []wikipage.PageIdentifier
	for k := range f.bindings {
		if k.Kind != kind {
			continue
		}
		if _, ok := seen[k.ProfileID]; ok {
			continue
		}
		seen[k.ProfileID] = struct{}{}
		out = append(out, k.ProfileID)
	}
	return out, nil
}

// SetListAllProfilesWithBindingsResponse queues a response for the
// next ListAllProfilesWithBindings call.
func (f *FakeBindingStore) SetListAllProfilesWithBindingsResponse(profiles []wikipage.PageIdentifier, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listAllProfilesWithBindingsResps = append(f.listAllProfilesWithBindingsResps, listProfilesResponse{Profiles: profiles, Err: err})
}
