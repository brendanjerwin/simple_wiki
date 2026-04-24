//revive:disable:dot-imports
package server_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

var _ = Describe("AgentChatContextStore", func() {
	var (
		pages *fakePageStore
		store *server.AgentChatContextStore
	)

	BeforeEach(func() {
		pages = newFakePageStore()
		store = server.NewAgentChatContextStore(pages)
	})

	Describe("Read", func() {
		Describe("when the page has no chat context", func() {
			var ctx *apiv1.ChatContext
			var err error

			BeforeEach(func() {
				ctx, err = store.Read("p")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an empty (non-nil) ChatContext", func() {
				Expect(ctx).NotTo(BeNil())
				Expect(ctx.GetUserGoals()).To(BeEmpty())
				Expect(ctx.GetBackgroundActivity()).To(BeEmpty())
			})
		})

		Describe("when the page has a chat context", func() {
			var ctx *apiv1.ChatContext

			BeforeEach(func() {
				_ = pages.WriteFrontMatter("p", wikipage.FrontMatter{
					"agent": map[string]any{
						"chat_context": map[string]any{
							"last_conversation_summary": "we discussed pastry",
							"user_goals":                []any{"finish order", "send ntfy"},
							"pending_items":             []any{"order confirmation"},
							"key_context":               "Friday 6pm",
						},
					},
				})
				var err error
				ctx, err = store.Read("p")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should round-trip the conversation summary", func() {
				Expect(ctx.GetLastConversationSummary()).To(Equal("we discussed pastry"))
			})

			It("should round-trip user_goals", func() {
				Expect(ctx.GetUserGoals()).To(ConsistOf("finish order", "send ntfy"))
			})

			It("should round-trip pending_items", func() {
				Expect(ctx.GetPendingItems()).To(ConsistOf("order confirmation"))
			})

			It("should round-trip key_context", func() {
				Expect(ctx.GetKeyContext()).To(Equal("Friday 6pm"))
			})
		})
	})

	Describe("UpdateMerge", func() {
		Describe("when no prior context exists", func() {
			var written *apiv1.ChatContext

			BeforeEach(func() {
				out, err := store.UpdateMerge("p", &apiv1.ChatContext{
					LastConversationSummary: "first turn",
					UserGoals:               []string{"goal-a"},
				})
				Expect(err).NotTo(HaveOccurred())
				written = out
			})

			It("should persist the conversation summary", func() {
				Expect(written.GetLastConversationSummary()).To(Equal("first turn"))
			})

			It("should server-stamp last_updated", func() {
				Expect(written.GetLastUpdated()).NotTo(BeNil())
				Expect(time.Since(written.GetLastUpdated().AsTime())).To(BeNumerically("<", 5*time.Second))
			})
		})

		Describe("when prior context exists and the update overlaps", func() {
			var merged *apiv1.ChatContext

			BeforeEach(func() {
				_, err := store.UpdateMerge("p", &apiv1.ChatContext{
					LastConversationSummary: "first",
					UserGoals:               []string{"a"},
					PendingItems:            []string{"x"},
					KeyContext:              "kept",
				})
				Expect(err).NotTo(HaveOccurred())

				merged, err = store.UpdateMerge("p", &apiv1.ChatContext{
					LastConversationSummary: "second",
					UserGoals:               []string{"b", "a"},
					PendingItems:            []string{"y"},
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should replace scalar fields", func() {
				Expect(merged.GetLastConversationSummary()).To(Equal("second"))
			})

			It("should preserve scalars not present in the update", func() {
				Expect(merged.GetKeyContext()).To(Equal("kept"))
			})

			It("should union user_goals (no duplicates, order preserved by first appearance)", func() {
				Expect(merged.GetUserGoals()).To(ConsistOf("a", "b"))
			})

			It("should union pending_items", func() {
				Expect(merged.GetPendingItems()).To(ConsistOf("x", "y"))
			})
		})

		Describe("preserving non-agent frontmatter", func() {
			BeforeEach(func() {
				_ = pages.WriteFrontMatter("p", wikipage.FrontMatter{
					"title": "My Page",
				})
				_, err := store.UpdateMerge("p", &apiv1.ChatContext{
					LastConversationSummary: "hello",
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should keep the title", func() {
				_, fm, _ := pages.ReadFrontMatter("p")
				Expect(fm["title"]).To(Equal("My Page"))
			})
		})
	})

	Describe("AppendBackgroundActivityAutomatic", func() {
		Describe("when no chat context exists yet", func() {
			BeforeEach(func() {
				err := store.AppendBackgroundActivityAutomatic("p", &apiv1.BackgroundActivityEntry{
					Timestamp:  timestamppb.New(time.Now()),
					ScheduleId: "s1",
					Status:     apiv1.ScheduleStatus_SCHEDULE_STATUS_OK,
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create the chat context with a single entry", func() {
				ctx, _ := store.Read("p")
				Expect(ctx.GetBackgroundActivity()).To(HaveLen(1))
				Expect(ctx.GetBackgroundActivity()[0].GetScheduleId()).To(Equal("s1"))
			})
		})

		Describe("when more than the cap of 50 entries are appended", func() {
			BeforeEach(func() {
				const totalEntries = 60
				for i := 0; i < totalEntries; i++ {
					err := store.AppendBackgroundActivityAutomatic("p", &apiv1.BackgroundActivityEntry{
						Timestamp:  timestamppb.New(time.Now()),
						ScheduleId: "s1",
						Status:     apiv1.ScheduleStatus_SCHEDULE_STATUS_OK,
					})
					Expect(err).NotTo(HaveOccurred())
				}
			})

			It("should cap at BackgroundActivityMax (50) entries", func() {
				ctx, _ := store.Read("p")
				Expect(ctx.GetBackgroundActivity()).To(HaveLen(server.BackgroundActivityMax))
			})
		})
	})

	Describe("AppendBackgroundActivitySummary", func() {
		Describe("when a matching recent entry exists", func() {
			BeforeEach(func() {
				Expect(store.AppendBackgroundActivityAutomatic("p", &apiv1.BackgroundActivityEntry{
					Timestamp:  timestamppb.New(time.Now()),
					ScheduleId: "weekly",
					Status:     apiv1.ScheduleStatus_SCHEDULE_STATUS_OK,
				})).To(Succeed())

				Expect(store.AppendBackgroundActivitySummary("p", "weekly", "drafted weekend order")).To(Succeed())
			})

			It("should attach the summary to the most recent matching entry", func() {
				ctx, _ := store.Read("p")
				Expect(ctx.GetBackgroundActivity()).To(HaveLen(1))
				Expect(ctx.GetBackgroundActivity()[0].GetSummary()).To(Equal("drafted weekend order"))
			})
		})

		Describe("when no matching entry exists", func() {
			var err error

			BeforeEach(func() {
				err = store.AppendBackgroundActivitySummary("p", "missing", "...")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should be a SummaryTargetNotFoundError", func() {
				var typed *server.SummaryTargetNotFoundError
				Expect(errors.As(err, &typed)).To(BeTrue())
			})
		})

		Describe("when several entries from different schedule_ids exist", func() {
			BeforeEach(func() {
				Expect(store.AppendBackgroundActivityAutomatic("p", &apiv1.BackgroundActivityEntry{
					Timestamp:  timestamppb.New(time.Now().Add(-2 * time.Hour)),
					ScheduleId: "alpha",
					Status:     apiv1.ScheduleStatus_SCHEDULE_STATUS_OK,
				})).To(Succeed())
				Expect(store.AppendBackgroundActivityAutomatic("p", &apiv1.BackgroundActivityEntry{
					Timestamp:  timestamppb.New(time.Now().Add(-1 * time.Hour)),
					ScheduleId: "beta",
					Status:     apiv1.ScheduleStatus_SCHEDULE_STATUS_OK,
				})).To(Succeed())
				Expect(store.AppendBackgroundActivityAutomatic("p", &apiv1.BackgroundActivityEntry{
					Timestamp:  timestamppb.New(time.Now()),
					ScheduleId: "alpha",
					Status:     apiv1.ScheduleStatus_SCHEDULE_STATUS_OK,
				})).To(Succeed())

				Expect(store.AppendBackgroundActivitySummary("p", "alpha", "newest alpha")).To(Succeed())
			})

			It("should attach the summary to the newest alpha entry", func() {
				ctx, _ := store.Read("p")
				summaries := []string{}
				for _, entry := range ctx.GetBackgroundActivity() {
					if entry.GetScheduleId() == "alpha" {
						summaries = append(summaries, entry.GetSummary())
					}
				}
				Expect(summaries).To(ContainElement("newest alpha"))
			})

			It("should leave the older alpha entry's summary blank", func() {
				ctx, _ := store.Read("p")
				blanks := 0
				for _, entry := range ctx.GetBackgroundActivity() {
					if entry.GetScheduleId() == "alpha" && entry.GetSummary() == "" {
						blanks++
					}
				}
				Expect(blanks).To(Equal(1))
			})

			It("should not touch beta's summary", func() {
				ctx, _ := store.Read("p")
				for _, entry := range ctx.GetBackgroundActivity() {
					if entry.GetScheduleId() == "beta" {
						Expect(entry.GetSummary()).To(Equal(""))
					}
				}
			})
		})
	})
})
