//revive:disable:dot-imports
package server_test

import (
	"errors"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// fakePageStore is an in-memory PageReader/PageWriter for store tests.
type fakePageStore struct {
	mu    sync.Mutex
	pages map[wikipage.PageIdentifier]wikipage.FrontMatter
}

func newFakePageStore() *fakePageStore {
	return &fakePageStore{pages: map[wikipage.PageIdentifier]wikipage.FrontMatter{}}
}

func (f *fakePageStore) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fm, ok := f.pages[id]
	if !ok {
		return id, wikipage.FrontMatter{}, nil
	}
	// Return a deep-ish copy by re-creating the top-level map; tests should not
	// mutate inner structures across calls anyway.
	out := wikipage.FrontMatter{}
	for k, v := range fm {
		out[k] = v
	}
	return id, out, nil
}

func (f *fakePageStore) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pages[id] = fm
	return nil
}

var _ = Describe("AgentScheduleStore", func() {
	var (
		pages *fakePageStore
		store *server.AgentScheduleStore
	)

	BeforeEach(func() {
		pages = newFakePageStore()
		store = server.NewAgentScheduleStore(pages)
	})

	Describe("List", func() {
		Describe("when the page has no frontmatter", func() {
			var schedules []*apiv1.AgentSchedule
			var err error

			BeforeEach(func() {
				schedules, err = store.List("empty_page")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an empty list", func() {
				Expect(schedules).To(BeEmpty())
			})
		})

		Describe("when the page has one schedule", func() {
			var schedules []*apiv1.AgentSchedule
			var err error

			BeforeEach(func() {
				_ = pages.WriteFrontMatter("seeded", wikipage.FrontMatter{
					"agent": map[string]any{
						"schedules": []any{
							map[string]any{
								"id":        "weekly",
								"cron":      "0 0 9 * * 1",
								"prompt":    "weekly check",
								"max_turns": float64(15),
								"enabled":   true,
							},
						},
					},
				})
				schedules, err = store.List("seeded")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return one schedule", func() {
				Expect(schedules).To(HaveLen(1))
			})

			It("should preserve the id", func() {
				Expect(schedules[0].GetId()).To(Equal("weekly"))
			})

			It("should preserve the cron", func() {
				Expect(schedules[0].GetCron()).To(Equal("0 0 9 * * 1"))
			})

			It("should preserve max_turns", func() {
				Expect(schedules[0].GetMaxTurns()).To(Equal(int32(15)))
			})

			It("should preserve enabled", func() {
				Expect(schedules[0].GetEnabled()).To(BeTrue())
			})
		})
	})

	Describe("Upsert", func() {
		Describe("when the schedule argument is nil", func() {
			var err error

			BeforeEach(func() {
				err = store.Upsert("p", nil)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should mention that schedule is required", func() {
				Expect(err).To(MatchError("schedule is required"))
			})
		})

		Describe("when the schedule id is empty", func() {
			var err error

			BeforeEach(func() {
				err = store.Upsert("p", &apiv1.AgentSchedule{Cron: "0 * * * * *"})
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should mention that schedule.id is required", func() {
				Expect(err).To(MatchError("schedule.id is required"))
			})
		})

		Describe("when the schedule is new", func() {
			var err error

			BeforeEach(func() {
				err = store.Upsert("page1", &apiv1.AgentSchedule{
					Id:       "first",
					Cron:     "*/30 * * * * *",
					Prompt:   "do the thing",
					MaxTurns: 20,
					Enabled:  true,
				})
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should be readable via List", func() {
				schedules, listErr := store.List("page1")
				Expect(listErr).NotTo(HaveOccurred())
				Expect(schedules).To(HaveLen(1))
				Expect(schedules[0].GetId()).To(Equal("first"))
			})
		})

		Describe("when a schedule with the same id already exists", func() {
			var firstErr, secondErr error

			BeforeEach(func() {
				firstErr = store.Upsert("page2", &apiv1.AgentSchedule{
					Id: "dup", Cron: "0 * * * * *", Prompt: "v1", Enabled: true,
				})
				secondErr = store.Upsert("page2", &apiv1.AgentSchedule{
					Id: "dup", Cron: "0 0 * * * *", Prompt: "v2", Enabled: false,
				})
			})

			It("should accept the first upsert", func() {
				Expect(firstErr).NotTo(HaveOccurred())
			})

			It("should accept the second upsert", func() {
				Expect(secondErr).NotTo(HaveOccurred())
			})

			It("should replace, not duplicate, the schedule", func() {
				schedules, _ := store.List("page2")
				Expect(schedules).To(HaveLen(1))
				Expect(schedules[0].GetCron()).To(Equal("0 0 * * * *"))
				Expect(schedules[0].GetPrompt()).To(Equal("v2"))
				Expect(schedules[0].GetEnabled()).To(BeFalse())
			})
		})

		Describe("preserving non-agent frontmatter", func() {
			BeforeEach(func() {
				_ = pages.WriteFrontMatter("untouched", wikipage.FrontMatter{
					"title":    "My Page",
					"keywords": []any{"a", "b"},
				})
				Expect(store.Upsert("untouched", &apiv1.AgentSchedule{
					Id: "x", Cron: "0 0 * * * *", Enabled: true,
				})).To(Succeed())
			})

			It("should keep the title", func() {
				_, fm, _ := pages.ReadFrontMatter("untouched")
				Expect(fm["title"]).To(Equal("My Page"))
			})

			It("should keep the keywords", func() {
				_, fm, _ := pages.ReadFrontMatter("untouched")
				Expect(fm["keywords"]).To(Equal([]any{"a", "b"}))
			})
		})
	})

	Describe("Delete", func() {
		BeforeEach(func() {
			Expect(store.Upsert("p", &apiv1.AgentSchedule{Id: "one", Cron: "0 * * * * *", Enabled: true})).To(Succeed())
			Expect(store.Upsert("p", &apiv1.AgentSchedule{Id: "two", Cron: "0 * * * * *", Enabled: true})).To(Succeed())
		})

		Describe("when the id exists", func() {
			BeforeEach(func() {
				Expect(store.Delete("p", "one")).To(Succeed())
			})

			It("should leave only the other schedule", func() {
				schedules, _ := store.List("p")
				Expect(schedules).To(HaveLen(1))
				Expect(schedules[0].GetId()).To(Equal("two"))
			})
		})

		Describe("when the id does not exist", func() {
			var err error

			BeforeEach(func() {
				err = store.Delete("p", "nonexistent")
			})

			It("should not return an error (idempotent)", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should leave the existing schedules intact", func() {
				schedules, _ := store.List("p")
				Expect(schedules).To(HaveLen(2))
			})
		})
	})

	Describe("TransitionStatus", func() {
		BeforeEach(func() {
			Expect(store.Upsert("p", &apiv1.AgentSchedule{
				Id: "s1", Cron: "0 * * * * *", Enabled: true,
			})).To(Succeed())
		})

		Describe("when transitioning from UNSPECIFIED to RUNNING", func() {
			var err error

			BeforeEach(func() {
				err = store.TransitionStatus("p", "s1", apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING, "", 0)
			})

			It("should succeed", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should update last_status to RUNNING", func() {
				schedules, _ := store.List("p")
				Expect(schedules[0].GetLastStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING))
			})

			It("should set last_run to a recent timestamp", func() {
				schedules, _ := store.List("p")
				Expect(schedules[0].GetLastRun()).NotTo(BeNil())
				ts := schedules[0].GetLastRun().AsTime()
				Expect(time.Since(ts)).To(BeNumerically("<", 5*time.Second))
			})
		})

		Describe("when transitioning RUNNING -> OK", func() {
			var err error

			BeforeEach(func() {
				Expect(store.TransitionStatus("p", "s1", apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING, "", 0)).To(Succeed())
				err = store.TransitionStatus("p", "s1", apiv1.ScheduleStatus_SCHEDULE_STATUS_OK, "", 42)
			})

			It("should succeed", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should record the duration", func() {
				schedules, _ := store.List("p")
				Expect(schedules[0].GetLastDurationSeconds()).To(Equal(int32(42)))
			})

			It("should clear last_error_message", func() {
				schedules, _ := store.List("p")
				Expect(schedules[0].GetLastErrorMessage()).To(Equal(""))
			})
		})

		Describe("when transitioning RUNNING -> ERROR with a message", func() {
			BeforeEach(func() {
				Expect(store.TransitionStatus("p", "s1", apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING, "", 0)).To(Succeed())
				Expect(store.TransitionStatus("p", "s1", apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR, "ntfy unreachable", 7)).To(Succeed())
			})

			It("should record the error message", func() {
				schedules, _ := store.List("p")
				Expect(schedules[0].GetLastErrorMessage()).To(Equal("ntfy unreachable"))
			})

			It("should record the duration", func() {
				schedules, _ := store.List("p")
				Expect(schedules[0].GetLastDurationSeconds()).To(Equal(int32(7)))
			})
		})

		Describe("when the transition is illegal", func() {
			var transitionErr error

			BeforeEach(func() {
				// First go OK so we are in a terminal state.
				Expect(store.TransitionStatus("p", "s1", apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING, "", 0)).To(Succeed())
				Expect(store.TransitionStatus("p", "s1", apiv1.ScheduleStatus_SCHEDULE_STATUS_OK, "", 1)).To(Succeed())
				// Now attempt OK -> ERROR (illegal).
				transitionErr = store.TransitionStatus("p", "s1", apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR, "boom", 2)
			})

			It("should return an IllegalScheduleTransitionError", func() {
				var typed *server.IllegalScheduleTransitionError
				Expect(errors.As(transitionErr, &typed)).To(BeTrue())
			})

			It("should not modify last_status on disk", func() {
				schedules, _ := store.List("p")
				Expect(schedules[0].GetLastStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_OK))
			})
		})

		Describe("when the schedule does not exist", func() {
			var err error

			BeforeEach(func() {
				err = store.TransitionStatus("p", "missing", apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING, "", 0)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		Describe("when a BackgroundActivitySink is wired and the transition is terminal", func() {
			var sink *recordingActivitySink

			BeforeEach(func() {
				sink = &recordingActivitySink{}
				store.SetBackgroundActivitySink(sink)
				Expect(store.TransitionStatus("p", "s1", apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING, "", 0)).To(Succeed())
				Expect(store.TransitionStatus("p", "s1", apiv1.ScheduleStatus_SCHEDULE_STATUS_OK, "", 5)).To(Succeed())
			})

			It("should call the sink exactly once (the terminal transition)", func() {
				Expect(sink.calls).To(HaveLen(1))
			})

			It("should pass the schedule id to the sink", func() {
				Expect(sink.calls[0].entry.GetScheduleId()).To(Equal("s1"))
			})

			It("should pass the terminal status", func() {
				Expect(sink.calls[0].entry.GetStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_OK))
			})

			It("should pass the page identifier", func() {
				Expect(sink.calls[0].page).To(Equal("p"))
			})
		})

		Describe("when a sink is wired and the transition is RUNNING (non-terminal)", func() {
			var sink *recordingActivitySink

			BeforeEach(func() {
				sink = &recordingActivitySink{}
				store.SetBackgroundActivitySink(sink)
				Expect(store.TransitionStatus("p", "s1", apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING, "", 0)).To(Succeed())
			})

			It("should not call the sink", func() {
				Expect(sink.calls).To(BeEmpty())
			})
		})

		Describe("when a sink is wired and returns an error on a terminal transition", func() {
			var sink *erroringActivitySink
			var transitionErr error

			BeforeEach(func() {
				sink = &erroringActivitySink{err: errors.New("sink boom")}
				store.SetBackgroundActivitySink(sink)
				Expect(store.TransitionStatus("p", "s1", apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING, "", 0)).To(Succeed())
				transitionErr = store.TransitionStatus("p", "s1", apiv1.ScheduleStatus_SCHEDULE_STATUS_OK, "", 5)
			})

			It("should still succeed (sink errors are best-effort)", func() {
				Expect(transitionErr).NotTo(HaveOccurred())
			})

			It("should still have called the sink", func() {
				Expect(sink.calls).To(Equal(1))
			})

			It("should still record the terminal status on the schedule", func() {
				schedules, _ := store.List("p")
				Expect(schedules[0].GetLastStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_OK))
			})
		})

		Describe("when ReadFrontMatter returns an error", func() {
			var err error

			BeforeEach(func() {
				errStore := &errorPageStore{readErr: errors.New("disk on fire")}
				bad := server.NewAgentScheduleStore(errStore)
				err = bad.TransitionStatus("p", "s1", apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING, "", 0)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should wrap the error with read frontmatter context", func() {
				Expect(err.Error()).To(ContainSubstring("read frontmatter"))
			})
		})

		Describe("when WriteFrontMatter returns an error", func() {
			var err error

			BeforeEach(func() {
				errStore := &errorPageStore{
					writeErr: errors.New("disk full"),
					fm: wikipage.FrontMatter{
						"agent": map[string]any{
							"schedules": []any{
								map[string]any{
									"id":      "s1",
									"cron":    "0 * * * * *",
									"enabled": true,
								},
							},
						},
					},
				}
				bad := server.NewAgentScheduleStore(errStore)
				err = bad.TransitionStatus("p", "s1", apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING, "", 0)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should wrap the error with write frontmatter context", func() {
				Expect(err.Error()).To(ContainSubstring("write frontmatter"))
			})
		})
	})

	Describe("List error handling", func() {
		Describe("when ReadFrontMatter returns an error", func() {
			var err error

			BeforeEach(func() {
				errStore := &errorPageStore{readErr: errors.New("disk gone")}
				bad := server.NewAgentScheduleStore(errStore)
				_, err = bad.List("p")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should wrap the error with read frontmatter context", func() {
				Expect(err.Error()).To(ContainSubstring("read frontmatter"))
			})
		})

		Describe("when agent.schedules has the wrong type", func() {
			var err error

			BeforeEach(func() {
				_ = pages.WriteFrontMatter("malformed", wikipage.FrontMatter{
					"agent": map[string]any{
						"schedules": "not-a-list",
					},
				})
				_, err = store.List("malformed")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should mention the unexpected type", func() {
				Expect(err.Error()).To(ContainSubstring("unexpected type"))
			})
		})

		Describe("when an item inside agent.schedules has the wrong type", func() {
			var err error

			BeforeEach(func() {
				_ = pages.WriteFrontMatter("indexed_malformed", wikipage.FrontMatter{
					"agent": map[string]any{
						"schedules": []any{
							map[string]any{
								"id":      "ok",
								"cron":    "0 * * * * *",
								"enabled": true,
							},
							"not-a-map",
						},
					},
				})
				_, err = store.List("indexed_malformed")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should include the offending index in the error message", func() {
				Expect(err.Error()).To(ContainSubstring("agent.schedules[1"))
			})
		})
	})

	Describe("Upsert preserving wiki-managed status fields", func() {
		var schedules []*apiv1.AgentSchedule

		BeforeEach(func() {
			Expect(store.Upsert("p", &apiv1.AgentSchedule{
				Id: "s1", Cron: "0 * * * * *", Prompt: "v1", Enabled: true,
			})).To(Succeed())
			Expect(store.TransitionStatus("p", "s1", apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING, "", 0)).To(Succeed())
			Expect(store.TransitionStatus("p", "s1", apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR, "boom", 99)).To(Succeed())

			// Caller attempts to change wiki-managed fields via Upsert; those
			// must be silently dropped in favor of the prior values.
			Expect(store.Upsert("p", &apiv1.AgentSchedule{
				Id:                  "s1",
				Cron:                "0 * * * * *",
				Prompt:              "v2",
				Enabled:             true,
				LastStatus:          apiv1.ScheduleStatus_SCHEDULE_STATUS_OK,
				LastErrorMessage:    "caller lied",
				LastDurationSeconds: 1,
			})).To(Succeed())

			var listErr error
			schedules, listErr = store.List("p")
			Expect(listErr).NotTo(HaveOccurred())
		})

		It("should preserve last_status from the prior record", func() {
			Expect(schedules[0].GetLastStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR))
		})

		It("should preserve last_error_message from the prior record", func() {
			Expect(schedules[0].GetLastErrorMessage()).To(Equal("boom"))
		})

		It("should preserve last_duration_seconds from the prior record", func() {
			Expect(schedules[0].GetLastDurationSeconds()).To(Equal(int32(99)))
		})

		It("should still apply caller-managed prompt updates", func() {
			Expect(schedules[0].GetPrompt()).To(Equal("v2"))
		})
	})

	Describe("Delete error handling", func() {
		Describe("when WriteFrontMatter returns an error", func() {
			var err error

			BeforeEach(func() {
				errStore := &errorPageStore{
					writeErr: errors.New("disk full"),
					fm: wikipage.FrontMatter{
						"agent": map[string]any{
							"schedules": []any{
								map[string]any{
									"id":      "victim",
									"cron":    "0 * * * * *",
									"enabled": true,
								},
							},
						},
					},
				}
				bad := server.NewAgentScheduleStore(errStore)
				err = bad.Delete("p", "victim")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should wrap the error with write frontmatter context", func() {
				Expect(err.Error()).To(ContainSubstring("write frontmatter"))
			})
		})
	})
})

