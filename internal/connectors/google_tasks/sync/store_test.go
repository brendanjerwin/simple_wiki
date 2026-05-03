//revive:disable:dot-imports
package sync_test

import (
	"errors"
	"os"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	taskssync "github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/sync"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// fakePages is the in-memory PageReaderMutator used by the sync
// package's tests. Methods on PageReaderMutator that the sync code
// doesn't exercise panic so a future change can't silently start
// hitting an unimplemented surface.
type fakePages struct {
	mu       sync.Mutex
	pages    map[wikipage.PageIdentifier]wikipage.FrontMatter
	markdown map[wikipage.PageIdentifier]wikipage.Markdown
}

func newFakePages() *fakePages {
	return &fakePages{pages: make(map[wikipage.PageIdentifier]wikipage.FrontMatter)}
}

func (s *fakePages) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fm, ok := s.pages[id]
	if !ok {
		return id, nil, os.ErrNotExist
	}
	return id, deepCopyFM(fm), nil
}

func (s *fakePages) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pages[id] = deepCopyFM(fm)
	return nil
}

func (s *fakePages) ReadMarkdown(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	if s.markdown == nil {
		return id, "", nil
	}
	return id, s.markdown[id], nil
}

func (s *fakePages) WriteMarkdown(id wikipage.PageIdentifier, md wikipage.Markdown) error {
	if s.markdown == nil {
		s.markdown = make(map[wikipage.PageIdentifier]wikipage.Markdown)
	}
	s.markdown[id] = md
	return nil
}

func (*fakePages) DeletePage(wikipage.PageIdentifier) error {
	panic("DeletePage not used by sync tests")
}

func (*fakePages) ModifyMarkdown(wikipage.PageIdentifier, func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	panic("ModifyMarkdown not used by sync tests")
}

// deepCopyFM is a recursive copy so tests can mutate captured
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

const (
	aliceProfile wikipage.PageIdentifier = "profile_alice"
	bobProfile   wikipage.PageIdentifier = "profile_bob"
)

func newStore(pages *fakePages) *taskssync.SubscriptionStore {
	store, err := taskssync.NewSubscriptionStore(pages)
	if err != nil {
		// In test setup; surface immediately so the spec author sees it.
		panic(err)
	}
	return store
}

// fixedTime is a deterministic timestamp used across encoder/decoder
// tests so byte-for-byte equality can be asserted.
func fixedTime(seconds int) time.Time {
	return time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC).Add(time.Duration(seconds) * time.Second)
}

var _ = Describe("NewSubscriptionStore", func() {
	When("pages is nil", func() {
		var newErr error

		BeforeEach(func() {
			_, newErr = taskssync.NewSubscriptionStore(nil)
		})

		It("should return an error", func() {
			Expect(newErr).To(MatchError(ContainSubstring("pages must not be nil")))
		})
	})
})

