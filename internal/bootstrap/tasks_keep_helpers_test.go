//revive:disable:dot-imports
package bootstrap

import (
	"context"
	"errors"
	"time"

	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/engine"
	enginetesting "github.com/brendanjerwin/simple_wiki/internal/connectors/engine/testing"
	googletasks "github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// --- no-op engine port stubs (for engine construction in pause/resume tests) ---

type noopChecklistReader struct{}

func (noopChecklistReader) ListItems(_ context.Context, _, _ string) (*apiv1.Checklist, error) {
	return &apiv1.Checklist{}, nil
}

type noopChecklistMutator struct{}

func (noopChecklistMutator) AddItemForSync(_ context.Context, _, _, _, _ string, _ bool, _ []string, _, _ string) (string, error) {
	return "", nil
}

func (noopChecklistMutator) UpdateItemForSync(_ context.Context, _, _, _, _, _ string, _ bool, _ []string, _ string) error {
	return nil
}

func (noopChecklistMutator) DeleteItemForSync(_ context.Context, _, _, _, _ string) error {
	return nil
}

func (noopChecklistMutator) AppendSyncEvent(_ context.Context, _, _, _, _ string) error {
	return nil
}

type noopSuppressor struct{}

func (noopSuppressor) Suppress(_ wikipage.PageIdentifier, _, _ string)   {}
func (noopSuppressor) Unsuppress(_ wikipage.PageIdentifier, _, _ string) {}

// stubEngineLogger swallows log messages.
type stubEngineLogger struct{}

func (stubEngineLogger) Info(_ string, _ ...any)  {}
func (stubEngineLogger) Warn(_ string, _ ...any)  {}
func (stubEngineLogger) Error(_ string, _ ...any) {}

// buildEngine creates a minimal engine wired to the provided adapter, lease
// table, and binding store with no-op wiki-side ports.
func buildEngine(
	adapter connectors.BackendAdapter,
	leaseTable *connectors.LeaseTable,
	store engine.BindingStore,
) *engine.Engine {
	fc := enginetesting.NewFakeClock(time.Now())
	eng, err := engine.NewEngine(
		adapter,
		leaseTable,
		noopChecklistReader{},
		noopChecklistMutator{},
		noopSuppressor{},
		stubEngineLogger{},
		fc,
		store,
	)
	if err != nil {
		panic("buildEngine: " + err.Error())
	}
	return eng
}

// --- realTimerScheduler ---------------------------------------------------

var _ = Describe("realTimerScheduler", func() {
	Describe("AfterFunc", func() {
		It("should return a non-nil Timer", func() {
			sched := realTimerScheduler{}
			timer := sched.AfterFunc(10*time.Second, func() {})
			Expect(timer).NotTo(BeNil())
			timer.Stop() // clean up
		})

		It("should stop the timer before it fires", func() {
			sched := realTimerScheduler{}
			fired := false
			timer := sched.AfterFunc(10*time.Second, func() { fired = true })
			stopped := timer.Stop()
			Expect(stopped).To(BeTrue())
			// Brief sleep to confirm the callback never ran.
			time.Sleep(20 * time.Millisecond)
			Expect(fired).To(BeFalse())
		})
	})
})

// --- tasksProfileTokenStore -----------------------------------------------

var _ = Describe("tasksProfileTokenStore", func() {
	var (
		pages     *memoryFakePages
		credStore *googletasks.FrontmatterCredentialStore
		store     *tasksProfileTokenStore
		profileID = wikipage.PageIdentifier("profile_alice_abc12345")
	)

	BeforeEach(func() {
		pages = newMemoryFakePages()
		var err error
		credStore, err = googletasks.NewFrontmatterCredentialStore(
			pages, googletasks.SystemClock{}, lumber.NewConsoleLogger(lumber.WARN), nil, nil,
		)
		Expect(err).NotTo(HaveOccurred())
		store = &tasksProfileTokenStore{store: credStore, profileID: profileID}
	})

	Describe("LoadRefreshToken", func() {
		When("no credentials are persisted on the profile", func() {
			var (
				token    string
				loadErr  error
			)

			BeforeEach(func() {
				token, loadErr = store.LoadRefreshToken(context.Background())
			})

			It("should return an error", func() {
				Expect(loadErr).To(HaveOccurred())
			})

			It("should return an empty token", func() {
				Expect(token).To(BeEmpty())
			})
		})

		When("a refresh token is persisted on the profile", func() {
			var (
				token   string
				loadErr error
			)

			BeforeEach(func() {
				pages.pages[profileID] = wikipage.FrontMatter{
					"wiki": map[string]any{
						"connectors": map[string]any{
							"google_tasks": map[string]any{
								"refresh_token": "rt-test-token",
							},
						},
					},
				}
				token, loadErr = store.LoadRefreshToken(context.Background())
			})

			It("should not error", func() {
				Expect(loadErr).NotTo(HaveOccurred())
			})

			It("should return the stored token", func() {
				Expect(token).To(Equal("rt-test-token"))
			})
		})
	})

	Describe("SaveRefreshToken", func() {
		When("the token is empty", func() {
			It("should return an error", func() {
				err := store.SaveRefreshToken(context.Background(), "")
				Expect(err).To(HaveOccurred())
			})
		})

		When("a non-empty token is provided", func() {
			var saveErr error

			BeforeEach(func() {
				saveErr = store.SaveRefreshToken(context.Background(), "rt-new-token")
			})

			It("should not error", func() {
				Expect(saveErr).NotTo(HaveOccurred())
			})

			It("should persist the token so LoadRefreshToken returns it", func() {
				loaded, err := store.LoadRefreshToken(context.Background())
				Expect(err).NotTo(HaveOccurred())
				Expect(loaded).To(Equal("rt-new-token"))
			})
		})
	})
})

// --- rebuildLeaseTableKeepFromBindings ------------------------------------

func seedKeepProfileWithBinding(pages *memoryFakePages, profileID wikipage.PageIdentifier, page, listName string) {
	GinkgoHelper()
	pages.pages[profileID] = wikipage.FrontMatter{
		"wiki": map[string]any{
			"connectors": map[string]any{
				"google_keep": map[string]any{
					"master_token": "mt-test-token",
					"bindings": []any{
						map[string]any{
							"page":      page,
							"list_name": listName,
							"state":     "active",
						},
					},
				},
			},
		},
	}
}

var _ = Describe("rebuildLeaseTableKeepFromBindings", func() {
	When("a Keep profile with a binding exists", func() {
		var (
			pages        *memoryFakePages
			bindingStore engine.BindingStore
			leaseTable   *connectors.LeaseTable
			fakeIndex    *fakeFrontmatterIndex
			site         *server.Site
			leaseCount   int
			rebuildErr   error
			profileID    = wikipage.PageIdentifier("profile_keepuser_abc12345")
			pageName     = "notes"
			listName     = "Groceries"
		)

		BeforeEach(func() {
			pages = newMemoryFakePages()
			seedKeepProfileWithBinding(pages, profileID, pageName, listName)

			fakeIndex = &fakeFrontmatterIndex{
				profilesByLeafKey: map[frontmatter.DottedKeyPath][]wikipage.PageIdentifier{
					"wiki.connectors.google_keep.master_token": {profileID},
				},
			}
			site = &server.Site{FrontmatterIndexQueryer: fakeIndex}
			leaseTable = connectors.NewLeaseTable()

			lister := &memoryProfileLister{
				hits: map[frontmatter.DottedKeyPath][]wikipage.PageIdentifier{
					"wiki.connectors.google_keep.bindings": {profileID},
				},
			}
			var err error
			bindingStore, err = engine.NewFrontmatterBindingStore(pages, lister, lumber.NewConsoleLogger(lumber.WARN))
			Expect(err).NotTo(HaveOccurred())

			leaseCount, rebuildErr = rebuildLeaseTableKeepFromBindings(leaseTable, site, bindingStore)
		})

		It("should not error", func() {
			Expect(rebuildErr).NotTo(HaveOccurred())
		})

		It("should count one lease", func() {
			Expect(leaseCount).To(Equal(1))
		})

		It("should record the lease so LookupOwner finds it", func() {
			owner, found := leaseTable.LookupOwner(connectors.ChecklistKey{Page: pageName, ListName: listName})
			Expect(found).To(BeTrue())
			Expect(owner.Kind).To(Equal(connectors.ConnectorKindGoogleKeep))
			Expect(owner.ProfileID).To(Equal(string(profileID)))
		})
	})
})

// --- keepFannedOutPausedChecker -------------------------------------------

func seedKeepProfileWithPausedBinding(pages *memoryFakePages, profileID wikipage.PageIdentifier, page, listName string) {
	GinkgoHelper()
	pages.pages[profileID] = wikipage.FrontMatter{
		"wiki": map[string]any{
			"connectors": map[string]any{
				"google_keep": map[string]any{
					"master_token": "mt-test-token",
					"bindings": []any{
						map[string]any{
							"page":          page,
							"list_name":     listName,
							"state":         "paused",
							"paused_reason": "auth_failed",
						},
					},
				},
			},
		},
	}
}

var _ = Describe("keepFannedOutPausedChecker", func() {
	var (
		pages        *memoryFakePages
		bindingStore engine.BindingStore
		fakeIndex    *fakeFrontmatterIndex
		checker      *keepFannedOutPausedChecker
		profileID    = wikipage.PageIdentifier("profile_keepuser_abc12345")
		pageName     = "notes"
		listName     = "Groceries"
	)

	BeforeEach(func() {
		pages = newMemoryFakePages()
		fakeIndex = &fakeFrontmatterIndex{}
		lister := &memoryProfileLister{}
		var err error
		bindingStore, err = engine.NewFrontmatterBindingStore(pages, lister, lumber.NewConsoleLogger(lumber.WARN))
		Expect(err).NotTo(HaveOccurred())
		checker = &keepFannedOutPausedChecker{
			bindings: bindingStore,
			index:    fakeIndex,
		}
	})

	When("no profiles have the Keep connector configured", func() {
		It("should return false", func() {
			Expect(checker.IsAnyChecklistBindingPaused(pageName, listName)).To(BeFalse())
		})
	})

	When("a profile has an active (non-paused) binding", func() {
		BeforeEach(func() {
			seedKeepProfileWithBinding(pages, profileID, pageName, listName)
			fakeIndex.profilesByLeafKey = map[frontmatter.DottedKeyPath][]wikipage.PageIdentifier{
				"wiki.connectors.google_keep.master_token": {profileID},
			}
			lister := &memoryProfileLister{
				hits: map[frontmatter.DottedKeyPath][]wikipage.PageIdentifier{
					"wiki.connectors.google_keep.bindings": {profileID},
				},
			}
			var err error
			bindingStore, err = engine.NewFrontmatterBindingStore(pages, lister, lumber.NewConsoleLogger(lumber.WARN))
			Expect(err).NotTo(HaveOccurred())
			checker = &keepFannedOutPausedChecker{bindings: bindingStore, index: fakeIndex}
		})

		It("should return false", func() {
			Expect(checker.IsAnyChecklistBindingPaused(pageName, listName)).To(BeFalse())
		})
	})

	When("a profile has a paused binding", func() {
		var isPaused bool

		BeforeEach(func() {
			seedKeepProfileWithPausedBinding(pages, profileID, pageName, listName)
			fakeIndex.profilesByLeafKey = map[frontmatter.DottedKeyPath][]wikipage.PageIdentifier{
				"wiki.connectors.google_keep.master_token": {profileID},
			}
			lister := &memoryProfileLister{
				hits: map[frontmatter.DottedKeyPath][]wikipage.PageIdentifier{
					"wiki.connectors.google_keep.bindings": {profileID},
				},
			}
			var err error
			bindingStore, err = engine.NewFrontmatterBindingStore(pages, lister, lumber.NewConsoleLogger(lumber.WARN))
			Expect(err).NotTo(HaveOccurred())
			checker = &keepFannedOutPausedChecker{bindings: bindingStore, index: fakeIndex}

			isPaused = checker.IsAnyChecklistBindingPaused(pageName, listName)
		})

		It("should return true", func() {
			Expect(isPaused).To(BeTrue())
		})
	})
})

// --- pauseAllTasksBindings / resumeAllTasksBindings -----------------------

var _ = Describe("pauseAllTasksBindings", func() {
	When("a profile has one active and one already-paused Tasks binding", func() {
		var (
			fbs        *enginetesting.FakeBindingStore
			eng        *engine.Engine
			pauseErr   error
			profileID  = wikipage.PageIdentifier("profile_tasksuser_abc12345")
			activePage = "groceries"
			pausedPage = "work"
			listName   = "Tasks"
		)

		BeforeEach(func() {
			fbs = enginetesting.NewFakeBindingStore()
			leaseTable := connectors.NewLeaseTable()

			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID,
				Page:      activePage,
				ListName:  listName,
				State:     connectors.BindingStateActive,
			}, connectors.ConnectorKindGoogleTasks)

			fbs.SeedBinding(connectors.Binding{
				ProfileID:    profileID,
				Page:         pausedPage,
				ListName:     listName,
				State:        connectors.BindingStatePaused,
				PausedReason: "auth_failed",
			}, connectors.ConnectorKindGoogleTasks)

			fa := &enginetesting.FakeAdapter{ConnectorKind: connectors.ConnectorKindGoogleTasks}
			eng = buildEngine(fa, leaseTable, fbs)

			pauseErr = pauseAllTasksBindings(context.Background(), eng, fbs, profileID, "auth_failed")
		})

		It("should not error", func() {
			Expect(pauseErr).NotTo(HaveOccurred())
		})

		It("should pause the active binding", func() {
			var pausedActive bool
			for _, r := range fbs.RecordedSaveBinding {
				if r.Binding.Page == activePage && r.Binding.State == connectors.BindingStatePaused {
					pausedActive = true
				}
			}
			Expect(pausedActive).To(BeTrue())
		})

		It("should not touch the already-paused binding", func() {
			for _, r := range fbs.RecordedSaveBinding {
				Expect(r.Binding.Page).NotTo(Equal(pausedPage),
					"already-paused binding should not be saved again")
			}
		})
	})
})

