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

	Describe("Read error handling", func() {
		Describe("when ReadFrontMatter returns an error", func() {
			var err error

			BeforeEach(func() {
				errStore := &errorPageStore{readErr: errors.New("disk on fire")}
				bad := server.NewAgentChatContextStore(errStore)
				_, err = bad.Read("p")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should wrap the error with read frontmatter context", func() {
				Expect(err.Error()).To(ContainSubstring("read frontmatter"))
			})
		})
	})

	Describe("UpdateMerge error handling", func() {
		Describe("when WriteFrontMatter returns an error", func() {
			var err error

			BeforeEach(func() {
				errStore := &errorPageStore{writeErr: errors.New("disk full")}
				bad := server.NewAgentChatContextStore(errStore)
				_, err = bad.UpdateMerge("p", &apiv1.ChatContext{
					LastConversationSummary: "hello",
				})
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should wrap the error with write frontmatter context", func() {
				Expect(err.Error()).To(ContainSubstring("write frontmatter"))
			})
		})
	})

	Describe("AppendBackgroundActivityAutomatic error handling", func() {
		Describe("when ReadFrontMatter returns an error", func() {
			var err error

			BeforeEach(func() {
				errStore := &errorPageStore{readErr: errors.New("disk on fire")}
				bad := server.NewAgentChatContextStore(errStore)
				err = bad.AppendBackgroundActivityAutomatic("p", &apiv1.BackgroundActivityEntry{
					Timestamp:  timestamppb.New(time.Now()),
					ScheduleId: "s1",
					Status:     apiv1.ScheduleStatus_SCHEDULE_STATUS_OK,
				})
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
				errStore := &errorPageStore{writeErr: errors.New("disk full")}
				bad := server.NewAgentChatContextStore(errStore)
				err = bad.AppendBackgroundActivityAutomatic("p", &apiv1.BackgroundActivityEntry{
					Timestamp:  timestamppb.New(time.Now()),
					ScheduleId: "s1",
					Status:     apiv1.ScheduleStatus_SCHEDULE_STATUS_OK,
				})
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should wrap the error with write frontmatter context", func() {
				Expect(err.Error()).To(ContainSubstring("write frontmatter"))
			})
		})
	})

	Describe("AppendBackgroundActivitySummary error handling", func() {
		Describe("when ReadFrontMatter returns an error", func() {
			var err error

			BeforeEach(func() {
				errStore := &errorPageStore{readErr: errors.New("disk on fire")}
				bad := server.NewAgentChatContextStore(errStore)
				_, err = bad.AppendBackgroundActivitySummary("p", "weekly", "drafted weekend order")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should wrap the error with read frontmatter context", func() {
				Expect(err.Error()).To(ContainSubstring("read frontmatter"))
			})
		})

		Describe("when frontmatter contains an invalid chat context", func() {
			var err error

			BeforeEach(func() {
				errStore := &errorPageStore{
					fm: wikipage.FrontMatter{
						"agent": map[string]any{
							"chat_context": map[string]any{
								"user_goals": map[string]any{"unexpected": "shape"},
							},
						},
					},
				}
				bad := server.NewAgentChatContextStore(errStore)
				_, err = bad.AppendBackgroundActivitySummary("p", "weekly", "drafted weekend order")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should describe the malformed JSON", func() {
				Expect(err.Error()).To(ContainSubstring("syntax error"))
			})
		})

		Describe("when WriteFrontMatter returns an error after a matching entry is found", func() {
			var err error

			BeforeEach(func() {
				// Pre-seed the error store with a chat_context whose
				// background_activity contains a matching schedule_id, so the
				// summary lookup succeeds and the write path is reached.
				errStore := &errorPageStore{
					writeErr: errors.New("disk full"),
					fm: wikipage.FrontMatter{
						"agent": map[string]any{
							"chat_context": map[string]any{
								"background_activity": []any{
									map[string]any{
										"schedule_id": "weekly",
										"status":      "SCHEDULE_STATUS_RUNNING",
									},
								},
							},
						},
					},
				}
				bad := server.NewAgentChatContextStore(errStore)
				_, err = bad.AppendBackgroundActivitySummary("p", "weekly", "drafted weekend order")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should wrap the error with write frontmatter context", func() {
				Expect(err.Error()).To(ContainSubstring("write frontmatter"))
			})
		})
	})

	Describe("UpdateMerge", func() {
		Describe("when the update argument is nil", func() {
			var err error

			BeforeEach(func() {
				_, err = store.UpdateMerge("p", nil)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should mention that update is required", func() {
				Expect(err).To(MatchError("update is required"))
			})
		})

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
		Describe("when the entry argument is nil", func() {
			var err error

			BeforeEach(func() {
				err = store.AppendBackgroundActivityAutomatic("p", nil)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should mention that entry is required", func() {
				Expect(err).To(MatchError("entry is required"))
			})
		})

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
			var (
				entry *apiv1.BackgroundActivityEntry
				err   error
			)

			BeforeEach(func() {
				Expect(store.AppendBackgroundActivityAutomatic("p", &apiv1.BackgroundActivityEntry{
					Timestamp:  timestamppb.New(time.Now()),
					ScheduleId: "weekly",
					Status:     apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING,
				})).To(Succeed())

				entry, err = store.AppendBackgroundActivitySummary("p", "weekly", "drafted weekend order")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should attach the summary to the most recent matching entry", func() {
				ctx, _ := store.Read("p")
				Expect(ctx.GetBackgroundActivity()).To(HaveLen(1))
				Expect(ctx.GetBackgroundActivity()[0].GetSummary()).To(Equal("drafted weekend order"))
			})

			It("should return the updated background activity", func() {
				Expect(entry.GetScheduleId()).To(Equal("weekly"))
				Expect(entry.GetSummary()).To(Equal("drafted weekend order"))
			})
		})

		Describe("when no matching entry exists", func() {
			var err error

			BeforeEach(func() {
				_, err = store.AppendBackgroundActivitySummary("p", "missing", "...")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should be a SummaryTargetNotFoundError", func() {
				var typed *server.SummaryTargetNotFoundError
				Expect(errors.As(err, &typed)).To(BeTrue())
			})
		})

		Describe("when entries exist but none match the schedule_id", func() {
			var err error

			BeforeEach(func() {
				Expect(store.AppendBackgroundActivityAutomatic("p", &apiv1.BackgroundActivityEntry{
					Timestamp:  timestamppb.New(time.Now()),
					ScheduleId: "alpha",
					Status:     apiv1.ScheduleStatus_SCHEDULE_STATUS_OK,
				})).To(Succeed())
				_, err = store.AppendBackgroundActivitySummary("p", "beta", "...")
			})

			It("should return a SummaryTargetNotFoundError", func() {
				var typed *server.SummaryTargetNotFoundError
				Expect(errors.As(err, &typed)).To(BeTrue())
			})
		})

		Describe("when only completed entries match the schedule_id", func() {
			var err error

			BeforeEach(func() {
				Expect(store.AppendBackgroundActivityAutomatic("p", &apiv1.BackgroundActivityEntry{
					Timestamp:  timestamppb.New(time.Now()),
					ScheduleId: "alpha",
					Status:     apiv1.ScheduleStatus_SCHEDULE_STATUS_OK,
				})).To(Succeed())
				_, err = store.AppendBackgroundActivitySummary("p", "alpha", "late summary")
			})

			It("should return a SummaryTargetNotFoundError", func() {
				var typed *server.SummaryTargetNotFoundError
				Expect(errors.As(err, &typed)).To(BeTrue())
			})

			It("should leave the completed entry unchanged", func() {
				ctx, _ := store.Read("p")
				Expect(ctx.GetBackgroundActivity()[0].GetSummary()).To(Equal(""))
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
					Status:     apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING,
				})).To(Succeed())

				_, err := store.AppendBackgroundActivitySummary("p", "alpha", "newest alpha")
				Expect(err).NotTo(HaveOccurred())
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

	Describe("CompleteBackgroundActivity", func() {
		Describe("when an OK run has a summary", func() {
			var finalStatus apiv1.ScheduleStatus

			BeforeEach(func() {
				Expect(store.AppendBackgroundActivityAutomatic("p", &apiv1.BackgroundActivityEntry{
					Timestamp:  timestamppb.New(time.Now()),
					ScheduleId: "weekly",
					Status:     apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING,
				})).To(Succeed())
				_, err := store.AppendBackgroundActivitySummary("p", "weekly", "drafted weekend order")
				Expect(err).NotTo(HaveOccurred())
				finalStatus, err = store.CompleteBackgroundActivity("p", "weekly", apiv1.ScheduleStatus_SCHEDULE_STATUS_OK)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should keep the status OK", func() {
				Expect(finalStatus).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_OK))
			})
		})

		Describe("when an OK run has no summary", func() {
			var finalStatus apiv1.ScheduleStatus

			BeforeEach(func() {
				Expect(store.AppendBackgroundActivityAutomatic("p", &apiv1.BackgroundActivityEntry{
					Timestamp:  timestamppb.New(time.Now()),
					ScheduleId: "weekly",
					Status:     apiv1.ScheduleStatus_SCHEDULE_STATUS_RUNNING,
				})).To(Succeed())
				var err error
				finalStatus, err = store.CompleteBackgroundActivity("p", "weekly", apiv1.ScheduleStatus_SCHEDULE_STATUS_OK)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return WARN", func() {
				Expect(finalStatus).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_WARN))
			})

			It("should record WARN in background activity", func() {
				ctx, _ := store.Read("p")
				Expect(ctx.GetBackgroundActivity()[0].GetStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_WARN))
			})
		})

		Describe("when a terminal completion arrives without a running entry", func() {
			var finalStatus apiv1.ScheduleStatus

			BeforeEach(func() {
				var err error
				finalStatus, err = store.CompleteBackgroundActivity("p", "weekly", apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should keep the terminal status", func() {
				Expect(finalStatus).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR))
			})

			It("should append a new background activity entry", func() {
				ctx, _ := store.Read("p")
				Expect(ctx.GetBackgroundActivity()).To(HaveLen(1))
			})

			It("should record the schedule id", func() {
				ctx, _ := store.Read("p")
				Expect(ctx.GetBackgroundActivity()[0].GetScheduleId()).To(Equal("weekly"))
			})

			It("should record the terminal status", func() {
				ctx, _ := store.Read("p")
				Expect(ctx.GetBackgroundActivity()[0].GetStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR))
			})
		})

		Describe("when only historical completed entries match the schedule_id", func() {
			var finalStatus apiv1.ScheduleStatus

			BeforeEach(func() {
				Expect(store.AppendBackgroundActivityAutomatic("p", &apiv1.BackgroundActivityEntry{
					Timestamp:  timestamppb.New(time.Now().Add(-2 * time.Hour)),
					ScheduleId: "weekly",
					Status:     apiv1.ScheduleStatus_SCHEDULE_STATUS_OK,
					Summary:    "old run",
				})).To(Succeed())

				var err error
				finalStatus, err = store.CompleteBackgroundActivity("p", "weekly", apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should keep the terminal status", func() {
				Expect(finalStatus).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR))
			})

			It("should preserve the historical entry", func() {
				ctx, _ := store.Read("p")
				Expect(ctx.GetBackgroundActivity()[0].GetSummary()).To(Equal("old run"))
				Expect(ctx.GetBackgroundActivity()[0].GetStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_OK))
			})

			It("should append the new completion entry", func() {
				ctx, _ := store.Read("p")
				Expect(ctx.GetBackgroundActivity()).To(HaveLen(2))
				Expect(ctx.GetBackgroundActivity()[1].GetStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR))
			})
		})

		Describe("when appending a terminal completion exceeds the activity cap", func() {
			var (
				finalStatus apiv1.ScheduleStatus
				ctx         *apiv1.ChatContext
			)

			BeforeEach(func() {
				for i := 0; i < server.BackgroundActivityMax; i++ {
					summary := "retained run"
					if i == 0 {
						summary = "oldest run"
					}
					Expect(store.AppendBackgroundActivityAutomatic("p", &apiv1.BackgroundActivityEntry{
						Timestamp:  timestamppb.New(time.Now().Add(time.Duration(i) * time.Minute)),
						ScheduleId: "old",
						Status:     apiv1.ScheduleStatus_SCHEDULE_STATUS_OK,
						Summary:    summary,
					})).To(Succeed())
				}

				var err error
				finalStatus, err = store.CompleteBackgroundActivity("p", "new", apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR)
				Expect(err).NotTo(HaveOccurred())
				ctx, err = store.Read("p")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should keep the requested terminal status", func() {
				Expect(finalStatus).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR))
			})

			It("should keep background activity capped", func() {
				Expect(ctx.GetBackgroundActivity()).To(HaveLen(server.BackgroundActivityMax))
			})

			It("should drop the oldest entry", func() {
				Expect(ctx.GetBackgroundActivity()[0].GetSummary()).To(Equal("retained run"))
			})

			It("should retain the appended terminal entry", func() {
				last := ctx.GetBackgroundActivity()[len(ctx.GetBackgroundActivity())-1]
				Expect(last.GetScheduleId()).To(Equal("new"))
				Expect(last.GetStatus()).To(Equal(apiv1.ScheduleStatus_SCHEDULE_STATUS_ERROR))
			})
		})
	})

	// Issue #973: a single fat-fingered last_updated value (e.g. a date-only
	// string instead of RFC3339) used to brick chat for the page — every Read
	// would error and no UpdateMerge could write through. The decode path is
	// now lenient: malformed timestamps are scrubbed silently and the server
	// re-stamps on the next legitimate write.
	Describe("Read with a malformed last_updated timestamp on disk (issue #973)", func() {
		var ctx *apiv1.ChatContext
		var err error

		BeforeEach(func() {
			_ = pages.WriteFrontMatter("brick_attempt", wikipage.FrontMatter{
				"agent": map[string]any{
					"chat_context": map[string]any{
						"last_conversation_summary": "kept across the bad field",
						"last_updated":              "2026-04-23", // date-only — invalid RFC3339
					},
				},
			})
			ctx, err = store.Read("brick_attempt")
		})

		It("should NOT return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should preserve the still-valid fields", func() {
			Expect(ctx.GetLastConversationSummary()).To(Equal("kept across the bad field"))
		})

		It("should drop the malformed last_updated rather than blocking the whole read", func() {
			Expect(ctx.GetLastUpdated()).To(BeNil())
		})
	})

	Describe("Read with a malformed timestamp inside a background_activity entry", func() {
		var ctx *apiv1.ChatContext
		var err error

		BeforeEach(func() {
			_ = pages.WriteFrontMatter("partial_log", wikipage.FrontMatter{
				"agent": map[string]any{
					"chat_context": map[string]any{
						"background_activity": []any{
							map[string]any{
								"timestamp":   "garbage",
								"schedule_id": "weekly",
								"status":      "SCHEDULE_STATUS_OK",
								"summary":     "had a bad ts",
							},
							map[string]any{
								"timestamp":   "2026-04-25T00:00:00Z",
								"schedule_id": "weekly",
								"status":      "SCHEDULE_STATUS_OK",
								"summary":     "had a good ts",
							},
						},
					},
				},
			})
			ctx, err = store.Read("partial_log")
		})

		It("should NOT return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should retain both entries (only the timestamp is scrubbed on the bad one)", func() {
			Expect(ctx.GetBackgroundActivity()).To(HaveLen(2))
		})

		It("should leave the bad entry's timestamp nil", func() {
			Expect(ctx.GetBackgroundActivity()[0].GetTimestamp()).To(BeNil())
		})

		It("should leave the bad entry's summary intact", func() {
			Expect(ctx.GetBackgroundActivity()[0].GetSummary()).To(Equal("had a bad ts"))
		})
	})
})
