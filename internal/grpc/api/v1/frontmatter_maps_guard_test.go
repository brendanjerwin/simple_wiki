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
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

var _ = Describe("Frontmatter maps.* namespace guard", func() {
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
		Describe("when payload contains a top-level maps key", func() {
			var err error

			BeforeEach(func() {
				body, _ := structpb.NewStruct(map[string]any{
					"maps": map[string]any{
						"yard": map[string]any{
							"markers": []any{},
						},
					},
				})
				_, err = server.MergeFrontmatter(ctx, &apiv1.MergeFrontmatterRequest{
					Page:        "p",
					Frontmatter: body,
				})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "maps"))
			})

			It("should direct callers to MapService", func() {
				Expect(err.Error()).To(ContainSubstring("MapService"))
			})

			It("should not have written to the page", func() {
				Expect(mock.WrittenIdentifier).To(Equal(wikipage.PageIdentifier("")))
			})
		})
	})

	Describe("ReplaceFrontmatter", func() {
		Describe("when payload contains a top-level maps key", func() {
			var err error

			BeforeEach(func() {
				body, _ := structpb.NewStruct(map[string]any{
					"maps": map[string]any{
						"yard": map[string]any{},
					},
				})
				_, err = server.ReplaceFrontmatter(ctx, &apiv1.ReplaceFrontmatterRequest{
					Page:        "p",
					Frontmatter: body,
				})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "maps"))
			})

			It("should not have written to the page", func() {
				Expect(mock.WrittenIdentifier).To(Equal(wikipage.PageIdentifier("")))
			})
		})

		Describe("when the page already has maps.* and payload omits maps", func() {
			var err error

			BeforeEach(func() {
				mock.Frontmatter = wikipage.FrontMatter{
					"title": "Existing",
					"maps": map[string]any{
						"yard": map[string]any{
							"markers": []any{
								map[string]any{"label": "Shed", "lat": 41.1, "lon": -72.2},
							},
						},
					},
				}

				body, _ := structpb.NewStruct(map[string]any{
					"title": "Replaced",
				})
				_, err = server.ReplaceFrontmatter(ctx, &apiv1.ReplaceFrontmatterRequest{
					Page:        "p",
					Frontmatter: body,
				})
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should preserve the maps subtree on disk", func() {
				Expect(mock.WrittenFrontmatter).To(HaveKey("maps"))
			})
		})
	})

	Describe("RemoveKeyAtPath", func() {
		Describe("when the path begins with maps", func() {
			var err error

			BeforeEach(func() {
				_, err = server.RemoveKeyAtPath(ctx, &apiv1.RemoveKeyAtPathRequest{
					Page: "p",
					KeyPath: []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "maps"}},
						{Component: &apiv1.PathComponent_Key{Key: "yard"}},
					},
				})
			})

			It("should return InvalidArgument", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "maps"))
			})
		})
	})
})
