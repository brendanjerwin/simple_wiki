//revive:disable:dot-imports
package engine_test

import (
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/engine"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// fakeFrontmatterReadWriter is the in-memory FrontmatterReadWriter the
// binding store tests inject. Mirrors the legacy SubscriptionStore's
// fakePages: deep-copies on read+write so tests can mutate captured
// snapshots without bleeding into the store's view of the world.
type fakeFrontmatterReadWriter struct {
	mu        sync.Mutex
	pages     map[wikipage.PageIdentifier]wikipage.FrontMatter
	readErr   map[wikipage.PageIdentifier]error
	writeErr  map[wikipage.PageIdentifier]error
	writeHook func(wikipage.PageIdentifier, wikipage.FrontMatter) // optional, called during Write while lock is held
}

func newFakeFrontmatterReadWriter() *fakeFrontmatterReadWriter {
	return &fakeFrontmatterReadWriter{
		pages:    map[wikipage.PageIdentifier]wikipage.FrontMatter{},
		readErr:  map[wikipage.PageIdentifier]error{},
		writeErr: map[wikipage.PageIdentifier]error{},
	}
}

func (f *fakeFrontmatterReadWriter) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err, ok := f.readErr[id]; ok && err != nil {
		return id, nil, err
	}
	fm, ok := f.pages[id]
	if !ok {
		return id, nil, os.ErrNotExist
	}
	return id, deepCopyFrontMatter(fm), nil
}

func (f *fakeFrontmatterReadWriter) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err, ok := f.writeErr[id]; ok && err != nil {
		return err
	}
	f.pages[id] = deepCopyFrontMatter(fm)
	if f.writeHook != nil {
		f.writeHook(id, deepCopyFrontMatter(fm))
	}
	return nil
}

// SetReadError programs the next ReadFrontMatter for id to return err.
func (f *fakeFrontmatterReadWriter) SetReadError(id wikipage.PageIdentifier, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.readErr[id] = err
}

// SetWriteError programs the next WriteFrontMatter for id to return err.
func (f *fakeFrontmatterReadWriter) SetWriteError(id wikipage.PageIdentifier, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writeErr[id] = err
}

// Seed inserts a frontmatter map for id without going through Write.
func (f *fakeFrontmatterReadWriter) Seed(id wikipage.PageIdentifier, fm wikipage.FrontMatter) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pages[id] = deepCopyFrontMatter(fm)
}

// Snapshot returns a deep copy of id's current frontmatter.
func (f *fakeFrontmatterReadWriter) Snapshot(id wikipage.PageIdentifier) wikipage.FrontMatter {
	f.mu.Lock()
	defer f.mu.Unlock()
	return deepCopyFrontMatter(f.pages[id])
}

func deepCopyFrontMatter(fm wikipage.FrontMatter) wikipage.FrontMatter {
	if fm == nil {
		return nil
	}
	out := make(wikipage.FrontMatter, len(fm))
	for k, v := range fm {
		out[k] = deepCopyAnyValue(v)
	}
	return out
}

func deepCopyAnyValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[k] = deepCopyAnyValue(vv)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, vv := range x {
			out[i] = deepCopyAnyValue(vv)
		}
		return out
	default:
		return v
	}
}

// fakeProfileLister is the programmable ProfileLister the binding
// store tests inject. Records every queried key.
type fakeProfileLister struct {
	mu          sync.Mutex
	profiles    map[wikipage.DottedKeyPath][]wikipage.PageIdentifier
	QueriedKeys []wikipage.DottedKeyPath
}

func newFakeProfileLister() *fakeProfileLister {
	return &fakeProfileLister{
		profiles: map[wikipage.DottedKeyPath][]wikipage.PageIdentifier{},
	}
}

func (f *fakeProfileLister) ListProfilesWithKey(key wikipage.DottedKeyPath) []wikipage.PageIdentifier {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.QueriedKeys = append(f.QueriedKeys, key)
	out := f.profiles[key]
	cp := make([]wikipage.PageIdentifier, len(out))
	copy(cp, out)
	return cp
}

// Set programs the response for key.
func (f *fakeProfileLister) Set(key wikipage.DottedKeyPath, profiles []wikipage.PageIdentifier) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]wikipage.PageIdentifier, len(profiles))
	copy(cp, profiles)
	f.profiles[key] = cp
}

// connectorSubtree navigates a frontmatter snapshot to
// wiki.connectors.<kind> and returns it. Centralizes the unchecked
// type assertions so individual specs don't have to suppress lint per
// site.
//
//revive:disable-next-line:unchecked-type-assertion
func connectorSubtree(fm wikipage.FrontMatter, kind string) map[string]any {
	wiki, ok := fm["wiki"].(map[string]any)
	Expect(ok).To(BeTrue(), "frontmatter has no wiki map")
	conns, ok := wiki["connectors"].(map[string]any)
	Expect(ok).To(BeTrue(), "wiki has no connectors map")
	c, ok := conns[kind].(map[string]any)
	Expect(ok).To(BeTrue(), "connectors has no %s map", kind)
	return c
}

