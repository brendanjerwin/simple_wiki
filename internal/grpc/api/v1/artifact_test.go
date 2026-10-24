//revive:disable:dot-imports
package v1_test

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"

	"google.golang.org/protobuf/types/known/structpb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// mustStruct builds a structpb.Struct, failing the test on error.
func mustStruct(m map[string]any) *structpb.Struct {
	st, err := structpb.NewStruct(m)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return st
}

const artifactCounterProgram = "counter = State(0)\nroot = Stack([\n  Text(\"count: {counter}\"),\n])\n"

func artifactStampsFromFM(fm wikipage.FrontMatter) map[string]any {
	wiki, ok := fm["wiki"].(map[string]any)
	ExpectWithOffset(1, ok).To(BeTrue())
	artifact, ok := wiki["artifact"].(map[string]any)
	ExpectWithOffset(1, ok).To(BeTrue())
	return artifact
}

var _ = Describe("ArtifactService", func() {
	var (
		ctx    context.Context
		mock   *MockPageReaderMutator
		server *v1.Server
	)

	BeforeEach(func() {
		ctx = context.Background()
		mock = &MockPageReaderMutator{
			Frontmatter: wikipage.FrontMatter{"identifier": "app-page"},
			Markdown:    wikipage.Markdown("# My notes\n\nBody prose.\n"),
		}
		server = mustNewServer(mock, nil, nil)
	})

	Describe("GetArtifact", func() {
		When("the page has no artifact", func() {
			var err error

			BeforeEach(func() {
				_, err = server.GetArtifact(ctx, &apiv1.GetArtifactRequest{Page: "app-page"})
			})

			It("should return NotFound", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.NotFound, ""))
			})
		})

		When("the page is empty input", func() {
			var err error

			BeforeEach(func() {
				_, err = server.GetArtifact(ctx, &apiv1.GetArtifactRequest{})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, ""))
			})
		})
	})

	Describe("SetArtifact", func() {
		When("writing a valid program to a page without one", func() {
			var (
				resp *apiv1.SetArtifactResponse
				err  error
			)

			BeforeEach(func() {
				resp, err = server.SetArtifact(ctx, &apiv1.SetArtifactRequest{
					Page:   "app-page",
					Source: artifactCounterProgram,
				})
			})

			It("should succeed", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should stamp version 1 and a hash", func() {
				Expect(resp.GetVersion()).To(Equal(int64(1)))
				Expect(resp.GetVersionHash()).To(HavePrefix("sha256:"))
				Expect(resp.GetLibrary()).To(Equal("wiki-core@1"))
			})

			It("should write the fence into the markdown body, preserving prose", func() {
				Expect(string(mock.Markdown)).To(ContainSubstring("root = Stack(["))
				Expect(string(mock.Markdown)).To(ContainSubstring("```" + "wiki-artifact"))
				Expect(string(mock.Markdown)).To(ContainSubstring("# My notes"))
				Expect(string(mock.Markdown)).To(ContainSubstring("Body prose."))
			})

			It("should stamp wiki.artifact frontmatter", func() {
				stamps := artifactStampsFromFM(mock.Frontmatter)
				Expect(stamps["lang"]).To(Equal("openui-lang"))
				Expect(stamps["version"]).To(Equal(int64(1)))
				Expect(stamps["library"]).To(Equal("wiki-core@1"))
				Expect(stamps["version_hash"]).To(Equal(resp.GetVersionHash()))
			})
		})

		When("writing a program with Go-template-breaking syntax", func() {
			var err error

			BeforeEach(func() {
				_, err = server.SetArtifact(ctx, &apiv1.SetArtifactRequest{
					Page:   "app-page",
					Source: "root = Text(\"{{ .Identifier }}\")\n",
				})
			})

			It("should succeed (the fence never reaches Go template parsing)", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the program is a gross violation", func() {
			It("should reject an empty source", func() {
				_, err := server.SetArtifact(ctx, &apiv1.SetArtifactRequest{Page: "app-page", Source: "  \n"})
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, ""))
			})

			It("should reject a program without a root statement", func() {
				_, err := server.SetArtifact(ctx, &apiv1.SetArtifactRequest{
					Page:   "app-page",
					Source: "label = Text(\"no root here\")\n",
				})
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, ""))
			})

			It("should reject an oversized program", func() {
				big := "root = Text(\"" + strings.Repeat("x", 1<<17) + "\")\n"
				_, err := server.SetArtifact(ctx, &apiv1.SetArtifactRequest{Page: "app-page", Source: big})
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, ""))
			})

			It("should reject a program containing its own fence terminator", func() {
				_, err := server.SetArtifact(ctx, &apiv1.SetArtifactRequest{
					Page:   "app-page",
					Source: "root = Text(\"```\")\n",
				})
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, ""))
			})
		})

		When("OCC conflict", func() {
			var firstHash string

			BeforeEach(func() {
				resp, err := server.SetArtifact(ctx, &apiv1.SetArtifactRequest{Page: "app-page", Source: artifactCounterProgram})
				Expect(err).NotTo(HaveOccurred())
				firstHash = resp.GetVersionHash()
			})

			It("should reject a stale expected hash with Aborted", func() {
				_, err := server.SetArtifact(ctx, &apiv1.SetArtifactRequest{
					Page:                "app-page",
					Source:              "root = Text(\"v2\")\n",
					ExpectedVersionHash: "sha256:stale",
				})
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Aborted, ""))
			})

			It("should accept the current hash and bump the version", func() {
				resp, err := server.SetArtifact(ctx, &apiv1.SetArtifactRequest{
					Page:                "app-page",
					Source:              "root = Text(\"v2\")\n",
					ExpectedVersionHash: firstHash,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.GetVersion()).To(Equal(int64(2)))
			})
		})
	})

	Describe("ClearArtifact", func() {
		When("the page has an artifact", func() {
			BeforeEach(func() {
				_, err := server.SetArtifact(ctx, &apiv1.SetArtifactRequest{Page: "app-page", Source: artifactCounterProgram})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should remove the fence and stamps, keeping prose", func() {
				_, err := server.ClearArtifact(ctx, &apiv1.ClearArtifactRequest{Page: "app-page"})
				Expect(err).NotTo(HaveOccurred())
				Expect(string(mock.Markdown)).NotTo(ContainSubstring("wiki-artifact"))
				Expect(mock.Frontmatter).NotTo(HaveKey("wiki"))
				Expect(string(mock.Markdown)).To(ContainSubstring("# My notes"))
			})

			It("should return NotFound when there is nothing to clear", func() {
				// First clear within THIS It: the BeforeEach artifact exists,
				// so the first call succeeds — the second must be NotFound.
				_, firstErr := server.ClearArtifact(ctx, &apiv1.ClearArtifactRequest{Page: "app-page"})
				Expect(firstErr).NotTo(HaveOccurred())
				_, secondErr := server.ClearArtifact(ctx, &apiv1.ClearArtifactRequest{Page: "app-page"})
				Expect(secondErr).To(HaveOccurred())
				Expect(secondErr).To(HaveGrpcStatusWithSubstr(codes.NotFound, ""))
			})
		})
	})
})

