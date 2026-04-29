//revive:disable:dot-imports
package bridge_test

import (
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/keep/bridge"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

func TestBridge(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "internal/keep/bridge")
}

// fakeStore is the same in-memory PageReaderMutator pattern the
// checklistmutator tests use. Only the methods bindings.go calls are
// implemented; the rest panic so a future change to the bindings code
// can't silently start hitting an unimplemented surface.
type fakeStore struct {
	mu       sync.Mutex
	pages    map[wikipage.PageIdentifier]wikipage.FrontMatter
	markdown map[wikipage.PageIdentifier]wikipage.Markdown
}

func newFakeStore() *fakeStore {
	return &fakeStore{pages: make(map[wikipage.PageIdentifier]wikipage.FrontMatter)}
}

func (s *fakeStore) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fm, ok := s.pages[id]
	if !ok {
		return id, nil, os.ErrNotExist
	}
	return id, deepCopyFM(fm), nil
}

func (s *fakeStore) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pages[id] = deepCopyFM(fm)
	return nil
}

// Methods on PageReaderMutator we don't exercise. Panic in case bindings
// starts using them later — tests should fail loudly so we add coverage.
func (s *fakeStore) ReadMarkdown(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	if s.markdown == nil {
		return id, "", nil
	}
	return id, s.markdown[id], nil
}
func (s *fakeStore) WriteMarkdown(id wikipage.PageIdentifier, md wikipage.Markdown) error {
	if s.markdown == nil {
		s.markdown = make(map[wikipage.PageIdentifier]wikipage.Markdown)
	}
	s.markdown[id] = md
	return nil
}
func (*fakeStore) DeletePage(wikipage.PageIdentifier) error {
	panic("DeletePage not used by bindings")
}
func (*fakeStore) ModifyMarkdown(wikipage.PageIdentifier, func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	panic("ModifyMarkdown not used by bindings")
}

// deepCopyFM is a quick recursive copy so tests can mutate their captured
// snapshots without affecting the store's internal state.
func deepCopyFM(fm wikipage.FrontMatter) wikipage.FrontMatter {
	if fm == nil {
		return nil
	}
	out := make(wikipage.FrontMatter, len(fm))
	for k, v := range fm {
		out[k] = deepCopyAny(v)
	}
	return out
}

func deepCopyAny(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[k] = deepCopyAny(vv)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, vv := range x {
			out[i] = deepCopyAny(vv)
		}
		return out
	default:
		return v
	}
}

const aliceProfile wikipage.PageIdentifier = "profile_alice"
const bobProfile wikipage.PageIdentifier = "profile_bob"

