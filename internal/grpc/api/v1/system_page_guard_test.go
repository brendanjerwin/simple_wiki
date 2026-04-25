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

var _ = Describe("system page guard", func() {
	var (
		ctx    context.Context
		mock   *MockPageReaderMutator
		server *v1.Server
	)

	BeforeEach(func() {
		ctx = context.Background()
		mock = &MockPageReaderMutator{
			Frontmatter: wikipage.FrontMatter{
				"identifier": "help",
				"system":     true,
			},
			Markdown: "# Help",
		}
		server = mustNewServer(mock, nil, nil)
	})

	Describe("UpdatePageContent on a system page", func() {
		var err error

		BeforeEach(func() {
			_, err = server.UpdatePageContent(ctx, &apiv1.UpdatePageContentRequest{
				PageName:           "help",
				NewContentMarkdown: "# Hijack",
			})
		})

		It("should reject with FailedPrecondition", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "system page"))
		})

		It("should not have written anything", func() {
			Expect(mock.WrittenMarkdown).To(BeEmpty())
		})
	})

	Describe("UpdateWholePage on a system page", func() {
		var err error

		BeforeEach(func() {
			_, err = server.UpdateWholePage(ctx, &apiv1.UpdateWholePageRequest{
				PageName:         "help",
				NewWholeMarkdown: "+++\ntitle = \"Hijack\"\n+++\nbody",
			})
		})

		It("should reject with FailedPrecondition", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "system page"))
		})
	})

	Describe("ClearPageContent on a system page", func() {
		var err error

		BeforeEach(func() {
			_, err = server.ClearPageContent(ctx, &apiv1.ClearPageContentRequest{
				PageName:     "help",
				ConfirmClear: true,
			})
		})

		It("should reject with FailedPrecondition", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "system page"))
		})
	})

	Describe("DeletePage on a system page", func() {
		var err error

		BeforeEach(func() {
			_, err = server.DeletePage(ctx, &apiv1.DeletePageRequest{
				PageName: "help",
			})
		})

		It("should reject with FailedPrecondition", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "system page"))
		})
	})

	Describe("MergeFrontmatter on a system page", func() {
		var err error

		BeforeEach(func() {
			body, _ := structpb.NewStruct(map[string]any{"title": "Hijack"})
			_, err = server.MergeFrontmatter(ctx, &apiv1.MergeFrontmatterRequest{
				Page:        "help",
				Frontmatter: body,
			})
		})

		It("should reject with FailedPrecondition", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "system page"))
		})
	})

	Describe("ReplaceFrontmatter on a system page", func() {
		var err error

		BeforeEach(func() {
			body, _ := structpb.NewStruct(map[string]any{"title": "Hijack"})
			_, err = server.ReplaceFrontmatter(ctx, &apiv1.ReplaceFrontmatterRequest{
				Page:        "help",
				Frontmatter: body,
			})
		})

		It("should reject with FailedPrecondition", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "system page"))
		})
	})

	Describe("RemoveKeyAtPath on a system page", func() {
		var err error

		BeforeEach(func() {
			_, err = server.RemoveKeyAtPath(ctx, &apiv1.RemoveKeyAtPathRequest{
				Page: "help",
				KeyPath: []*apiv1.PathComponent{
					{Component: &apiv1.PathComponent_Key{Key: "title"}},
				},
			})
		})

		It("should reject with FailedPrecondition", func() {
			Expect(err).To(HaveGrpcStatusWithSubstr(codes.FailedPrecondition, "system page"))
		})
	})
})
