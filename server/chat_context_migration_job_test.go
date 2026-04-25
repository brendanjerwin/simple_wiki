//revive:disable:dot-imports
package server_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// fakeMigrationIndex satisfies AgentScheduleQueryer for migration tests.
// QueryKeyExistence returns whatever pages are configured for the requested
// key path; tests configure the legacy key only.
type fakeMigrationIndex struct {
	byKey map[string][]wikipage.PageIdentifier
}

func (f *fakeMigrationIndex) QueryKeyExistence(key string) []wikipage.PageIdentifier {
	return f.byKey[key]
}

// extractChatContextSummary digs the last_conversation_summary out of a
// post-migration frontmatter map without panicking on type assertions. Used
// by the migration tests where the on-disk shape is the agreed-upon
// invariant.
func extractChatContextSummary(fm wikipage.FrontMatter) string {
	agent, ok := fm["agent"].(map[string]any)
	if !ok {
		return ""
	}
	ctx, ok := agent["chat_context"].(map[string]any)
	if !ok {
		return ""
	}
	summary, ok := ctx["last_conversation_summary"].(string)
	if !ok {
		return ""
	}
	return summary
}

var _ = Describe("ChatContextMigrationJob", func() {
	var (
		pages *fakePageStore
		idx   *fakeMigrationIndex
		job   *server.ChatContextMigrationJob
	)

	BeforeEach(func() {
		pages = newFakePageStore()
		idx = &fakeMigrationIndex{byKey: map[string][]wikipage.PageIdentifier{}}
		job = server.NewChatContextMigrationJob(pages, idx)
	})

	Describe("GetName", func() {
		It("should be ChatContextMigration", func() {
			Expect(job.GetName()).To(Equal("ChatContextMigration"))
		})
	})

	Describe("Execute", func() {
		Describe("when there are no pages with legacy ai_agent_chat_context", func() {
			BeforeEach(func() {
				Expect(job.Execute()).To(Succeed())
			})

			It("should not write anything", func() {
				_, fm, _ := pages.ReadFrontMatter("any")
				Expect(fm).To(BeEmpty())
			})
		})

		Describe("when a page has legacy ai_agent_chat_context", func() {
			BeforeEach(func() {
				_ = pages.WriteFrontMatter("legacy", wikipage.FrontMatter{
					"title": "Pastry Project",
					"ai_agent_chat_context": map[string]any{
						"last_conversation_summary": "we discussed pastry",
						"user_goals":                []any{"finish weekend order"},
					},
				})
				idx.byKey["ai_agent_chat_context"] = []wikipage.PageIdentifier{"legacy"}
				Expect(job.Execute()).To(Succeed())
			})

			It("should move the data under agent.chat_context", func() {
				_, fm, _ := pages.ReadFrontMatter("legacy")
				agent, ok := fm["agent"].(map[string]any)
				Expect(ok).To(BeTrue())
				ctx, ok := agent["chat_context"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(ctx["last_conversation_summary"]).To(Equal("we discussed pastry"))
			})

			It("should remove the legacy key", func() {
				_, fm, _ := pages.ReadFrontMatter("legacy")
				_, present := fm["ai_agent_chat_context"]
				Expect(present).To(BeFalse())
			})

			It("should preserve other top-level keys", func() {
				_, fm, _ := pages.ReadFrontMatter("legacy")
				Expect(fm["title"]).To(Equal("Pastry Project"))
			})
		})

		Describe("when run a second time on a page that's already migrated", func() {
			BeforeEach(func() {
				_ = pages.WriteFrontMatter("done", wikipage.FrontMatter{
					"agent": map[string]any{
						"chat_context": map[string]any{
							"last_conversation_summary": "already moved",
						},
					},
				})
				// Index reports no legacy keys (because the migration ran already).
				Expect(job.Execute()).To(Succeed())
			})

			It("should leave the existing data alone", func() {
				_, fm, _ := pages.ReadFrontMatter("done")
				Expect(extractChatContextSummary(fm)).To(Equal("already moved"))
			})
		})

		Describe("when the destination already has agent.chat_context", func() {
			BeforeEach(func() {
				_ = pages.WriteFrontMatter("collision", wikipage.FrontMatter{
					"agent": map[string]any{
						"chat_context": map[string]any{
							"last_conversation_summary": "existing",
							"user_goals":                []any{"existing goal"},
						},
					},
					"ai_agent_chat_context": map[string]any{
						"last_conversation_summary": "legacy summary (should NOT clobber)",
						"user_goals":                []any{"legacy goal"},
					},
				})
				idx.byKey["ai_agent_chat_context"] = []wikipage.PageIdentifier{"collision"}
				Expect(job.Execute()).To(Succeed())
			})

			It("should keep the existing summary (do not clobber)", func() {
				_, fm, _ := pages.ReadFrontMatter("collision")
				Expect(extractChatContextSummary(fm)).To(Equal("existing"))
			})

			It("should still remove the legacy key (it's been considered)", func() {
				_, fm, _ := pages.ReadFrontMatter("collision")
				_, present := fm["ai_agent_chat_context"]
				Expect(present).To(BeFalse())
			})
		})
	})
})