var _ = Describe("BindingStore.LoadState", func() {
	var (
		store *bridge.BindingStore
		pages *fakeStore
	)

	BeforeEach(func() {
		pages = newFakeStore()
		store = bridge.NewBindingStore(pages)
	})

	When("the profile page does not exist", func() {
		var (
			state   bridge.ConnectorState
			loadErr error
		)

		BeforeEach(func() {
			state, loadErr = store.LoadState(aliceProfile)
		})

		It("should not error", func() {
			Expect(loadErr).ToNot(HaveOccurred())
		})

		It("should return zero state", func() {
			Expect(state).To(Equal(bridge.ConnectorState{}))
		})
	})

	When("the profile exists but has no connector frontmatter", func() {
		var (
			state   bridge.ConnectorState
			loadErr error
		)

		BeforeEach(func() {
			Expect(pages.WriteFrontMatter(aliceProfile, wikipage.FrontMatter{
				"identifier": "profile_alice",
			})).To(Succeed())
			state, loadErr = store.LoadState(aliceProfile)
		})

		It("should not error", func() {
			Expect(loadErr).ToNot(HaveOccurred())
		})

		It("should return zero state", func() {
			Expect(state).To(Equal(bridge.ConnectorState{}))
		})
	})

	When("the profile has a complete connector state on disk", func() {
		var (
			state   bridge.ConnectorState
			loadErr error
		)

		BeforeEach(func() {
			Expect(pages.WriteFrontMatter(aliceProfile, wikipage.FrontMatter{
				"wiki": map[string]any{
					"connectors": map[string]any{
						"google_keep": map[string]any{
							"email":            "alice@example.com",
							"master_token":     "oauth2rt_1/fake",
							"connected_at":     "2026-04-25T17:14:00Z",
							"last_verified_at": "2026-04-25T17:30:00Z",
							"bindings": []any{
								map[string]any{
									"page":            "shopping_lists",
									"list_name":       "groceries",
									"keep_note_id":    "srv-list-1",
									"keep_note_title": "groceries",
									"bound_at":        "2026-04-25T17:14:00Z",
								},
							},
						},
					},
				},
			})).To(Succeed())
			state, loadErr = store.LoadState(aliceProfile)
		})

		It("should not error", func() {
			Expect(loadErr).ToNot(HaveOccurred())
		})

		It("should populate Email and MasterToken", func() {
			Expect(state.Email).To(Equal("alice@example.com"))
			Expect(state.MasterToken).To(Equal("oauth2rt_1/fake"))
		})

		It("should parse ConnectedAt as a UTC time", func() {
			expected := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			Expect(state.ConnectedAt).To(BeTemporally("~", expected, time.Second))
		})

		It("should populate the bindings slice", func() {
			Expect(state.Bindings).To(HaveLen(1))
			Expect(state.Bindings[0].Page).To(Equal("shopping_lists"))
			Expect(state.Bindings[0].ListName).To(Equal("groceries"))
			Expect(state.Bindings[0].KeepNoteID).To(Equal("srv-list-1"))
		})
	})

	When("decoding an old binding file without the new fields", func() {
		var (
			state   bridge.ConnectorState
			loadErr error
		)

		BeforeEach(func() {
			// Legacy on-disk shape: flat string id_map, no KeepCursor,
			// no TruncatedTickStreak, no MigratedFingerprints, no
			// per-item structured fields.
			Expect(pages.WriteFrontMatter(aliceProfile, wikipage.FrontMatter{
				"wiki": map[string]any{
					"connectors": map[string]any{
						"google_keep": map[string]any{
							"email":        "alice@example.com",
							"master_token": "oauth2rt_1/fake",
							"bindings": []any{
								map[string]any{
									"page":            "shopping_lists",
									"list_name":       "groceries",
									"keep_note_id":    "srv-list-1",
									"keep_note_title": "groceries",
									"bound_at":        "2026-04-25T17:14:00Z",
									"item_id_map": map[string]any{
										"wiki-uid-1": "srv-A",
										"wiki-uid-2": "srv-B",
									},
								},
							},
						},
					},
				},
			})).To(Succeed())
			state, loadErr = store.LoadState(aliceProfile)
		})

		It("should not error", func() {
			Expect(loadErr).ToNot(HaveOccurred())
		})

		It("should load the legacy flat id_map as ItemBinding entries with only ServerID populated", func() {
			Expect(state.Bindings).To(HaveLen(1))
			Expect(state.Bindings[0].ItemIDMap).To(HaveKey("wiki-uid-1"))
			Expect(state.Bindings[0].ItemIDMap["wiki-uid-1"].ServerID).To(Equal("srv-A"))
		})

		It("should default the new binding-level fields to zero values", func() {
			b := state.Bindings[0]
			Expect(b.KeepCursor).To(Equal(""))
			Expect(b.TruncatedTickStreak).To(Equal(0))
			Expect(b.MigratedFingerprints).To(BeFalse())
		})

		It("should default per-item synced/observed/failure fields to zero values", func() {
			ib := state.Bindings[0].ItemIDMap["wiki-uid-1"]
			Expect(ib.SyncedText).To(Equal(""))
			Expect(ib.SyncedChecked).To(BeFalse())
			Expect(ib.SyncedSortValue).To(Equal(""))
			Expect(ib.LastObservedWikiText).To(Equal(""))
			Expect(ib.LastObservedWikiChecked).To(BeFalse())
			Expect(ib.LastObservedWikiSortValue).To(Equal(""))
			Expect(ib.PushFailureCount).To(Equal(0))
			Expect(ib.LastFailureCode).To(Equal(""))
			Expect(ib.NextAttemptAt.IsZero()).To(BeTrue())
		})

		When("the loaded state is re-saved and re-loaded", func() {
			var roundTripped bridge.ConnectorState

			BeforeEach(func() {
				Expect(store.SaveState(aliceProfile, state)).To(Succeed())
				var err error
				roundTripped, err = store.LoadState(aliceProfile)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should preserve zero values for all new fields after a save→load round trip", func() {
				Expect(roundTripped.Bindings).To(HaveLen(1))
				b := roundTripped.Bindings[0]
				Expect(b.KeepCursor).To(Equal(""))
				Expect(b.TruncatedTickStreak).To(Equal(0))
				Expect(b.MigratedFingerprints).To(BeFalse())
				ib := b.ItemIDMap["wiki-uid-1"]
				Expect(ib.ServerID).To(Equal("srv-A"))
				Expect(ib.SyncedText).To(Equal(""))
				Expect(ib.PushFailureCount).To(Equal(0))
			})
		})
	})

	When("decoding a binding with the new structured shape", func() {
		var (
			state   bridge.ConnectorState
			loadErr error
		)

		BeforeEach(func() {
			Expect(pages.WriteFrontMatter(aliceProfile, wikipage.FrontMatter{
				"wiki": map[string]any{
					"connectors": map[string]any{
						"google_keep": map[string]any{
							"email":        "alice@example.com",
							"master_token": "oauth2rt_1/fake",
							"bindings": []any{
								map[string]any{
									"page":                  "shopping_lists",
									"list_name":             "groceries",
									"keep_note_id":          "srv-list-1",
									"bound_at":              "2026-04-25T17:14:00Z",
									"keep_cursor":           "v-abc-123",
									"truncated_tick_streak": 3,
									"migrated_fingerprints": true,
									"item_id_map": map[string]any{
										"wiki-uid-1": map[string]any{
											"server_id":          "srv-A",
											"synced_text":        "Apples #produce",
											"synced_checked":     true,
											"synced_sort_value":  "1000",
											"push_failure_count": 2,
											"last_failure_code":  "rate_limited",
											"next_attempt_at":    "2026-05-01T12:01:00Z",
										},
									},
								},
							},
						},
					},
				},
			})).To(Succeed())
			state, loadErr = store.LoadState(aliceProfile)
		})

		It("should not error", func() {
			Expect(loadErr).ToNot(HaveOccurred())
		})

		It("should populate the new binding-level fields", func() {
			b := state.Bindings[0]
			Expect(b.KeepCursor).To(Equal("v-abc-123"))
			Expect(b.TruncatedTickStreak).To(Equal(3))
			Expect(b.MigratedFingerprints).To(BeTrue())
		})

		It("should populate the new per-item structured fields", func() {
			ib := state.Bindings[0].ItemIDMap["wiki-uid-1"]
			Expect(ib.ServerID).To(Equal("srv-A"))
			Expect(ib.SyncedText).To(Equal("Apples #produce"))
			Expect(ib.SyncedChecked).To(BeTrue())
			Expect(ib.SyncedSortValue).To(Equal("1000"))
			Expect(ib.PushFailureCount).To(Equal(2))
			Expect(ib.LastFailureCode).To(Equal("rate_limited"))
		})

		It("should parse next_attempt_at as a UTC time", func() {
			ib := state.Bindings[0].ItemIDMap["wiki-uid-1"]
			expected := time.Date(2026, 5, 1, 12, 1, 0, 0, time.UTC)
			Expect(ib.NextAttemptAt).To(BeTemporally("~", expected, time.Second))
		})

		When("the loaded structured state is re-saved and re-loaded", func() {
			var roundTripped bridge.ConnectorState

			BeforeEach(func() {
				Expect(store.SaveState(aliceProfile, state)).To(Succeed())
				var err error
				roundTripped, err = store.LoadState(aliceProfile)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should round-trip NextAttemptAt", func() {
				ib := roundTripped.Bindings[0].ItemIDMap["wiki-uid-1"]
				expected := time.Date(2026, 5, 1, 12, 1, 0, 0, time.UTC)
				Expect(ib.NextAttemptAt).To(BeTemporally("~", expected, time.Second))
			})

			It("should round-trip PushFailureCount", func() {
				ib := roundTripped.Bindings[0].ItemIDMap["wiki-uid-1"]
				Expect(ib.PushFailureCount).To(Equal(2))
			})

			It("should round-trip LastFailureCode", func() {
				ib := roundTripped.Bindings[0].ItemIDMap["wiki-uid-1"]
				Expect(ib.LastFailureCode).To(Equal("rate_limited"))
			})
		})
	})
})