// recordingActivitySink captures every AppendBackgroundActivityAutomatic call
// for assertions.
type recordingActivitySink struct {
	calls []recordedActivityCall
}

type recordedActivityCall struct {
	page  string
	entry *apiv1.BackgroundActivityEntry
}

func (r *recordingActivitySink) AppendBackgroundActivityAutomatic(page string, entry *apiv1.BackgroundActivityEntry) error {
	r.calls = append(r.calls, recordedActivityCall{page: page, entry: entry})
	return nil
}

// erroringActivitySink always returns an error from
// AppendBackgroundActivityAutomatic. Used to verify that schedule transitions
// succeed even when the best-effort sink fails.
type erroringActivitySink struct {
	err   error
	calls int
}

func (e *erroringActivitySink) AppendBackgroundActivityAutomatic(_ string, _ *apiv1.BackgroundActivityEntry) error {
	e.calls++
	return e.err
}

// errorPageStore is a fake page store with configurable read/write errors. It
// satisfies the same interface as fakePageStore but lets tests inject
// failures.
type errorPageStore struct {
	readErr  error
	writeErr error
	fm       wikipage.FrontMatter
}

func (e *errorPageStore) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	if e.readErr != nil {
		return id, nil, e.readErr
	}
	if e.fm == nil {
		return id, wikipage.FrontMatter{}, nil
	}
	out := wikipage.FrontMatter{}
	for k, v := range e.fm {
		out[k] = v
	}
	return id, out, nil
}

func (e *errorPageStore) WriteFrontMatter(_ wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	if e.writeErr != nil {
		return e.writeErr
	}
	e.fm = fm
	return nil
}