var _ = Describe("ArtifactStateService", func() {
	var (
		ctx    context.Context
		mock   *MockPageReaderMutator
		server *v1.Server
	)

	BeforeEach(func() {
		ctx = context.Background()
		mock = &MockPageReaderMutator{
			Frontmatter: wikipage.FrontMatter{"identifier": "app-page"},
			Markdown:    wikipage.Markdown("# My notes\n\nBody prose.\n"),
		}
		server = mustNewServer(mock, nil, nil)
	})

	Describe("GetArtifactState", func() {
		When("no state has ever been persisted", func() {
			var (
				resp *apiv1.GetArtifactStateResponse
				err  error
			)

			BeforeEach(func() {
				resp, err = server.GetArtifactState(ctx, &apiv1.GetArtifactStateRequest{Page: "app-page"})
			})

			It("should return an empty state and a hash", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.GetState().AsMap()).To(BeEmpty())
				Expect(resp.GetStateVersionHash()).To(HavePrefix("sha256:"))
			})
		})
	})

	Describe("MergeArtifactState", func() {
		When("merging into fresh state", func() {
			var (
				resp *apiv1.MergeArtifactStateResponse
				err  error
			)

			BeforeEach(func() {
				_, _ = server.SetArtifact(ctx, &apiv1.SetArtifactRequest{Page: "app-page", Source: artifactCounterProgram})
				resp, err = server.MergeArtifactState(ctx, &apiv1.MergeArtifactStateRequest{
					Page:  "app-page",
					State: mustStruct(map[string]any{"count": float64(3), "tags": []any{"a"}}),
				})
			})

			It("should succeed and return the merged state", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.GetState().AsMap()).To(HaveKeyWithValue("count", float64(3)))
			})

			It("must NOT destroy the page body (the counter-delete regression)", func() {
				Expect(string(mock.Markdown)).To(ContainSubstring("root = Stack(["))
				Expect(string(mock.Markdown)).To(ContainSubstring("Body prose."))
			})
		})

		When("OCC conflict on state", func() {
			var firstStateHash string

			BeforeEach(func() {
				resp, err := server.MergeArtifactState(ctx, &apiv1.MergeArtifactStateRequest{
					Page:  "app-page",
					State: mustStruct(map[string]any{"count": float64(1)}),
				})
				Expect(err).NotTo(HaveOccurred())
				firstStateHash = resp.GetStateVersionHash()
			})

			It("should reject a stale state hash with Aborted", func() {
				_, err := server.MergeArtifactState(ctx, &apiv1.MergeArtifactStateRequest{
					Page:                     "app-page",
					State:                    mustStruct(map[string]any{"count": float64(2)}),
					StateExpectedVersionHash: "sha256:stale",
				})
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Aborted, ""))
			})

			It("should accept the current hash", func() {
				resp, err := server.MergeArtifactState(ctx, &apiv1.MergeArtifactStateRequest{
					Page:                     "app-page",
					State:                    mustStruct(map[string]any{"count": float64(2)}),
					StateExpectedVersionHash: firstStateHash,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.GetState().AsMap()["count"]).To(Equal(float64(2)))
			})
		})

		When("a state hash is independent of page-markdown edits", func() {
			It("should not be invalidated by an unrelated page edit", func() {
				resp, err := server.MergeArtifactState(ctx, &apiv1.MergeArtifactStateRequest{
					Page:  "app-page",
					State: mustStruct(map[string]any{"count": float64(1)}),
				})
				Expect(err).NotTo(HaveOccurred())

				// An unrelated page-markdown edit does not change the state hash
				// (namespace-scoped OCC — the two-streams rule).
				mock.Markdown = wikipage.Markdown("# Unrelated edit\n")
				subsequent, err := server.MergeArtifactState(ctx, &apiv1.MergeArtifactStateRequest{
					Page:                     "app-page",
					State:                    mustStruct(map[string]any{"count": float64(2)}),
					StateExpectedVersionHash: resp.GetStateVersionHash(),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(subsequent.GetState().AsMap()["count"]).To(Equal(float64(2)))
			})
		})
	})

	Describe("DeleteArtifactState", func() {
		It("should remove the subtree and keep the page body", func() {
			_, mErr := server.SetArtifact(ctx, &apiv1.SetArtifactRequest{Page: "app-page", Source: artifactCounterProgram})
			Expect(mErr).NotTo(HaveOccurred())
			_, err := server.GetArtifactState(ctx, &apiv1.GetArtifactStateRequest{Page: "app-page"})
			Expect(err).NotTo(HaveOccurred())
			_, dErr := server.DeleteArtifactState(ctx, &apiv1.DeleteArtifactStateRequest{Page: "app-page"})
			Expect(dErr).NotTo(HaveOccurred())
			Expect(string(mock.Markdown)).To(ContainSubstring("root = Stack(["))
			resp, gErr := server.GetArtifactState(ctx, &apiv1.GetArtifactStateRequest{Page: "app-page"})
			Expect(gErr).NotTo(HaveOccurred())
			Expect(resp.GetState().AsMap()).To(BeEmpty())
		})
	})
})

