//revive:disable:dot-imports
package v1_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/structpb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	wikiserver "github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

var _ = Describe("system page guard", func() {
	var (
		ctx    context.Context
		mock   *MockPageReaderMutator
		server *v1.Server
	)

	BeforeEach(func() {
		ctx = context.Background()
		mock = &MockPageReaderMutator{
			Frontmatter: wikipage.FrontMatter{
				"identifier": "help",
				"wiki": map[string]any{
					"system": true,
				},
			},
			Markdown: "# Help",
		}
		server = mustNewServer(mock, nil, nil)
	})

	Describe("UpdatePageContent on a system page", func() {
		var err error

		BeforeEach(func() {
			_, err = server.UpdatePageContent(ctx, &apiv1.UpdatePageContentRequest{
				PageName:           "help",
				NewContentMarkdown: "# Hijack",
			})
		})

		It("should reject with FailedPrecondition", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "system page"))
		})

		It("should not have written anything", func() {
			Expect(mock.WrittenMarkdown).To(BeEmpty())
		})
	})

	Describe("UpdateWholePage on a system page", func() {
		var err error

		BeforeEach(func() {
			_, err = server.UpdateWholePage(ctx, &apiv1.UpdateWholePageRequest{
				PageName:         "help",
				NewWholeMarkdown: "+++\ntitle = \"Hijack\"\n+++\nbody",
			})
		})

		It("should reject with FailedPrecondition", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "system page"))
		})
	})

	Describe("ClearPageContent on a system page", func() {
		var err error

		BeforeEach(func() {
			_, err = server.ClearPageContent(ctx, &apiv1.ClearPageContentRequest{
				PageName:     "help",
				ConfirmClear: true,
			})
		})

		It("should reject with FailedPrecondition", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "system page"))
		})
	})

	Describe("DeletePage on a system page", func() {
		var err error

		BeforeEach(func() {
			_, err = server.DeletePage(ctx, &apiv1.DeletePageRequest{
				PageName: "help",
			})
		})

		It("should reject with FailedPrecondition", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "system page"))
		})
	})

	Describe("MergeFrontmatter on a system page", func() {
		var err error

		BeforeEach(func() {
			body, _ := structpb.NewStruct(map[string]any{"title": "Hijack"})
			_, err = server.MergeFrontmatter(ctx, &apiv1.MergeFrontmatterRequest{
				Page:        "help",
				Frontmatter: body,
			})
		})

		It("should reject with FailedPrecondition", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "system page"))
		})
	})

	Describe("ReplaceFrontmatter on a system page", func() {
		var err error

		BeforeEach(func() {
			body, _ := structpb.NewStruct(map[string]any{"title": "Hijack"})
			_, err = server.ReplaceFrontmatter(ctx, &apiv1.ReplaceFrontmatterRequest{
				Page:        "help",
				Frontmatter: body,
			})
		})

		It("should reject with FailedPrecondition", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "system page"))
		})
	})

	Describe("RemoveKeyAtPath on a system page", func() {
		var err error

		BeforeEach(func() {
			_, err = server.RemoveKeyAtPath(ctx, &apiv1.RemoveKeyAtPathRequest{
				Page: "help",
				KeyPath: []*apiv1.PathComponent{
					{Component: &apiv1.PathComponent_Key{Key: "title"}},
				},
			})
		})

		It("should reject with FailedPrecondition", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "system page"))
		})
	})

	// AgentMetadataService RPCs need both stores configured to reach the
	// guard. The guard call is intentionally placed before the
	// store-not-configured check so a system-page rejection wins over an
	// internal-wiring error.
	Describe("agent_metadata RPCs on a system page", func() {
		var (
			schedules *wikiserver.AgentScheduleStore
			contexts  *wikiserver.AgentChatContextStore
		)

		BeforeEach(func() {
			pages := newInMemoryPageStore()
			schedules = wikiserver.NewAgentScheduleStore(pages)
			contexts = wikiserver.NewAgentChatContextStore(pages)
			schedules.SetBackgroundActivitySink(contexts)
			server = mustNewServer(mock, nil, nil).
				WithAgentScheduleStore(schedules).
				WithAgentChatContextStore(contexts)
		})

		Describe("UpsertSchedule on a system page", func() {
			var err error

			BeforeEach(func() {
				_, err = server.UpsertSchedule(ctx, &apiv1.UpsertScheduleRequest{
					Page: "help",
					Schedule: &apiv1.AgentSchedule{
						Id:   "weekly",
						Cron: "0 0 9 * * 1",
					},
				})
			})

			It("should reject with FailedPrecondition", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "system page"))
			})

			It("should not have persisted any schedule", func() {
				list, _ := schedules.List("help")
				Expect(list).To(BeEmpty())
			})
		})

		Describe("DeleteSchedule on a system page", func() {
			var err error

			BeforeEach(func() {
				_, err = server.DeleteSchedule(ctx, &apiv1.DeleteScheduleRequest{
					Page:       "help",
					ScheduleId: "weekly",
				})
			})

			It("should reject with FailedPrecondition", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "system page"))
			})
		})

		Describe("UpdateChatContext on a system page", func() {
			var err error

			BeforeEach(func() {
				_, err = server.UpdateChatContext(ctx, &apiv1.UpdateChatContextRequest{
					Page: "help",
					ChatContext: &apiv1.ChatContext{
						LastConversationSummary: "should not land",
					},
				})
			})

			It("should reject with FailedPrecondition", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "system page"))
			})

			It("should not have persisted the context", func() {
				ctxRead, _ := contexts.Read("help")
				Expect(ctxRead.GetLastConversationSummary()).To(Equal(""))
			})
		})

		Describe("AppendBackgroundActivitySummary on a system page", func() {
			var err error

			BeforeEach(func() {
				_, err = server.AppendBackgroundActivitySummary(ctx, &apiv1.AppendBackgroundActivitySummaryRequest{
					Page:       "help",
					ScheduleId: "weekly",
					Summary:    "should not land",
				})
			})

			It("should reject with FailedPrecondition", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "system page"))
			})
		})
	})
})