var _ = Describe("resumeAllTasksBindings", func() {
	When("a profile has a recently-paused Tasks binding", func() {
		var (
			fbs       *enginetesting.FakeBindingStore
			eng       *engine.Engine
			resumeErr error
			profileID = wikipage.PageIdentifier("profile_tasksuser_abc12345")
			pageName  = "groceries"
			listName  = "Tasks"
		)

		BeforeEach(func() {
			fbs = enginetesting.NewFakeBindingStore()
			leaseTable := connectors.NewLeaseTable()
			leaseTable.SignalReady()

			// Binding paused just now — within the 7-day horizon.
			fbs.SeedBinding(connectors.Binding{
				ProfileID:    profileID,
				Page:         pageName,
				ListName:     listName,
				State:        connectors.BindingStatePaused,
				PausedReason: "auth_failed",
				PausedAt:     time.Now(),
			}, connectors.ConnectorKindGoogleTasks)

			fa := &enginetesting.FakeAdapter{ConnectorKind: connectors.ConnectorKindGoogleTasks}
			eng = buildEngine(fa, leaseTable, fbs)

			resumeErr = resumeAllTasksBindings(context.Background(), eng, fbs, profileID)
		})

		It("should not error", func() {
			Expect(resumeErr).NotTo(HaveOccurred())
		})

		It("should flip the binding to active", func() {
			var resumed bool
			for _, r := range fbs.RecordedSaveBinding {
				if r.Binding.Page == pageName && r.Binding.State == connectors.BindingStateActive {
					resumed = true
				}
			}
			Expect(resumed).To(BeTrue())
		})
	})
})