var _ = Describe("artifact placement guard", func() {
	var (
		ctx    context.Context
		mock   *MockPageReaderMutator
		server *v1.Server
	)

	BeforeEach(func() {
		ctx = context.Background()
		prose := "# Notes\n\n" + strings.Repeat("Substantial roadmap prose. ", 60) + "\n" // >1KB
		mock = &MockPageReaderMutator{
			Frontmatter: wikipage.FrontMatter{"identifier": "roadmap"},
			Markdown:    wikipage.Markdown(prose),
		}
		server = mustNewServer(mock, nil, nil)
	})
	Describe("SetArtifact default path", func() {
		It("should compose (keep prose) without any confirmation flag", func() {
			_, err := server.SetArtifact(ctx, &apiv1.SetArtifactRequest{
				Page:   "roadmap",
				Source: "root = Stack([Text(\"roadmap dashboard\")])\n",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(string(mock.Markdown)).To(ContainSubstring("Substantial roadmap prose."))
			Expect(string(mock.Markdown)).To(ContainSubstring("root = Stack(["))
		})
	})

	Describe("UpdatePageContent fence-only candidate", func() {
		It("rejects the prose-clear (edit path has no confirm field)", func() {
			_, err := server.UpdatePageContent(ctx, &apiv1.UpdatePageContentRequest{
				Page:               "roadmap",
				NewContentMarkdown: "```wiki-artifact\n$n = 0\nroot = Card(\"Lone\", [])\n```\n",
			})
			Expect(err).To(MatchError(ContainSubstring("FailedPrecondition")))
			Expect(err.Error()).To(ContainSubstring("drop 100% of the page's prose"))
			// Unchanged: the prose survives.
			Expect(string(mock.Markdown)).To(ContainSubstring("Substantial roadmap prose."))
		})
	})

	Describe("SetArtifact confirm_replace_content path", func() {
		It("clears the prose when the flag is set (app-standalone conversion)", func() {
			_, err := server.SetArtifact(ctx, &apiv1.SetArtifactRequest{
				Page:                  "roadmap",
				Source:                "root = Stack([Text(\"dashboard\")])\n",
				ConfirmReplaceContent: true,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(string(mock.Markdown)).NotTo(ContainSubstring("Substantial roadmap prose."))
			Expect(string(mock.Markdown)).To(ContainSubstring("root = Stack(["))
		})
	})
})
