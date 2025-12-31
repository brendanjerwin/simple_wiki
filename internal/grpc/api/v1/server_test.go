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
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
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
	MarkdownWriteErr   error // Separate error for WriteMarkdown
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
	// Shallow copy the frontmatter (sufficient for test isolation)
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
	if m.MarkdownWriteErr != nil {
		return m.MarkdownWriteErr
	}
	return nil
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

// noOpFrontmatterIndexQueryer is a minimal mock for tests that don't need frontmatter indexing.
type noOpFrontmatterIndexQueryer struct{}

func (noOpFrontmatterIndexQueryer) QueryExactMatch(wikipage.DottedKeyPath, wikipage.Value) []wikipage.PageIdentifier {
	return nil
}
func (noOpFrontmatterIndexQueryer) QueryKeyExistence(wikipage.DottedKeyPath) []wikipage.PageIdentifier {
	return nil
}
func (noOpFrontmatterIndexQueryer) QueryPrefixMatch(wikipage.DottedKeyPath, string) []wikipage.PageIdentifier {
	return nil
}
func (noOpFrontmatterIndexQueryer) GetValue(wikipage.PageIdentifier, wikipage.DottedKeyPath) wikipage.Value {
	return ""
}

// noOpBleveIndexQueryer is a minimal mock for tests that don't need search indexing.
type noOpBleveIndexQueryer struct{}

func (noOpBleveIndexQueryer) Query(string) ([]bleve.SearchResult, error) { return nil, nil }

// noOpPageReaderMutator is a minimal mock for tests that don't need page operations.
type noOpPageReaderMutator struct{}

func (noOpPageReaderMutator) ReadFrontMatter(wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	return "", nil, os.ErrNotExist
}
func (noOpPageReaderMutator) WriteFrontMatter(wikipage.PageIdentifier, wikipage.FrontMatter) error {
	return nil
}
func (noOpPageReaderMutator) ReadMarkdown(wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	return "", "", os.ErrNotExist
}
func (noOpPageReaderMutator) WriteMarkdown(wikipage.PageIdentifier, wikipage.Markdown) error {
	return nil
}
func (noOpPageReaderMutator) DeletePage(wikipage.PageIdentifier) error { return nil }

// mustNewServer creates a server with the given dependencies, failing the test if creation fails.
// Use this for tests where server creation should not fail.
func mustNewServer(
	pageReaderMutator wikipage.PageReaderMutator,
	bleveIndexQueryer bleve.IQueryBleveIndex,
	frontmatterIndexQueryer wikipage.IQueryFrontmatterIndex,
) *v1.Server {
	return mustNewServerWithJobCoordinator(pageReaderMutator, bleveIndexQueryer, frontmatterIndexQueryer, nil)
}

