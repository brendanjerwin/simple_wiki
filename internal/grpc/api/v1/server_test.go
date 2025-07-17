//revive:disable:dot-imports
package v1_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/brendanjerwin/simple_wiki/common"
	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	v1 "github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
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
			server = v1.NewServer("v0.0.0", "commit", time.Now(), mockPageReadWriter, lumber.NewConsoleLogger(lumber.WARN))
			res, err = server.GetFrontmatter(ctx, req)
		})

		When("the PageReadWriter is not configured", func() {
			BeforeEach(func() {
				mockPageReadWriter = nil
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.Internal, "PageReadWriter not available"))
				Expect(res).To(BeNil())
			})
		})

		When("the requested page does not exist", func() {
			BeforeEach(func() {
				mockPageReadWriter.Err = os.ErrNotExist
			})

			It("should return a not found error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.NotFound, "page not found: test-page"))
				Expect(res).To(BeNil())
			})
		})

		When("PageReadWriter returns a generic error", func() {
			var genericError error
			BeforeEach(func() {
				genericError = errors.New("kaboom")
				mockPageReadWriter.Err = genericError
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

	Describe("MergeFrontmatter", func() {
		var (
			req                *apiv1.MergeFrontmatterRequest
			resp               *apiv1.MergeFrontmatterResponse
			err                error
			mockPageReadWriter *MockPageReadWriter
			pageName           string
		)

		BeforeEach(func() {
			pageName = "test-page"
			mockPageReadWriter = &MockPageReadWriter{}
			req = &apiv1.MergeFrontmatterRequest{
				Page: pageName,
			}
		})

		JustBeforeEach(func() {
			server = v1.NewServer("v0.0.0", "commit", time.Now(), mockPageReadWriter, lumber.NewConsoleLogger(lumber.WARN))
			resp, err = server.MergeFrontmatter(ctx, req)
		})

		When("the PageReadWriter is not configured", func() {
			BeforeEach(func() {
				mockPageReadWriter = nil
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.Internal, "PageReadWriter not available"))
				Expect(resp).To(BeNil())
			})
		})

		When("reading the frontmatter fails with a generic error", func() {
			BeforeEach(func() {
				mockPageReadWriter.Err = errors.New("read error")
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to read frontmatter"))
				Expect(resp).To(BeNil())
			})
		})

		When("writing the frontmatter fails", func() {
			BeforeEach(func() {
				mockPageReadWriter.WriteErr = errors.New("write error")
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to write frontmatter"))
				Expect(resp).To(BeNil())
			})
		})

		When("the page does not exist", func() {
			var newFrontmatterPb *structpb.Struct
			var newFrontmatter common.FrontMatter

			BeforeEach(func() {
				mockPageReadWriter.Err = os.ErrNotExist

				newFrontmatter = common.FrontMatter{"title": "New Title"}
				var err error
				newFrontmatterPb, err = structpb.NewStruct(newFrontmatter)
				Expect(err).NotTo(HaveOccurred())
				req.Frontmatter = newFrontmatterPb
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write the new frontmatter", func() {
				Expect(mockPageReadWriter.WrittenIdentifier).To(Equal(common.PageIdentifier(pageName)))
				Expect(mockPageReadWriter.WrittenFrontmatter).To(Equal(newFrontmatter))
			})

			It("should return the new frontmatter", func() {
				Expect(resp.Frontmatter).To(Equal(newFrontmatterPb))
			})
		})

		When("the page exists", func() {
			var existingFrontmatter common.FrontMatter
			var newFrontmatter common.FrontMatter
			var mergedFrontmatter common.FrontMatter

			BeforeEach(func() {
				existingFrontmatter = common.FrontMatter{"title": "Old Title", "tags": []any{"old"}}
				newFrontmatter = common.FrontMatter{"title": "New Title", "author": "test"}

				mergedFrontmatter = common.FrontMatter{
					"title":  "New Title",
					"tags":   []any{"old"},
					"author": "test",
				}

				mockPageReadWriter.Frontmatter = existingFrontmatter
				var err error
				req.Frontmatter, err = structpb.NewStruct(newFrontmatter)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write the merged frontmatter", func() {
				Expect(mockPageReadWriter.WrittenIdentifier).To(Equal(common.PageIdentifier(pageName)))
				Expect(mockPageReadWriter.WrittenFrontmatter).To(Equal(mergedFrontmatter))
			})

			It("should return the merged frontmatter", func() {
				expectedPb, err := structpb.NewStruct(mergedFrontmatter)
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
			server = v1.NewServer("v0.0.0", "commit", time.Now(), mockPageReadWriter, lumber.NewConsoleLogger(lumber.WARN))
			resp, err = server.ReplaceFrontmatter(ctx, req)
		})

		When("the PageReadWriter is not configured", func() {
			BeforeEach(func() {
				mockPageReadWriter = nil
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.Internal, "PageReadWriter not available"))
				Expect(resp).To(BeNil())
			})
		})

		When("writing the frontmatter fails", func() {
			BeforeEach(func() {
				mockPageReadWriter.WriteErr = errors.New("disk full")
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
				Expect(mockPageReadWriter.WrittenIdentifier).To(Equal(common.PageIdentifier(pageName)))
				Expect(mockPageReadWriter.WrittenFrontmatter).To(Equal(newFrontmatter))
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
				Expect(mockPageReadWriter.WrittenIdentifier).To(Equal(common.PageIdentifier(pageName)))
				Expect(mockPageReadWriter.WrittenFrontmatter).To(BeNil())
			})

			It("should return nil frontmatter", func() {
				Expect(resp.Frontmatter).To(BeNil())
			})
		})
	})

	Describe("RemoveKeyAtPath", func() {
		var (
			req                *apiv1.RemoveKeyAtPathRequest
			resp               *apiv1.RemoveKeyAtPathResponse
			err                error
			mockPageReadWriter *MockPageReadWriter
			pageName           string
		)

		BeforeEach(func() {
			pageName = "test-page"
			mockPageReadWriter = &MockPageReadWriter{}
			req = &apiv1.RemoveKeyAtPathRequest{
				Page: pageName,
			}
		})

		JustBeforeEach(func() {
			server = v1.NewServer("v0.0.0", "commit", time.Now(), mockPageReadWriter, lumber.NewConsoleLogger(lumber.WARN))
			resp, err = server.RemoveKeyAtPath(ctx, req)
		})

		When("the PageReadWriter is not configured", func() {
			BeforeEach(func() {
				mockPageReadWriter = nil
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.Internal, "PageReadWriter not available"))
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
				mockPageReadWriter.Err = os.ErrNotExist
				req.KeyPath = []*apiv1.PathComponent{{Component: &apiv1.PathComponent_Key{Key: "a"}}}
			})

			It("should return a not found error and no response", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.NotFound, "page not found"))
				Expect(resp).To(BeNil())
			})
		})

		When("the frontmatter is nil", func() {
			BeforeEach(func() {
				mockPageReadWriter.Frontmatter = nil
				req.KeyPath = []*apiv1.PathComponent{{Component: &apiv1.PathComponent_Key{Key: "a"}}}
			})
			It("should return a not found error for the key and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.NotFound, "key 'a' not found"))
				Expect(resp).To(BeNil())
			})
		})

		When("removing a key successfully", func() {
			var initialFm common.FrontMatter
			BeforeEach(func() {
				initialFm = common.FrontMatter{
					"a": "b",
					"c": map[string]any{
						"d": "e",
					},
					"f": []any{"g", "h", map[string]any{"i": "j"}},
				}
				mockPageReadWriter.Frontmatter = initialFm
			})

			When("from the top-level map", func() {
				var expectedFm common.FrontMatter
				BeforeEach(func() {
					req.KeyPath = []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "a"}},
					}
					expectedFm = common.FrontMatter{
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
					Expect(mockPageReadWriter.WrittenFrontmatter).To(Equal(expectedFm))
				})

				It("should return the correctly modified frontmatter", func() {
					expectedPb, err := structpb.NewStruct(expectedFm)
					Expect(err).NotTo(HaveOccurred())
					Expect(resp.Frontmatter).To(Equal(expectedPb))
				})
			})

			When("from a nested map", func() {
				var expectedFm common.FrontMatter
				BeforeEach(func() {
					req.KeyPath = []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "c"}},
						{Component: &apiv1.PathComponent_Key{Key: "d"}},
					}
					expectedFm = common.FrontMatter{
						"a": "b",
						"c": map[string]any{},
						"f": []any{"g", "h", map[string]any{"i": "j"}},
					}
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should write the correctly modified frontmatter", func() {
					Expect(mockPageReadWriter.WrittenFrontmatter).To(Equal(expectedFm))
				})

				It("should return the correctly modified frontmatter", func() {
					expectedPb, err := structpb.NewStruct(expectedFm)
					Expect(err).NotTo(HaveOccurred())
					Expect(resp.Frontmatter).To(Equal(expectedPb))
				})
			})

			When("from a slice", func() {
				var expectedFm common.FrontMatter
				BeforeEach(func() {
					req.KeyPath = []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "f"}},
						{Component: &apiv1.PathComponent_Index{Index: 1}},
					}
					expectedFm = common.FrontMatter{
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
					Expect(mockPageReadWriter.WrittenFrontmatter).To(Equal(expectedFm))
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
				mockPageReadWriter.Frontmatter = common.FrontMatter{
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
	})

	Describe("LoggingInterceptor", func() {
		var (
			server  *v1.Server
			logger  *lumber.ConsoleLogger
			ctx     context.Context
			req     interface{}
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

			server = v1.NewServer("test-version", "test-commit", time.Now(), nil, logger)
		})

		When("a successful gRPC call is made", func() {
			var (
				resp interface{}
				err  error
			)

			BeforeEach(func() {
				handler = func(ctx context.Context, req interface{}) (interface{}, error) {
					time.Sleep(10 * time.Millisecond) // Simulate some work
					return &apiv1.GetVersionResponse{Version: "test"}, nil
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
				resp interface{}
				err  error
			)

			BeforeEach(func() {
				handler = func(ctx context.Context, req interface{}) (interface{}, error) {
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
			var (
				err error
			)

			BeforeEach(func() {
				handler = func(ctx context.Context, req interface{}) (interface{}, error) {
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
				resp interface{}
				err  error
			)

			BeforeEach(func() {
				server = v1.NewServer("test-version", "test-commit", time.Now(), nil, nil)

				handler = func(ctx context.Context, req interface{}) (interface{}, error) {
					return &apiv1.GetVersionResponse{Version: "test"}, nil
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
})
