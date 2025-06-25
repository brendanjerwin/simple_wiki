package v1_test

import (
	"context"
	"errors"
	"os"
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

// MockPageReadWriter is a mock implementation of common.PageReadWriter for testing.
type MockPageReadWriter struct {
	Frontmatter        common.FrontMatter
	Markdown           common.Markdown
	Err                error
	WrittenFrontmatter common.FrontMatter
	WrittenMarkdown    common.Markdown
	WrittenIdentifier  common.PageIdentifier
	WriteErr           error
}

func (m *MockPageReadWriter) ReadFrontMatter(identifier common.PageIdentifier) (common.PageIdentifier, common.FrontMatter, error) {
	if m.Err != nil {
		return "", nil, m.Err
	}
	return identifier, m.Frontmatter, nil
}

func (m *MockPageReadWriter) WriteFrontMatter(identifier common.PageIdentifier, fm common.FrontMatter) error {
	m.WrittenIdentifier = identifier
	m.WrittenFrontmatter = fm
	return m.WriteErr
}

func (m *MockPageReadWriter) WriteMarkdown(identifier common.PageIdentifier, md common.Markdown) error {
	m.WrittenIdentifier = identifier
	m.WrittenMarkdown = md
	return m.WriteErr
}

func (m *MockPageReadWriter) ReadMarkdown(identifier common.PageIdentifier) (common.PageIdentifier, common.Markdown, error) {
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
			req                *apiv1.GetFrontmatterRequest
			res                *apiv1.GetFrontmatterResponse
			err                error
			mockPageReadWriter *MockPageReadWriter
		)

		BeforeEach(func() {
			req = &apiv1.GetFrontmatterRequest{
				Page: "test-page",
			}
			mockPageReadWriter = &MockPageReadWriter{}
		})

		JustBeforeEach(func() {
			server = v1.NewServer("v0.0.0", "commit", time.Now(), mockPageReadWriter)
			res, err = server.GetFrontmatter(ctx, req)
		})

		When("the PageReadWriter is not configured", func() {
			BeforeEach(func() {
				mockPageReadWriter = nil
			})

			It("should return an internal error", func() {
				Expect(res).To(BeNil())
				Expect(err).To(HaveOccurred())
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.Internal))
				Expect(st.Message()).To(Equal("PageReadWriter not available"))
			})
		})

		When("the requested page does not exist", func() {
			BeforeEach(func() {
				mockPageReadWriter.Err = os.ErrNotExist
			})

			It("should return a not found error", func() {
				Expect(res).To(BeNil())
				Expect(err).To(HaveOccurred())
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.NotFound))
				Expect(st.Message()).To(Equal("page not found: test-page"))
			})
		})

		When("PageReadWriter returns a generic error", func() {
			var genericError error
			BeforeEach(func() {
				genericError = errors.New("kaboom")
				mockPageReadWriter.Err = genericError
			})

			It("should return an internal error", func() {
				Expect(res).To(BeNil())
				Expect(err).To(HaveOccurred())
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.Internal))
				Expect(st.Message()).To(Equal("failed to read frontmatter: kaboom"))
			})
		})

		When("the requested page exists", func() {
			var expectedFm map[string]any

			BeforeEach(func() {
				expectedFm = map[string]any{
					"title": "Test Page",
					"tags":  []any{"test", "ginkgo"},
				}
				mockPageReadWriter.Frontmatter = expectedFm
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

	Describe("ReplaceFrontmatter", func() {
		var (
			req                *apiv1.ReplaceFrontmatterRequest
			resp               *apiv1.ReplaceFrontmatterResponse
			err                error
			mockPageReadWriter *MockPageReadWriter
			pageName           string
			newFrontmatter     common.FrontMatter
			newFrontmatterPb   *structpb.Struct
		)

		BeforeEach(func() {
			pageName = "test-page"
			newFrontmatter = common.FrontMatter{"title": "New Title", "tags": []any{"a", "b"}}
			var err error
			newFrontmatterPb, err = structpb.NewStruct(newFrontmatter)
			Expect(err).NotTo(HaveOccurred())

			mockPageReadWriter = &MockPageReadWriter{}

			req = &apiv1.ReplaceFrontmatterRequest{
				Page:        pageName,
				Frontmatter: newFrontmatterPb,
			}
		})

		JustBeforeEach(func() {
			server = v1.NewServer("v0.0.0", "commit", time.Now(), mockPageReadWriter)
			resp, err = server.ReplaceFrontmatter(ctx, req)
		})

		When("the PageReadWriter is not configured", func() {
			BeforeEach(func() {
				mockPageReadWriter = nil
			})

			It("should return an internal error", func() {
				Expect(resp).To(BeNil())
				Expect(err).To(HaveOccurred())
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.Internal))
				Expect(st.Message()).To(Equal("PageReadWriter not available"))
			})
		})

		When("writing the frontmatter fails", func() {
			BeforeEach(func() {
				mockPageReadWriter.WriteErr = errors.New("disk full")
			})

			It("should return an internal error", func() {
				Expect(resp).To(BeNil())
				Expect(err).To(HaveOccurred())
				st, ok := status.FromError(err)
				Expect(ok).To(BeTrue())
				Expect(st.Code()).To(Equal(codes.Internal))
				Expect(st.Message()).To(ContainSubstring("failed to write frontmatter"))
			})
		})

		When("the request is successful", func() {
			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
			})

			It("should write the new frontmatter to the page", func() {
				Expect(mockPageReadWriter.WrittenIdentifier).To(Equal(common.PageIdentifier(pageName)))
				Expect(mockPageReadWriter.WrittenFrontmatter).To(Equal(newFrontmatter))
			})

			It("should return the new frontmatter", func() {
				Expect(resp.Frontmatter).To(Equal(newFrontmatterPb))
			})
		})
	})
})