// mustNewServerWithJobCoordinator creates a server with the given dependencies including job coordinator.
// Use this for tests that need to interact with the job queue.
func mustNewServerWithJobCoordinator(
	pageReaderMutator wikipage.PageReaderMutator,
	bleveIndexQueryer bleve.IQueryBleveIndex,
	frontmatterIndexQueryer wikipage.IQueryFrontmatterIndex,
	jobCoordinator jobs.JobCoordinator,
) *v1.Server {
	if pageReaderMutator == nil {
		pageReaderMutator = noOpPageReaderMutator{}
	}
	if bleveIndexQueryer == nil {
		bleveIndexQueryer = noOpBleveIndexQueryer{}
	}
	if frontmatterIndexQueryer == nil {
		frontmatterIndexQueryer = noOpFrontmatterIndexQueryer{}
	}
	server, err := v1.NewServer(
		"test-commit",
		time.Now(),
		pageReaderMutator,
		bleveIndexQueryer,
		jobCoordinator,
		lumber.NewConsoleLogger(lumber.WARN),
		nil, // markdownRenderer is optional
		nil, // templateExecutor is optional
		frontmatterIndexQueryer,
	)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "mustNewServerWithJobCoordinator failed")
	return server
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
			req                   *apiv1.GetFrontmatterRequest
			res                   *apiv1.GetFrontmatterResponse
			err                   error
			mockPageReaderMutator *MockPageReaderMutator
		)

		BeforeEach(func() {
			req = &apiv1.GetFrontmatterRequest{
				Page: "test-page",
			}
			mockPageReaderMutator = &MockPageReaderMutator{}
		})

		JustBeforeEach(func() {
			server = mustNewServer(mockPageReaderMutator, nil, nil)
			res, err = server.GetFrontmatter(ctx, req)
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
			var expectedStruct *structpb.Struct

			BeforeEach(func() {
				expectedFm = map[string]any{
					"title": "Test Page",
					"tags":  []any{"test", "ginkgo"},
				}
				mockPageReaderMutator.Frontmatter = expectedFm
				
				var structErr error
				expectedStruct, structErr = structpb.NewStruct(expectedFm)
				Expect(structErr).NotTo(HaveOccurred())
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the page's frontmatter", func() {
				Expect(res).NotTo(BeNil())
				Expect(res.Frontmatter).To(Equal(expectedStruct))
			})
		})

		When("the requested page has frontmatter with identifier key", func() {
			var frontmatterWithIdentifier map[string]any
			var expectedFilteredFm map[string]any
			var expectedStruct *structpb.Struct

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
				
				var structErr error
				expectedStruct, structErr = structpb.NewStruct(expectedFilteredFm)
				Expect(structErr).NotTo(HaveOccurred())
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the frontmatter without the identifier key", func() {
				Expect(res).NotTo(BeNil())
				Expect(res.Frontmatter).To(Equal(expectedStruct))
			})
		})

		When("the requested page has frontmatter with nested identifier keys", func() {
			var frontmatterWithNestedIdentifier map[string]any
			var expectedStruct *structpb.Struct

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
				
				// Nested identifier keys should be preserved, only root-level filtered
				var structErr error
				expectedStruct, structErr = structpb.NewStruct(frontmatterWithNestedIdentifier)
				Expect(structErr).NotTo(HaveOccurred())
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the frontmatter with nested identifier keys preserved", func() {
				Expect(res).NotTo(BeNil())
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
			server = mustNewServer(mockPageReaderMutator, nil, nil)
			resp, err = server.MergeFrontmatter(ctx, req)
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
			server = mustNewServer(mockPageReaderMutator, nil, nil)
			resp, err = server.ReplaceFrontmatter(ctx, req)
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
			server = mustNewServer(mockPageReaderMutator, nil, nil)
			resp, err = server.RemoveKeyAtPath(ctx, req)
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

			server = mustNewServer(nil, nil, nil)
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
				// Create a server with nil logger - mustNewServer provides default mocks for required deps
				// but we need to verify nil logger handling, so we create manually with minimal deps
				var serverErr error
				server, serverErr = v1.NewServer(
					"test-commit",
					time.Now(),
					noOpPageReaderMutator{},
					noOpBleveIndexQueryer{},
					nil, // jobProgressProvider
					nil, // logger is nil - this is what we're testing
					nil, // markdownRenderer
					nil, // templateExecutor
					noOpFrontmatterIndexQueryer{},
				)
				Expect(serverErr).NotTo(HaveOccurred())

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
			server = mustNewServer(mockPageReaderMutator, nil, nil)
			resp, err = server.DeletePage(ctx, req)
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
			server = mustNewServer(nil, nil, nil)
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
			server = mustNewServer(nil, nil, nil)
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
			req                         *apiv1.SearchContentRequest
			resp                        *apiv1.SearchContentResponse
			err                         error
			mockBleveIndexQueryer       *MockBleveIndexQueryer
			mockFrontmatterIndexQueryer *FlexibleMockFrontmatterIndexQueryer
		)

		BeforeEach(func() {
			req = &apiv1.SearchContentRequest{
				Query: "test query",
			}
			mockBleveIndexQueryer = &MockBleveIndexQueryer{}
			mockFrontmatterIndexQueryer = &FlexibleMockFrontmatterIndexQueryer{
				ExactMatchResults: make(map[string][]string),
				GetValueResults:   make(map[string]map[string]string),
			}
		})

		JustBeforeEach(func() {
			server = mustNewServer(nil, mockBleveIndexQueryer, mockFrontmatterIndexQueryer)
			resp, err = server.SearchContent(ctx, req)
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

			It("should return zero for total unfiltered count when no filters are applied", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.TotalUnfilteredCount).To(Equal(int32(0))) // No filters, so no warning needed
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
				server = mustNewServer(nil, mockBleveIndexQueryer, mockFrontmatterIndexQueryer)
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

			It("should return the total unfiltered count", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.TotalUnfilteredCount).To(Equal(int32(3))) // 3 total results before filtering
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

				It("should still return the total unfiltered count", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(resp).NotTo(BeNil())
					Expect(resp.TotalUnfilteredCount).To(Equal(int32(3))) // 3 total results before filtering
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
				server = mustNewServer(nil, mockBleveIndexQueryer, mockFrontmatterIndexQueryer)
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
				server = mustNewServer(nil, mockBleveIndexQueryer, mockFrontmatterIndexQueryer)
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
				server = mustNewServer(nil, mockBleveIndexQueryer, mockFrontmatterIndexQueryer)
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
				server = mustNewServer(nil, mockBleveIndexQueryer, mockFrontmatterIndexQueryer)
				resp, err = server.SearchContent(ctx, req)
			})

			It("should return only results without the excluded key", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].Identifier).To(Equal("item-one"))
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
				server = mustNewServer(nil, mockBleveIndexQueryer, mockFrontmatterIndexQueryer)
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
				server = mustNewServer(nil, mockBleveIndexQueryer, mockFrontmatterIndexQueryer)
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

		When("frontmatter_keys_to_return_in_results is specified", func() {
			var (
				mockFrontmatterIndexQueryer *FlexibleMockFrontmatterIndexQueryer
				searchResults               []bleve.SearchResult
			)

			BeforeEach(func() {
				mockFrontmatterIndexQueryer = &FlexibleMockFrontmatterIndexQueryer{
					ExactMatchResults: make(map[string][]string),
					GetValueResults:   make(map[string]map[string]string),
				}
				// Search returns a page with various frontmatter fields
				searchResults = []bleve.SearchResult{
					{Identifier: "test_page", Title: "Test Page", Fragment: "Test content"},
				}
				mockBleveIndexQueryer.Results = searchResults
				// Page has multiple frontmatter fields
				mockFrontmatterIndexQueryer.GetValueResults["test_page"] = map[string]string{
					"author":      "John Doe",
					"category":    "Technology",
					"tags":        "golang,testing",
					"draft":       "false",
				}
				// Request specific frontmatter keys to return
				req = &apiv1.SearchContentRequest{
					Query:                             "test",
					FrontmatterKeysToReturnInResults: []string{"author", "category", "missing_key"},
				}
			})

			JustBeforeEach(func() {
				server = mustNewServer(nil, mockBleveIndexQueryer, mockFrontmatterIndexQueryer)
				resp, err = server.SearchContent(ctx, req)
			})

			It("should return requested frontmatter keys in results", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].Frontmatter).NotTo(BeNil())
				Expect(resp.Results[0].Frontmatter).To(HaveKey("author"))
				Expect(resp.Results[0].Frontmatter["author"]).To(Equal("John Doe"))
				Expect(resp.Results[0].Frontmatter).To(HaveKey("category"))
				Expect(resp.Results[0].Frontmatter["category"]).To(Equal("Technology"))
			})

			It("should not include frontmatter keys not requested", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].Frontmatter).NotTo(HaveKey("tags"))
				Expect(resp.Results[0].Frontmatter).NotTo(HaveKey("draft"))
			})

			It("should not include missing keys in frontmatter map", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].Frontmatter).NotTo(HaveKey("missing_key"))
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
				server = mustNewServer(nil, mockBleveIndexQueryer, mockFrontmatterIndexQueryer)
				resp, err = server.SearchContent(ctx, req)
			})

			It("should include inventory context with container ID", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].InventoryContext).NotTo(BeNil())
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

			When("item has circular reference in container chain", func() {
				BeforeEach(func() {
					// Create circular reference: A -> B -> C -> A
					mockFrontmatterIndexQueryer.GetValueResults["screwdriver"] = map[string]string{
						"inventory.container": "container_a",
					}
					mockFrontmatterIndexQueryer.GetValueResults["container_a"] = map[string]string{
						"title":               "Container A",
						"inventory.container": "container_b",
					}
					mockFrontmatterIndexQueryer.GetValueResults["container_b"] = map[string]string{
						"title":               "Container B",
						"inventory.container": "container_c",
					}
					mockFrontmatterIndexQueryer.GetValueResults["container_c"] = map[string]string{
						"title":               "Container C",
						"inventory.container": "container_a", // Circular!
					}
				})

				It("should detect circular reference and stop building path", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(resp).NotTo(BeNil())
					Expect(resp.Results).To(HaveLen(1))
					Expect(resp.Results[0].InventoryContext).NotTo(BeNil())
					
					// Should have stopped when it detected the circular reference
					// Path built from immediate container to root, so order is: container_c, container_b, container_a
					Expect(resp.Results[0].InventoryContext.Path).To(HaveLen(3))
					Expect(resp.Results[0].InventoryContext.Path[0].Identifier).To(Equal("container_c"))
					Expect(resp.Results[0].InventoryContext.Path[1].Identifier).To(Equal("container_b"))
					Expect(resp.Results[0].InventoryContext.Path[2].Identifier).To(Equal("container_a"))
				})
			})

			When("item has container chain exceeding max depth", func() {
				BeforeEach(func() {
					// Create a chain of 25 containers (exceeds maxDepth of 20)
					mockFrontmatterIndexQueryer.GetValueResults["screwdriver"] = map[string]string{
						"inventory.container": "container_0",
					}
					for i := 0; i < 25; i++ {
						containerID := fmt.Sprintf("container_%d", i)
						nextID := fmt.Sprintf("container_%d", i+1)
						mockFrontmatterIndexQueryer.GetValueResults[containerID] = map[string]string{
							"title":               fmt.Sprintf("Container %d", i),
							"inventory.container": nextID,
						}
					}
					// Last container has no parent
					mockFrontmatterIndexQueryer.GetValueResults["container_25"] = map[string]string{
						"title": "Container 25",
					}
				})

				It("should stop at max depth of 20", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(resp).NotTo(BeNil())
					Expect(resp.Results).To(HaveLen(1))
					Expect(resp.Results[0].InventoryContext).NotTo(BeNil())
					
					// Should have stopped at maxDepth (20)
					Expect(resp.Results[0].InventoryContext.Path).To(HaveLen(20))
					
					// Verify depth values are correct (0 to 19)
					for i := 0; i < 20; i++ {
						Expect(resp.Results[0].InventoryContext.Path[i].Depth).To(Equal(int32(i)))
					}
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
				server = mustNewServer(nil, mockBleveIndexQueryer, mockFrontmatterIndexQueryer)
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
			var serverErr error
			server, serverErr = v1.NewServer(
				"commit",
				time.Now(),
				mockPageReaderMutator,
				noOpBleveIndexQueryer{},
				nil,
				lumber.NewConsoleLogger(lumber.WARN),
				mockMarkdownRenderer,
				mockTemplateExecutor,
				mockFrontmatterIndexQueryer,
			)
			Expect(serverErr).NotTo(HaveOccurred())
			resp, err = server.ReadPage(ctx, req)
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

	Describe("ParseCSVPreview", func() {
		var (
			req                   *apiv1.ParseCSVPreviewRequest
			resp                  *apiv1.ParseCSVPreviewResponse
			err                   error
			mockPageReaderMutator *MockPageReaderMutator
		)

		BeforeEach(func() {
			mockPageReaderMutator = &MockPageReaderMutator{
				Err: os.ErrNotExist, // By default, pages don't exist
			}
			req = &apiv1.ParseCSVPreviewRequest{}
		})

		JustBeforeEach(func() {
			server = mustNewServer(mockPageReaderMutator, nil, nil)
			resp, err = server.ParseCSVPreview(ctx, req)
		})

		When("csv_content is empty", func() {
			BeforeEach(func() {
				req.CsvContent = ""
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "csv_content cannot be empty"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("csv_content is invalid CSV", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier\n\"unclosed quote"
			})

			It("should return an invalid argument error with parsing details", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "failed to parse CSV"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("csv_content has no identifier column", func() {
			BeforeEach(func() {
				req.CsvContent = "title,description\nTest,A test page"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return parsing errors in the response", func() {
				Expect(resp).NotTo(BeNil())
				Expect(resp.ParsingErrors).To(ContainElement("CSV must have 'identifier' column"))
			})
		})

		When("csv_content has no data rows", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return parsing errors in the response", func() {
				Expect(resp).NotTo(BeNil())
				Expect(resp.ParsingErrors).To(ContainElement("CSV has no data rows"))
			})
		})

		When("csv_content has valid rows for new pages", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title,description\ntest_page,Test Page,A test description\nanother_page,Another Page,Another description"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the correct total record count", func() {
				Expect(resp.TotalRecords).To(Equal(int32(2)))
			})

			It("should return the correct create count", func() {
				Expect(resp.CreateCount).To(Equal(int32(2)))
			})

			It("should return zero update count", func() {
				Expect(resp.UpdateCount).To(Equal(int32(0)))
			})

			It("should return zero error count", func() {
				Expect(resp.ErrorCount).To(Equal(int32(0)))
			})

			It("should indicate pages do not exist", func() {
				Expect(resp.Records).To(HaveLen(2))
				Expect(resp.Records[0].PageExists).To(BeFalse())
				Expect(resp.Records[1].PageExists).To(BeFalse())
			})

			It("should have correct identifiers", func() {
				Expect(resp.Records[0].Identifier).To(Equal("test_page"))
				Expect(resp.Records[1].Identifier).To(Equal("another_page"))
			})

			It("should have correct row numbers", func() {
				Expect(resp.Records[0].RowNumber).To(Equal(int32(1)))
				Expect(resp.Records[1].RowNumber).To(Equal(int32(2)))
			})

			It("should have parsed frontmatter", func() {
				Expect(resp.Records[0].Frontmatter).NotTo(BeNil())
				Expect(resp.Records[0].Frontmatter.AsMap()["title"]).To(Equal("Test Page"))
			})
		})

		When("csv_content has rows for existing pages", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title\nexisting_page,Updated Title"
				mockPageReaderMutator = &MockPageReaderMutator{
					FrontmatterByID: map[string]map[string]any{
						"existing_page": {"title": "Old Title"},
					},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should indicate page exists", func() {
				Expect(resp.Records).To(HaveLen(1))
				Expect(resp.Records[0].PageExists).To(BeTrue())
			})

			It("should return correct update count", func() {
				Expect(resp.UpdateCount).To(Equal(int32(1)))
			})

			It("should return zero create count", func() {
				Expect(resp.CreateCount).To(Equal(int32(0)))
			})
		})

		When("csv_content has rows with validation errors", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title\n,Missing Identifier\nvalid_page,Valid Title"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return correct error count", func() {
				Expect(resp.ErrorCount).To(Equal(int32(1)))
			})

			It("should have validation errors on the invalid record", func() {
				Expect(resp.Records[0].ValidationErrors).NotTo(BeEmpty())
			})

			It("should still process valid records", func() {
				Expect(resp.Records[1].ValidationErrors).To(BeEmpty())
				Expect(resp.Records[1].Identifier).To(Equal("valid_page"))
			})
		})

		When("csv_content has template column", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,template,title\ntest_page,inv_item,Test Item"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should parse the template correctly", func() {
				Expect(resp.Records[0].Template).To(Equal("inv_item"))
			})

			It("should not have validation errors for built-in templates", func() {
				Expect(resp.Records[0].ValidationErrors).To(BeEmpty())
			})
		})

		When("csv_content references a non-existent template", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,template,title\ntest_page,nonexistent_template,Test Item"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should add a validation error for the missing template", func() {
				Expect(resp.Records[0].ValidationErrors).To(ContainElement(ContainSubstring("template 'nonexistent_template' does not exist")))
			})
		})

		When("csv_content has array column notation", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,tags[],tags[]\ntest_page,tag1,tag2"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should parse array operations", func() {
				Expect(resp.Records[0].ArrayOps).To(HaveLen(2))
			})
		})

		When("csv_content has DELETE sentinel for scalar field", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,description\ntest_page,[[DELETE]]"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should track fields to delete", func() {
				Expect(resp.Records[0].FieldsToDelete).To(ContainElement("description"))
			})
		})

		When("csv_content has nested field paths", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,inventory.container\ntest_page,toolbox"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create nested frontmatter structure", func() {
				fm := resp.Records[0].Frontmatter.AsMap()
				inventory, ok := fm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(inventory["container"]).To(Equal("toolbox"))
			})
		})

		When("csv_content has mixed existing and new pages", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title\nexisting_page,Updated Title\nnew_page,New Page"
				mockPageReaderMutator = &MockPageReaderMutator{
					FrontmatterByID: map[string]map[string]any{
						"existing_page": {"title": "Old Title"},
					},
				}
			})

			It("should return correct counts", func() {
				Expect(resp.TotalRecords).To(Equal(int32(2)))
				Expect(resp.UpdateCount).To(Equal(int32(1)))
				Expect(resp.CreateCount).To(Equal(int32(1)))
				Expect(resp.ErrorCount).To(Equal(int32(0)))
			})
		})

		When("csv_content has duplicate identifiers", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title\ntest_page,First Title\ntest_page,Second Title"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should flag duplicate identifiers as validation errors", func() {
				Expect(resp.Records[1].ValidationErrors).To(ContainElement(ContainSubstring("duplicate identifier")))
			})

			It("should count the duplicate as an error", func() {
				Expect(resp.ErrorCount).To(Equal(int32(1)))
			})
		})

		When("csv_content has invalid identifier format", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title\nInvalid-Identifier,Test"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should flag invalid identifiers", func() {
				Expect(resp.Records[0].ValidationErrors).NotTo(BeEmpty())
			})
		})
	})

	Describe("StartPageImportJob", func() {
		var (
			req                   *apiv1.StartPageImportJobRequest
			resp                  *apiv1.StartPageImportJobResponse
			err                   error
			mockPageReaderMutator *MockPageReaderMutator
			mockJobCoordinator    *MockJobQueueCoordinator
		)

		BeforeEach(func() {
			mockPageReaderMutator = &MockPageReaderMutator{
				Err: os.ErrNotExist, // By default, pages don't exist
			}
			mockJobCoordinator = &MockJobQueueCoordinator{}
			req = &apiv1.StartPageImportJobRequest{}
		})

		JustBeforeEach(func() {
			var coordinator jobs.JobCoordinator
			if mockJobCoordinator != nil {
				coordinator = mockJobCoordinator.AsCoordinator()
			}
			server = mustNewServerWithJobCoordinator(mockPageReaderMutator, nil, nil, coordinator)
			resp, err = server.StartPageImportJob(ctx, req)
		})

		When("csv_content is empty", func() {
			BeforeEach(func() {
				req.CsvContent = ""
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "csv_content cannot be empty"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("job queue coordinator is nil", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title\ntest_page,Test"
				mockJobCoordinator = nil
			})

			It("should return an unavailable error", func() {
				Expect(err).To(HaveGrpcStatus(codes.Unavailable, "job queue coordinator not available"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("csv_content has parsing errors", func() {
			BeforeEach(func() {
				req.CsvContent = "title,description\nTest,A test page"
			})

			It("should return an invalid argument error with parsing error details", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "CSV has parsing errors"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("csv_content has no valid records", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title\n,Missing Identifier\n,Also Missing"
			})

			It("should not return an error", func() {
				// Invalid records are now processed individually and will fail,
				// rather than rejecting the entire batch upfront
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return success with total record count", func() {
				// All records are counted, including invalid ones
				// Invalid records will be processed and recorded as failures
				Expect(resp.Success).To(BeTrue())
				Expect(resp.RecordCount).To(Equal(int32(2)))
			})
		})

		When("csv_content has valid records", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title\ntest_page,Test Page\nanother_page,Another Page"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return success", func() {
				Expect(resp.Success).To(BeTrue())
			})

			It("should return the correct record count", func() {
				Expect(resp.RecordCount).To(Equal(int32(2)))
			})

			It("should return a job ID", func() {
				Expect(resp.JobId).NotTo(BeEmpty())
			})
		})

		When("csv_content has mixed valid and invalid records", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title\nvalid_page,Valid Page\n,Invalid Page\nanother_valid,Another Valid"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return success", func() {
				Expect(resp.Success).To(BeTrue())
			})

			It("should count all records including invalid ones", func() {
				// All records are now processed as individual jobs
				// Invalid records will fail individually and be reported
				Expect(resp.RecordCount).To(Equal(int32(3)))
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
			var serverErr error
			server, serverErr = v1.NewServer(
				"commit",
				time.Now(),
				mockPageReaderMutator,
				noOpBleveIndexQueryer{},
				nil,
				lumber.NewConsoleLogger(lumber.WARN),
				nil,
				nil,
				noOpFrontmatterIndexQueryer{},
			)
			Expect(serverErr).NotTo(HaveOccurred())
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

// MockJobQueueCoordinator is a mock implementation for testing job queue interactions.
type MockJobQueueCoordinator struct{}

// AsCoordinator returns a real JobQueueCoordinator for testing.
// This is needed because the server expects a *jobs.JobQueueCoordinator, not an interface.
func (m *MockJobQueueCoordinator) AsCoordinator() *jobs.JobQueueCoordinator {
	if m == nil {
		return nil
	}
	// Create a coordinator with a no-op dispatcher for testing
	mockDispatcherFactory := func(maxWorkers, maxQueue int) jobs.Dispatcher {
		return &noOpDispatcher{}
	}
	return jobs.NewJobQueueCoordinatorWithFactory(lumber.NewConsoleLogger(lumber.WARN), mockDispatcherFactory)
}

// noOpDispatcher is a dispatcher that does nothing for testing purposes.
type noOpDispatcher struct{}

func (*noOpDispatcher) Start() {}

func (*noOpDispatcher) Dispatch(_ func()) error {
	return nil
}