var _ = Describe("SubscriptionStore.LoadState", func() {
	var (
		store *taskssync.SubscriptionStore
		pages *fakePages
	)

	BeforeEach(func() {
		pages = newFakePages()
		store = newStore(pages)
	})

	When("the profile page does not exist", func() {
		var (
			state   taskssync.ConnectorState
			loadErr error
		)

		BeforeEach(func() {
			state, loadErr = store.LoadState(aliceProfile)
		})

		It("should not error", func() {
			Expect(loadErr).ToNot(HaveOccurred())
		})

		It("should return zero state", func() {
			Expect(state).To(Equal(taskssync.ConnectorState{}))
		})
	})

	When("the profile exists but has no connector frontmatter", func() {
		var (
			state   taskssync.ConnectorState
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
			Expect(state).To(Equal(taskssync.ConnectorState{}))
		})
	})

	When("the profile has a complete connector state on disk", func() {
		var (
			state   taskssync.ConnectorState
			loadErr error
		)

		BeforeEach(func() {
			Expect(pages.WriteFrontMatter(aliceProfile, wikipage.FrontMatter{
				"wiki": map[string]any{
					"connectors": map[string]any{
						"google_tasks": map[string]any{
							"email":            "alice@example.com",
							"refresh_token":    "1//rt-fake",
							"connected_at":     "2026-04-25T17:14:00Z",
							"last_verified_at": "2026-04-25T17:30:00Z",
							"subscriptions": []any{
								map[string]any{
									"page":              "shopping_lists",
									"list_name":         "groceries",
									"remote_list_id":    "tlist-1",
									"remote_list_title": "Groceries",
									"item_id_map": map[string]any{
										"01HF": "task-1",
									},
									"item_etags": map[string]any{
										"task-1": "etag-1",
									},
									"last_updated_min":        "2026-04-25T17:14:00Z",
									"last_successful_sync_at": "2026-04-25T17:14:30Z",
									"state":                   "active",
									"subscribed_at":           "2026-04-25T17:14:00Z",
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

		It("should populate Email and RefreshToken", func() {
			Expect(state.Email).To(Equal("alice@example.com"))
			Expect(state.RefreshToken).To(Equal("1//rt-fake"))
		})

		It("should parse ConnectedAt as a UTC time", func() {
			expected := time.Date(2026, 4, 25, 17, 14, 0, 0, time.UTC)
			Expect(state.ConnectedAt).To(BeTemporally("~", expected, time.Second))
		})

		It("should populate the subscriptions slice", func() {
			Expect(state.Subscriptions).To(HaveLen(1))
			Expect(state.Subscriptions[0].Page).To(Equal("shopping_lists"))
			Expect(state.Subscriptions[0].ListName).To(Equal("groceries"))
			Expect(state.Subscriptions[0].RemoteListID).To(Equal("tlist-1"))
			Expect(state.Subscriptions[0].RemoteListTitle).To(Equal("Groceries"))
		})

		It("should populate ItemIDMap", func() {
			Expect(state.Subscriptions[0].ItemIDMap).To(HaveKeyWithValue("01HF", "task-1"))
		})

		It("should populate ItemEtags", func() {
			Expect(state.Subscriptions[0].ItemEtags).To(HaveKeyWithValue("task-1", "etag-1"))
		})

		It("should default state to Active when present as 'active'", func() {
			Expect(state.Subscriptions[0].State).To(Equal(taskssync.SubscriptionStateActive))
		})
	})

	When("the subscription has no state field", func() {
		var (
			state   taskssync.ConnectorState
			loadErr error
		)

		BeforeEach(func() {
			Expect(pages.WriteFrontMatter(aliceProfile, wikipage.FrontMatter{
				"wiki": map[string]any{
					"connectors": map[string]any{
						"google_tasks": map[string]any{
							"refresh_token": "rt",
							"subscriptions": []any{
								map[string]any{
									"page":           "p",
									"list_name":      "l",
									"remote_list_id": "tl",
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

		It("should default to Active", func() {
			Expect(state.Subscriptions[0].State).To(Equal(taskssync.SubscriptionStateActive))
		})
	})

	When("a connected_at value is unparseable", func() {
		var loadErr error

		BeforeEach(func() {
			Expect(pages.WriteFrontMatter(aliceProfile, wikipage.FrontMatter{
				"wiki": map[string]any{
					"connectors": map[string]any{
						"google_tasks": map[string]any{
							"connected_at": "not-a-time",
						},
					},
				},
			})).To(Succeed())
			_, loadErr = store.LoadState(aliceProfile)
		})

		It("should error rather than silently zeroing the value", func() {
			Expect(loadErr).To(MatchError(ContainSubstring("connected_at")))
		})
	})
})

var _ = Describe("SubscriptionStore.SaveState", func() {
	var (
		store *taskssync.SubscriptionStore
		pages *fakePages
	)

	BeforeEach(func() {
		pages = newFakePages()
		store = newStore(pages)
	})

	When("saving a fully-populated state", func() {
		var (
			saved   taskssync.ConnectorState
			roundtripped taskssync.ConnectorState
			roundtripErr error
		)

		BeforeEach(func() {
			saved = taskssync.ConnectorState{
				Email:          "alice@example.com",
				RefreshToken:   "1//rt-fake",
				ConnectedAt:    fixedTime(0),
				LastVerifiedAt: fixedTime(60),
				Subscriptions: []taskssync.Subscription{{
					Page:                 "shopping_lists",
					ListName:             "groceries",
					RemoteListID:         "tlist-1",
					RemoteListTitle:      "Groceries",
					ItemIDMap:            map[string]string{"01HF": "task-1"},
					ItemEtags:            map[string]string{"task-1": "etag-1"},
					LastUpdatedMin:       fixedTime(120),
					LastSuccessfulSyncAt: fixedTime(150),
					State:                taskssync.SubscriptionStateActive,
					SubscribedAt:         fixedTime(0),
				}},
			}
			Expect(store.SaveState(aliceProfile, saved)).To(Succeed())
			roundtripped, roundtripErr = store.LoadState(aliceProfile)
		})

		It("should not error on round-trip load", func() {
			Expect(roundtripErr).ToNot(HaveOccurred())
		})

		It("should round-trip Email", func() {
			Expect(roundtripped.Email).To(Equal(saved.Email))
		})

		It("should round-trip RefreshToken", func() {
			Expect(roundtripped.RefreshToken).To(Equal(saved.RefreshToken))
		})

		It("should round-trip the subscription list", func() {
			Expect(roundtripped.Subscriptions).To(HaveLen(1))
			Expect(roundtripped.Subscriptions[0].Page).To(Equal("shopping_lists"))
			Expect(roundtripped.Subscriptions[0].RemoteListID).To(Equal("tlist-1"))
		})

		It("should round-trip ItemIDMap", func() {
			Expect(roundtripped.Subscriptions[0].ItemIDMap).To(HaveKeyWithValue("01HF", "task-1"))
		})

		It("should round-trip the LastUpdatedMin cursor", func() {
			Expect(roundtripped.Subscriptions[0].LastUpdatedMin).To(BeTemporally("~", saved.Subscriptions[0].LastUpdatedMin, time.Second))
		})
	})

	When("saving a paused subscription", func() {
		var roundtripped taskssync.ConnectorState

		BeforeEach(func() {
			saved := taskssync.ConnectorState{
				Email:        "alice@example.com",
				RefreshToken: "rt",
				Subscriptions: []taskssync.Subscription{{
					Page:           "p",
					ListName:       "l",
					RemoteListID:   "tl",
					State:          taskssync.SubscriptionStatePaused,
					PausedReason:   taskssync.PausedReasonAuthFailed,
					PausedAt:       fixedTime(0),
					LastUpdatedMin: fixedTime(120),
					ItemIDMap:      map[string]string{"u1": "task-x"},
				}},
			}
			Expect(store.SaveState(aliceProfile, saved)).To(Succeed())
			var err error
			roundtripped, err = store.LoadState(aliceProfile)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should preserve the paused state", func() {
			Expect(roundtripped.Subscriptions[0].State).To(Equal(taskssync.SubscriptionStatePaused))
		})

		It("should preserve the paused reason", func() {
			Expect(roundtripped.Subscriptions[0].PausedReason).To(Equal(taskssync.PausedReasonAuthFailed))
		})

		It("should preserve the cursor (frozen during pause)", func() {
			Expect(roundtripped.Subscriptions[0].LastUpdatedMin).To(BeTemporally("~", fixedTime(120), time.Second))
		})

		It("should preserve the item_id_map (so reconnect can resume)", func() {
			Expect(roundtripped.Subscriptions[0].ItemIDMap).To(HaveKeyWithValue("u1", "task-x"))
		})
	})

	When("saving a state with no subscriptions and an empty refresh token", func() {
		var roundtripped taskssync.ConnectorState

		BeforeEach(func() {
			Expect(store.SaveState(aliceProfile, taskssync.ConnectorState{})).To(Succeed())
			var err error
			roundtripped, err = store.LoadState(aliceProfile)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should round-trip as the zero state", func() {
			Expect(roundtripped).To(Equal(taskssync.ConnectorState{}))
		})
	})
})

var _ = Describe("SubscriptionStore.AddSubscription", func() {
	var (
		store *taskssync.SubscriptionStore
		pages *fakePages
	)

	BeforeEach(func() {
		pages = newFakePages()
		store = newStore(pages)
	})

	When("the connector is not configured", func() {
		var addErr error

		BeforeEach(func() {
			addErr = store.AddSubscription(aliceProfile, taskssync.Subscription{
				Page: "p", ListName: "l", RemoteListID: "tl",
			})
		})

		It("should return ErrConnectorNotConfigured", func() {
			Expect(addErr).To(MatchError(taskssync.ErrConnectorNotConfigured))
		})
	})

	When("the calling profile already owns a subscription for (page, list_name)", func() {
		var addErr error

		BeforeEach(func() {
			Expect(store.SaveState(aliceProfile, taskssync.ConnectorState{
				RefreshToken: "rt",
				Subscriptions: []taskssync.Subscription{{
					Page: "p", ListName: "l", RemoteListID: "tl-old",
				}},
			})).To(Succeed())
			addErr = store.AddSubscription(aliceProfile, taskssync.Subscription{
				Page: "p", ListName: "l", RemoteListID: "tl-new",
			})
		})

		It("should return ErrAlreadySubscribedForChecklist", func() {
			Expect(addErr).To(MatchError(taskssync.ErrAlreadySubscribedForChecklist))
		})
	})

	When("the connector is configured and there is no collision", func() {
		var addErr error

		BeforeEach(func() {
			Expect(store.SaveState(aliceProfile, taskssync.ConnectorState{RefreshToken: "rt"})).To(Succeed())
			addErr = store.AddSubscription(aliceProfile, taskssync.Subscription{
				Page: "p", ListName: "l", RemoteListID: "tl",
			})
		})

		It("should not error", func() {
			Expect(addErr).ToNot(HaveOccurred())
		})

		It("should be findable on the next read", func() {
			sub, found, err := store.FindSubscription(aliceProfile, "p", "l")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(sub.RemoteListID).To(Equal("tl"))
		})
	})
})

var _ = Describe("SubscriptionStore.RemoveSubscription", func() {
	var (
		store *taskssync.SubscriptionStore
		pages *fakePages
	)

	BeforeEach(func() {
		pages = newFakePages()
		store = newStore(pages)
	})

	When("no subscription matches", func() {
		var removeErr error

		BeforeEach(func() {
			Expect(store.SaveState(aliceProfile, taskssync.ConnectorState{RefreshToken: "rt"})).To(Succeed())
			removeErr = store.RemoveSubscription(aliceProfile, "p", "l")
		})

		It("should return ErrSubscriptionNotFound", func() {
			Expect(removeErr).To(MatchError(taskssync.ErrSubscriptionNotFound))
		})
	})

	When("a matching subscription exists", func() {
		var removeErr error

		BeforeEach(func() {
			Expect(store.SaveState(aliceProfile, taskssync.ConnectorState{
				RefreshToken: "rt",
				Subscriptions: []taskssync.Subscription{{
					Page: "p", ListName: "l", RemoteListID: "tl",
				}},
			})).To(Succeed())
			removeErr = store.RemoveSubscription(aliceProfile, "p", "l")
		})

		It("should not error", func() {
			Expect(removeErr).ToNot(HaveOccurred())
		})

		It("should make the subscription unfindable", func() {
			_, found, err := store.FindSubscription(aliceProfile, "p", "l")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeFalse())
		})
	})
})

var _ = Describe("SubscriptionStore.UpdateSubscription", func() {
	var (
		store *taskssync.SubscriptionStore
		pages *fakePages
	)

	BeforeEach(func() {
		pages = newFakePages()
		store = newStore(pages)
	})

	When("a subscription exists for the supplied (page, list_name)", func() {
		var updateErr error
		var sub taskssync.Subscription

		BeforeEach(func() {
			Expect(store.SaveState(aliceProfile, taskssync.ConnectorState{
				RefreshToken: "rt",
				Subscriptions: []taskssync.Subscription{{
					Page: "p", ListName: "l", RemoteListID: "tl",
					LastUpdatedMin: fixedTime(0),
				}},
			})).To(Succeed())
			sub = taskssync.Subscription{
				Page: "p", ListName: "l", RemoteListID: "tl",
				LastUpdatedMin: fixedTime(120),
				ItemIDMap:      map[string]string{"u1": "t1"},
			}
			updateErr = store.UpdateSubscription(aliceProfile, sub)
		})

		It("should not error", func() {
			Expect(updateErr).ToNot(HaveOccurred())
		})

		It("should persist the updated cursor", func() {
			got, found, err := store.FindSubscription(aliceProfile, "p", "l")
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(got.LastUpdatedMin).To(BeTemporally("~", fixedTime(120), time.Second))
		})

		It("should persist the updated item_id_map", func() {
			got, _, _ := store.FindSubscription(aliceProfile, "p", "l")
			Expect(got.ItemIDMap).To(HaveKeyWithValue("u1", "t1"))
		})
	})

	When("no matching subscription exists", func() {
		var updateErr error

		BeforeEach(func() {
			Expect(store.SaveState(aliceProfile, taskssync.ConnectorState{RefreshToken: "rt"})).To(Succeed())
			updateErr = store.UpdateSubscription(aliceProfile, taskssync.Subscription{
				Page: "p", ListName: "l",
			})
		})

		It("should return ErrSubscriptionNotFound", func() {
			Expect(errors.Is(updateErr, taskssync.ErrSubscriptionNotFound)).To(BeTrue())
		})
	})
})

var _ = Describe("SubscriptionStore.WithProfileLock", func() {
	var (
		store *taskssync.SubscriptionStore
		pages *fakePages
	)

	BeforeEach(func() {
		pages = newFakePages()
		store = newStore(pages)
	})

	When("the callback succeeds", func() {
		var (
			fnCalled bool
			lockErr  error
		)

		BeforeEach(func() {
			fnCalled = false
			lockErr = store.WithProfileLock(aliceProfile, func() error {
				fnCalled = true
				return nil
			})
		})

		It("should call the function", func() {
			Expect(fnCalled).To(BeTrue())
		})

		It("should not error", func() {
			Expect(lockErr).ToNot(HaveOccurred())
		})
	})

	When("the callback returns an error", func() {
		var lockErr error

		BeforeEach(func() {
			lockErr = store.WithProfileLock(aliceProfile, func() error {
				return errors.New("fn error")
			})
		})

		It("should propagate the error", func() {
			Expect(lockErr).To(MatchError("fn error"))
		})
	})
})

var _ = Describe("SubscriptionStore.LoadStateLocked / SaveStateLocked", func() {
	var (
		store *taskssync.SubscriptionStore
		pages *fakePages
	)

	BeforeEach(func() {
		pages = newFakePages()
		store = newStore(pages)
	})

	When("used within WithProfileLock", func() {
		var (
			originalEmail string
			saveErr       error
		)

		BeforeEach(func() {
			// First save some state via the public API so there's something to load.
			initial := taskssync.ConnectorState{
				Email:        "alice@example.com",
				RefreshToken: "rt-initial",
			}
			Expect(store.SaveState(aliceProfile, initial)).To(Succeed())

			// Use WithProfileLock to exercise LoadStateLocked + SaveStateLocked.
			saveErr = store.WithProfileLock(aliceProfile, func() error {
				loadedState, err := store.LoadStateLocked(aliceProfile)
				if err != nil {
					return err
				}
				originalEmail = loadedState.Email // capture before mutation
				loadedState.Email = "alice-updated@example.com"
				return store.SaveStateLocked(aliceProfile, loadedState)
			})
		})

		It("should not error on WithProfileLock", func() {
			Expect(saveErr).ToNot(HaveOccurred())
		})

		It("should load the previously saved state via LoadStateLocked", func() {
			Expect(originalEmail).To(Equal("alice@example.com"))
		})

		It("should persist the updated state via SaveStateLocked", func() {
			state, err := store.LoadState(aliceProfile)
			Expect(err).ToNot(HaveOccurred())
			Expect(state.Email).To(Equal("alice-updated@example.com"))
		})
	})
})