// --- pauseAllKeepBindings / resumeAllKeepBindings -------------------------

var _ = Describe("pauseAllKeepBindings", func() {
	When("a profile has one active Keep binding", func() {
		var (
			fbs       *enginetesting.FakeBindingStore
			eng       *engine.Engine
			pauseErr  error
			profileID = wikipage.PageIdentifier("profile_keepuser2_abc12345")
			pageName  = "notes"
			listName  = "Reminders"
		)

		BeforeEach(func() {
			fbs = enginetesting.NewFakeBindingStore()
			leaseTable := connectors.NewLeaseTable()

			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID,
				Page:      pageName,
				ListName:  listName,
				State:     connectors.BindingStateActive,
			}, connectors.ConnectorKindGoogleKeep)

			fa := &enginetesting.FakeAdapter{ConnectorKind: connectors.ConnectorKindGoogleKeep}
			eng = buildEngine(fa, leaseTable, fbs)

			pauseErr = pauseAllKeepBindings(context.Background(), eng, fbs, profileID, "auth_failed")
		})

		It("should not error", func() {
			Expect(pauseErr).NotTo(HaveOccurred())
		})

		It("should pause the active binding", func() {
			var paused bool
			for _, r := range fbs.RecordedSaveBinding {
				if r.Binding.Page == pageName && r.Binding.State == connectors.BindingStatePaused {
					paused = true
				}
			}
			Expect(paused).To(BeTrue())
		})
	})
})

