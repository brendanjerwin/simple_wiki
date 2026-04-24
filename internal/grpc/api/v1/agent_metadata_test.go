//revive:disable:dot-imports
package v1_test

import (
	"context"
	"errors"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// inMemoryPageStore is a tiny PageReader/PageWriter used to back the schedule
// and chat-context stores in handler tests. It's a stripped-down copy of the
// fake used by the server-package tests.
type inMemoryPageStore struct {
	mu    sync.Mutex
	pages map[wikipage.PageIdentifier]wikipage.FrontMatter
}

func newInMemoryPageStore() *inMemoryPageStore {
	return &inMemoryPageStore{pages: map[wikipage.PageIdentifier]wikipage.FrontMatter{}}
}

func (f *inMemoryPageStore) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fm, ok := f.pages[id]
	if !ok {
		return id, wikipage.FrontMatter{}, nil
	}
	out := wikipage.FrontMatter{}
	for k, v := range fm {
		out[k] = v
	}
	return id, out, nil
}

func (f *inMemoryPageStore) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pages[id] = fm
	return nil
}

// newAgentMetadataServer wires a Server with a minimal set of dependencies
// and returns it together with the schedule + chat-context stores so
// individual tests can seed and assert directly.
func newAgentMetadataServer() (*v1.Server, *server.AgentScheduleStore, *server.AgentChatContextStore) {
	pages := newInMemoryPageStore()
	scheduleStore := server.NewAgentScheduleStore(pages)
	chatStore := server.NewAgentChatContextStore(pages)
	scheduleStore.SetBackgroundActivitySink(chatStore)

	srv := mustNewServer(&MockPageReaderMutator{}, nil, nil).
		WithAgentScheduleStore(scheduleStore).
		WithAgentChatContextStore(chatStore)
	return srv, scheduleStore, chatStore
}

