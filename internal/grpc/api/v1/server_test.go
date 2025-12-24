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
	"github.com/brendanjerwin/simple_wiki/index/bleve"
	"github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
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

// MockPageReaderMutator is a mock implementation of wikipage.PageReaderMutator for testing.
type MockPageReaderMutator struct {
	Frontmatter        wikipage.FrontMatter
	FrontmatterByID    map[string]map[string]any // For multi-page scenarios
	Markdown           wikipage.Markdown
	Err                error
	WrittenFrontmatter wikipage.FrontMatter
	WrittenMarkdown    wikipage.Markdown
	WrittenIdentifier  wikipage.PageIdentifier
	WriteErr           error
	DeletedIdentifier  wikipage.PageIdentifier
	DeleteErr          error
	// WrittenFrontmatterByID tracks all writes per identifier for multi-page scenarios
	WrittenFrontmatterByID map[string]map[string]any
}

func (m *MockPageReaderMutator) ReadFrontMatter(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	if m.Err != nil {
		return "", nil, m.Err
	}
	// Check WrittenFrontmatterByID first to get the latest written value
	if m.WrittenFrontmatterByID != nil {
		if fm, ok := m.WrittenFrontmatterByID[string(identifier)]; ok {
			return identifier, fm, nil
		}
	}
	// Check FrontmatterByID for initial multi-page scenarios
	if m.FrontmatterByID != nil {
		if fm, ok := m.FrontmatterByID[string(identifier)]; ok {
			return identifier, fm, nil
		}
		return "", nil, os.ErrNotExist
	}
	return identifier, m.Frontmatter, nil
}

func (m *MockPageReaderMutator) WriteFrontMatter(identifier wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	m.WrittenIdentifier = identifier
	m.WrittenFrontmatter = fm
	// Track writes per identifier for multi-page scenarios
	if m.WrittenFrontmatterByID == nil {
		m.WrittenFrontmatterByID = make(map[string]map[string]any)
	}
	// Deep copy the frontmatter to avoid mutation issues
	fmCopy := make(map[string]any)
	for k, v := range fm {
		fmCopy[k] = v
	}
	m.WrittenFrontmatterByID[string(identifier)] = fmCopy
	return m.WriteErr
}

func (m *MockPageReaderMutator) WriteMarkdown(identifier wikipage.PageIdentifier, md wikipage.Markdown) error {
	m.WrittenIdentifier = identifier
	m.WrittenMarkdown = md
	return m.WriteErr
}

func (m *MockPageReaderMutator) ReadMarkdown(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	if m.Err != nil {
		return "", "", m.Err
	}
	return identifier, m.Markdown, nil
}

func (m *MockPageReaderMutator) DeletePage(identifier wikipage.PageIdentifier) error {
	m.DeletedIdentifier = identifier
	return m.DeleteErr
}


// MockJobStreamServer is a mock implementation of apiv1.SystemInfoService_StreamJobStatusServer for testing.
type MockJobStreamServer struct {
	SentMessages []*apiv1.GetJobStatusResponse
	SendErr      error
	ContextDone  bool
}

func (m *MockJobStreamServer) Send(response *apiv1.GetJobStatusResponse) error {
	if m.SendErr != nil {
		return m.SendErr
	}
	m.SentMessages = append(m.SentMessages, response)
	return nil
}

func (m *MockJobStreamServer) Context() context.Context {
	if m.ContextDone {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}
	return context.Background()
}

func (*MockJobStreamServer) SetHeader(metadata.MD) error {
	return nil
}

func (*MockJobStreamServer) SendHeader(metadata.MD) error {
	return nil
}

func (*MockJobStreamServer) SetTrailer(metadata.MD) {
}

func (*MockJobStreamServer) SendMsg(any) error {
	return nil
}

func (*MockJobStreamServer) RecvMsg(any) error {
	return nil
}

// MockBleveIndexQueryer is a mock implementation of bleve.IQueryBleveIndex for testing.
type MockBleveIndexQueryer struct {
	Results []bleve.SearchResult
	Err     error
}