var _ = Describe("pauseAllKeepBindings — error paths", func() {
	When("store.LoadBindings returns an error", func() {
		var (
			fbs       *enginetesting.FakeBindingStore
			eng       *engine.Engine
			pauseErr  error
			profileID = wikipage.PageIdentifier("profile_keeperr_abc12345")
		)

		BeforeEach(func() {
			fbs = enginetesting.NewFakeBindingStore()
			leaseTable := connectors.NewLeaseTable()
			fa := &enginetesting.FakeAdapter{ConnectorKind: connectors.ConnectorKindGoogleKeep}
			eng = buildEngine(fa, leaseTable, fbs)

			fbs.SetLoadBindingsError(errors.New("disk read error"))
			pauseErr = pauseAllKeepBindings(context.Background(), eng, fbs, profileID, "auth_failed")
		})

		It("should return the load error", func() {
			Expect(pauseErr).To(MatchError(ContainSubstring("load bindings")))
		})
	})

	When("engine.TransitionToPaused returns an error for an active binding", func() {
		var (
			fbs       *enginetesting.FakeBindingStore
			eng       *engine.Engine
			pauseErr  error
			profileID = wikipage.PageIdentifier("profile_keeperr2_abc12345")
			pageName  = "errpage"
			listName  = "ErrList"
		)

		BeforeEach(func() {
			fbs = enginetesting.NewFakeBindingStore()
			leaseTable := connectors.NewLeaseTable()

			fbs.SeedBinding(connectors.Binding{
				ProfileID: profileID,
				Page:      pageName,
				ListName:  listName,
				State:     connectors.BindingStateActive,
			}, connectors.ConnectorKindGoogleKeep)

			// Queue a SaveBinding error so TransitionToPaused fails.
			fbs.SetSaveBindingError(errors.New("save failed"))

			fa := &enginetesting.FakeAdapter{ConnectorKind: connectors.ConnectorKindGoogleKeep}
			eng = buildEngine(fa, leaseTable, fbs)

			pauseErr = pauseAllKeepBindings(context.Background(), eng, fbs, profileID, "auth_failed")
		})

		It("should return the first error from TransitionToPaused", func() {
			Expect(pauseErr).To(HaveOccurred())
		})
	})
})