var _ = Describe("BindingStore.AddBinding", func() {
	var (
		store *bridge.BindingStore
		pages *fakeStore
		now   time.Time
	)

	BeforeEach(func() {
		pages = newFakeStore()
		store = bridge.NewBindingStore(pages)
		now = time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
		// Seed Alice's profile with a connector state but no bindings.
		Expect(pages.WriteFrontMatter(aliceProfile, wikipage.FrontMatter{
			"wiki": map[string]any{
				"connectors": map[string]any{
					"google_keep": map[string]any{
						"email":        "alice@example.com",
						"master_token": "oauth2rt_1/fake",
						"connected_at": "2026-04-25T17:00:00Z",
					},
				},
			},
		})).To(Succeed())
		Expect(pages.WriteFrontMatter(bobProfile, wikipage.FrontMatter{
			"wiki": map[string]any{
				"connectors": map[string]any{
					"google_keep": map[string]any{
						"email":        "bob@example.com",
						"master_token": "oauth2rt_1/bobtoken",
					},
				},
			},
		})).To(Succeed())
	})

	When("adding a fresh binding", func() {
		var addErr error

		BeforeEach(func() {
			addErr = store.AddBinding(aliceProfile, bridge.Binding{
				Page:          "shopping_lists",
				ListName:      "groceries",
				KeepNoteID:    "srv-list-1",
				KeepNoteTitle: "groceries",
				BoundAt:       now,
			})
		})

		It("should not error", func() {
			Expect(addErr).ToNot(HaveOccurred())
		})

		It("should make the binding visible in LoadState", func() {
			s, err := store.LoadState(aliceProfile)
			Expect(err).ToNot(HaveOccurred())
			Expect(s.Bindings).To(HaveLen(1))
			Expect(s.Bindings[0].KeepNoteID).To(Equal("srv-list-1"))
		})
	})

	When("the same user binds two different checklists to different Keep notes", func() {
		var addErr error

		BeforeEach(func() {
			Expect(store.AddBinding(aliceProfile, bridge.Binding{
				Page:       "shopping_lists",
				ListName:   "groceries",
				KeepNoteID: "srv-list-1",
				BoundAt:    now,
			})).To(Succeed())
			addErr = store.AddBinding(aliceProfile, bridge.Binding{
				Page:       "shopping_lists",
				ListName:   "weekend_chores",
				KeepNoteID: "srv-list-2",
				BoundAt:    now,
			})
		})

		It("should not error", func() {
			Expect(addErr).ToNot(HaveOccurred())
		})

		It("should keep both bindings", func() {
			s, _ := store.LoadState(aliceProfile)
			Expect(s.Bindings).To(HaveLen(2))
		})
	})

	When("the same user tries to bind the same (page, list_name) twice", func() {
		var addErr error

		BeforeEach(func() {
			Expect(store.AddBinding(aliceProfile, bridge.Binding{
				Page:       "shopping_lists",
				ListName:   "groceries",
				KeepNoteID: "srv-list-1",
				BoundAt:    now,
			})).To(Succeed())
			addErr = store.AddBinding(aliceProfile, bridge.Binding{
				Page:       "shopping_lists",
				ListName:   "groceries",
				KeepNoteID: "srv-list-2",
				BoundAt:    now,
			})
		})

		It("should return ErrAlreadyBoundForChecklist", func() {
			Expect(errors.Is(addErr, bridge.ErrAlreadyBoundForChecklist)).To(BeTrue())
		})

		It("should not have created a second binding", func() {
			s, _ := store.LoadState(aliceProfile)
			Expect(s.Bindings).To(HaveLen(1))
		})
	})

	When("the same user tries to bind two different checklists to the same Keep note", func() {
		var addErr error

		BeforeEach(func() {
			Expect(store.AddBinding(aliceProfile, bridge.Binding{
				Page:       "shopping_lists",
				ListName:   "groceries",
				KeepNoteID: "srv-list-1",
				BoundAt:    now,
			})).To(Succeed())
			addErr = store.AddBinding(aliceProfile, bridge.Binding{
				Page:       "weekend",
				ListName:   "chores",
				KeepNoteID: "srv-list-1", // same Keep note
				BoundAt:    now,
			})
		})

		It("should return ErrAlreadyBoundToKeepNote", func() {
			Expect(errors.Is(addErr, bridge.ErrAlreadyBoundToKeepNote)).To(BeTrue())
		})

		It("should not have created a second binding", func() {
			s, _ := store.LoadState(aliceProfile)
			Expect(s.Bindings).To(HaveLen(1))
		})
	})

	When("two different users each bind the same (page, list_name)", func() {
		var bobErr error

		BeforeEach(func() {
			Expect(store.AddBinding(aliceProfile, bridge.Binding{
				Page:       "shopping_lists",
				ListName:   "groceries",
				KeepNoteID: "srv-alice-1",
				BoundAt:    now,
			})).To(Succeed())
			bobErr = store.AddBinding(bobProfile, bridge.Binding{
				Page:       "shopping_lists",
				ListName:   "groceries",
				KeepNoteID: "srv-bob-1",
				BoundAt:    now,
			})
		})

		It("should not error (per-user-only collision rules)", func() {
			Expect(bobErr).ToNot(HaveOccurred())
		})

		It("should leave Alice's binding intact", func() {
			s, _ := store.LoadState(aliceProfile)
			Expect(s.Bindings).To(HaveLen(1))
			Expect(s.Bindings[0].KeepNoteID).To(Equal("srv-alice-1"))
		})

		It("should record Bob's binding on Bob's profile", func() {
			s, _ := store.LoadState(bobProfile)
			Expect(s.Bindings).To(HaveLen(1))
			Expect(s.Bindings[0].KeepNoteID).To(Equal("srv-bob-1"))
		})
	})

	When("the user has no connector configured", func() {
		var addErr error

		BeforeEach(func() {
			// Wipe Alice's connector by writing a profile with no connector
			// frontmatter at all.
			Expect(pages.WriteFrontMatter(aliceProfile, wikipage.FrontMatter{
				"identifier": "profile_alice",
			})).To(Succeed())

			addErr = store.AddBinding(aliceProfile, bridge.Binding{
				Page:       "shopping_lists",
				ListName:   "groceries",
				KeepNoteID: "srv-1",
				BoundAt:    now,
			})
		})

		It("should return ErrConnectorNotConfigured", func() {
			Expect(errors.Is(addErr, bridge.ErrConnectorNotConfigured)).To(BeTrue())
		})
	})
})

