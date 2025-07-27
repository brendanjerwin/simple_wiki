//revive:disable:dot-imports
package v1_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"strings"
	"testing"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

// gRPCStatusMatcher is a Gomega matcher for checking gRPC status errors.
type gRPCStatusMatcher struct {
	expectedCode    codes.Code
	expectedMsg     string // for exact match
	expectedMsgPart string // for substring match
}

// HaveGrpcStatus creates a matcher for a gRPC status with an exact message match.
func HaveGrpcStatus(code codes.Code, msg string) types.GomegaMatcher {
	return &gRPCStatusMatcher{expectedCode: code, expectedMsg: msg}
}

// HaveGrpcStatusWithSubstr creates a matcher for a gRPC status with a substring message match.
func HaveGrpcStatusWithSubstr(code codes.Code, substr string) types.GomegaMatcher {
	return &gRPCStatusMatcher{expectedCode: code, expectedMsgPart: substr}
}

func (m *gRPCStatusMatcher) Match(actual any) (success bool, err error) {
	if actual == nil {
		return false, nil
	}
	actualErr, ok := actual.(error)
	if !ok {
		return false, fmt.Errorf("gRPCStatusMatcher expects an error. Got\n\t%#v", actual)
	}

	st, ok := status.FromError(actualErr)
	if !ok {
		return false, fmt.Errorf("error is not a gRPC status. Got\n\t%#v", actualErr)
	}

	if st.Code() != m.expectedCode {
		return false, nil
	}

	if m.expectedMsg != "" && st.Message() != m.expectedMsg {
		return false, nil
	}

	if m.expectedMsgPart != "" && !strings.Contains(st.Message(), m.expectedMsgPart) {
		return false, nil
	}

	return true, nil
}

func (m *gRPCStatusMatcher) FailureMessage(actual any) (message string) {
	actualErr, ok := actual.(error)
	if !ok {
		return fmt.Sprintf("Expected an error, but got\n\t%#v", actual)
	}
	gomegaFailureMessage := fmt.Sprintf("Expected\n\t%#v", actual)

	st, ok := status.FromError(actualErr)
	if !ok {
		return fmt.Sprintf("%s\nto be a gRPC status error, but it's not.", gomegaFailureMessage)
	}

	var expectedMsgDesc string
	if m.expectedMsg != "" {
		expectedMsgDesc = fmt.Sprintf("and message '%s'", m.expectedMsg)
	} else if m.expectedMsgPart != "" {
		expectedMsgDesc = fmt.Sprintf("and message containing '%s'", m.expectedMsgPart)
	}

	return fmt.Sprintf("%s\nto have code %s %s\nbut got code %s and message '%s'", gomegaFailureMessage, m.expectedCode, expectedMsgDesc, st.Code(), st.Message())
}

func (m *gRPCStatusMatcher) NegatedFailureMessage(actual any) (message string) {
	actualErr, ok := actual.(error)
	if !ok {
		return fmt.Sprintf("Expected not an error, but got\n\t%#v", actual)
	}
	gomegaFailureMessage := fmt.Sprintf("Expected\n\t%#v", actual)

	_, ok = status.FromError(actualErr)
	if !ok {
		return fmt.Sprintf("%s\nnot to be a gRPC status error, but it's not (which is expected).", gomegaFailureMessage)
	}

	var expectedMsgDesc string
	if m.expectedMsg != "" {
		expectedMsgDesc = fmt.Sprintf("and message '%s'", m.expectedMsg)
	} else if m.expectedMsgPart != "" {
		expectedMsgDesc = fmt.Sprintf("and message containing '%s'", m.expectedMsgPart)
	}

	return fmt.Sprintf("%s\nnot to have code %s %s", gomegaFailureMessage, m.expectedCode, expectedMsgDesc)
}

func TestServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "gRPC V1 Server Suite")
}

// MockPageReadMutator is a mock implementation of wikipage.PageReadMutator for testing.
type MockPageReadMutator struct {
	Frontmatter        wikipage.FrontMatter
	Markdown           wikipage.Markdown
	Err                error
	WrittenFrontmatter wikipage.FrontMatter
	WrittenMarkdown    wikipage.Markdown
	WrittenIdentifier  wikipage.PageIdentifier
	WriteErr           error
	DeletedIdentifier  wikipage.PageIdentifier
	DeleteErr          error
}

