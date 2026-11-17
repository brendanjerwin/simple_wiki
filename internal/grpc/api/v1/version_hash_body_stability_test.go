//revive:disable:dot-imports
package v1_test

// Tests for issue #1440: version_hash must be body-only so metadata-only writes
// (AppendBackgroundActivitySummary, checklist mutations, etc.) do not churn the
// token and cause spurious 409s on UpdatePageContent find-and-replace.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// computeExpectedBodyHashOnly computes the SHA-256 hash of the markdown body
// only, mirroring the new computeBodyHash function in page_management.go.
// Unlike computeExpectedPageHash, frontmatter is intentionally excluded so
// metadata-only writes do not move the token.
func computeExpectedBodyHashOnly(md string) string {
	h := sha256.Sum256([]byte(md))
	return hex.EncodeToString(h[:])
}

var _ = Describe("version_hash body stability (issue #1440)", func() {
	var testCtx context.Context

	BeforeEach(func() {
		testCtx = context.Background()
	})

	const (
		pageName = "test-wiki-page"
		body     = "# Title\n\nThis is the original body text.\n\nMore content here.\n"
	)

	var (
		initialFM = wikipage.FrontMatter{"identifier": pageName}
	)

	// Builds a mock whose frontmatter can be mutated between calls to simulate
	// a metadata-only write happening between a ReadPageOutline and an
	// UpdatePageContent find-and-replace.
	buildMockWithFM := func(fm wikipage.FrontMatter) *MockPageReaderMutator {
		return &MockPageReaderMutator{
			Markdown:    wikipage.Markdown(body),
			Frontmatter: fm,
		}
	}

	Describe("ReadPageOutline version_hash", func() {
		When("frontmatter changes but body bytes are identical", func() {
			var (
				hashBefore string
				hashAfter  string
			)

			BeforeEach(func() {
				mock := buildMockWithFM(initialFM)
				srv := mustNewServer(mock, nil, nil)

				resp1, err1 := srv.ReadPageOutline(testCtx,&apiv1.ReadPageOutlineRequest{Page: pageName})
				Expect(err1).NotTo(HaveOccurred())
				hashBefore = resp1.VersionHash

				// Simulate metadata-only write: add background_activity to agent.chat_context.
				mock.Frontmatter = wikipage.FrontMatter{
					"identifier": pageName,
					"agent": map[string]any{
						"chat_context": map[string]any{
							"background_activity": []any{"some summary"},
						},
					},
				}

				resp2, err2 := srv.ReadPageOutline(testCtx,&apiv1.ReadPageOutlineRequest{Page: pageName})
				Expect(err2).NotTo(HaveOccurred())
				hashAfter = resp2.VersionHash
			})

			It("should return the same version_hash before and after the metadata-only write", func() {
				Expect(hashBefore).To(Equal(hashAfter))
			})

			It("should return a version_hash equal to the body-only hash", func() {
				Expect(hashBefore).To(Equal(computeExpectedBodyHashOnly(body)))
			})
		})
	})

	Describe("UpdatePageContent find-and-replace after a metadata-only write", func() {
		When("the caller holds a hash taken before the metadata write", func() {
			var err error

			BeforeEach(func() {
				mock := buildMockWithFM(initialFM)
				srv := mustNewServer(mock, nil, nil)

				// Step 1: read outline → capture hash.
				outline, outlineErr := srv.ReadPageOutline(testCtx,&apiv1.ReadPageOutlineRequest{Page: pageName})
				Expect(outlineErr).NotTo(HaveOccurred())
				hashBeforeMetadataWrite := outline.VersionHash

				// Step 2: metadata-only write (body unchanged, frontmatter grows).
				mock.Frontmatter = wikipage.FrontMatter{
					"identifier": pageName,
					"agent": map[string]any{
						"chat_context": map[string]any{
							"background_activity": []any{"overseer cycle 372 summary"},
						},
					},
				}

				// Step 3: find-and-replace using the pre-metadata-write hash.
				oldText := "original body text"
				_, err = srv.UpdatePageContent(testCtx,&apiv1.UpdatePageContentRequest{
					Page:                pageName,
					NewContentMarkdown:  "replacement text",
					OldContentMarkdown:  &oldText,
					ExpectedVersionHash: &hashBeforeMetadataWrite,
				})
			})

			// This is the repro for issue #1440: a spurious 409 when
			// the body was never modified.
			It("should NOT return a 409 (Aborted) content version mismatch", func() {
				Expect(err).NotTo(HaveGrpcStatusWithSubstr(codes.Aborted, "content version mismatch"))
			})

			It("should succeed", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("UpdatePageContent still rejects genuine concurrent body edits", func() {
		When("the body was actually modified by a concurrent writer", func() {
			var err error

			BeforeEach(func() {
				mock := buildMockWithFM(initialFM)
				srv := mustNewServer(mock, nil, nil)

				// Read outline → capture hash of original body.
				outline, outlineErr := srv.ReadPageOutline(testCtx,&apiv1.ReadPageOutlineRequest{Page: pageName})
				Expect(outlineErr).NotTo(HaveOccurred())
				hashBeforeBodyEdit := outline.VersionHash

				// Simulate a genuine concurrent body edit.
				concurrentBody := "# Title\n\nCONCURRENTLY MODIFIED body text.\n\nMore content here.\n"
				concurrentMD := wikipage.Markdown(concurrentBody)
				mock.ConcurrentModificationMarkdown = &concurrentMD

				// Try find-and-replace with the stale (pre-body-edit) hash.
				oldText := "original body text"
				_, err = srv.UpdatePageContent(testCtx,&apiv1.UpdatePageContentRequest{
					Page:                pageName,
					NewContentMarkdown:  "replacement text",
					OldContentMarkdown:  &oldText,
					ExpectedVersionHash: &hashBeforeBodyEdit,
				})
			})

			It("should return a 409 (Aborted) content version mismatch", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Aborted, "content version mismatch"))
			})

			It("should not return a successful response", func() {
				// err is non-nil, so there's no resp to check; the existence of err is the assertion.
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
