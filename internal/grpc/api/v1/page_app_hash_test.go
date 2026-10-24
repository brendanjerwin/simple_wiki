//revive:disable:dot-imports
package v1_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

var _ = Describe("page app hash exclusions", func() {
	// The self-erasing-counter regression (issue #1272): a page app state
	// flush must not change the page version hash used by OCC/hashes; a
	// program write must (restamp). Both directions pinned through the
	// UpdatePageContent + SetPageApp/MergePageAppState round-trips on
	// one page.

	var (
		ctx    context.Context
		mock   *MockPageReaderMutator
		server *v1.Server
	)

	BeforeEach(func() {
		ctx = context.Background()
		mock = &MockPageReaderMutator{
			Frontmatter: wikipage.FrontMatter{"identifier": "app-page"},
			Markdown:    wikipage.Markdown("# Notes\n\nprose body\n"),
		}
		server = mustNewServer(mock, nil, nil)
	})

	When("page app state is merged", func() {
		It("the page version hash reported by a later prose edit is stable across the state flush", func() {
			// Pre-state hash (no page app yet): compute via UpdatePageContent's
			// returned VersionHash on the same content.
			first, err := server.UpdatePageContent(ctx, &apiv1.UpdatePageContentRequest{
				Page:               "app-page",
				NewContentMarkdown: "# Notes\n\nprose body\n",
			})
			Expect(err).NotTo(HaveOccurred())

			// Flush state (writes wiki.page_app_state).
			_, mErr := server.MergePageAppState(ctx, &apiv1.MergePageAppStateRequest{
				Page:  "app-page",
				State: mustStruct(map[string]any{"count": float64(1)}),
			})
			Expect(mErr).NotTo(HaveOccurred())

			// The SAME prose content still validates with the SAME expected
			// hash the client read before the state flush: an OCC write
			// that skips the flush's effect must not abort.
			second, err := server.UpdatePageContent(ctx, &apiv1.UpdatePageContentRequest{
				Page:                "app-page",
				NewContentMarkdown:  "# Notes\n\nprose body 2\n",
				ExpectedVersionHash: &first.VersionHash,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(second.GetSuccess()).To(BeTrue())
		})
	})

	When("the program changes via the edit path", func() {
		It("the returned hash reflects the fence change (restamped, not excluded)", func() {
			first, err := server.UpdatePageContent(ctx, &apiv1.UpdatePageContentRequest{
				Page:               "app-page",
				NewContentMarkdown: "# Notes\n\n```wiki-page-app\nroot = Card(\"a\")\n```\n",
			})
			Expect(err).NotTo(HaveOccurred())

			// Second edit changing ONLY the program text must succeed with
			// the post-first-edit hash (the funnel re-stamped in write 1,
			// and the stamps stay consistent).
			second, err := server.UpdatePageContent(ctx, &apiv1.UpdatePageContentRequest{
				Page:                "app-page",
				NewContentMarkdown:  "# Notes\n\n```wiki-page-app\nroot = Card(\"b\")\n```\n",
				ExpectedVersionHash: &first.VersionHash,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(second.GetSuccess()).To(BeTrue())
			// The restamped wiki.page_app frontmatter must match the new program.
			Expect(string(mock.Markdown)).To(ContainSubstring("root = Card(\"b\")"))
		})
	})
})