func (m *MockPageReadMutator) ReadFrontMatter(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	if m.Err != nil {
		return "", nil, m.Err
	}
	return identifier, m.Frontmatter, nil
}

func (m *MockPageReadMutator) WriteFrontMatter(identifier wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	m.WrittenIdentifier = identifier
	m.WrittenFrontmatter = fm
	return m.WriteErr
}

func (m *MockPageReadMutator) WriteMarkdown(identifier wikipage.PageIdentifier, md wikipage.Markdown) error {
	m.WrittenIdentifier = identifier
	m.WrittenMarkdown = md
	return m.WriteErr
}

func (m *MockPageReadMutator) ReadMarkdown(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	if m.Err != nil {
		return "", "", m.Err
	}
	return identifier, m.Markdown, nil
}

func (m *MockPageReadMutator) DeletePage(identifier wikipage.PageIdentifier) error {
	m.DeletedIdentifier = identifier
	return m.DeleteErr
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
			mockPageReadMutator *MockPageReadMutator
		)

		BeforeEach(func() {
			req = &apiv1.GetFrontmatterRequest{
				Page: "test-page",
			}
			mockPageReadMutator = &MockPageReadMutator{}
		})

		JustBeforeEach(func() {
			server = v1.NewServer("commit", time.Now(), mockPageReadMutator, lumber.NewConsoleLogger(lumber.WARN))
			res, err = server.GetFrontmatter(ctx, req)
		})

		When("the PageReadMutator is not configured", func() {
			BeforeEach(func() {
				mockPageReadMutator = nil
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.Internal, "PageReadMutator not available"))
				Expect(res).To(BeNil())
			})
		})

		When("the requested page does not exist", func() {
			BeforeEach(func() {
				mockPageReadMutator.Err = os.ErrNotExist
			})

			It("should return a not found error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.NotFound, "page not found: test-page"))
				Expect(res).To(BeNil())
			})
		})

		When("PageReadMutator returns a generic error", func() {
			var genericError error
			BeforeEach(func() {
				genericError = errors.New("kaboom")
				mockPageReadMutator.Err = genericError
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.Internal, "failed to read frontmatter: kaboom"))
				Expect(res).To(BeNil())
			})
		})

		When("the requested page exists", func() {
			var expectedFm map[string]any

			BeforeEach(func() {
				expectedFm = map[string]any{
					"title": "Test Page",
					"tags":  []any{"test", "ginkgo"},
				}
				mockPageReadMutator.Frontmatter = expectedFm
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

		When("the requested page has frontmatter with identifier key", func() {
			var frontmatterWithIdentifier map[string]any
			var expectedFilteredFm map[string]any

			BeforeEach(func() {
				frontmatterWithIdentifier = map[string]any{
					"title":      "Test Page",
					"identifier": "test-page",
					"tags":       []any{"test", "ginkgo"},
				}
				expectedFilteredFm = map[string]any{
					"title": "Test Page",
					"tags":  []any{"test", "ginkgo"},
				}
				mockPageReadMutator.Frontmatter = frontmatterWithIdentifier
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the frontmatter without the identifier key", func() {
				Expect(res).NotTo(BeNil())
				expectedStruct, err := structpb.NewStruct(expectedFilteredFm)
				Expect(err).NotTo(HaveOccurred())
				Expect(res.Frontmatter).To(Equal(expectedStruct))
			})
		})

		When("the requested page has frontmatter with nested identifier keys", func() {
			var frontmatterWithNestedIdentifier map[string]any

			BeforeEach(func() {
				frontmatterWithNestedIdentifier = map[string]any{
					"title": "Test Page",
					"metadata": map[string]any{
						"identifier": "nested-identifier-should-be-allowed",
						"author":     "test-author",
					},
					"tags": []any{
						map[string]any{
							"identifier": "tag-identifier-should-be-allowed",
							"name":       "test-tag",
						},
					},
				}
				mockPageReadMutator.Frontmatter = frontmatterWithNestedIdentifier
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the frontmatter with nested identifier keys preserved", func() {
				Expect(res).NotTo(BeNil())
				// Nested identifier keys should be preserved, only root-level filtered
				expectedStruct, err := structpb.NewStruct(frontmatterWithNestedIdentifier)
				Expect(err).NotTo(HaveOccurred())
				Expect(res.Frontmatter).To(Equal(expectedStruct))
			})
		})
	})

	Describe("MergeFrontmatter", func() {
		var (
			req                *apiv1.MergeFrontmatterRequest
			resp               *apiv1.MergeFrontmatterResponse
			err                error
			mockPageReadMutator *MockPageReadMutator
			pageName           string
		)

		BeforeEach(func() {
			pageName = "test-page"
			mockPageReadMutator = &MockPageReadMutator{}
			req = &apiv1.MergeFrontmatterRequest{
				Page: pageName,
			}
		})

		JustBeforeEach(func() {
			server = v1.NewServer("commit", time.Now(), mockPageReadMutator, lumber.NewConsoleLogger(lumber.WARN))
			resp, err = server.MergeFrontmatter(ctx, req)
		})

		When("the PageReadMutator is not configured", func() {
			BeforeEach(func() {
				mockPageReadMutator = nil
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.Internal, "PageReadMutator not available"))
				Expect(resp).To(BeNil())
			})
		})

		When("reading the frontmatter fails with a generic error", func() {
			BeforeEach(func() {
				mockPageReadMutator.Err = errors.New("read error")
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to read frontmatter"))
				Expect(resp).To(BeNil())
			})
		})

		When("writing the frontmatter fails", func() {
			BeforeEach(func() {
				mockPageReadMutator.WriteErr = errors.New("write error")
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to write frontmatter"))
				Expect(resp).To(BeNil())
			})
		})

		When("the page does not exist", func() {
			var newFrontmatterPb *structpb.Struct
			var newFrontmatter wikipage.FrontMatter

			BeforeEach(func() {
				mockPageReadMutator.Err = os.ErrNotExist

				newFrontmatter = wikipage.FrontMatter{"title": "New Title"}
				var err error
				newFrontmatterPb, err = structpb.NewStruct(newFrontmatter)
				Expect(err).NotTo(HaveOccurred())
				req.Frontmatter = newFrontmatterPb
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write the new frontmatter", func() {
				Expect(mockPageReadMutator.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReadMutator.WrittenFrontmatter).To(Equal(newFrontmatter))
			})

			It("should return the new frontmatter", func() {
				Expect(resp.Frontmatter).To(Equal(newFrontmatterPb))
			})
		})

		When("the page exists", func() {
			var existingFrontmatter wikipage.FrontMatter
			var newFrontmatter wikipage.FrontMatter
			var mergedFrontmatter wikipage.FrontMatter

			BeforeEach(func() {
				existingFrontmatter = wikipage.FrontMatter{"title": "Old Title", "tags": []any{"old"}}
				newFrontmatter = wikipage.FrontMatter{"title": "New Title", "author": "test"}

				mergedFrontmatter = wikipage.FrontMatter{
					"title":  "New Title",
					"tags":   []any{"old"},
					"author": "test",
				}

				mockPageReadMutator.Frontmatter = existingFrontmatter
				var err error
				req.Frontmatter, err = structpb.NewStruct(newFrontmatter)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write the merged frontmatter", func() {
				Expect(mockPageReadMutator.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReadMutator.WrittenFrontmatter).To(Equal(mergedFrontmatter))
			})

			It("should return the merged frontmatter without identifier key", func() {
				expectedPb, err := structpb.NewStruct(mergedFrontmatter)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Frontmatter).To(Equal(expectedPb))
			})
		})

		When("the frontmatter contains an identifier key", func() {
			BeforeEach(func() {
				frontmatterWithIdentifier := wikipage.FrontMatter{
					"title":      "New Title",
					"identifier": "malicious-identifier",
				}
				var err error
				req.Frontmatter, err = structpb.NewStruct(frontmatterWithIdentifier)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "identifier key cannot be modified"))
				Expect(resp).To(BeNil())
			})
		})

		When("the frontmatter contains nested identifier keys", func() {
			var existingFrontmatter wikipage.FrontMatter
			var newFrontmatter wikipage.FrontMatter
			var expectedMergedFm wikipage.FrontMatter

			BeforeEach(func() {
				existingFrontmatter = wikipage.FrontMatter{
					"title": "Existing Title",
					"metadata": map[string]any{
						"author": "existing-author",
					},
				}
				newFrontmatter = wikipage.FrontMatter{
					"tags": []any{
						map[string]any{
							"identifier": "tag-identifier-should-be-allowed",
							"name":       "new-tag",
						},
					},
					"metadata": map[string]any{
						"identifier": "nested-identifier-should-be-allowed", 
						"version":    "1.0",
					},
				}
				expectedMergedFm = wikipage.FrontMatter{
					"title": "Existing Title",
					"tags": []any{
						map[string]any{
							"identifier": "tag-identifier-should-be-allowed",
							"name":       "new-tag",
						},
					},
					"metadata": map[string]any{
						"identifier": "nested-identifier-should-be-allowed",
						"version":    "1.0",
						// Note: "author" from existing metadata is overwritten by maps.Copy
					},
				}
				
				mockPageReadMutator.Frontmatter = existingFrontmatter
				var err error
				req.Frontmatter, err = structpb.NewStruct(newFrontmatter)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write the merged frontmatter with nested identifier keys preserved", func() {
				Expect(mockPageReadMutator.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReadMutator.WrittenFrontmatter).To(Equal(expectedMergedFm))
			})

			It("should return the merged frontmatter with nested identifier keys preserved", func() {
				expectedPb, err := structpb.NewStruct(expectedMergedFm)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Frontmatter).To(Equal(expectedPb))
			})
		})
	})

	Describe("ReplaceFrontmatter", func() {
		var (
			req                *apiv1.ReplaceFrontmatterRequest
			resp               *apiv1.ReplaceFrontmatterResponse
			err                error
			mockPageReadMutator *MockPageReadMutator
			pageName           string
			newFrontmatter     wikipage.FrontMatter
			newFrontmatterPb   *structpb.Struct
		)

		BeforeEach(func() {
			pageName = "test-page"
			newFrontmatter = wikipage.FrontMatter{"title": "New Title", "tags": []any{"a", "b"}}
			var err error
			newFrontmatterPb, err = structpb.NewStruct(newFrontmatter)
			Expect(err).NotTo(HaveOccurred())

			mockPageReadMutator = &MockPageReadMutator{}

			req = &apiv1.ReplaceFrontmatterRequest{
				Page:        pageName,
				Frontmatter: newFrontmatterPb,
			}
		})

		JustBeforeEach(func() {
			server = v1.NewServer("commit", time.Now(), mockPageReadMutator, lumber.NewConsoleLogger(lumber.WARN))
			resp, err = server.ReplaceFrontmatter(ctx, req)
		})

		When("the PageReadMutator is not configured", func() {
			BeforeEach(func() {
				mockPageReadMutator = nil
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.Internal, "PageReadMutator not available"))
				Expect(resp).To(BeNil())
			})
		})

		When("writing the frontmatter fails", func() {
			BeforeEach(func() {
				mockPageReadMutator.WriteErr = errors.New("disk full")
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to write frontmatter"))
				Expect(resp).To(BeNil())
			})
		})

		When("the request is successful", func() {
			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a response", func() {
				Expect(resp).NotTo(BeNil())
			})

			It("should write the new frontmatter to the page", func() {
				expectedWrittenFm := maps.Clone(newFrontmatter)
				expectedWrittenFm["identifier"] = pageName
				Expect(mockPageReadMutator.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReadMutator.WrittenFrontmatter).To(Equal(expectedWrittenFm))
			})

			It("should return the new frontmatter", func() {
				Expect(resp.Frontmatter).To(Equal(newFrontmatterPb))
			})
		})

		When("the request has a nil frontmatter", func() {
			BeforeEach(func() {
				req.Frontmatter = nil
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a response", func() {
				Expect(resp).NotTo(BeNil())
			})

			It("should write nil frontmatter", func() {
				Expect(mockPageReadMutator.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReadMutator.WrittenFrontmatter).To(BeNil())
			})

			It("should return nil frontmatter", func() {
				Expect(resp.Frontmatter).To(BeNil())
			})
		})

		When("the request contains an identifier key", func() {
			var frontmatterWithIdentifier wikipage.FrontMatter
			var expectedWrittenFm wikipage.FrontMatter
			var expectedResponseFm wikipage.FrontMatter

			BeforeEach(func() {
				frontmatterWithIdentifier = wikipage.FrontMatter{
					"title":      "New Title",
					"identifier": "malicious-identifier",
					"tags":       []any{"a", "b"},
				}
				expectedWrittenFm = wikipage.FrontMatter{
					"title":      "New Title",
					"identifier": pageName, // Should be set to correct page name
					"tags":       []any{"a", "b"},
				}
				expectedResponseFm = wikipage.FrontMatter{
					"title": "New Title",
					"tags":  []any{"a", "b"},
				}

				var err error
				req.Frontmatter, err = structpb.NewStruct(frontmatterWithIdentifier)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write frontmatter with correct identifier", func() {
				Expect(mockPageReadMutator.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReadMutator.WrittenFrontmatter).To(Equal(expectedWrittenFm))
			})

			It("should return frontmatter without identifier key", func() {
				expectedPb, err := structpb.NewStruct(expectedResponseFm)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Frontmatter).To(Equal(expectedPb))
			})
		})

		When("the request contains nested identifier keys", func() {
			var frontmatterWithNestedIdentifier wikipage.FrontMatter
			var expectedWrittenFm wikipage.FrontMatter
			var expectedResponseFm wikipage.FrontMatter

			BeforeEach(func() {
				frontmatterWithNestedIdentifier = wikipage.FrontMatter{
					"title": "New Title",
					"metadata": map[string]any{
						"identifier": "nested-identifier-should-be-allowed",
						"author":     "test-author",
					},
					"tags": []any{
						map[string]any{
							"identifier": "tag-identifier-should-be-allowed",
							"name":       "test-tag",
						},
					},
				}
				expectedWrittenFm = wikipage.FrontMatter{
					"title":      "New Title",
					"identifier": pageName, // Should be set to correct page name
					"metadata": map[string]any{
						"identifier": "nested-identifier-should-be-allowed",
						"author":     "test-author",
					},
					"tags": []any{
						map[string]any{
							"identifier": "tag-identifier-should-be-allowed",
							"name":       "test-tag",
						},
					},
				}
				expectedResponseFm = wikipage.FrontMatter{
					"title": "New Title",
					"metadata": map[string]any{
						"identifier": "nested-identifier-should-be-allowed",
						"author":     "test-author",
					},
					"tags": []any{
						map[string]any{
							"identifier": "tag-identifier-should-be-allowed",
							"name":       "test-tag",
						},
					},
				}

				var err error
				req.Frontmatter, err = structpb.NewStruct(frontmatterWithNestedIdentifier)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write frontmatter with nested identifier keys preserved and correct root identifier", func() {
				Expect(mockPageReadMutator.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReadMutator.WrittenFrontmatter).To(Equal(expectedWrittenFm))
			})

			It("should return frontmatter with nested identifier keys preserved", func() {
				expectedPb, err := structpb.NewStruct(expectedResponseFm)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Frontmatter).To(Equal(expectedPb))
			})
		})
	})

	Describe("RemoveKeyAtPath", func() {
		var (
			req                *apiv1.RemoveKeyAtPathRequest
			resp               *apiv1.RemoveKeyAtPathResponse
			err                error
			mockPageReadMutator *MockPageReadMutator
			pageName           string
		)

		BeforeEach(func() {
			pageName = "test-page"
			mockPageReadMutator = &MockPageReadMutator{}
			req = &apiv1.RemoveKeyAtPathRequest{
				Page: pageName,
			}
		})

		JustBeforeEach(func() {
			server = v1.NewServer("commit", time.Now(), mockPageReadMutator, lumber.NewConsoleLogger(lumber.WARN))
			resp, err = server.RemoveKeyAtPath(ctx, req)
		})

		When("the PageReadMutator is not configured", func() {
			BeforeEach(func() {
				mockPageReadMutator = nil
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.Internal, "PageReadMutator not available"))
				Expect(resp).To(BeNil())
			})
		})

		When("the key_path is empty", func() {
			BeforeEach(func() {
				req.KeyPath = []*apiv1.PathComponent{}
			})

			It("should return an invalid argument error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "key_path cannot be empty"))
				Expect(resp).To(BeNil())
			})
		})

		When("the page does not exist", func() {
			BeforeEach(func() {
				mockPageReadMutator.Err = os.ErrNotExist
				req.KeyPath = []*apiv1.PathComponent{{Component: &apiv1.PathComponent_Key{Key: "a"}}}
			})

			It("should return a not found error and no response", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.NotFound, "page not found"))
				Expect(resp).To(BeNil())
			})
		})

		When("the frontmatter is nil", func() {
			BeforeEach(func() {
				mockPageReadMutator.Frontmatter = nil
				req.KeyPath = []*apiv1.PathComponent{{Component: &apiv1.PathComponent_Key{Key: "a"}}}
			})
			It("should return a not found error for the key and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.NotFound, "key 'a' not found"))
				Expect(resp).To(BeNil())
			})
		})

		When("removing a key successfully", func() {
			var initialFm wikipage.FrontMatter
			BeforeEach(func() {
				initialFm = wikipage.FrontMatter{
					"a": "b",
					"c": map[string]any{
						"d": "e",
					},
					"f": []any{"g", "h", map[string]any{"i": "j"}},
				}
				mockPageReadMutator.Frontmatter = initialFm
			})

			When("from the top-level map", func() {
				var expectedFm wikipage.FrontMatter
				BeforeEach(func() {
					req.KeyPath = []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "a"}},
					}
					expectedFm = wikipage.FrontMatter{
						"c": map[string]any{
							"d": "e",
						},
						"f": []any{"g", "h", map[string]any{"i": "j"}},
					}
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should write the correctly modified frontmatter", func() {
					Expect(mockPageReadMutator.WrittenFrontmatter).To(Equal(expectedFm))
				})

				It("should return the correctly modified frontmatter", func() {
					expectedPb, err := structpb.NewStruct(expectedFm)
					Expect(err).NotTo(HaveOccurred())
					Expect(resp.Frontmatter).To(Equal(expectedPb))
				})
			})

			When("from a nested map", func() {
				var expectedFm wikipage.FrontMatter
				BeforeEach(func() {
					req.KeyPath = []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "c"}},
						{Component: &apiv1.PathComponent_Key{Key: "d"}},
					}
					expectedFm = wikipage.FrontMatter{
						"a": "b",
						"c": map[string]any{},
						"f": []any{"g", "h", map[string]any{"i": "j"}},
					}
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should write the correctly modified frontmatter", func() {
					Expect(mockPageReadMutator.WrittenFrontmatter).To(Equal(expectedFm))
				})

				It("should return the correctly modified frontmatter", func() {
					expectedPb, err := structpb.NewStruct(expectedFm)
					Expect(err).NotTo(HaveOccurred())
					Expect(resp.Frontmatter).To(Equal(expectedPb))
				})
			})

			When("from a slice", func() {
				var expectedFm wikipage.FrontMatter
				BeforeEach(func() {
					req.KeyPath = []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "f"}},
						{Component: &apiv1.PathComponent_Index{Index: 1}},
					}
					expectedFm = wikipage.FrontMatter{
						"a": "b",
						"c": map[string]any{
							"d": "e",
						},
						"f": []any{"g", map[string]any{"i": "j"}},
					}
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should write the correctly modified frontmatter", func() {
					Expect(mockPageReadMutator.WrittenFrontmatter).To(Equal(expectedFm))
				})

				It("should return the correctly modified frontmatter", func() {
					expectedPb, err := structpb.NewStruct(expectedFm)
					Expect(err).NotTo(HaveOccurred())
					Expect(resp.Frontmatter).To(Equal(expectedPb))
				})
			})
		})

		When("the path is invalid", func() {
			BeforeEach(func() {
				mockPageReadMutator.Frontmatter = wikipage.FrontMatter{
					"a": "b",
					"f": []any{"g"},
				}
			})

			When("a key is not found", func() {
				BeforeEach(func() {
					req.KeyPath = []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "z"}},
					}
				})
				It("returns a not found error and no response", func() {
					Expect(err).To(HaveGrpcStatus(codes.NotFound, "key 'z' not found"))
					Expect(resp).To(BeNil())
				})
			})

			When("an index is out of bounds", func() {
				BeforeEach(func() {
					req.KeyPath = []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "f"}},
						{Component: &apiv1.PathComponent_Index{Index: 99}},
					}
				})
				It("returns an out of range error and no response", func() {
					Expect(err).To(HaveGrpcStatusWithSubstr(codes.OutOfRange, "index 99 is out of range"))
					Expect(resp).To(BeNil())
				})
			})

			When("a key is used on a slice", func() {
				BeforeEach(func() {
					req.KeyPath = []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "f"}},
						{Component: &apiv1.PathComponent_Key{Key: "z"}},
					}
				})
				It("returns an invalid argument error and no response", func() {
					Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "not an index for a slice"))
					Expect(resp).To(BeNil())
				})
			})

			When("an index is used on a map", func() {
				BeforeEach(func() {
					req.KeyPath = []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Index{Index: 0}},
					}
				})
				It("returns an invalid argument error and no response", func() {
					Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "not a key for a map"))
					Expect(resp).To(BeNil())
				})
			})

			When("traversing through a primitive value", func() {
				BeforeEach(func() {
					req.KeyPath = []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "a"}},
						{Component: &apiv1.PathComponent_Key{Key: "b"}},
					}
				})
				It("returns an invalid argument error and no response", func() {
					Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "path is deeper than data structure"))
					Expect(resp).To(BeNil())
				})
			})
		})

		When("attempting to remove the identifier key", func() {
			BeforeEach(func() {
				mockPageReadMutator.Frontmatter = wikipage.FrontMatter{
					"title":      "Test Page",
					"identifier": "test-page",
				}
				req.KeyPath = []*apiv1.PathComponent{
					{Component: &apiv1.PathComponent_Key{Key: "identifier"}},
				}
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "identifier key cannot be removed"))
				Expect(resp).To(BeNil())
			})
		})

		When("removing a regular key with identifier present", func() {
			var initialFm wikipage.FrontMatter
			var expectedFm wikipage.FrontMatter

			BeforeEach(func() {
				initialFm = wikipage.FrontMatter{
					"title":      "Test Page",
					"identifier": "test-page",
					"tags":       []any{"test"},
				}
				expectedFm = wikipage.FrontMatter{
					"title":      "Test Page",
					"identifier": "test-page",
				}
				mockPageReadMutator.Frontmatter = initialFm
				req.KeyPath = []*apiv1.PathComponent{
					{Component: &apiv1.PathComponent_Key{Key: "tags"}},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write the correctly modified frontmatter with identifier preserved", func() {
				Expect(mockPageReadMutator.WrittenFrontmatter).To(Equal(expectedFm))
			})

			It("should return the modified frontmatter without identifier key", func() {
				expectedResponseFm := wikipage.FrontMatter{
					"title": "Test Page",
				}
				expectedPb, err := structpb.NewStruct(expectedResponseFm)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Frontmatter).To(Equal(expectedPb))
			})
		})

		When("removing a nested identifier key", func() {
			var initialFm wikipage.FrontMatter
			var expectedFm wikipage.FrontMatter

			BeforeEach(func() {
				initialFm = wikipage.FrontMatter{
					"title":      "Test Page",
					"identifier": "test-page",
					"metadata": map[string]any{
						"identifier": "nested-identifier-should-be-removable",
						"author":     "test-author",
						"version":    "1.0",
					},
				}
				expectedFm = wikipage.FrontMatter{
					"title":      "Test Page",
					"identifier": "test-page",
					"metadata": map[string]any{
						"author":  "test-author",
						"version": "1.0",
					},
				}

				mockPageReadMutator.Frontmatter = initialFm
				req.KeyPath = []*apiv1.PathComponent{
					{Component: &apiv1.PathComponent_Key{Key: "metadata"}},
					{Component: &apiv1.PathComponent_Key{Key: "identifier"}},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write the correctly modified frontmatter with nested identifier removed", func() {
				Expect(mockPageReadMutator.WrittenFrontmatter).To(Equal(expectedFm))
			})

			It("should return the modified frontmatter without root identifier key", func() {
				expectedResponseFm := wikipage.FrontMatter{
					"title": "Test Page",
					"metadata": map[string]any{
						"author":  "test-author",
						"version": "1.0",
					},
				}
				expectedPb, err := structpb.NewStruct(expectedResponseFm)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Frontmatter).To(Equal(expectedPb))
			})
		})
	})

	Describe("LoggingInterceptor", func() {
		var (
			server  *v1.Server
			logger  *lumber.ConsoleLogger
			ctx     context.Context
			req     any
			info    *grpc.UnaryServerInfo
			handler grpc.UnaryHandler
		)

		BeforeEach(func() {
			ctx = context.Background()
			req = &apiv1.GetVersionRequest{}
			info = &grpc.UnaryServerInfo{
				FullMethod: "/api.v1.Version/GetVersion",
			}

			// Create a mock logger
			logger = lumber.NewConsoleLogger(lumber.INFO)

			server = v1.NewServer("test-commit", time.Now(), nil, logger)
		})

		When("a successful gRPC call is made", func() {
			var (
				resp any
				err  error
			)

			BeforeEach(func() {
				handler = func(ctx context.Context, req any) (any, error) {
					time.Sleep(10 * time.Millisecond) // Simulate some work
					return &apiv1.GetVersionResponse{Commit: "test"}, nil
				}

				interceptor := server.LoggingInterceptor()
				resp, err = interceptor(ctx, req, info, handler)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the response", func() {
				Expect(resp).NotTo(BeNil())
				Expect(resp).To(BeAssignableToTypeOf(&apiv1.GetVersionResponse{}))
			})
		})

		When("a gRPC call fails", func() {
			var (
				resp any
				err  error
			)

			BeforeEach(func() {
				handler = func(ctx context.Context, req any) (any, error) {
					time.Sleep(5 * time.Millisecond) // Simulate some work
					return nil, status.Error(codes.Internal, "test error")
				}

				interceptor := server.LoggingInterceptor()
				resp, err = interceptor(ctx, req, info, handler)
			})

			It("should return the error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(HaveGrpcStatus(codes.Internal, "test error"))
			})

			It("should return nil response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("a handler panics", func() {
			var err error

			BeforeEach(func() {
				handler = func(ctx context.Context, req any) (any, error) {
					panic("test panic")
				}

				interceptor := server.LoggingInterceptor()

				// Capture the panic
				defer func() {
					if r := recover(); r != nil {
						err = status.Error(codes.Internal, "panic occurred")
					}
				}()

				_, err = interceptor(ctx, req, info, handler)
			})

			It("should propagate the panic", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		When("logger is nil", func() {
			var (
				resp any
				err  error
			)

			BeforeEach(func() {
				server = v1.NewServer("test-commit", time.Now(), nil, nil)

				handler = func(ctx context.Context, req any) (any, error) {
					return &apiv1.GetVersionResponse{Commit: "test"}, nil
				}

				interceptor := server.LoggingInterceptor()
				resp, err = interceptor(ctx, req, info, handler)
			})

			It("should not panic", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the response", func() {
				Expect(resp).NotTo(BeNil())
			})
		})
	})

	Describe("DeletePage", func() {
		var (
			req                *apiv1.DeletePageRequest
			resp               *apiv1.DeletePageResponse
			err                error
			mockPageReadMutator *MockPageReadMutator
		)

		BeforeEach(func() {
			req = &apiv1.DeletePageRequest{
				PageName: "test-page",
			}
			mockPageReadMutator = &MockPageReadMutator{}
		})

		JustBeforeEach(func() {
			server = v1.NewServer("commit", time.Now(), mockPageReadMutator, lumber.NewConsoleLogger(lumber.WARN))
			resp, err = server.DeletePage(ctx, req)
		})

		When("the PageReadMutator is not configured", func() {
			BeforeEach(func() {
				mockPageReadMutator = nil
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.Internal, "PageReadMutator not available"))
				Expect(resp).To(BeNil())
			})
		})

		When("the page does not exist", func() {
			BeforeEach(func() {
				mockPageReadMutator.DeleteErr = os.ErrNotExist
			})

			It("should return a not found error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.NotFound, "page not found: test-page"))
				Expect(resp).To(BeNil())
			})
		})

		When("deletion fails with a generic error", func() {
			BeforeEach(func() {
				mockPageReadMutator.DeleteErr = errors.New("disk error")
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to delete page"))
				Expect(resp).To(BeNil())
			})
		})

		When("deletion is successful", func() {
			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a success response", func() {
				Expect(resp).NotTo(BeNil())
				Expect(resp.Success).To(BeTrue())
				Expect(resp.Error).To(BeEmpty())
			})

			It("should call delete on the PageReadMutator", func() {
				Expect(mockPageReadMutator.DeletedIdentifier).To(Equal(wikipage.PageIdentifier("test-page")))
			})
		})
	})
})