var _ = Describe("BindingStore.RemoveBinding", func() {
	var (
		store *bridge.BindingStore
		pages *fakeStore
		now   time.Time
	)

	BeforeEach(func() {
		pages = newFakeStore()
		store = bridge.NewBindingStore(pages)
		now = time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
		Expect(pages.WriteFrontMatter(aliceProfile, wikipage.FrontMatter{
			"wiki": map[string]any{
				"connectors": map[string]any{
					"google_keep": map[string]any{
						"email":        "alice@example.com",
						"master_token": "oauth2rt_1/fake",
					},
				},
			},
		})).To(Succeed())
		Expect(store.AddBinding(aliceProfile, bridge.Binding{
			Page:       "shopping_lists",
			ListName:   "groceries",
			KeepNoteID: "srv-1",
			BoundAt:    now,
		})).To(Succeed())
	})

	When("removing an existing binding", func() {
		var removeErr error

		BeforeEach(func() {
			removeErr = store.RemoveBinding(aliceProfile, "shopping_lists", "groceries")
		})

		It("should not error", func() {
			Expect(removeErr).ToNot(HaveOccurred())
		})

		It("should leave the binding list empty", func() {
			s, _ := store.LoadState(aliceProfile)
			Expect(s.Bindings).To(BeEmpty())
		})
	})

	When("removing a binding that does not exist", func() {
		var removeErr error

		BeforeEach(func() {
			removeErr = store.RemoveBinding(aliceProfile, "shopping_lists", "nope")
		})

		It("should return ErrBindingNotFound", func() {
			Expect(errors.Is(removeErr, bridge.ErrBindingNotFound)).To(BeTrue())
		})
	})
})