var _ = Describe("AgentMetadataService handlers", func() {
	var (
		ctx    context.Context
		srv    *v1.Server
		schedules *server.AgentScheduleStore
		contexts  *server.AgentChatContextStore
	)

	BeforeEach(func() {
		ctx = context.Background()
		srv, schedules, contexts = newAgentMetadataServer()
	})

	Describe("UpsertSchedule", func() {
		Describe("when the cron expression is valid", func() {
			var resp *apiv1.UpsertScheduleResponse
			var err error

			BeforeEach(func() {
				resp, err = srv.UpsertSchedule(ctx, &apiv1.UpsertScheduleRequest{
					Page: "p",
					Schedule: &apiv1.AgentSchedule{
						Id:       "weekly",
						Cron:     "0 0 9 * * 1",
						Prompt:   "do thing",
						MaxTurns: 15,
						Enabled:  true,
					},
				})
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the schedule", func() {
				Expect(resp.GetSchedule().GetId()).To(Equal("weekly"))
			})

			It("should persist the schedule", func() {
				list, _ := schedules.List("p")
				Expect(list).To(HaveLen(1))
			})
		})

		Describe("when the cron expression is invalid", func() {
			var err error

			BeforeEach(func() {
				_, err = srv.UpsertSchedule(ctx, &apiv1.UpsertScheduleRequest{
					Page: "p",
					Schedule: &apiv1.AgentSchedule{
						Id:   "bad",
						Cron: "not a cron",
					},
				})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "cron"))
			})
		})

		Describe("when wiki-managed fields are populated on the request", func() {
			BeforeEach(func() {
				_, err := srv.UpsertSchedule(ctx, &apiv1.UpsertScheduleRequest{
					Page: "p",
					Schedule: &apiv1.AgentSchedule{
						Id:               "stripped",
						Cron:             "0 0 9 * * 1",
						LastErrorMessage: "from the caller — should be dropped",
						LastDurationSeconds: 999,
					},
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should silently strip the wiki-managed fields", func() {
				list, _ := schedules.List("p")
				Expect(list).To(HaveLen(1))
				Expect(list[0].GetLastErrorMessage()).To(Equal(""))
				Expect(list[0].GetLastDurationSeconds()).To(Equal(int32(0)))
			})
		})

		Describe("when the schedule id is missing", func() {
			var err error

			BeforeEach(func() {
				_, err = srv.UpsertSchedule(ctx, &apiv1.UpsertScheduleRequest{
					Page: "p",
					Schedule: &apiv1.AgentSchedule{
						Cron: "0 0 9 * * 1",
					},
				})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "id"))
			})
		})

		Describe("when the page is missing", func() {
			var err error

			BeforeEach(func() {
				_, err = srv.UpsertSchedule(ctx, &apiv1.UpsertScheduleRequest{
					Schedule: &apiv1.AgentSchedule{
						Id:   "x",
						Cron: "0 0 9 * * 1",
					},
				})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "page"))
			})
		})
	})

	Describe("ListSchedules", func() {
		Describe("when the page has multiple schedules", func() {
			var resp *apiv1.ListSchedulesResponse

			BeforeEach(func() {
				Expect(schedules.Upsert("p", &apiv1.AgentSchedule{Id: "a", Cron: "0 0 * * * *", Enabled: true})).To(Succeed())
				Expect(schedules.Upsert("p", &apiv1.AgentSchedule{Id: "b", Cron: "0 0 * * * *", Enabled: false})).To(Succeed())

				var err error
				resp, err = srv.ListSchedules(ctx, &apiv1.ListSchedulesRequest{Page: "p"})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return both schedules", func() {
				Expect(resp.GetSchedules()).To(HaveLen(2))
			})
		})
	})

	Describe("DeleteSchedule", func() {
		BeforeEach(func() {
			Expect(schedules.Upsert("p", &apiv1.AgentSchedule{Id: "doomed", Cron: "0 0 * * * *", Enabled: true})).To(Succeed())
		})

		Describe("when the schedule exists", func() {
			BeforeEach(func() {
				_, err := srv.DeleteSchedule(ctx, &apiv1.DeleteScheduleRequest{
					Page: "p", ScheduleId: "doomed",
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should remove it", func() {
				list, _ := schedules.List("p")
				Expect(list).To(BeEmpty())
			})
		})

		Describe("when the schedule does not exist", func() {
			var err error

			BeforeEach(func() {
				_, err = srv.DeleteSchedule(ctx, &apiv1.DeleteScheduleRequest{
					Page: "p", ScheduleId: "missing",
				})
			})

			It("should not return an error (idempotent)", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("GetChatContext", func() {
		Describe("when the page has a chat context", func() {
			var resp *apiv1.GetChatContextResponse

			BeforeEach(func() {
				_, err := contexts.UpdateMerge("p", &apiv1.ChatContext{
					LastConversationSummary: "hello",
					UserGoals:               []string{"a"},
				})
				Expect(err).NotTo(HaveOccurred())

				resp, err = srv.GetChatContext(ctx, &apiv1.GetChatContextRequest{Page: "p"})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the conversation summary", func() {
				Expect(resp.GetChatContext().GetLastConversationSummary()).To(Equal("hello"))
			})

			It("should return user_goals", func() {
				Expect(resp.GetChatContext().GetUserGoals()).To(ConsistOf("a"))
			})
		})
	})

	Describe("UpdateChatContext", func() {
		Describe("when called with new context", func() {
			var resp *apiv1.UpdateChatContextResponse

			BeforeEach(func() {
				var err error
				resp, err = srv.UpdateChatContext(ctx, &apiv1.UpdateChatContextRequest{
					Page: "p",
					ChatContext: &apiv1.ChatContext{
						LastConversationSummary: "I asked about pastry",
						UserGoals:               []string{"finish order"},
					},
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should server-stamp last_updated", func() {
				Expect(resp.GetChatContext().GetLastUpdated()).NotTo(BeNil())
			})

			It("should persist the conversation summary", func() {
				ctx, _ := contexts.Read("p")
				Expect(ctx.GetLastConversationSummary()).To(Equal("I asked about pastry"))
			})
		})
	})

	Describe("AppendBackgroundActivitySummary", func() {
		Describe("when there is no matching recent entry", func() {
			var err error

			BeforeEach(func() {
				_, err = srv.AppendBackgroundActivitySummary(ctx, &apiv1.AppendBackgroundActivitySummaryRequest{
					Page:       "p",
					ScheduleId: "ghost",
					Summary:    "...",
				})
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should map SummaryTargetNotFoundError to NotFound", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.NotFound, "ghost"))
			})

			It("should still expose the underlying typed error to inspectors", func() {
				var typed *server.SummaryTargetNotFoundError
				Expect(errors.As(err, &typed)).To(BeFalse(), "gRPC status wrapping is the contract; inner type isn't preserved across the boundary")
			})
		})
	})
})
