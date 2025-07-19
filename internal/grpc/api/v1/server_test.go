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

// MockPageReadWriter is a mock implementation of wikipage.PageReadWriter for testing.
type MockPageReadWriter struct {
	Frontmatter        wikipage.FrontMatter
	Markdown           wikipage.Markdown
	Err                error
	WrittenFrontmatter wikipage.FrontMatter
	WrittenMarkdown    wikipage.Markdown
	WrittenIdentifier  wikipage.PageIdentifier
	WriteErr           error
}

func (m *MockPageReadWriter) ReadFrontMatter(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	if m.Err != nil {
		return "", nil, m.Err
	}
	return identifier, m.Frontmatter, nil
}

func (m *MockPageReadWriter) WriteFrontMatter(identifier wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	m.WrittenIdentifier = identifier
	m.WrittenFrontmatter = fm
	return m.WriteErr
}

func (m *MockPageReadWriter) WriteMarkdown(identifier wikipage.PageIdentifier, md wikipage.Markdown) error {
	m.WrittenIdentifier = identifier
	m.WrittenMarkdown = md
	return m.WriteErr
}

func (m *MockPageReadWriter) ReadMarkdown(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
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
			server = v1.NewServer("commit", time.Now(), mockPageReadWriter, lumber.NewConsoleLogger(lumber.WARN))
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

		When("the page has complex nested frontmatter with arrays", func() {
			var complexFm map[string]any

			BeforeEach(func() {
				complexFm = map[string]any{
					"identifier": "inventory_item",
					"title":      "Inventory Item",
					"rename_this_section": map[string]any{
						"total": "32",
					},
					"inventory": map[string]any{
						"container": "lab_small_parts",
						"items": []any{
							"AKG Wired Earbuds",
							"Steel Series Arctis 5 Headphone 3.5mm Adapter Cable",
							"Steel Series Arctis 5 Headphone USB Dongle",
							"Male 3.5mm to Male 3.5mm Coiled Cable",
							"Random Earbud Tips",
							"3.5mm to RCA Cable",
							"Male 3.5mm to Male 3.5mm Cable",
						},
					},
				}
				mockPageReadWriter.Frontmatter = complexFm
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the complex frontmatter structure correctly", func() {
				Expect(res).NotTo(BeNil())
				expectedStruct, err := structpb.NewStruct(complexFm)
				Expect(err).NotTo(HaveOccurred())
				Expect(res.Frontmatter).To(Equal(expectedStruct))
			})

			It("should correctly handle nested object and array data", func() {
				Expect(res).NotTo(BeNil())
				
				// Verify the structure can be converted back to a map
				resultMap := res.Frontmatter.AsMap()
				
				// Check top-level fields
				Expect(resultMap["identifier"]).To(Equal("inventory_item"))
				Expect(resultMap["title"]).To(Equal("Inventory Item"))
				
				// Check nested section
				renameSection, ok := resultMap["rename_this_section"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(renameSection["total"]).To(Equal("32"))
				
				// Check nested inventory section with array
				inventory, ok := resultMap["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(inventory["container"]).To(Equal("lab_small_parts"))
				
				items, ok := inventory["items"].([]any)
				Expect(ok).To(BeTrue())
				Expect(items).To(HaveLen(7))
				Expect(items[0]).To(Equal("AKG Wired Earbuds"))
				Expect(items[6]).To(Equal("Male 3.5mm to Male 3.5mm Cable"))
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
			server = v1.NewServer("commit", time.Now(), mockPageReadWriter, lumber.NewConsoleLogger(lumber.WARN))
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
			var newFrontmatter wikipage.FrontMatter

			BeforeEach(func() {
				mockPageReadWriter.Err = os.ErrNotExist

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
				Expect(mockPageReadWriter.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReadWriter.WrittenFrontmatter).To(Equal(newFrontmatter))
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

				mockPageReadWriter.Frontmatter = existingFrontmatter
				var err error
				req.Frontmatter, err = structpb.NewStruct(newFrontmatter)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write the merged frontmatter", func() {
				Expect(mockPageReadWriter.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReadWriter.WrittenFrontmatter).To(Equal(mergedFrontmatter))
			})

			It("should return the merged frontmatter", func() {
				expectedPb, err := structpb.NewStruct(mergedFrontmatter)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Frontmatter).To(Equal(expectedPb))
			})
		})

		When("merging complex frontmatter with arrays and nested objects", func() {
			var initialFm, mergeFm, expectedFm wikipage.FrontMatter

			BeforeEach(func() {
				// Initial complex frontmatter structure
				initialFm = map[string]any{
					"identifier": "inventory_item",
					"title":      "Original Item",
					"inventory": map[string]any{
						"container": "lab_small_parts",
						"items": []any{
							"Original Item 1",
							"Original Item 2",
						},
					},
				}
				mockPageReadWriter.Frontmatter = initialFm

				// Merge data with array and nested changes
				mergeFm = map[string]any{
					"title": "Updated Inventory Item",
					"inventory": map[string]any{
						"location": "main_lab",
						"items": []any{
							"Updated Item 1",
							"Updated Item 2",
							"New Item 3",
						},
					},
					"new_section": map[string]any{
						"total": "42",
					},
				}

				// Expected result after merge (maps.Copy replaces nested objects)
				expectedFm = map[string]any{
					"identifier": "inventory_item",
					"title":      "Updated Inventory Item",
					"inventory": map[string]any{
						"location": "main_lab",
						"items": []any{
							"Updated Item 1",
							"Updated Item 2",
							"New Item 3",
						},
					},
					"new_section": map[string]any{
						"total": "42",
					},
				}

				mergeFmPb, err := structpb.NewStruct(mergeFm)
				Expect(err).NotTo(HaveOccurred())
				req.Frontmatter = mergeFmPb
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write the correctly merged complex frontmatter", func() {
				Expect(mockPageReadWriter.WrittenFrontmatter).To(Equal(expectedFm))
			})

			It("should return the correctly merged complex frontmatter", func() {
				expectedPb, err := structpb.NewStruct(expectedFm)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Frontmatter).To(Equal(expectedPb))
			})

			It("should preserve arrays and nested structures correctly", func() {
				resultMap := resp.Frontmatter.AsMap()
				
				// Check that arrays are properly handled
				inventory, ok := resultMap["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				
				items, ok := inventory["items"].([]any)
				Expect(ok).To(BeTrue())
				Expect(items).To(HaveLen(3))
				Expect(items[2]).To(Equal("New Item 3"))
				
				// Check that new nested sections are added
				newSection, ok := resultMap["new_section"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(newSection["total"]).To(Equal("42"))
				
				// Note: maps.Copy replaces the entire inventory object, so container is not preserved
				Expect(inventory["container"]).To(BeNil())
				Expect(inventory["location"]).To(Equal("main_lab"))
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
			newFrontmatter     wikipage.FrontMatter
			newFrontmatterPb   *structpb.Struct
		)

		BeforeEach(func() {
			pageName = "test-page"
			newFrontmatter = wikipage.FrontMatter{"title": "New Title", "tags": []any{"a", "b"}}
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
			server = v1.NewServer("commit", time.Now(), mockPageReadWriter, lumber.NewConsoleLogger(lumber.WARN))
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
				Expect(mockPageReadWriter.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
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
				Expect(mockPageReadWriter.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReadWriter.WrittenFrontmatter).To(BeNil())
			})

			It("should return nil frontmatter", func() {
				Expect(resp.Frontmatter).To(BeNil())
			})
		})

		When("replacing with complex frontmatter containing arrays and nested objects", func() {
			var complexFm wikipage.FrontMatter
			var complexFmPb *structpb.Struct

			BeforeEach(func() {
				// Set up complex frontmatter structure to replace with
				complexFm = map[string]any{
					"identifier": "inventory_item",
					"title":      "Complex Inventory Item",
					"rename_this_section": map[string]any{
						"total": "32",
					},
					"inventory": map[string]any{
						"container": "lab_small_parts",
						"items": []any{
							"AKG Wired Earbuds",
							"Steel Series Arctis 5 Headphone 3.5mm Adapter Cable",
							"Steel Series Arctis 5 Headphone USB Dongle",
							"Male 3.5mm to Male 3.5mm Coiled Cable",
							"Random Earbud Tips",
							"3.5mm to RCA Cable",
							"Male 3.5mm to Male 3.5mm Cable",
						},
					},
				}

				var err error
				complexFmPb, err = structpb.NewStruct(complexFm)
				Expect(err).NotTo(HaveOccurred())

				req.Frontmatter = complexFmPb
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write the complex frontmatter structure correctly", func() {
				Expect(mockPageReadWriter.WrittenFrontmatter).To(Equal(complexFm))
			})

			It("should return the complex frontmatter structure correctly", func() {
				Expect(resp).NotTo(BeNil())
				Expect(resp.Frontmatter).To(Equal(complexFmPb))
			})

			Describe("when verifying complex frontmatter structure handling", func() {
				var resultMap map[string]any

				BeforeEach(func() {
					// Verify the structure can be converted back to a map
					resultMap = resp.Frontmatter.AsMap()
				})

				It("should provide a valid response", func() {
					Expect(resp).NotTo(BeNil())
				})

				Describe("when checking top-level fields", func() {
					It("should preserve the identifier field", func() {
						Expect(resultMap["identifier"]).To(Equal("inventory_item"))
					})

					It("should preserve the title field", func() {
						Expect(resultMap["title"]).To(Equal("Complex Inventory Item"))
					})
				})

				Describe("when checking nested section handling", func() {
					var renameSection map[string]any
					var sectionExists bool

					BeforeEach(func() {
						renameSection, sectionExists = resultMap["rename_this_section"].(map[string]any)
					})

					It("should preserve the nested section", func() {
						Expect(sectionExists).To(BeTrue())
					})

					It("should preserve nested section fields", func() {
						Expect(renameSection["total"]).To(Equal("32"))
					})
				})

				Describe("when checking inventory section with arrays", func() {
					var inventory map[string]any
					var inventoryExists bool

					BeforeEach(func() {
						inventory, inventoryExists = resultMap["inventory"].(map[string]any)
					})

					It("should preserve the inventory section", func() {
						Expect(inventoryExists).To(BeTrue())
					})

					It("should preserve inventory container field", func() {
						Expect(inventory["container"]).To(Equal("lab_small_parts"))
					})

					Describe("when checking array handling", func() {
						var items []any
						var itemsExists bool

						BeforeEach(func() {
							items, itemsExists = inventory["items"].([]any)
						})

						It("should preserve the items array", func() {
							Expect(itemsExists).To(BeTrue())
						})

						It("should preserve array length", func() {
							Expect(items).To(HaveLen(7))
						})

						It("should preserve first array item", func() {
							Expect(items[0]).To(Equal("AKG Wired Earbuds"))
						})

						It("should preserve last array item", func() {
							Expect(items[6]).To(Equal("Male 3.5mm to Male 3.5mm Cable"))
						})
					})
				})
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
			server = v1.NewServer("commit", time.Now(), mockPageReadWriter, lumber.NewConsoleLogger(lumber.WARN))
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
			var initialFm wikipage.FrontMatter
			BeforeEach(func() {
				initialFm = wikipage.FrontMatter{
					"a": "b",
					"c": map[string]any{
						"d": "e",
					},
					"f": []any{"g", "h", map[string]any{"i": "j"}},
				}
				mockPageReadWriter.Frontmatter = initialFm
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
					Expect(mockPageReadWriter.WrittenFrontmatter).To(Equal(expectedFm))
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
					Expect(mockPageReadWriter.WrittenFrontmatter).To(Equal(expectedFm))
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
				mockPageReadWriter.Frontmatter = wikipage.FrontMatter{
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
})
