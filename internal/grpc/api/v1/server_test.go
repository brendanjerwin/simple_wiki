package v1_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/brendanjerwin/simple_wiki/common"
	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	v1 "github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "gRPC V1 Server Suite")
}

// MockPageReader satisfies the common.PageReader interface for testing.
type MockPageReader struct {
	Frontmatter common.FrontMatter
	Markdown    common.Markdown
	Err         error
}

func (m *MockPageReader) ReadFrontMatter(identifier common.PageIdentifier) (common.PageIdentifier, common.FrontMatter, error) {
	if m.Err != nil {
		return "", nil, m.Err
	}
	return identifier, m.Frontmatter, nil
}

func (m *MockPageReader) ReadMarkdown(identifier common.PageIdentifier) (common.PageIdentifier, common.Markdown, error) {
	if m.Err != nil {
		return "", "", m.Err
	}
	return identifier, m.Markdown, nil
}

var _ = Describe("Server", func() {
	var (
		server *v1.Server
		ctx    context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("GetFrontmatter", func() {
		var (
			req        *apiv1.GetFrontmatterRequest
			res        *apiv1.GetFrontmatterResponse
			err        error
			mockReader *MockPageReader
		)

		BeforeEach(func() {
			req = &apiv1.GetFrontmatterRequest{
				Page: "test-page",
			}
			mockReader = &MockPageReader{}
		})

		JustBeforeEach(func() {
			server = v1.NewServer("v0.0.0", "commit", time.Now(), mockReader)
			res, err = server.GetFrontmatter(ctx, req)
		})

		When("the PageReader is not configured", func() {
			BeforeEach(func() {
				mockReader = nil
			})

			It("should return an internal error", func() {
				Expect(res).To(BeNil())
				Expect(err).To(HaveOccurred())
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.Internal))
				Expect(st.Message()).To(Equal("PageReader not available"))
			})
		})

		When("the requested page does not exist", func() {
			BeforeEach(func() {
				mockReader.Err = errors.New("not found")
			})

			It("should return a not found error", func() {
				Expect(res).To(BeNil())
				Expect(err).To(HaveOccurred())
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.NotFound))
				Expect(st.Message()).To(ContainSubstring("page not found: test-page"))
			})
		})

		When("the requested page exists", func() {
			var expectedFm map[string]any

			BeforeEach(func() {
				expectedFm = map[string]any{
					"title": "Test Page",
					"tags":  []any{"test", "ginkgo"},
				}
				mockReader.Frontmatter = expectedFm
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the page's frontmatter", func() {
				Expect(res).NotTo(BeNil())
				expectedStruct, err := structpb.NewStruct(expectedFm)
				Expect(err).NotTo(HaveOccurred())
				Expect(res.Frontmatter).To(Equal(expectedStruct))
			})
		})
	})
})