var _ = Describe("resumeAllKeepBindings — error paths", func() {
	When("store.LoadBindings returns an error", func() {
		var (
			fbs       *enginetesting.FakeBindingStore
			eng       *engine.Engine
			resumeErr error
			profileID = wikipage.PageIdentifier("profile_keeperr3_abc12345")
		)

		BeforeEach(func() {
			fbs = enginetesting.NewFakeBindingStore()
			leaseTable := connectors.NewLeaseTable()
			fa := &enginetesting.FakeAdapter{ConnectorKind: connectors.ConnectorKindGoogleKeep}
			eng = buildEngine(fa, leaseTable, fbs)

			fbs.SetLoadBindingsError(errors.New("disk read error"))
			resumeErr = resumeAllKeepBindings(context.Background(), eng, fbs, profileID)
		})

		It("should return the load error", func() {
			Expect(resumeErr).To(MatchError(ContainSubstring("load bindings")))
		})
	})
})

var _ = Describe("resumeAllKeepBindings", func() {
	When("a profile has a recently-paused Keep binding", func() {
		var (
			fbs       *enginetesting.FakeBindingStore
			eng       *engine.Engine
			resumeErr error
			profileID = wikipage.PageIdentifier("profile_keepuser2_abc12345")
			pageName  = "notes"
			listName  = "Reminders"
		)

		BeforeEach(func() {
			fbs = enginetesting.NewFakeBindingStore()
			leaseTable := connectors.NewLeaseTable()
			leaseTable.SignalReady()

			fbs.SeedBinding(connectors.Binding{
				ProfileID:    profileID,
				Page:         pageName,
				ListName:     listName,
				State:        connectors.BindingStatePaused,
				PausedReason: "auth_failed",
				PausedAt:     time.Now(),
			}, connectors.ConnectorKindGoogleKeep)

			fa := &enginetesting.FakeAdapter{ConnectorKind: connectors.ConnectorKindGoogleKeep}
			eng = buildEngine(fa, leaseTable, fbs)

			resumeErr = resumeAllKeepBindings(context.Background(), eng, fbs, profileID)
		})

		It("should not error", func() {
			Expect(resumeErr).NotTo(HaveOccurred())
		})

		It("should flip the binding to active", func() {
			var resumed bool
			for _, r := range fbs.RecordedSaveBinding {
				if r.Binding.Page == pageName && r.Binding.State == connectors.BindingStateActive {
					resumed = true
				}
			}
			Expect(resumed).To(BeTrue())
		})
	})
})