func (m *MockBleveIndexQueryer) Query(query string) ([]bleve.SearchResult, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Results, nil
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
			mockPageReaderMutator *MockPageReaderMutator
		)

		BeforeEach(func() {
			req = &apiv1.GetFrontmatterRequest{
				Page: "test-page",
			}
			mockPageReaderMutator = &MockPageReaderMutator{}
		})

		JustBeforeEach(func() {
			server = v1.NewServer("commit", time.Now(), mockPageReaderMutator, nil, nil, lumber.NewConsoleLogger(lumber.WARN), nil, nil, nil)
			res, err = server.GetFrontmatter(ctx, req)
		})

		When("the PageReaderMutator is not configured", func() {
			BeforeEach(func() {
				mockPageReaderMutator = nil
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.Internal, "PageReaderMutator not available"))
				Expect(res).To(BeNil())
			})
		})

		When("the requested page does not exist", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = os.ErrNotExist
			})

			It("should return a not found error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.NotFound, "page not found: test-page"))
				Expect(res).To(BeNil())
			})
		})

		When("PageReaderMutator returns a generic error", func() {
			var genericError error
			BeforeEach(func() {
				genericError = errors.New("kaboom")
				mockPageReaderMutator.Err = genericError
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
				mockPageReaderMutator.Frontmatter = expectedFm
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
				mockPageReaderMutator.Frontmatter = frontmatterWithIdentifier
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
				mockPageReaderMutator.Frontmatter = frontmatterWithNestedIdentifier
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
			mockPageReaderMutator *MockPageReaderMutator
			pageName           string
		)

		BeforeEach(func() {
			pageName = "test-page"
			mockPageReaderMutator = &MockPageReaderMutator{}
			req = &apiv1.MergeFrontmatterRequest{
				Page: pageName,
			}
		})

		JustBeforeEach(func() {
			server = v1.NewServer("commit", time.Now(), mockPageReaderMutator, nil, nil, lumber.NewConsoleLogger(lumber.WARN), nil, nil, nil)
			resp, err = server.MergeFrontmatter(ctx, req)
		})

		When("the PageReaderMutator is not configured", func() {
			BeforeEach(func() {
				mockPageReaderMutator = nil
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.Internal, "PageReaderMutator not available"))
				Expect(resp).To(BeNil())
			})
		})

		When("reading the frontmatter fails with a generic error", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = errors.New("read error")
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to read frontmatter"))
				Expect(resp).To(BeNil())
			})
		})

		When("writing the frontmatter fails", func() {
			BeforeEach(func() {
				mockPageReaderMutator.WriteErr = errors.New("write error")
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
				mockPageReaderMutator.Err = os.ErrNotExist

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
				Expect(mockPageReaderMutator.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(newFrontmatter))
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

				mockPageReaderMutator.Frontmatter = existingFrontmatter
				var err error
				req.Frontmatter, err = structpb.NewStruct(newFrontmatter)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write the merged frontmatter", func() {
				Expect(mockPageReaderMutator.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(mergedFrontmatter))
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
				
				mockPageReaderMutator.Frontmatter = existingFrontmatter
				var err error
				req.Frontmatter, err = structpb.NewStruct(newFrontmatter)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write the merged frontmatter with nested identifier keys preserved", func() {
				Expect(mockPageReaderMutator.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(expectedMergedFm))
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
			mockPageReaderMutator *MockPageReaderMutator
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

			mockPageReaderMutator = &MockPageReaderMutator{}

			req = &apiv1.ReplaceFrontmatterRequest{
				Page:        pageName,
				Frontmatter: newFrontmatterPb,
			}
		})

		JustBeforeEach(func() {
			server = v1.NewServer("commit", time.Now(), mockPageReaderMutator, nil, nil, lumber.NewConsoleLogger(lumber.WARN), nil, nil, nil)
			resp, err = server.ReplaceFrontmatter(ctx, req)
		})

		When("the PageReaderMutator is not configured", func() {
			BeforeEach(func() {
				mockPageReaderMutator = nil
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.Internal, "PageReaderMutator not available"))
				Expect(resp).To(BeNil())
			})
		})

		When("writing the frontmatter fails", func() {
			BeforeEach(func() {
				mockPageReaderMutator.WriteErr = errors.New("disk full")
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
				Expect(mockPageReaderMutator.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(expectedWrittenFm))
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
				Expect(mockPageReaderMutator.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReaderMutator.WrittenFrontmatter).To(BeNil())
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
				Expect(mockPageReaderMutator.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(expectedWrittenFm))
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
				Expect(mockPageReaderMutator.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(expectedWrittenFm))
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
			mockPageReaderMutator *MockPageReaderMutator
			pageName           string
		)

		BeforeEach(func() {
			pageName = "test-page"
			mockPageReaderMutator = &MockPageReaderMutator{}
			req = &apiv1.RemoveKeyAtPathRequest{
				Page: pageName,
			}
		})

		JustBeforeEach(func() {
			server = v1.NewServer("commit", time.Now(), mockPageReaderMutator, nil, nil, lumber.NewConsoleLogger(lumber.WARN), nil, nil, nil)
			resp, err = server.RemoveKeyAtPath(ctx, req)
		})

		When("the PageReaderMutator is not configured", func() {
			BeforeEach(func() {
				mockPageReaderMutator = nil
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.Internal, "PageReaderMutator not available"))
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
				mockPageReaderMutator.Err = os.ErrNotExist
				req.KeyPath = []*apiv1.PathComponent{{Component: &apiv1.PathComponent_Key{Key: "a"}}}
			})

			It("should return a not found error and no response", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.NotFound, "page not found"))
				Expect(resp).To(BeNil())
			})
		})

		When("the frontmatter is nil", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Frontmatter = nil
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
				mockPageReaderMutator.Frontmatter = initialFm
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
					Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(expectedFm))
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
					Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(expectedFm))
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
					Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(expectedFm))
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
				mockPageReaderMutator.Frontmatter = wikipage.FrontMatter{
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
				mockPageReaderMutator.Frontmatter = wikipage.FrontMatter{
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
				mockPageReaderMutator.Frontmatter = initialFm
				req.KeyPath = []*apiv1.PathComponent{
					{Component: &apiv1.PathComponent_Key{Key: "tags"}},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write the correctly modified frontmatter with identifier preserved", func() {
				Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(expectedFm))
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

				mockPageReaderMutator.Frontmatter = initialFm
				req.KeyPath = []*apiv1.PathComponent{
					{Component: &apiv1.PathComponent_Key{Key: "metadata"}},
					{Component: &apiv1.PathComponent_Key{Key: "identifier"}},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write the correctly modified frontmatter with nested identifier removed", func() {
				Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(expectedFm))
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

			server = v1.NewServer("test-commit", time.Now(), nil, nil, nil, logger, nil, nil, nil)
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
				server = v1.NewServer("test-commit", time.Now(), nil, nil, nil, nil, nil, nil, nil)

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
			mockPageReaderMutator *MockPageReaderMutator
		)

		BeforeEach(func() {
			req = &apiv1.DeletePageRequest{
				PageName: "test-page",
			}
			mockPageReaderMutator = &MockPageReaderMutator{}
		})

		JustBeforeEach(func() {
			server = v1.NewServer("commit", time.Now(), mockPageReaderMutator, nil, nil, lumber.NewConsoleLogger(lumber.WARN), nil, nil, nil)
			resp, err = server.DeletePage(ctx, req)
		})

		When("the PageReaderMutator is not configured", func() {
			BeforeEach(func() {
				mockPageReaderMutator = nil
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.Internal, "PageReaderMutator not available"))
				Expect(resp).To(BeNil())
			})
		})

		When("the page does not exist", func() {
			BeforeEach(func() {
				mockPageReaderMutator.DeleteErr = os.ErrNotExist
			})

			It("should return a not found error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.NotFound, "page not found: test-page"))
				Expect(resp).To(BeNil())
			})
		})

		When("deletion fails with a generic error", func() {
			BeforeEach(func() {
				mockPageReaderMutator.DeleteErr = errors.New("disk error")
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

			It("should call delete on the PageReaderMutator", func() {
				Expect(mockPageReaderMutator.DeletedIdentifier).To(Equal(wikipage.PageIdentifier("test-page")))
			})
		})
	})


	Describe("GetJobStatus", func() {
		var (
			req *apiv1.GetJobStatusRequest
			res *apiv1.GetJobStatusResponse
			err error
		)

		BeforeEach(func() {
			req = &apiv1.GetJobStatusRequest{}
			server = v1.NewServer("commit", time.Now(), nil, nil, nil, lumber.NewConsoleLogger(lumber.WARN), nil, nil, nil)
			res, err = server.GetJobStatus(ctx, req)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return empty job queues for now", func() {
			Expect(res).NotTo(BeNil())
			Expect(res.JobQueues).To(BeEmpty())
		})
	})

	Describe("StreamJobStatus", func() {
		var (
			req          *apiv1.StreamJobStatusRequest
			streamServer *MockJobStreamServer
		)

		BeforeEach(func() {
			req = &apiv1.StreamJobStatusRequest{}
			streamServer = &MockJobStreamServer{}
			server = v1.NewServer("commit", time.Now(), nil, nil, nil, lumber.NewConsoleLogger(lumber.WARN), nil, nil, nil)
		})

		var (
			err          error
			firstMessage *apiv1.GetJobStatusResponse
		)

		BeforeEach(func() {
			// Set up context that gets cancelled after initial send
			streamServer.ContextDone = true
			err = server.StreamJobStatus(req, streamServer)
			if len(streamServer.SentMessages) > 0 {
				firstMessage = streamServer.SentMessages[0]
			}
		})

		It("should handle context cancellation", func() {
			Expect(err).To(Equal(context.Canceled))
		})

		It("should send initial empty response", func() {
			Expect(streamServer.SentMessages).To(HaveLen(1))
			Expect(firstMessage).NotTo(BeNil())
			Expect(firstMessage.JobQueues).To(BeEmpty())
		})
	})

	Describe("SearchContent", func() {
		var (
			req                   *apiv1.SearchContentRequest
			resp                  *apiv1.SearchContentResponse
			err                   error
			mockBleveIndexQueryer *MockBleveIndexQueryer
		)

		BeforeEach(func() {
			req = &apiv1.SearchContentRequest{
				Query: "test query",
			}
			mockBleveIndexQueryer = &MockBleveIndexQueryer{}
		})

		JustBeforeEach(func() {
			server = v1.NewServer("commit", time.Now(), nil, mockBleveIndexQueryer, nil, lumber.NewConsoleLogger(lumber.WARN), nil, nil, nil)
			resp, err = server.SearchContent(ctx, req)
		})

		When("the search index is not available", func() {
			BeforeEach(func() {
				mockBleveIndexQueryer = nil
			})

			It("should return an internal error", func() {
				Expect(err).To(HaveGrpcStatus(codes.Internal, "Search index is not available"))
			})
		})

		When("a valid query is provided", func() {
			var searchResults []bleve.SearchResult

			BeforeEach(func() {
				searchResults = []bleve.SearchResult{
					{
						Identifier: "test-page",
						Title:      "Test Page",
						Fragment:   "This is a test fragment",
						Highlights: []bleve.HighlightSpan{
							{Start: 10, End: 14}, // "test"
						},
					},
				}
				mockBleveIndexQueryer.Results = searchResults
			})

			It("should return search results", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].Identifier).To(Equal("test-page"))
				Expect(resp.Results[0].Title).To(Equal("Test Page"))
				Expect(resp.Results[0].Fragment).To(Equal("This is a test fragment"))
				Expect(resp.Results[0].Highlights).To(HaveLen(1))
				Expect(resp.Results[0].Highlights[0].Start).To(Equal(int32(10)))
				Expect(resp.Results[0].Highlights[0].End).To(Equal(int32(14)))
			})
		})

		When("the search index returns an error", func() {
			BeforeEach(func() {
				mockBleveIndexQueryer.Err = errors.New("index error")
			})

			It("should return an internal error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to search"))
			})
		})

		When("an empty query is provided", func() {
			BeforeEach(func() {
				req.Query = ""
			})

			It("should return invalid argument error", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "query cannot be empty"))
			})
		})

		When("a frontmatter key include filter is provided", func() {
			var (
				mockFrontmatterIndexQueryer *MockFrontmatterIndexQueryer
				searchResults               []bleve.SearchResult
			)

			BeforeEach(func() {
				mockFrontmatterIndexQueryer = &MockFrontmatterIndexQueryer{}
				// Search returns 3 results (bleve may return non-munged identifiers)
				searchResults = []bleve.SearchResult{
					{Identifier: "page-with-inventory", Title: "Inventory Page", Fragment: "This has inventory"},
					{Identifier: "page-without-inventory", Title: "Normal Page", Fragment: "This is normal"},
					{Identifier: "another-inventory-page", Title: "Another Inventory", Fragment: "More inventory"},
				}
				mockBleveIndexQueryer.Results = searchResults
				// Only 2 pages have the "inventory" frontmatter key
				// Note: Frontmatter index stores MUNGED identifiers (snake_case)
				mockFrontmatterIndexQueryer.KeyExistsResults = []wikipage.PageIdentifier{"page_with_inventory", "another_inventory_page"}
				req.FrontmatterKeyIncludeFilters = []string{"inventory"}
			})

			JustBeforeEach(func() {
				server = v1.NewServer("commit", time.Now(), nil, mockBleveIndexQueryer, nil, lumber.NewConsoleLogger(lumber.WARN), nil, nil, mockFrontmatterIndexQueryer)
				resp, err = server.SearchContent(ctx, req)
			})

			It("should return only pages that have the include filter key", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(2))
				identifiers := []string{resp.Results[0].Identifier, resp.Results[1].Identifier}
				Expect(identifiers).To(ContainElements("page-with-inventory", "another-inventory-page"))
			})

			It("should exclude pages that do NOT have the include filter key", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				identifiers := make([]string, len(resp.Results))
				for i, r := range resp.Results {
					identifiers[i] = r.Identifier
				}
				Expect(identifiers).NotTo(ContainElement("page-without-inventory"))
			})

			When("the frontmatter index is not available", func() {
				BeforeEach(func() {
					mockFrontmatterIndexQueryer = nil
				})

				JustBeforeEach(func() {
					server = v1.NewServer("commit", time.Now(), nil, mockBleveIndexQueryer, nil, lumber.NewConsoleLogger(lumber.WARN), nil, nil, nil)
					resp, err = server.SearchContent(ctx, req)
				})

				It("should return an internal error", func() {
					Expect(err).To(HaveGrpcStatus(codes.Internal, "Frontmatter index not available for filtering"))
				})
			})

			When("no pages match the filter", func() {
				BeforeEach(func() {
					mockFrontmatterIndexQueryer.KeyExistsResults = []wikipage.PageIdentifier{}
				})

				It("should return empty results", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(resp).NotTo(BeNil())
					Expect(resp.Results).To(BeEmpty())
				})
			})
		})

		When("search returns pages that all lack the include filter key", func() {
			var (
				mockFrontmatterIndexQueryer *MockFrontmatterIndexQueryer
				searchResults               []bleve.SearchResult
			)

			BeforeEach(func() {
				mockFrontmatterIndexQueryer = &MockFrontmatterIndexQueryer{}
				// Search returns pages that do NOT have the required key
				searchResults = []bleve.SearchResult{
					{Identifier: "page-without-key-1", Title: "Page Without Key 1", Fragment: "No key here"},
					{Identifier: "page-without-key-2", Title: "Page Without Key 2", Fragment: "No key here either"},
					{Identifier: "page-without-key-3", Title: "Page Without Key 3", Fragment: "Still no key"},
				}
				mockBleveIndexQueryer.Results = searchResults
				// The include filter key exists ONLY on pages that are NOT in the search results
				mockFrontmatterIndexQueryer.KeyExistsResults = []wikipage.PageIdentifier{"completely-different-page", "another-unrelated-page"}
				req.FrontmatterKeyIncludeFilters = []string{"inventory.container"}
			})

			JustBeforeEach(func() {
				server = v1.NewServer("commit", time.Now(), nil, mockBleveIndexQueryer, nil, lumber.NewConsoleLogger(lumber.WARN), nil, nil, mockFrontmatterIndexQueryer)
				resp, err = server.SearchContent(ctx, req)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return empty results since none have the required key", func() {
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(BeEmpty())
			})
		})

		When("search results have non-munged identifiers but frontmatter index uses munged identifiers", func() {
			var (
				mockFrontmatterIndexQueryer *MockFrontmatterIndexQueryer
				searchResults               []bleve.SearchResult
			)

			BeforeEach(func() {
				mockFrontmatterIndexQueryer = &MockFrontmatterIndexQueryer{}
				// Bleve returns identifiers in original format (e.g., with hyphens, mixed case)
				searchResults = []bleve.SearchResult{
					{Identifier: "My-Inventory-Item", Title: "My Inventory Item", Fragment: "Has inventory"},
					{Identifier: "Another-Item", Title: "Another Item", Fragment: "Also has inventory"},
					{Identifier: "Non-Inventory-Page", Title: "Non Inventory", Fragment: "No inventory here"},
				}
				mockBleveIndexQueryer.Results = searchResults
				// Frontmatter index stores identifiers in munged format (lowercase snake_case)
				// Note: "My-Inventory-Item" becomes "my_inventory_item" when munged
				mockFrontmatterIndexQueryer.KeyExistsResults = []wikipage.PageIdentifier{
					"my_inventory_item",    // munged version of "My-Inventory-Item"
					"another_item",         // munged version of "Another-Item"
				}
				req.FrontmatterKeyIncludeFilters = []string{"inventory.container"}
			})

			JustBeforeEach(func() {
				server = v1.NewServer("commit", time.Now(), nil, mockBleveIndexQueryer, nil, lumber.NewConsoleLogger(lumber.WARN), nil, nil, mockFrontmatterIndexQueryer)
				resp, err = server.SearchContent(ctx, req)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should correctly match pages by munging search result identifiers", func() {
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(2))
				identifiers := []string{resp.Results[0].Identifier, resp.Results[1].Identifier}
				// Should return the ORIGINAL identifiers from bleve, not the munged ones
				Expect(identifiers).To(ContainElements("My-Inventory-Item", "Another-Item"))
			})

			It("should exclude pages without the include filter key", func() {
				Expect(resp).NotTo(BeNil())
				identifiers := make([]string, len(resp.Results))
				for i, r := range resp.Results {
					identifiers[i] = r.Identifier
				}
				Expect(identifiers).NotTo(ContainElement("Non-Inventory-Page"))
			})
		})

		When("multiple frontmatter key include filters are provided", func() {
			var (
				mockFrontmatterIndexQueryer *MockFrontmatterIndexQueryer
				searchResults               []bleve.SearchResult
			)

			BeforeEach(func() {
				mockFrontmatterIndexQueryer = &MockFrontmatterIndexQueryer{
					KeyExistsResultsMap: make(map[string][]wikipage.PageIdentifier),
				}
				// Search returns 4 results (bleve may return non-munged identifiers)
				searchResults = []bleve.SearchResult{
					{Identifier: "page-with-both", Title: "Both Keys", Fragment: "Has both"},
					{Identifier: "page-with-inventory", Title: "Inventory Only", Fragment: "Has inventory"},
					{Identifier: "page-with-container", Title: "Container Only", Fragment: "Has container"},
					{Identifier: "page-with-neither", Title: "Neither", Fragment: "Has neither"},
				}
				mockBleveIndexQueryer.Results = searchResults
				// Pages with "inventory" key (MUNGED identifiers in frontmatter index)
				mockFrontmatterIndexQueryer.KeyExistsResultsMap["inventory"] = []wikipage.PageIdentifier{
					"page_with_both", "page_with_inventory",
				}
				// Pages with "inventory.container" key (MUNGED identifiers)
				mockFrontmatterIndexQueryer.KeyExistsResultsMap["inventory.container"] = []wikipage.PageIdentifier{
					"page_with_both", "page_with_container",
				}
				// Require both keys (intersection)
				req.FrontmatterKeyIncludeFilters = []string{"inventory", "inventory.container"}
			})

			JustBeforeEach(func() {
				server = v1.NewServer("commit", time.Now(), nil, mockBleveIndexQueryer, nil, lumber.NewConsoleLogger(lumber.WARN), nil, nil, mockFrontmatterIndexQueryer)
				resp, err = server.SearchContent(ctx, req)
			})

			It("should return only results matching ALL include filters", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].Identifier).To(Equal("page-with-both"))
			})
		})

		When("a frontmatter key exclude filter is provided", func() {
			var (
				mockFrontmatterIndexQueryer *MockFrontmatterIndexQueryer
				searchResults               []bleve.SearchResult
			)

			BeforeEach(func() {
				mockFrontmatterIndexQueryer = &MockFrontmatterIndexQueryer{}
				// Search returns 3 results: 2 containers (have items key) and 1 item (no items key)
				searchResults = []bleve.SearchResult{
					{Identifier: "container-one", Title: "Container One", Fragment: "This is a container"},
					{Identifier: "item-one", Title: "Item One", Fragment: "This is an item"},
					{Identifier: "container-two", Title: "Container Two", Fragment: "Another container"},
				}
				mockBleveIndexQueryer.Results = searchResults
				// 2 pages have the "inventory.items" key (containers) - MUNGED identifiers
				mockFrontmatterIndexQueryer.KeyExistsResults = []wikipage.PageIdentifier{"container_one", "container_two"}
				req.FrontmatterKeyExcludeFilters = []string{"inventory.items"}
			})

			JustBeforeEach(func() {
				server = v1.NewServer("commit", time.Now(), nil, mockBleveIndexQueryer, nil, lumber.NewConsoleLogger(lumber.WARN), nil, nil, mockFrontmatterIndexQueryer)
				resp, err = server.SearchContent(ctx, req)
			})

			It("should return only results without the excluded key", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].Identifier).To(Equal("item-one"))
			})

			When("the frontmatter index is not available", func() {
				BeforeEach(func() {
					mockFrontmatterIndexQueryer = nil
				})

				JustBeforeEach(func() {
					server = v1.NewServer("commit", time.Now(), nil, mockBleveIndexQueryer, nil, lumber.NewConsoleLogger(lumber.WARN), nil, nil, nil)
					resp, err = server.SearchContent(ctx, req)
				})

				It("should return an internal error", func() {
					Expect(err).To(HaveGrpcStatus(codes.Internal, "Frontmatter index not available for filtering"))
				})
			})

			When("no pages have the excluded key", func() {
				BeforeEach(func() {
					mockFrontmatterIndexQueryer.KeyExistsResults = []wikipage.PageIdentifier{}
				})

				It("should return all results", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(resp).NotTo(BeNil())
					Expect(resp.Results).To(HaveLen(3))
				})
			})
		})

		When("multiple frontmatter key exclude filters are provided", func() {
			var (
				mockFrontmatterIndexQueryer *MockFrontmatterIndexQueryer
				searchResults               []bleve.SearchResult
			)

			BeforeEach(func() {
				mockFrontmatterIndexQueryer = &MockFrontmatterIndexQueryer{
					KeyExistsResultsMap: make(map[string][]wikipage.PageIdentifier),
				}
				// Search returns 4 results (bleve may return non-munged identifiers)
				searchResults = []bleve.SearchResult{
					{Identifier: "page-with-both-excluded", Title: "Both Excluded", Fragment: "Has both bad keys"},
					{Identifier: "page-with-items", Title: "Has Items", Fragment: "Has items key"},
					{Identifier: "page-with-archived", Title: "Has Archived", Fragment: "Has archived key"},
					{Identifier: "page-clean", Title: "Clean Page", Fragment: "Has neither"},
				}
				mockBleveIndexQueryer.Results = searchResults
				// Pages with "inventory.items" key (MUNGED identifiers)
				mockFrontmatterIndexQueryer.KeyExistsResultsMap["inventory.items"] = []wikipage.PageIdentifier{
					"page_with_both_excluded", "page_with_items",
				}
				// Pages with "archived" key (MUNGED identifiers)
				mockFrontmatterIndexQueryer.KeyExistsResultsMap["archived"] = []wikipage.PageIdentifier{
					"page_with_both_excluded", "page_with_archived",
				}
				// Exclude both keys (union of exclusions)
				req.FrontmatterKeyExcludeFilters = []string{"inventory.items", "archived"}
			})

			JustBeforeEach(func() {
				server = v1.NewServer("commit", time.Now(), nil, mockBleveIndexQueryer, nil, lumber.NewConsoleLogger(lumber.WARN), nil, nil, mockFrontmatterIndexQueryer)
				resp, err = server.SearchContent(ctx, req)
			})

			It("should return only results matching NONE of the exclude filters", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].Identifier).To(Equal("page-clean"))
			})
		})

		When("both inclusion and exclusion filters are provided", func() {
			var (
				mockFrontmatterIndexQueryer *MockFrontmatterIndexQueryer
				searchResults               []bleve.SearchResult
			)

			BeforeEach(func() {
				mockFrontmatterIndexQueryer = &MockFrontmatterIndexQueryer{
					KeyExistsResultsMap: make(map[string][]wikipage.PageIdentifier),
				}
				// Search returns 4 results (bleve may return non-munged identifiers)
				searchResults = []bleve.SearchResult{
					{Identifier: "inventory-container", Title: "Container", Fragment: "A container"},
					{Identifier: "inventory-item", Title: "Item", Fragment: "An item"},
					{Identifier: "non-inventory-page", Title: "Regular Page", Fragment: "Not inventory"},
					{Identifier: "another-inventory-item", Title: "Another Item", Fragment: "Another item"},
				}
				mockBleveIndexQueryer.Results = searchResults
				// 3 pages have "inventory" key (MUNGED identifiers)
				mockFrontmatterIndexQueryer.KeyExistsResultsMap["inventory"] = []wikipage.PageIdentifier{
					"inventory_container", "inventory_item", "another_inventory_item",
				}
				// 1 page has "inventory.items" key (it's a container) (MUNGED identifier)
				mockFrontmatterIndexQueryer.KeyExistsResultsMap["inventory.items"] = []wikipage.PageIdentifier{
					"inventory_container",
				}
				req.FrontmatterKeyIncludeFilters = []string{"inventory"}
				req.FrontmatterKeyExcludeFilters = []string{"inventory.items"}
			})

			JustBeforeEach(func() {
				server = v1.NewServer("commit", time.Now(), nil, mockBleveIndexQueryer, nil, lumber.NewConsoleLogger(lumber.WARN), nil, nil, mockFrontmatterIndexQueryer)
				resp, err = server.SearchContent(ctx, req)
			})

			It("should return only inventory items (has inventory, no items)", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(2))
				identifiers := []string{resp.Results[0].Identifier, resp.Results[1].Identifier}
				Expect(identifiers).To(ContainElements("inventory-item", "another-inventory-item"))
				Expect(identifiers).NotTo(ContainElement("inventory-container"))
				Expect(identifiers).NotTo(ContainElement("non-inventory-page"))
			})
		})

		When("result is an inventory item with container", func() {
			var (
				mockFrontmatterIndexQueryer *FlexibleMockFrontmatterIndexQueryer
				searchResults               []bleve.SearchResult
			)

			BeforeEach(func() {
				mockFrontmatterIndexQueryer = &FlexibleMockFrontmatterIndexQueryer{
					ExactMatchResults: make(map[string][]string),
					GetValueResults:   make(map[string]map[string]string),
				}
				// Search returns an item with a container
				searchResults = []bleve.SearchResult{
					{Identifier: "screwdriver", Title: "Screwdriver", Fragment: "A useful tool"},
				}
				mockBleveIndexQueryer.Results = searchResults
				// Item has container "toolbox"
				mockFrontmatterIndexQueryer.GetValueResults["screwdriver"] = map[string]string{
					"inventory.container": "toolbox",
				}
				// Container has a title
				mockFrontmatterIndexQueryer.GetValueResults["toolbox"] = map[string]string{
					"title": "My Toolbox",
				}
			})

			JustBeforeEach(func() {
				server = v1.NewServer("commit", time.Now(), nil, mockBleveIndexQueryer, nil, lumber.NewConsoleLogger(lumber.WARN), nil, nil, mockFrontmatterIndexQueryer)
				resp, err = server.SearchContent(ctx, req)
			})

			It("should include inventory context with container ID", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].InventoryContext).NotTo(BeNil())
				Expect(resp.Results[0].InventoryContext.ContainerId).To(Equal("toolbox"))
			})

			It("should include the container's title in inventory context", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].InventoryContext).NotTo(BeNil())
				Expect(resp.Results[0].InventoryContext.ContainerTitle).To(Equal("My Toolbox"))
			})

			When("container has no title", func() {
				BeforeEach(func() {
					mockFrontmatterIndexQueryer.GetValueResults["toolbox"] = map[string]string{}
				})

				It("should include inventory context with empty container title", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(resp).NotTo(BeNil())
					Expect(resp.Results).To(HaveLen(1))
					Expect(resp.Results[0].InventoryContext).NotTo(BeNil())
					Expect(resp.Results[0].InventoryContext.ContainerId).To(Equal("toolbox"))
					Expect(resp.Results[0].InventoryContext.ContainerTitle).To(Equal(""))
				})
			})

			It("should set IsInventoryRelated to true", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].InventoryContext).NotTo(BeNil())
				Expect(resp.Results[0].InventoryContext.IsInventoryRelated).To(BeTrue())
			})

			It("should include path with single container element", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].InventoryContext).NotTo(BeNil())
				Expect(resp.Results[0].InventoryContext.Path).To(HaveLen(1))
				Expect(resp.Results[0].InventoryContext.Path[0].Identifier).To(Equal("toolbox"))
				Expect(resp.Results[0].InventoryContext.Path[0].Title).To(Equal("My Toolbox"))
			})

			When("item has nested containers", func() {
				BeforeEach(func() {
					// Item is in toolbox, toolbox is in garage, garage is in house
					mockFrontmatterIndexQueryer.GetValueResults["screwdriver"] = map[string]string{
						"inventory.container": "toolbox",
					}
					mockFrontmatterIndexQueryer.GetValueResults["toolbox"] = map[string]string{
						"title":               "My Toolbox",
						"inventory.container": "garage",
					}
					mockFrontmatterIndexQueryer.GetValueResults["garage"] = map[string]string{
						"title":               "Main Garage",
						"inventory.container": "house",
					}
					mockFrontmatterIndexQueryer.GetValueResults["house"] = map[string]string{
						"title": "My House",
					}
				})

				It("should build full path from root to immediate container", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(resp).NotTo(BeNil())
					Expect(resp.Results).To(HaveLen(1))
					Expect(resp.Results[0].InventoryContext).NotTo(BeNil())
					Expect(resp.Results[0].InventoryContext.Path).To(HaveLen(3))
					
					// Path should be: house > garage > toolbox
					Expect(resp.Results[0].InventoryContext.Path[0].Identifier).To(Equal("house"))
					Expect(resp.Results[0].InventoryContext.Path[0].Title).To(Equal("My House"))
					Expect(resp.Results[0].InventoryContext.Path[0].Depth).To(Equal(int32(0)))
					
					Expect(resp.Results[0].InventoryContext.Path[1].Identifier).To(Equal("garage"))
					Expect(resp.Results[0].InventoryContext.Path[1].Title).To(Equal("Main Garage"))
					Expect(resp.Results[0].InventoryContext.Path[1].Depth).To(Equal(int32(1)))
					
					Expect(resp.Results[0].InventoryContext.Path[2].Identifier).To(Equal("toolbox"))
					Expect(resp.Results[0].InventoryContext.Path[2].Title).To(Equal("My Toolbox"))
					Expect(resp.Results[0].InventoryContext.Path[2].Depth).To(Equal(int32(2)))
				})
			})

			When("path element has no title", func() {
				BeforeEach(func() {
					mockFrontmatterIndexQueryer.GetValueResults["screwdriver"] = map[string]string{
						"inventory.container": "toolbox",
					}
					mockFrontmatterIndexQueryer.GetValueResults["toolbox"] = map[string]string{
						"inventory.container": "garage",
					}
					mockFrontmatterIndexQueryer.GetValueResults["garage"] = map[string]string{
						"title": "Main Garage",
					}
				})

				It("should include empty title in path element", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(resp).NotTo(BeNil())
					Expect(resp.Results).To(HaveLen(1))
					Expect(resp.Results[0].InventoryContext).NotTo(BeNil())
					Expect(resp.Results[0].InventoryContext.Path).To(HaveLen(2))
					
					Expect(resp.Results[0].InventoryContext.Path[0].Identifier).To(Equal("garage"))
					Expect(resp.Results[0].InventoryContext.Path[0].Title).To(Equal("Main Garage"))
					
					Expect(resp.Results[0].InventoryContext.Path[1].Identifier).To(Equal("toolbox"))
					Expect(resp.Results[0].InventoryContext.Path[1].Title).To(Equal(""))
				})
			})
		})

		When("result is not an inventory item", func() {
			var (
				mockFrontmatterIndexQueryer *FlexibleMockFrontmatterIndexQueryer
				searchResults               []bleve.SearchResult
			)

			BeforeEach(func() {
				mockFrontmatterIndexQueryer = &FlexibleMockFrontmatterIndexQueryer{
					ExactMatchResults: make(map[string][]string),
					GetValueResults:   make(map[string]map[string]string),
				}
				searchResults = []bleve.SearchResult{
					{Identifier: "regular_page", Title: "Regular Page", Fragment: "Some content"},
				}
				mockBleveIndexQueryer.Results = searchResults
				mockFrontmatterIndexQueryer.GetValueResults["regular_page"] = map[string]string{
					"title": "Regular Page",
				}
			})

			JustBeforeEach(func() {
				server = v1.NewServer("commit", time.Now(), nil, mockBleveIndexQueryer, nil, lumber.NewConsoleLogger(lumber.WARN), nil, nil, mockFrontmatterIndexQueryer)
				resp, err = server.SearchContent(ctx, req)
			})

			It("should not include inventory context", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].InventoryContext).To(BeNil())
			})
		})
	})

	Describe("ReadPage", func() {
		var (
			req                      *apiv1.ReadPageRequest
			resp                     *apiv1.ReadPageResponse
			err                      error
			mockPageReaderMutator    *MockPageReaderMutator
			mockMarkdownRenderer     *MockMarkdownRenderer
			mockTemplateExecutor     *MockTemplateExecutor
			mockFrontmatterIndexQueryer *MockFrontmatterIndexQueryer
		)

		BeforeEach(func() {
			req = &apiv1.ReadPageRequest{
				PageName: "test-page",
			}
			mockPageReaderMutator = &MockPageReaderMutator{}
			mockMarkdownRenderer = &MockMarkdownRenderer{}
			mockTemplateExecutor = &MockTemplateExecutor{}
			mockFrontmatterIndexQueryer = &MockFrontmatterIndexQueryer{}
		})

		JustBeforeEach(func() {
			server = v1.NewServer(
				"commit",
				time.Now(),
				mockPageReaderMutator,
				nil,
				nil,
				lumber.NewConsoleLogger(lumber.WARN),
				mockMarkdownRenderer,
				mockTemplateExecutor,
				mockFrontmatterIndexQueryer,
			)
			resp, err = server.ReadPage(ctx, req)
		})

		When("the PageReaderMutator is not configured", func() {
			BeforeEach(func() {
				mockPageReaderMutator = nil
			})

			It("should return an internal error", func() {
				Expect(err).To(HaveGrpcStatus(codes.Internal, "PageReaderMutator not available"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("the page does not exist", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = os.ErrNotExist
			})

			It("should return a not found error", func() {
				Expect(err).To(HaveGrpcStatus(codes.NotFound, "page not found: test-page"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("reading markdown fails with a generic error", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = errors.New("disk read error")
			})

			It("should return an internal error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to read page"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("rendering fails", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Markdown = "# Test Page"
				mockPageReaderMutator.Frontmatter = map[string]any{"title": "Test"}
				mockMarkdownRenderer.Err = errors.New("rendering error")
			})

			It("should return an internal error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to render page"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("the page exists with valid content", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Markdown = "# Test Page\n\nThis is test content."
				mockPageReaderMutator.Frontmatter = map[string]any{
					"title": "Test Page",
					"tags":  []any{"test"},
				}
				mockTemplateExecutor.Result = []byte("# Test Page\n\nThis is test content.")
				mockMarkdownRenderer.Result = []byte("<h1>Test Page</h1>\n<p>This is test content.</p>")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a response", func() {
				Expect(resp).NotTo(BeNil())
			})

			It("should return the markdown content", func() {
				Expect(resp.ContentMarkdown).To(Equal("# Test Page\n\nThis is test content."))
			})

			It("should return the frontmatter as TOML", func() {
				Expect(resp.FrontMatterToml).To(ContainSubstring("title = 'Test Page'"))
			})

			It("should return the rendered HTML", func() {
				Expect(resp.RenderedContentHtml).To(Equal("<h1>Test Page</h1>\n<p>This is test content.</p>"))
			})

			It("should return the rendered markdown", func() {
				Expect(resp.RenderedContentMarkdown).To(Equal("# Test Page\n\nThis is test content."))
			})
		})

		When("the page exists with no frontmatter", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Markdown = "# Simple Page"
				mockPageReaderMutator.Frontmatter = nil
				mockTemplateExecutor.Result = []byte("# Simple Page")
				mockMarkdownRenderer.Result = []byte("<h1>Simple Page</h1>")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return empty frontmatter TOML", func() {
				Expect(resp.FrontMatterToml).To(BeEmpty())
			})

			It("should return the markdown content", func() {
				Expect(resp.ContentMarkdown).To(Equal("# Simple Page"))
			})
		})
	})

	Describe("GenerateIdentifier", func() {
		var (
			req                   *apiv1.GenerateIdentifierRequest
			resp                  *apiv1.GenerateIdentifierResponse
			err                   error
			mockPageReaderMutator *MockPageReaderMutator
		)

		BeforeEach(func() {
			req = &apiv1.GenerateIdentifierRequest{
				Text: "Phillips Screwdriver",
			}
			mockPageReaderMutator = &MockPageReaderMutator{
				Err: os.ErrNotExist, // By default, no pages exist
			}
		})

		JustBeforeEach(func() {
			server = v1.NewServer(
				"commit",
				time.Now(),
				mockPageReaderMutator,
				nil,
				nil,
				lumber.NewConsoleLogger(lumber.WARN),
				nil,
				nil,
				nil,
			)
			resp, err = server.GenerateIdentifier(ctx, req)
		})

		When("the text is empty", func() {
			BeforeEach(func() {
				req.Text = ""
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "text is required"))
			})
		})

		When("the text is provided and no page exists with the identifier", func() {
			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the munged identifier", func() {
				Expect(resp.Identifier).To(Equal("phillips_screwdriver"))
			})

			It("should indicate the identifier is unique", func() {
				Expect(resp.IsUnique).To(BeTrue())
			})

			It("should not return existing page info", func() {
				Expect(resp.ExistingPage).To(BeNil())
			})
		})

		When("a page already exists with the generated identifier", func() {
			BeforeEach(func() {
				mockPageReaderMutator = &MockPageReaderMutator{
					FrontmatterByID: map[string]map[string]any{
						"phillips_screwdriver": {
							"title": "Existing Phillips Screwdriver",
							"inventory": map[string]any{
								"container": "toolbox",
							},
						},
					},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the munged identifier", func() {
				Expect(resp.Identifier).To(Equal("phillips_screwdriver"))
			})

			It("should indicate the identifier is not unique", func() {
				Expect(resp.IsUnique).To(BeFalse())
			})

			It("should return existing page info with identifier", func() {
				Expect(resp.ExistingPage).NotTo(BeNil())
				Expect(resp.ExistingPage.Identifier).To(Equal("phillips_screwdriver"))
			})

			It("should return existing page info with title", func() {
				Expect(resp.ExistingPage.Title).To(Equal("Existing Phillips Screwdriver"))
			})

			It("should return existing page info with container", func() {
				Expect(resp.ExistingPage.Container).To(Equal("toolbox"))
			})
		})

		When("ensure_unique is requested and a page already exists", func() {
			BeforeEach(func() {
				req.EnsureUnique = true
				mockPageReaderMutator = &MockPageReaderMutator{
					FrontmatterByID: map[string]map[string]any{
						"phillips_screwdriver": {
							"title": "Existing Phillips Screwdriver",
						},
					},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a unique identifier with suffix", func() {
				Expect(resp.Identifier).To(Equal("phillips_screwdriver_1"))
			})

			It("should indicate the identifier is unique", func() {
				Expect(resp.IsUnique).To(BeTrue())
			})

			It("should not return existing page info", func() {
				Expect(resp.ExistingPage).To(BeNil())
			})
		})

		When("ensure_unique is requested and multiple pages exist with suffixes", func() {
			BeforeEach(func() {
				req.EnsureUnique = true
				mockPageReaderMutator = &MockPageReaderMutator{
					FrontmatterByID: map[string]map[string]any{
						"phillips_screwdriver":   {"title": "First"},
						"phillips_screwdriver_1": {"title": "Second"},
						"phillips_screwdriver_2": {"title": "Third"},
					},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the next available suffix", func() {
				Expect(resp.Identifier).To(Equal("phillips_screwdriver_3"))
			})

			It("should indicate the identifier is unique", func() {
				Expect(resp.IsUnique).To(BeTrue())
			})
		})

		When("the PageReaderMutator is nil", func() {
			BeforeEach(func() {
				mockPageReaderMutator = nil
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the munged identifier", func() {
				Expect(resp.Identifier).To(Equal("phillips_screwdriver"))
			})

			It("should indicate the identifier is unique (cannot verify)", func() {
				Expect(resp.IsUnique).To(BeTrue())
			})
		})
	})
})

