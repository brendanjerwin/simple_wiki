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

	Describe("GetSelectedAgentID", func() {
		Describe("when the page has no agent namespace", func() {
			var agentID string
			var err error

			BeforeEach(func() {
				agentID, err = store.GetSelectedAgentID("p")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an empty agent ID", func() {
				Expect(agentID).To(BeEmpty())
			})
		})

		Describe("when the page has an agent namespace but no selected_agent_id", func() {
			var agentID string
			var err error

			BeforeEach(func() {
				_ = pages.WriteFrontMatter("p", wikipage.FrontMatter{
					"agent": map[string]any{
						"schedules": []any{},
					},
				}, wikipage.AnonymousIdentity)

				agentID, err = store.GetSelectedAgentID("p")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an empty agent ID", func() {
				Expect(agentID).To(BeEmpty())
			})
		})

		Describe("when the page has a selected_agent_id", func() {
			var agentID string
			var err error

			BeforeEach(func() {
				_ = pages.WriteFrontMatter("p", wikipage.FrontMatter{
					"agent": map[string]any{
						"selected_agent_id": "local-acp",
					},
				}, wikipage.AnonymousIdentity)

				agentID, err = store.GetSelectedAgentID("p")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the stored agent ID", func() {
				Expect(agentID).To(Equal("local-acp"))
			})
		})

		Describe("when ReadFrontMatter returns an error", func() {
			var err error

			BeforeEach(func() {
				errStore := &errorPageStore{readErr: errors.New("disk failure")}
				badStore := server.NewAgentPageBindingStore(errStore)
				_, err = badStore.GetSelectedAgentID("p")
			})

			It("should wrap and return the error", func() {
				Expect(err).To(MatchError(ContainSubstring("disk failure")))
			})
		})
	})

	Describe("SetSelectedAgentID", func() {
		Describe("when setting an agent ID on a page with no prior agent namespace", func() {
			var err error

			BeforeEach(func() {
				err = store.SetSelectedAgentID("p", "local-acp")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should persist the agent ID in agent.selected_agent_id", func() {
				agentID, readErr := store.GetSelectedAgentID("p")
				Expect(readErr).NotTo(HaveOccurred())
				Expect(agentID).To(Equal("local-acp"))
			})
		})

		Describe("when setting an agent ID preserves existing frontmatter keys", func() {
			var err error

			BeforeEach(func() {
				_ = pages.WriteFrontMatter("q", wikipage.FrontMatter{
					"title": "My Page",
				}, wikipage.AnonymousIdentity)
				err = store.SetSelectedAgentID("q", "local-acp")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should preserve pre-existing top-level frontmatter keys", func() {
				_, fm, readErr := pages.ReadFrontMatter("q")
				Expect(readErr).NotTo(HaveOccurred())
				Expect(fm["title"]).To(Equal("My Page"))
			})
		})

		Describe("when setting an agent ID on a page that already has an agent namespace", func() {
			var err error

			BeforeEach(func() {
				_ = pages.WriteFrontMatter("p", wikipage.FrontMatter{
					"agent": map[string]any{
						"chat_context": map[string]any{"last_conversation_summary": "hello"},
					},
				}, wikipage.AnonymousIdentity)

				err = store.SetSelectedAgentID("p", "local-acp")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should persist the agent ID", func() {
				agentID, readErr := store.GetSelectedAgentID("p")
				Expect(readErr).NotTo(HaveOccurred())
				Expect(agentID).To(Equal("local-acp"))
			})

			It("should preserve the existing agent namespace contents", func() {
				_, fm, readErr := pages.ReadFrontMatter("p")
				Expect(readErr).NotTo(HaveOccurred())

				agentNS, ok := fm["agent"].(map[string]any)
				Expect(ok).To(BeTrue())
				_, hasChatCtx := agentNS["chat_context"]
				Expect(hasChatCtx).To(BeTrue(), "chat_context should be preserved")
			})
		})

		Describe("when clearing the agent ID by passing empty string", func() {
			var err error

			BeforeEach(func() {
				_ = pages.WriteFrontMatter("p", wikipage.FrontMatter{
					"agent": map[string]any{
						"selected_agent_id": "local-acp",
					},
				}, wikipage.AnonymousIdentity)

				err = store.SetSelectedAgentID("p", "")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should remove the agent ID from frontmatter", func() {
				agentID, readErr := store.GetSelectedAgentID("p")
				Expect(readErr).NotTo(HaveOccurred())
				Expect(agentID).To(BeEmpty())
			})
		})

		Describe("when ReadFrontMatter returns an error", func() {
			var err error

			BeforeEach(func() {
				errStore := &errorPageStore{readErr: errors.New("disk failure")}
				badStore := server.NewAgentPageBindingStore(errStore)
				err = badStore.SetSelectedAgentID("p", "local-acp")
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
				err = badStore.SetSelectedAgentID("p", "local-acp")
			})

			It("should wrap and return the error", func() {
				Expect(err).To(MatchError(ContainSubstring("write failure")))
			})
		})
	})
})
