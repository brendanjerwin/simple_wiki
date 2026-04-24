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
	})
})