var _ = Describe("BindingStore.SaveState", func() {
	var (
		store *bridge.BindingStore
		pages *fakeStore
	)

	BeforeEach(func() {
		pages = newFakeStore()
		store = bridge.NewBindingStore(pages)
		Expect(pages.WriteFrontMatter(aliceProfile, wikipage.FrontMatter{
			"identifier": "profile_alice",
		})).To(Succeed())
	})

	When("saving a complete state", func() {
		var (
			saveErr  error
			loadBack bridge.ConnectorState
			loadErr  error
		)

		BeforeEach(func() {
			saveErr = store.SaveState(aliceProfile, bridge.ConnectorState{
				Email:          "alice@example.com",
				MasterToken:    "oauth2rt_1/fake",
				ConnectedAt:    time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC),
				LastVerifiedAt: time.Date(2026, 4, 25, 17, 30, 0, 0, time.UTC),
			})
			loadBack, loadErr = store.LoadState(aliceProfile)
		})

		It("should not error on save", func() {
			Expect(saveErr).ToNot(HaveOccurred())
		})

		It("should be readable via LoadState (round-trip)", func() {
			Expect(loadErr).ToNot(HaveOccurred())
			Expect(loadBack.Email).To(Equal("alice@example.com"))
			Expect(loadBack.MasterToken).To(Equal("oauth2rt_1/fake"))
		})

		It("should preserve the identifier-pre-existing frontmatter", func() {
			_, fm, err := pages.ReadFrontMatter(aliceProfile)
			Expect(err).ToNot(HaveOccurred())
			Expect(fm["identifier"]).To(Equal("profile_alice"))
		})
	})
})