// connectorBindings extracts the bindings[] slice from the
// connector subtree.
//
//revive:disable-next-line:unchecked-type-assertion
func connectorBindings(connector map[string]any) []any {
	v, ok := connector["bindings"].([]any)
	Expect(ok).To(BeTrue(), "connector has no bindings list")
	return v
}

// bindingEntry treats one element of a bindings[] slice as a map.
//
//revive:disable-next-line:unchecked-type-assertion
func bindingEntry(entry any) map[string]any {
	m, ok := entry.(map[string]any)
	Expect(ok).To(BeTrue(), "binding entry is not a map")
	return m
}

// adapterStateOf extracts the adapter_state subtree from a binding entry.
//
//revive:disable-next-line:unchecked-type-assertion
func adapterStateOf(entry map[string]any) map[string]any {
	m, ok := entry["adapter_state"].(map[string]any)
	Expect(ok).To(BeTrue(), "binding entry has no adapter_state map")
	return m
}

// silentLogger is a Logger that swallows messages — the binding store
// may emit Info/Warn/Error lines, but tests don't assert on log content.
type silentLogger struct{}

func (silentLogger) Info(_ string, _ ...any)  {}
func (silentLogger) Warn(_ string, _ ...any)  {}
func (silentLogger) Error(_ string, _ ...any) {}

// bindingStoreFixedTime is a deterministic timestamp used across the
// binding-store tests so equality can be asserted byte-for-byte.
func bindingStoreFixedTime(seconds int) time.Time {
	return time.Date(2026, 5, 4, 14, 0, 0, 0, time.UTC).Add(time.Duration(seconds) * time.Second)
}

const (
	bindingStoreProfileAlice wikipage.PageIdentifier = "profile_alice"
	bindingStoreProfileBob   wikipage.PageIdentifier = "profile_bob"
)