// --- systemWallClock ------------------------------------------------------

var _ = Describe("systemWallClock", func() {
	It("should return a non-zero time", func() {
		clk := systemWallClock{}
		Expect(clk.Now().IsZero()).To(BeFalse())
	})
})

// --- safeTail -------------------------------------------------------------

var _ = Describe("safeTail", func() {
	It("should return the last n chars when s is longer than n", func() {
		Expect(safeTail("abcdef", 3)).To(Equal("def"))
	})

	It("should return the full string when s is shorter than n", func() {
		Expect(safeTail("ab", 10)).To(Equal("ab"))
	})

	It("should return the full string when s is exactly n chars", func() {
		Expect(safeTail("abc", 3)).To(Equal("abc"))
	})
})

// --- newKeepAuthHTTPClient ------------------------------------------------

var _ = Describe("newKeepAuthHTTPClient", func() {
	It("should return a non-nil http.Client", func() {
		client := newKeepAuthHTTPClient()
		Expect(client).NotTo(BeNil())
	})

	It("should have a configured timeout", func() {
		client := newKeepAuthHTTPClient()
		Expect(client.Timeout).To(BeNumerically(">", 0))
	})
})

// --- rebuildLeaseTable (wrapper) ------------------------------------------

var _ = Describe("rebuildLeaseTable", func() {
	When("both wirings are nil", func() {
		It("should not error and produce zero total count", func() {
			leaseTable := connectors.NewLeaseTable()
			logger := lumber.NewConsoleLogger(lumber.WARN)
			err := rebuildLeaseTable(leaseTable, nil, nil, nil, logger)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

// --- engineSyncJob --------------------------------------------------------

// stubConnector is a minimal connectors.Connector for engineSyncJob tests.
type stubConnector struct {
	syncCalled bool
	syncKey    connectors.BindingKey
}

func (*stubConnector) Kind() connectors.ConnectorKind                      { return "stub" }
func (*stubConnector) PausedReason(_ connectors.BindingKey) (string, bool) { return "", false }
func (*stubConnector) ForceFullResync(_ context.Context, _ connectors.BindingKey) error {
	return nil
}
func (s *stubConnector) Sync(_ context.Context, key connectors.BindingKey) error {
	s.syncCalled = true
	s.syncKey = key
	return nil
}

var _ connectors.Connector = (*stubConnector)(nil)

var _ = Describe("engineSyncJob", func() {
	Describe("GetName", func() {
		It("should return the queue name", func() {
			job := &engineSyncJob{queueName: "MyQueue"}
			Expect(job.GetName()).To(Equal("MyQueue"))
		})
	})

	Describe("Execute", func() {
		It("should call Sync on the connector with the stored key", func() {
			connector := &stubConnector{}
			key := connectors.BindingKey{ProfileID: "profile_alice_abc", Page: "groceries", ListName: "Shopping"}
			job := &engineSyncJob{connector: connector, key: key, queueName: "TestQueue"}

			err := job.Execute()

			Expect(err).NotTo(HaveOccurred())
			Expect(connector.syncCalled).To(BeTrue())
			Expect(connector.syncKey).To(Equal(key))
		})
	})
})

// --- frontmatterIndexProfileLister ----------------------------------------

var _ = Describe("frontmatterIndexProfileLister", func() {
	Describe("ListProfilesWithKey", func() {
		It("should delegate to the index's QueryKeyExistence", func() {
			profileID := wikipage.PageIdentifier("profile_test_abc12345")
			idx := &fakeFrontmatterIndex{
				profilesByLeafKey: map[frontmatter.DottedKeyPath][]wikipage.PageIdentifier{
					"wiki.connectors.google_tasks.bindings": {profileID},
				},
			}
			lister := &frontmatterIndexProfileLister{index: idx}
			result := lister.ListProfilesWithKey("wiki.connectors.google_tasks.bindings")
			Expect(result).To(ConsistOf(profileID))
		})
	})
})
