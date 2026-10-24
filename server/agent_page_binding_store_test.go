//revive:disable:dot-imports
package server_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

var _ = Describe("AgentPageBindingStore", func() {
	var (
		pages *fakePageStore
		store *server.AgentPageBindingStore
	)

	BeforeEach(func() {
		pages = newFakePageStore()
		store = server.NewAgentPageBindingStore(pages)
	})

	Describe("GetBinding", func() {
		Describe("when the page has no agent namespace", func() {
			var binding *server.PageAgentBinding
			var err error

			BeforeEach(func() {
				binding, err = store.GetBinding("p")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return nil", func() {
				Expect(binding).To(BeNil())
			})
		})

		Describe("when the page has a binding", func() {
			var binding *server.PageAgentBinding

			BeforeEach(func() {
				_ = pages.WriteFrontMatter("p", wikipage.FrontMatter{
					"agent": map[string]any{
						"chat_agent": map[string]any{
							"agent_id":     "local-acp",
							"display_name": "Local Agent",
							"updated_by":   "alice",
							"updated_at":   "2026-10-24T17:00:00Z",
						},
					},
				}, wikipage.AnonymousIdentity)

				binding, _ = store.GetBinding("p")
			})

			It("should decode all fields", func() {
				Expect(binding.AgentID).To(Equal("local-acp"))
				Expect(binding.DisplayName).To(Equal("Local Agent"))
				Expect(binding.UpdatedBy).To(Equal("alice"))
				Expect(binding.UpdatedAt).To(Equal("2026-10-24T17:00:00Z"))
			})
		})

		Describe("when ReadFrontMatter returns an error", func() {
			var err error

			BeforeEach(func() {
				errStore := &errorPageStore{readErr: errors.New("disk failure")}
				badStore := server.NewAgentPageBindingStore(errStore)
				_, err = badStore.GetBinding("p")
			})

			It("should wrap and return the error", func() {
				Expect(err).To(MatchError(ContainSubstring("disk failure")))
			})
		})
	})

	Describe("SetBinding", func() {
		Describe("when setting a binding on a page with no prior agent namespace", func() {
			var updated bindingWriteResult

			BeforeEach(func() {
				var err error
				updated.updatedAt, err = store.SetBinding("p", &server.AgentBindingInput{
					AgentID:     "local-acp",
					DisplayName: "Local Agent",
					UpdatedBy:   "alice",
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should persist the binding fields", func() {
				binding, readErr := store.GetBinding("p")
				Expect(readErr).NotTo(HaveOccurred())
				Expect(binding.AgentID).To(Equal("local-acp"))
				Expect(binding.DisplayName).To(Equal("Local Agent"))
				Expect(binding.UpdatedBy).To(Equal("alice"))
			})

			It("should stamp a non-empty RFC3339 updated_at", func() {
				Expect(updated.updatedAt).NotTo(BeEmpty())
				Expect(server.IsValidRFC3339String(updated.updatedAt)).To(BeTrue())
			})
		})

		Describe("when setting a binding preserves existing frontmatter", func() {
			BeforeEach(func() {
				_ = pages.WriteFrontMatter("q", wikipage.FrontMatter{
					"title": "My Page",
					"agent": map[string]any{
						"chat_context": map[string]any{"last_conversation_summary": "hello"},
					},
				}, wikipage.AnonymousIdentity)
			})

			It("should preserve chat_context and title on write", func() {
				_, err := store.SetBinding("q", &server.AgentBindingInput{
					AgentID: "local-acp", UpdatedBy: "alice",
				})
				Expect(err).NotTo(HaveOccurred())

				_, fm, readErr := pages.ReadFrontMatter("q")
				Expect(readErr).NotTo(HaveOccurred())
				Expect(fm["title"]).To(Equal("My Page"))
				agentNS, ok := fm["agent"].(map[string]any)
				Expect(ok).To(BeTrue())
				_, hasChatCtx := agentNS["chat_context"]
				Expect(hasChatCtx).To(BeTrue(), "chat_context should be preserved")
			})
		})

		Describe("with an OCC mismatch", func() {
			var err error

			BeforeEach(func() {
				_, _ = store.SetBinding("p", &server.AgentBindingInput{
					AgentID: "local-acp", UpdatedBy: "alice",
				})
				_, err = store.SetBinding("p", &server.AgentBindingInput{
					AgentID: "other-acp", UpdatedBy: "bob",
					ExpectedUpdatedAt: "2001-01-01T00:00:00Z",
				})
			})

			It("should return a sentinel mismatch error", func() {
				Expect(err).To(MatchError(server.ErrBindingConflict))
			})
		})

		Describe("with a matching OCC timestamp", func() {
			var err error

			BeforeEach(func() {
				firstUpdated, _ := store.SetBinding("p", &server.AgentBindingInput{
					AgentID: "local-acp", UpdatedBy: "alice",
				})
				_, err = store.SetBinding("p", &server.AgentBindingInput{
					AgentID: "other-acp", UpdatedBy: "bob",
					ExpectedUpdatedAt: firstUpdated,
				})
			})

			It("should apply the write", func() {
				Expect(err).NotTo(HaveOccurred())
				binding, _ := store.GetBinding("p")
				Expect(binding.AgentID).To(Equal("other-acp"))
			})
		})

		Describe("when ReadFrontMatter returns an error", func() {
			var err error

			BeforeEach(func() {
				errStore := &errorPageStore{readErr: errors.New("disk failure")}
				badStore := server.NewAgentPageBindingStore(errStore)
				_, err = badStore.SetBinding("p", &server.AgentBindingInput{AgentID: "local-acp"})
			})

			It("should wrap and return the error", func() {
				Expect(err).To(MatchError(ContainSubstring("disk failure")))
			})
		})

		Describe("when WriteFrontMatter returns an error", func() {
			var err error

			BeforeEach(func() {
				errStore := &errorPageStore{writeErr: errors.New("write failure")}
				badStore := server.NewAgentPageBindingStore(errStore)
				_, err = badStore.SetBinding("p", &server.AgentBindingInput{AgentID: "local-acp"})
			})

			It("should wrap and return the error", func() {
				Expect(err).To(MatchError(ContainSubstring("write failure")))
			})
		})
	})
})

type bindingWriteResult struct {
	updatedAt string
}