var _ = Describe("FrontmatterBindingStore", func() {
	Describe("NewFrontmatterBindingStore", func() {
		var (
			pages    *fakeFrontmatterReadWriter
			profiles *fakeProfileLister
		)

		BeforeEach(func() {
			pages = newFakeFrontmatterReadWriter()
			profiles = newFakeProfileLister()
		})

		It("should error when pages is nil", func() {
			_, err := engine.NewFrontmatterBindingStore(nil, profiles, silentLogger{})
			Expect(err).To(MatchError(ContainSubstring("pages must not be nil")))
		})

		It("should error when profiles is nil", func() {
			_, err := engine.NewFrontmatterBindingStore(pages, nil, silentLogger{})
			Expect(err).To(MatchError(ContainSubstring("profiles must not be nil")))
		})

		It("should error when logger is nil", func() {
			_, err := engine.NewFrontmatterBindingStore(pages, profiles, nil)
			Expect(err).To(MatchError(ContainSubstring("logger must not be nil")))
		})

		It("should construct successfully with all dependencies", func() {
			store, err := engine.NewFrontmatterBindingStore(pages, profiles, silentLogger{})
			Expect(err).NotTo(HaveOccurred())
			Expect(store).NotTo(BeNil())
		})
	})

	Describe("LoadBindings", func() {
		var (
			pages    *fakeFrontmatterReadWriter
			profiles *fakeProfileLister
			store    *engine.FrontmatterBindingStore
		)

		BeforeEach(func() {
			pages = newFakeFrontmatterReadWriter()
			profiles = newFakeProfileLister()
			var err error
			store, err = engine.NewFrontmatterBindingStore(pages, profiles, silentLogger{})
			Expect(err).NotTo(HaveOccurred())
		})

		When("the profile page does not exist", func() {
			var (
				bindings []connectors.Binding
				err      error
			)

			BeforeEach(func() {
				bindings, err = store.LoadBindings(bindingStoreProfileAlice, connectors.ConnectorKindGoogleTasks)
			})

			It("should return no error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an empty slice", func() {
				Expect(bindings).To(BeEmpty())
			})
		})

		When("the profile has no bindings under the connector key", func() {
			var (
				bindings []connectors.Binding
				err      error
			)

			BeforeEach(func() {
				pages.Seed(bindingStoreProfileAlice, wikipage.FrontMatter{
					"wiki": map[string]any{
						"connectors": map[string]any{
							"google_tasks": map[string]any{
								"refresh_token": "rt",
							},
						},
					},
				})
				bindings, err = store.LoadBindings(bindingStoreProfileAlice, connectors.ConnectorKindGoogleTasks)
			})

			It("should return no error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an empty slice", func() {
				Expect(bindings).To(BeEmpty())
			})
		})

		When("the profile has new-shape bindings[]", func() {
			var (
				bindings []connectors.Binding
				err      error
			)

			BeforeEach(func() {
				pages.Seed(bindingStoreProfileAlice, wikipage.FrontMatter{
					"wiki": map[string]any{
						"connectors": map[string]any{
							"google_tasks": map[string]any{
								"refresh_token": "rt",
								"bindings": []any{
									map[string]any{
										"page":              "shopping",
										"list_name":         "groceries",
										"remote_handle":     "tasklist-id-xyz",
										"remote_list_title": "Groceries",
										"state":             "active",
										"last_synced_seq":   int64(1247),
										"bound_at":          bindingStoreFixedTime(0).Format(time.RFC3339),
										"adapter_state": map[string]any{
											"item_id_map":      map[string]any{"uid-1": "task-1"},
											"item_etags":       map[string]any{"task-1": "etag-1"},
											"last_updated_min": "2026-05-04T14:00:00Z",
										},
									},
								},
							},
						},
					},
				})
				bindings, err = store.LoadBindings(bindingStoreProfileAlice, connectors.ConnectorKindGoogleTasks)
			})

			It("should return no error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return one binding", func() {
				Expect(bindings).To(HaveLen(1))
			})

			It("should populate identity fields", func() {
				Expect(bindings[0].ProfileID).To(Equal(bindingStoreProfileAlice))
				Expect(bindings[0].Page).To(Equal("shopping"))
				Expect(bindings[0].ListName).To(Equal("groceries"))
				Expect(bindings[0].RemoteHandle).To(Equal("tasklist-id-xyz"))
				Expect(bindings[0].RemoteListTitle).To(Equal("Groceries"))
			})

			It("should populate engine state fields", func() {
				Expect(bindings[0].State).To(Equal(connectors.BindingStateActive))
				Expect(bindings[0].LastSyncedSeq).To(Equal(int64(1247)))
				Expect(bindings[0].BoundAt).To(Equal(bindingStoreFixedTime(0)))
			})

			It("should populate AdapterState with adapter-specific fields", func() {
				Expect(bindings[0].AdapterState).To(HaveKey("item_id_map"))
				Expect(bindings[0].AdapterState).To(HaveKey("item_etags"))
				Expect(bindings[0].AdapterState).To(HaveKey("last_updated_min"))
			})
		})

		When("the profile has legacy-shape subscriptions[]", func() {
			var (
				bindings []connectors.Binding
				err      error
			)

			BeforeEach(func() {
				pages.Seed(bindingStoreProfileAlice, wikipage.FrontMatter{
					"wiki": map[string]any{
						"connectors": map[string]any{
							"google_tasks": map[string]any{
								"refresh_token": "rt",
								"subscriptions": []any{
									map[string]any{
										"page":              "shopping",
										"list_name":         "groceries",
										"remote_list_id":    "tasklist-id-xyz",
										"remote_list_title": "Groceries",
										"state":             "paused",
										"paused_reason":     "auth_failed",
										"paused_at":         bindingStoreFixedTime(0).Format(time.RFC3339),
										"subscribed_at":     bindingStoreFixedTime(-3600).Format(time.RFC3339),
										"last_synced_seq":   int64(99),
										"item_id_map":       map[string]any{"uid-1": "task-1"},
										"item_etags":        map[string]any{"task-1": "etag-1"},
										"last_updated_min":  "2026-05-04T14:00:00Z",
									},
								},
							},
						},
					},
				})
				bindings, err = store.LoadBindings(bindingStoreProfileAlice, connectors.ConnectorKindGoogleTasks)
			})

			It("should return no error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should translate the legacy entry into a Binding", func() {
				Expect(bindings).To(HaveLen(1))
			})

			It("should map identity fields", func() {
				Expect(bindings[0].Page).To(Equal("shopping"))
				Expect(bindings[0].ListName).To(Equal("groceries"))
			})

			It("should map remote_list_id to RemoteHandle", func() {
				Expect(bindings[0].RemoteHandle).To(Equal("tasklist-id-xyz"))
			})

			It("should map state and paused fields", func() {
				Expect(bindings[0].State).To(Equal(connectors.BindingStatePaused))
				Expect(bindings[0].PausedReason).To(Equal("auth_failed"))
				Expect(bindings[0].PausedAt).To(Equal(bindingStoreFixedTime(0)))
			})

			It("should map subscribed_at to BoundAt", func() {
				Expect(bindings[0].BoundAt).To(Equal(bindingStoreFixedTime(-3600)))
			})

			It("should map last_synced_seq", func() {
				Expect(bindings[0].LastSyncedSeq).To(Equal(int64(99)))
			})

			It("should put adapter-specific fields under AdapterState", func() {
				Expect(bindings[0].AdapterState).To(HaveKey("item_id_map"))
				Expect(bindings[0].AdapterState).To(HaveKey("item_etags"))
				Expect(bindings[0].AdapterState).To(HaveKey("last_updated_min"))
			})

			It("should not leak engine-owned fields into AdapterState", func() {
				Expect(bindings[0].AdapterState).NotTo(HaveKey("page"))
				Expect(bindings[0].AdapterState).NotTo(HaveKey("list_name"))
				Expect(bindings[0].AdapterState).NotTo(HaveKey("remote_list_id"))
				Expect(bindings[0].AdapterState).NotTo(HaveKey("state"))
				Expect(bindings[0].AdapterState).NotTo(HaveKey("paused_reason"))
				Expect(bindings[0].AdapterState).NotTo(HaveKey("paused_at"))
				Expect(bindings[0].AdapterState).NotTo(HaveKey("subscribed_at"))
				Expect(bindings[0].AdapterState).NotTo(HaveKey("last_synced_seq"))
				Expect(bindings[0].AdapterState).NotTo(HaveKey("remote_list_title"))
			})
		})

		When("the profile has BOTH new-shape bindings[] and legacy subscriptions[]", func() {
			var (
				bindings []connectors.Binding
				err      error
			)

			BeforeEach(func() {
				pages.Seed(bindingStoreProfileAlice, wikipage.FrontMatter{
					"wiki": map[string]any{
						"connectors": map[string]any{
							"google_tasks": map[string]any{
								"bindings": []any{
									map[string]any{
										"page":          "shopping",
										"list_name":     "new",
										"remote_handle": "new-handle",
									},
								},
								"subscriptions": []any{
									map[string]any{
										"page":           "shopping",
										"list_name":      "legacy",
										"remote_list_id": "legacy-handle",
									},
								},
							},
						},
					},
				})
				bindings, err = store.LoadBindings(bindingStoreProfileAlice, connectors.ConnectorKindGoogleTasks)
			})

			It("should return no error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return only the new-shape entries (legacy ignored)", func() {
				Expect(bindings).To(HaveLen(1))
				Expect(bindings[0].ListName).To(Equal("new"))
				Expect(bindings[0].RemoteHandle).To(Equal("new-handle"))
			})
		})

		When("ReadFrontMatter returns a non-NotExist error", func() {
			var (
				bindings []connectors.Binding
				err      error
			)

			BeforeEach(func() {
				pages.SetReadError(bindingStoreProfileAlice, errors.New("backend exploded"))
				bindings, err = store.LoadBindings(bindingStoreProfileAlice, connectors.ConnectorKindGoogleTasks)
			})

			It("should return a wrapped error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("backend exploded"))
			})

			It("should not return any bindings", func() {
				Expect(bindings).To(BeNil())
			})
		})
	})

	Describe("FindBinding", func() {
		var (
			pages    *fakeFrontmatterReadWriter
			profiles *fakeProfileLister
			store    *engine.FrontmatterBindingStore
		)

		BeforeEach(func() {
			pages = newFakeFrontmatterReadWriter()
			profiles = newFakeProfileLister()
			var err error
			store, err = engine.NewFrontmatterBindingStore(pages, profiles, silentLogger{})
			Expect(err).NotTo(HaveOccurred())
		})

		When("the binding exists in the new shape", func() {
			var (
				binding connectors.Binding
				found   bool
				err     error
			)

			BeforeEach(func() {
				pages.Seed(bindingStoreProfileAlice, wikipage.FrontMatter{
					"wiki": map[string]any{
						"connectors": map[string]any{
							"google_tasks": map[string]any{
								"bindings": []any{
									map[string]any{
										"page":          "shopping",
										"list_name":     "groceries",
										"remote_handle": "tasklist-id",
									},
								},
							},
						},
					},
				})
				binding, found, err = store.FindBinding(bindingStoreProfileAlice, connectors.ConnectorKindGoogleTasks, "shopping", "groceries")
			})

			It("should return found=true", func() {
				Expect(found).To(BeTrue())
			})

			It("should return no error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should populate the Binding", func() {
				Expect(binding.Page).To(Equal("shopping"))
				Expect(binding.ListName).To(Equal("groceries"))
				Expect(binding.RemoteHandle).To(Equal("tasklist-id"))
			})
		})

		When("the binding does not exist", func() {
			var (
				binding connectors.Binding
				found   bool
				err     error
			)

			BeforeEach(func() {
				binding, found, err = store.FindBinding(bindingStoreProfileAlice, connectors.ConnectorKindGoogleTasks, "missing", "missing")
			})

			It("should return found=false", func() {
				Expect(found).To(BeFalse())
			})

			It("should return no error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a zero Binding", func() {
				Expect(binding).To(Equal(connectors.Binding{}))
			})
		})

		When("ReadFrontMatter returns an error", func() {
			var (
				binding connectors.Binding
				found   bool
				err     error
			)

			BeforeEach(func() {
				pages.SetReadError(bindingStoreProfileAlice, errors.New("read failed"))
				binding, found, err = store.FindBinding(bindingStoreProfileAlice, connectors.ConnectorKindGoogleTasks, "shopping", "groceries")
			})

			It("should return a wrapped error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("read failed"))
			})

			It("should return found=false and a zero Binding", func() {
				Expect(found).To(BeFalse())
				Expect(binding).To(Equal(connectors.Binding{}))
			})
		})
	})

	Describe("SaveBinding", func() {
		var (
			pages    *fakeFrontmatterReadWriter
			profiles *fakeProfileLister
			store    *engine.FrontmatterBindingStore
		)

		BeforeEach(func() {
			pages = newFakeFrontmatterReadWriter()
			profiles = newFakeProfileLister()
			var err error
			store, err = engine.NewFrontmatterBindingStore(pages, profiles, silentLogger{})
			Expect(err).NotTo(HaveOccurred())
		})

		When("there are no existing bindings", func() {
			var saveErr error

			BeforeEach(func() {
				pages.Seed(bindingStoreProfileAlice, wikipage.FrontMatter{
					"wiki": map[string]any{
						"connectors": map[string]any{
							"google_tasks": map[string]any{
								"refresh_token": "rt",
							},
						},
					},
				})
				saveErr = store.SaveBinding(bindingStoreProfileAlice, connectors.ConnectorKindGoogleTasks, connectors.Binding{
					ProfileID:    bindingStoreProfileAlice,
					Page:         "shopping",
					ListName:     "groceries",
					RemoteHandle: "tasklist-1",
					State:        connectors.BindingStateActive,
					BoundAt:      bindingStoreFixedTime(0),
					AdapterState: connectors.AdapterState{
						"item_id_map": map[string]any{"uid-1": "task-1"},
					},
				})
			})

			It("should return no error", func() {
				Expect(saveErr).NotTo(HaveOccurred())
			})

			It("should write the new binding under wiki.connectors.<kind>.bindings", func() {
				bindings, err := store.LoadBindings(bindingStoreProfileAlice, connectors.ConnectorKindGoogleTasks)
				Expect(err).NotTo(HaveOccurred())
				Expect(bindings).To(HaveLen(1))
				Expect(bindings[0].ListName).To(Equal("groceries"))
				Expect(bindings[0].RemoteHandle).To(Equal("tasklist-1"))
			})

			It("should preserve the connector's other state (refresh_token)", func() {
				snap := pages.Snapshot(bindingStoreProfileAlice)
				gt := connectorSubtree(snap, "google_tasks")
				Expect(gt["refresh_token"]).To(Equal("rt"))
			})

			It("should write the new shape (bindings[] not subscriptions[])", func() {
				snap := pages.Snapshot(bindingStoreProfileAlice)
				gt := connectorSubtree(snap, "google_tasks")
				Expect(gt).To(HaveKey("bindings"))
				Expect(gt).NotTo(HaveKey("subscriptions"))
			})

			It("should write adapter_state under the binding entry", func() {
				snap := pages.Snapshot(bindingStoreProfileAlice)
				bindings := connectorBindings(connectorSubtree(snap, "google_tasks"))
				entry := bindingEntry(bindings[0])
				Expect(entry).To(HaveKey("adapter_state"))
				as := adapterStateOf(entry)
				Expect(as).To(HaveKey("item_id_map"))
			})
		})

		When("a binding for (page, listName) already exists", func() {
			var saveErr error

			BeforeEach(func() {
				pages.Seed(bindingStoreProfileAlice, wikipage.FrontMatter{
					"wiki": map[string]any{
						"connectors": map[string]any{
							"google_tasks": map[string]any{
								"bindings": []any{
									map[string]any{
										"page":            "shopping",
										"list_name":       "groceries",
										"remote_handle":   "old-handle",
										"last_synced_seq": int64(5),
									},
								},
							},
						},
					},
				})
				saveErr = store.SaveBinding(bindingStoreProfileAlice, connectors.ConnectorKindGoogleTasks, connectors.Binding{
					Page:          "shopping",
					ListName:      "groceries",
					RemoteHandle:  "new-handle",
					State:         connectors.BindingStateActive,
					LastSyncedSeq: 99,
				})
			})

			It("should return no error", func() {
				Expect(saveErr).NotTo(HaveOccurred())
			})

			It("should replace the existing entry in place", func() {
				bindings, err := store.LoadBindings(bindingStoreProfileAlice, connectors.ConnectorKindGoogleTasks)
				Expect(err).NotTo(HaveOccurred())
				Expect(bindings).To(HaveLen(1))
				Expect(bindings[0].RemoteHandle).To(Equal("new-handle"))
				Expect(bindings[0].LastSyncedSeq).To(Equal(int64(99)))
			})
		})

		When("the profile has another adapter's bindings (Keep)", func() {
			var saveErr error

			BeforeEach(func() {
				pages.Seed(bindingStoreProfileAlice, wikipage.FrontMatter{
					"wiki": map[string]any{
						"connectors": map[string]any{
							"google_keep": map[string]any{
								"bindings": []any{
									map[string]any{
										"page":          "shopping",
										"list_name":     "keep_list",
										"remote_handle": "keep-note-1",
									},
								},
							},
							"google_tasks": map[string]any{
								"refresh_token": "rt",
							},
						},
					},
				})
				saveErr = store.SaveBinding(bindingStoreProfileAlice, connectors.ConnectorKindGoogleTasks, connectors.Binding{
					Page:         "shopping",
					ListName:     "groceries",
					RemoteHandle: "tasklist-1",
				})
			})

			It("should return no error", func() {
				Expect(saveErr).NotTo(HaveOccurred())
			})

			It("should preserve the Keep bindings", func() {
				snap := pages.Snapshot(bindingStoreProfileAlice)
				keep := connectorSubtree(snap, "google_keep")
				Expect(keep).To(HaveKey("bindings"))
				keepBindings := connectorBindings(keep)
				Expect(keepBindings).To(HaveLen(1))
			})
		})

		When("WriteFrontMatter returns an error", func() {
			var saveErr error

			BeforeEach(func() {
				pages.SetWriteError(bindingStoreProfileAlice, errors.New("write boom"))
				saveErr = store.SaveBinding(bindingStoreProfileAlice, connectors.ConnectorKindGoogleTasks, connectors.Binding{
					Page:     "shopping",
					ListName: "groceries",
				})
			})

			It("should return a wrapped error", func() {
				Expect(saveErr).To(HaveOccurred())
				Expect(saveErr.Error()).To(ContainSubstring("write boom"))
			})
		})

		When("ReadFrontMatter returns os.ErrNotExist", func() {
			var saveErr error

			BeforeEach(func() {
				saveErr = store.SaveBinding(bindingStoreProfileAlice, connectors.ConnectorKindGoogleTasks, connectors.Binding{
					Page:         "shopping",
					ListName:     "groceries",
					RemoteHandle: "tasklist-1",
				})
			})

			It("should not error (missing profile is treated as fresh)", func() {
				Expect(saveErr).NotTo(HaveOccurred())
			})

			It("should write the binding to the new profile page", func() {
				bindings, err := store.LoadBindings(bindingStoreProfileAlice, connectors.ConnectorKindGoogleTasks)
				Expect(err).NotTo(HaveOccurred())
				Expect(bindings).To(HaveLen(1))
			})
		})
	})

	Describe("DeleteBinding", func() {
		var (
			pages    *fakeFrontmatterReadWriter
			profiles *fakeProfileLister
			store    *engine.FrontmatterBindingStore
		)

		BeforeEach(func() {
			pages = newFakeFrontmatterReadWriter()
			profiles = newFakeProfileLister()
			var err error
			store, err = engine.NewFrontmatterBindingStore(pages, profiles, silentLogger{})
			Expect(err).NotTo(HaveOccurred())
		})

		When("the binding exists", func() {
			var deleteErr error

			BeforeEach(func() {
				pages.Seed(bindingStoreProfileAlice, wikipage.FrontMatter{
					"wiki": map[string]any{
						"connectors": map[string]any{
							"google_tasks": map[string]any{
								"bindings": []any{
									map[string]any{"page": "p1", "list_name": "l1", "remote_handle": "h1"},
									map[string]any{"page": "p2", "list_name": "l2", "remote_handle": "h2"},
								},
							},
						},
					},
				})
				deleteErr = store.DeleteBinding(bindingStoreProfileAlice, connectors.ConnectorKindGoogleTasks, "p1", "l1")
			})

			It("should return no error", func() {
				Expect(deleteErr).NotTo(HaveOccurred())
			})

			It("should remove the entry", func() {
				bindings, err := store.LoadBindings(bindingStoreProfileAlice, connectors.ConnectorKindGoogleTasks)
				Expect(err).NotTo(HaveOccurred())
				Expect(bindings).To(HaveLen(1))
				Expect(bindings[0].Page).To(Equal("p2"))
			})
		})

		When("the binding does not exist", func() {
			var deleteErr error

			BeforeEach(func() {
				pages.Seed(bindingStoreProfileAlice, wikipage.FrontMatter{
					"wiki": map[string]any{
						"connectors": map[string]any{
							"google_tasks": map[string]any{
								"bindings": []any{
									map[string]any{"page": "p2", "list_name": "l2", "remote_handle": "h2"},
								},
							},
						},
					},
				})
				deleteErr = store.DeleteBinding(bindingStoreProfileAlice, connectors.ConnectorKindGoogleTasks, "missing", "missing")
			})

			It("should return no error (no-op)", func() {
				Expect(deleteErr).NotTo(HaveOccurred())
			})

			It("should preserve other entries", func() {
				bindings, err := store.LoadBindings(bindingStoreProfileAlice, connectors.ConnectorKindGoogleTasks)
				Expect(err).NotTo(HaveOccurred())
				Expect(bindings).To(HaveLen(1))
			})
		})

		When("the profile has no bindings at all", func() {
			var deleteErr error

			BeforeEach(func() {
				deleteErr = store.DeleteBinding(bindingStoreProfileAlice, connectors.ConnectorKindGoogleTasks, "missing", "missing")
			})

			It("should return no error (no-op)", func() {
				Expect(deleteErr).NotTo(HaveOccurred())
			})
		})

		When("the profile has another adapter's bindings", func() {
			var deleteErr error

			BeforeEach(func() {
				pages.Seed(bindingStoreProfileAlice, wikipage.FrontMatter{
					"wiki": map[string]any{
						"connectors": map[string]any{
							"google_keep": map[string]any{
								"bindings": []any{
									map[string]any{"page": "shopping", "list_name": "k1", "remote_handle": "kh1"},
								},
							},
							"google_tasks": map[string]any{
								"bindings": []any{
									map[string]any{"page": "shopping", "list_name": "t1", "remote_handle": "th1"},
								},
							},
						},
					},
				})
				deleteErr = store.DeleteBinding(bindingStoreProfileAlice, connectors.ConnectorKindGoogleTasks, "shopping", "t1")
			})

			It("should return no error", func() {
				Expect(deleteErr).NotTo(HaveOccurred())
			})

			It("should preserve the other adapter's bindings", func() {
				snap := pages.Snapshot(bindingStoreProfileAlice)
				keep := connectorSubtree(snap, "google_keep")
				Expect(connectorBindings(keep)).To(HaveLen(1))
			})
		})
	})

	Describe("WithProfileLock", func() {
		var (
			pages    *fakeFrontmatterReadWriter
			profiles *fakeProfileLister
			store    *engine.FrontmatterBindingStore
		)

		BeforeEach(func() {
			pages = newFakeFrontmatterReadWriter()
			profiles = newFakeProfileLister()
			var err error
			store, err = engine.NewFrontmatterBindingStore(pages, profiles, silentLogger{})
			Expect(err).NotTo(HaveOccurred())
		})

		When("fn runs successfully", func() {
			var (
				err    error
				called bool
			)

			BeforeEach(func() {
				err = store.WithProfileLock(bindingStoreProfileAlice, func() error {
					called = true
					return nil
				})
			})

			It("should invoke fn", func() {
				Expect(called).To(BeTrue())
			})

			It("should return no error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should release the lock so a subsequent call can run", func() {
				err2 := store.WithProfileLock(bindingStoreProfileAlice, func() error { return nil })
				Expect(err2).NotTo(HaveOccurred())
			})
		})

		When("fn returns an error", func() {
			var err error
			sentinel := errors.New("fn failed")

			BeforeEach(func() {
				err = store.WithProfileLock(bindingStoreProfileAlice, func() error {
					return sentinel
				})
			})

			It("should propagate the error", func() {
				Expect(err).To(MatchError(sentinel))
			})

			It("should still release the lock", func() {
				err2 := store.WithProfileLock(bindingStoreProfileAlice, func() error { return nil })
				Expect(err2).NotTo(HaveOccurred())
			})
		})

		When("two concurrent callers target the same profile", func() {
			var (
				maxConcurrent  int32
				active         atomic.Int32
				doneA, doneB   chan struct{}
				errA, errB     error
			)

			BeforeEach(func() {
				doneA = make(chan struct{})
				doneB = make(chan struct{})
				barrier := make(chan struct{})

				go func() {
					errA = store.WithProfileLock(bindingStoreProfileAlice, func() error {
						current := active.Add(1)
						defer active.Add(-1)
						for {
							got := atomic.LoadInt32(&maxConcurrent)
							if current <= got || atomic.CompareAndSwapInt32(&maxConcurrent, got, current) {
								break
							}
						}
						<-barrier
						time.Sleep(20 * time.Millisecond)
						return nil
					})
					close(doneA)
				}()

				// Wait for A to acquire the lock, then start B.
				time.Sleep(10 * time.Millisecond)
				go func() {
					errB = store.WithProfileLock(bindingStoreProfileAlice, func() error {
						current := active.Add(1)
						defer active.Add(-1)
						for {
							got := atomic.LoadInt32(&maxConcurrent)
							if current <= got || atomic.CompareAndSwapInt32(&maxConcurrent, got, current) {
								break
							}
						}
						return nil
					})
					close(doneB)
				}()

				close(barrier)
				<-doneA
				<-doneB
			})

			It("should serialize the critical sections", func() {
				Expect(atomic.LoadInt32(&maxConcurrent)).To(Equal(int32(1)))
			})

			It("should not error in either goroutine", func() {
				Expect(errA).NotTo(HaveOccurred())
				Expect(errB).NotTo(HaveOccurred())
			})
		})

		When("two concurrent callers target different profiles", func() {
			var (
				bothActive bool
				doneA      chan struct{}
				doneB      chan struct{}
				errA       error
				errB       error
			)

			BeforeEach(func() {
				doneA = make(chan struct{})
				doneB = make(chan struct{})
				aReady := make(chan struct{})
				bReady := make(chan struct{})
				release := make(chan struct{})

				go func() {
					errA = store.WithProfileLock(bindingStoreProfileAlice, func() error {
						close(aReady)
						<-release
						return nil
					})
					close(doneA)
				}()
				go func() {
					errB = store.WithProfileLock(bindingStoreProfileBob, func() error {
						close(bReady)
						<-release
						return nil
					})
					close(doneB)
				}()

				// Both should reach their critical sections before we release.
				<-aReady
				<-bReady
				bothActive = true
				close(release)
				<-doneA
				<-doneB
			})

			It("should run both critical sections concurrently", func() {
				Expect(bothActive).To(BeTrue())
			})

			It("should not error in either goroutine", func() {
				Expect(errA).NotTo(HaveOccurred())
				Expect(errB).NotTo(HaveOccurred())
			})
		})
	})

	Describe("ListAllProfilesWithBindings", func() {
		var (
			pages    *fakeFrontmatterReadWriter
			profiles *fakeProfileLister
			store    *engine.FrontmatterBindingStore
		)

		BeforeEach(func() {
			pages = newFakeFrontmatterReadWriter()
			profiles = newFakeProfileLister()
			var err error
			store, err = engine.NewFrontmatterBindingStore(pages, profiles, silentLogger{})
			Expect(err).NotTo(HaveOccurred())
		})

		When("profiles have new-shape bindings for the queried kind", func() {
			var (
				out []wikipage.PageIdentifier
				err error
			)

			BeforeEach(func() {
				profiles.Set("wiki.connectors.google_tasks.bindings", []wikipage.PageIdentifier{bindingStoreProfileAlice})
				out, err = store.ListAllProfilesWithBindings(connectors.ConnectorKindGoogleTasks)
			})

			It("should return no error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should include profiles with the new-shape key", func() {
				Expect(out).To(ContainElement(bindingStoreProfileAlice))
			})
		})

		When("profiles have legacy subscriptions[] for the queried kind", func() {
			var (
				out []wikipage.PageIdentifier
				err error
			)

			BeforeEach(func() {
				profiles.Set("wiki.connectors.google_tasks.bindings", nil)
				profiles.Set("wiki.connectors.google_tasks.subscriptions", []wikipage.PageIdentifier{bindingStoreProfileBob})
				out, err = store.ListAllProfilesWithBindings(connectors.ConnectorKindGoogleTasks)
			})

			It("should return no error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should include profiles whose data is still in legacy shape", func() {
				Expect(out).To(ContainElement(bindingStoreProfileBob))
			})
		})

		When("a profile has bindings for both shapes", func() {
			var (
				out []wikipage.PageIdentifier
				err error
			)

			BeforeEach(func() {
				profiles.Set("wiki.connectors.google_tasks.bindings", []wikipage.PageIdentifier{bindingStoreProfileAlice})
				profiles.Set("wiki.connectors.google_tasks.subscriptions", []wikipage.PageIdentifier{bindingStoreProfileAlice})
				out, err = store.ListAllProfilesWithBindings(connectors.ConnectorKindGoogleTasks)
			})

			It("should return no error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should include the profile only once (deduplicated)", func() {
				count := 0
				for _, p := range out {
					if p == bindingStoreProfileAlice {
						count++
					}
				}
				Expect(count).To(Equal(1))
			})
		})

		When("a profile has bindings only for another adapter kind", func() {
			var (
				out []wikipage.PageIdentifier
				err error
			)

			BeforeEach(func() {
				profiles.Set("wiki.connectors.google_keep.bindings", []wikipage.PageIdentifier{bindingStoreProfileAlice})
				out, err = store.ListAllProfilesWithBindings(connectors.ConnectorKindGoogleTasks)
			})

			It("should return no error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not include profiles with bindings for the other kind", func() {
				Expect(out).NotTo(ContainElement(bindingStoreProfileAlice))
			})
		})
	})
})
