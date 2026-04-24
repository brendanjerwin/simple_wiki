//revive:disable:dot-imports
package v1_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/structpb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	v1 "github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

var _ = Describe("Frontmatter agent.* namespace guard", func() {
	var (
		ctx    context.Context
		mock   *MockPageReaderMutator
		server *v1.Server
	)

	BeforeEach(func() {
		ctx = context.Background()
		mock = &MockPageReaderMutator{}
		server = mustNewServer(mock, nil, nil)
	})

	Describe("MergeFrontmatter", func() {
		Describe("when payload contains a top-level agent key", func() {
			var err error

			BeforeEach(func() {
				body, _ := structpb.NewStruct(map[string]any{
					"agent": map[string]any{
						"schedules": []any{},
					},
				})
				_, err = server.MergeFrontmatter(ctx, &apiv1.MergeFrontmatterRequest{
					Page:        "p",
					Frontmatter: body,
				})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "agent"))
			})

			It("should mention AgentMetadataService in the error", func() {
				Expect(err.Error()).To(ContainSubstring("AgentMetadataService"))
			})

			It("should not have written to the page", func() {
				Expect(mock.WrittenIdentifier).To(Equal(wikipage.PageIdentifier("")))
			})
		})

		Describe("when payload does not contain agent", func() {
			var err error

			BeforeEach(func() {
				body, _ := structpb.NewStruct(map[string]any{
					"title": "Hello",
				})
				_, err = server.MergeFrontmatter(ctx, &apiv1.MergeFrontmatterRequest{
					Page:        "p",
					Frontmatter: body,
				})
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("ReplaceFrontmatter", func() {
		Describe("when payload contains a top-level agent key", func() {
			var err error

			BeforeEach(func() {
				body, _ := structpb.NewStruct(map[string]any{
					"agent": map[string]any{
						"schedules": []any{},
					},
				})
				_, err = server.ReplaceFrontmatter(ctx, &apiv1.ReplaceFrontmatterRequest{
					Page:        "p",
					Frontmatter: body,
				})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "agent"))
			})

			It("should not have written to the page", func() {
				Expect(mock.WrittenIdentifier).To(Equal(wikipage.PageIdentifier("")))
			})
		})

		Describe("when the page already has agent.* and payload omits agent", func() {
			var existing wikipage.FrontMatter

			BeforeEach(func() {
				existing = wikipage.FrontMatter{
					"title": "Existing",
					"agent": map[string]any{
						"schedules": []any{
							map[string]any{
								"id":      "s1",
								"cron":    "0 0 * * * *",
								"prompt":  "do thing",
								"enabled": true,
							},
						},
					},
				}
				mock.Frontmatter = existing

				body, _ := structpb.NewStruct(map[string]any{
					"title":    "Replaced",
					"keywords": []any{"a"},
				})
				_, err := server.ReplaceFrontmatter(ctx, &apiv1.ReplaceFrontmatterRequest{
					Page:        "p",
					Frontmatter: body,
				})
				Expect(err).NotTo(HaveOccurred())
			})

			It("should preserve the agent subtree on disk", func() {
				written := mock.WrittenFrontmatter
				Expect(written).To(HaveKey("agent"))
				agent, ok := written["agent"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(agent).To(HaveKey("schedules"))
			})

			It("should write the caller-supplied non-agent keys", func() {
				written := mock.WrittenFrontmatter
				Expect(written["title"]).To(Equal("Replaced"))
				Expect(written["keywords"]).To(Equal([]any{"a"}))
			})
		})
	})

	Describe("RemoveKeyAtPath", func() {
		Describe("when the path is exactly 'agent'", func() {
			var err error

			BeforeEach(func() {
				_, err = server.RemoveKeyAtPath(ctx, &apiv1.RemoveKeyAtPathRequest{
					Page: "p",
					KeyPath: []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "agent"}},
					},
				})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "agent"))
			})

			It("should not have written to the page", func() {
				Expect(mock.WrittenIdentifier).To(Equal(wikipage.PageIdentifier("")))
			})
		})

		Describe("when the path begins with 'agent.'", func() {
			var err error

			BeforeEach(func() {
				_, err = server.RemoveKeyAtPath(ctx, &apiv1.RemoveKeyAtPathRequest{
					Page: "p",
					KeyPath: []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "agent"}},
						{Component: &apiv1.PathComponent_Key{Key: "schedules"}},
					},
				})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "agent"))
			})
		})
	})

	Describe("GetFrontmatter", func() {
		Describe("when the page has agent.* data", func() {
			var got *apiv1.GetFrontmatterResponse
			var err error

			BeforeEach(func() {
				mock.Frontmatter = wikipage.FrontMatter{
					"title": "T",
					"agent": map[string]any{
						"schedules": []any{
							map[string]any{"id": "s1", "cron": "0 0 * * * *"},
						},
					},
				}
				got, err = server.GetFrontmatter(ctx, &apiv1.GetFrontmatterRequest{Page: "p"})
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should include agent.* in the response (read is not gated)", func() {
				m := got.GetFrontmatter().AsMap()
				Expect(m).To(HaveKey("agent"))
			})
		})
	})
})