// MockMarkdownRenderer is a mock implementation of wikipage.IRenderMarkdownToHTML
type MockMarkdownRenderer struct {
	Result []byte
	Err    error
}

func (m *MockMarkdownRenderer) Render(input []byte) ([]byte, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Result, nil
}

// MockTemplateExecutor is a mock implementation of wikipage.IExecuteTemplate
type MockTemplateExecutor struct {
	Result []byte
	Err    error
}

func (m *MockTemplateExecutor) ExecuteTemplate(templateString string, fm wikipage.FrontMatter, reader wikipage.PageReader, query wikipage.IQueryFrontmatterIndex) ([]byte, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.Result != nil {
		return m.Result, nil
	}
	return []byte(templateString), nil
}

// MockFrontmatterIndexQueryer is a mock implementation of wikipage.IQueryFrontmatterIndex
type MockFrontmatterIndexQueryer struct {
	ExactMatchResults   []wikipage.PageIdentifier
	KeyExistsResults    []wikipage.PageIdentifier
	KeyExistsResultsMap map[string][]wikipage.PageIdentifier
	PrefixMatchResults  []wikipage.PageIdentifier
	GetValueResult      string
}

func (m *MockFrontmatterIndexQueryer) QueryExactMatch(dottedKeyPath wikipage.DottedKeyPath, value wikipage.Value) []wikipage.PageIdentifier {
	return m.ExactMatchResults
}

func (m *MockFrontmatterIndexQueryer) QueryKeyExistence(dottedKeyPath wikipage.DottedKeyPath) []wikipage.PageIdentifier {
	// Use map if available, otherwise fall back to simple results
	if m.KeyExistsResultsMap != nil {
		if results, ok := m.KeyExistsResultsMap[string(dottedKeyPath)]; ok {
			return results
		}
		return nil
	}
	return m.KeyExistsResults
}

func (m *MockFrontmatterIndexQueryer) QueryPrefixMatch(dottedKeyPath wikipage.DottedKeyPath, valuePrefix string) []wikipage.PageIdentifier {
	return m.PrefixMatchResults
}

func (m *MockFrontmatterIndexQueryer) GetValue(identifier wikipage.PageIdentifier, dottedKeyPath wikipage.DottedKeyPath) wikipage.Value {
	return m.GetValueResult
}
