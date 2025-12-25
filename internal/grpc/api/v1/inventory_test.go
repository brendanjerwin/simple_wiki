//revive:disable:dot-imports
package v1_test

import (
	"context"
	"errors"
	"os"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
)

var _ = Describe("InventoryManagementService", func() {
	var (
		server *v1.Server
		ctx    context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("CreateInventoryItem", func() {
		var (
			req                      *apiv1.CreateInventoryItemRequest
			resp                     *apiv1.CreateInventoryItemResponse
			err                      error
			mockPageReaderMutator    *MockPageReaderMutator
			mockFrontmatterIndexQueryer *MockFrontmatterIndexQueryer
		)

		BeforeEach(func() {
			req = &apiv1.CreateInventoryItemRequest{
				ItemIdentifier: "test-item",
			}
			mockPageReaderMutator = &MockPageReaderMutator{}
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
				nil,
				nil,
				mockFrontmatterIndexQueryer,
			)
			resp, err = server.CreateInventoryItem(ctx, req)
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

		When("the item_identifier is empty", func() {
			BeforeEach(func() {
				req.ItemIdentifier = ""
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "item_identifier is required"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("the page already exists", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Frontmatter = map[string]any{"title": "Existing Item"}
			})

			It("should not return a gRPC error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return success=false", func() {
				Expect(resp.Success).To(BeFalse())
			})

			It("should return an error message", func() {
				Expect(resp.Error).To(ContainSubstring("already exists"))
			})
		})

		When("creating a new item without container", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = os.ErrNotExist
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return success", func() {
				Expect(resp.Success).To(BeTrue())
			})

			It("should return the munged identifier", func() {
				Expect(resp.ItemIdentifier).To(Equal("test_item"))
			})

			It("should return a summary", func() {
				Expect(resp.Summary).To(ContainSubstring("Created"))
			})

			It("should write the frontmatter", func() {
				Expect(mockPageReaderMutator.WrittenFrontmatter).NotTo(BeNil())
			})

			It("should write the markdown", func() {
				Expect(mockPageReaderMutator.WrittenMarkdown).NotTo(BeEmpty())
			})
		})

		When("creating a new item with a container", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = os.ErrNotExist
				req.Container = "my-drawer"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return success", func() {
				Expect(resp.Success).To(BeTrue())
			})

			It("should include container in the summary", func() {
				Expect(resp.Summary).To(ContainSubstring("my_drawer"))
			})

			It("should set the inventory.container in frontmatter", func() {
				inventory, ok := mockPageReaderMutator.WrittenFrontmatter["inventory"].(map[string]any)
				Expect(ok).To(BeTrue(), "expected inventory to be map[string]any")
				Expect(inventory["container"]).To(Equal("my_drawer"))
			})
		})

		When("creating a new item with a custom title", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = os.ErrNotExist
				req.Title = "My Custom Title"
			})

			It("should use the custom title", func() {
				Expect(mockPageReaderMutator.WrittenFrontmatter["title"]).To(Equal("My Custom Title"))
			})
		})

		When("creating a new item with a description", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = os.ErrNotExist
				req.Description = "A useful phillips head screwdriver"
			})

			It("should write the description to frontmatter", func() {
				Expect(mockPageReaderMutator.WrittenFrontmatter["description"]).To(Equal("A useful phillips head screwdriver"))
			})
		})

		When("creating a new item without a description", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = os.ErrNotExist
				req.Description = ""
			})

			It("should not include description in frontmatter", func() {
				_, hasDescription := mockPageReaderMutator.WrittenFrontmatter["description"]
				Expect(hasDescription).To(BeFalse())
			})
		})
	})

	Describe("MoveInventoryItem", func() {
		var (
			req                      *apiv1.MoveInventoryItemRequest
			resp                     *apiv1.MoveInventoryItemResponse
			err                      error
			mockPageReaderMutator    *MockPageReaderMutator
		)

		BeforeEach(func() {
			req = &apiv1.MoveInventoryItemRequest{
				ItemIdentifier: "test-item",
				NewContainer:   "new-container",
			}
			mockPageReaderMutator = &MockPageReaderMutator{}
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
			resp, err = server.MoveInventoryItem(ctx, req)
		})

		When("the item does not exist", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = os.ErrNotExist
			})

			It("should not return a gRPC error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return success=false", func() {
				Expect(resp.Success).To(BeFalse())
			})

			It("should return an error message", func() {
				Expect(resp.Error).To(ContainSubstring("not found"))
			})
		})

		When("moving an item to a new container", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Frontmatter = map[string]any{
					"title": "Test Item",
					"inventory": map[string]any{
						"container": "old_container",
						"items":     []string{},
					},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return success", func() {
				Expect(resp.Success).To(BeTrue())
			})

			It("should return the previous container", func() {
				Expect(resp.PreviousContainer).To(Equal("old_container"))
			})

			It("should return the new container", func() {
				Expect(resp.NewContainer).To(Equal("new_container"))
			})

			It("should update the frontmatter", func() {
				inventory, ok := mockPageReaderMutator.WrittenFrontmatter["inventory"].(map[string]any)
				Expect(ok).To(BeTrue(), "expected inventory to be map[string]any")
				Expect(inventory["container"]).To(Equal("new_container"))
			})
		})

		When("removing an item from container (making root-level)", func() {
			BeforeEach(func() {
				req.NewContainer = ""
				mockPageReaderMutator.Frontmatter = map[string]any{
					"title": "Test Item",
					"inventory": map[string]any{
						"container": "old_container",
					},
				}
			})

			It("should return success", func() {
				Expect(resp.Success).To(BeTrue())
			})

			It("should remove the container from frontmatter", func() {
				inventory, ok := mockPageReaderMutator.WrittenFrontmatter["inventory"].(map[string]any)
				Expect(ok).To(BeTrue(), "expected inventory to be map[string]any")
				_, hasContainer := inventory["container"]
				Expect(hasContainer).To(BeFalse())
			})
		})

		When("item is already in the target container", func() {
			BeforeEach(func() {
				req.NewContainer = "same_container"
				mockPageReaderMutator.Frontmatter = map[string]any{
					"title": "Test Item",
					"inventory": map[string]any{
						"container": "same_container",
					},
				}
			})

			It("should return success", func() {
				Expect(resp.Success).To(BeTrue())
			})

			It("should indicate item is already there", func() {
				Expect(resp.Summary).To(ContainSubstring("already"))
			})
		})

		When("item is already a root-level item and moving to root", func() {
			BeforeEach(func() {
				req.NewContainer = ""
				mockPageReaderMutator.Frontmatter = map[string]any{
					"title": "Test Item",
					"inventory": map[string]any{},
				}
			})

			It("should indicate item is already root-level", func() {
				Expect(resp.Summary).To(ContainSubstring("root-level"))
			})
		})

		When("the PageReaderMutator is not configured", func() {
			BeforeEach(func() {
				mockPageReaderMutator = nil
			})

			It("should return an internal error", func() {
				Expect(err).To(HaveGrpcStatus(codes.Internal, "PageReaderMutator not available"))
			})
		})

		When("the item_identifier is empty", func() {
			BeforeEach(func() {
				req.ItemIdentifier = ""
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "item_identifier is required"))
			})
		})

		When("reading frontmatter fails with unexpected error", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = errors.New("database connection failed")
			})

			It("should return an internal error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		When("writing frontmatter fails", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Frontmatter = map[string]any{
					"title": "Test Item",
					"inventory": map[string]any{
						"container": "old_container",
					},
				}
				mockPageReaderMutator.WriteErr = errors.New("write failed")
			})

			It("should return an internal error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		When("moving item without existing inventory section", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Frontmatter = map[string]any{
					"title": "Test Item",
				}
			})

			It("should return success", func() {
				Expect(resp.Success).To(BeTrue())
			})

			It("should create inventory section and set container", func() {
				inventory, ok := mockPageReaderMutator.WrittenFrontmatter["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(inventory["container"]).To(Equal("new_container"))
			})
		})

		When("moving root-level item into container", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Frontmatter = map[string]any{
					"title": "Test Item",
					"inventory": map[string]any{},
				}
			})

			It("should return success", func() {
				Expect(resp.Success).To(BeTrue())
			})

			It("should include 'into container' in summary", func() {
				Expect(resp.Summary).To(ContainSubstring("into container"))
			})
		})

		When("moving item between containers with existing items lists", func() {
			BeforeEach(func() {
				mockPageReaderMutator.FrontmatterByID = map[string]map[string]any{
					"test_item": {
						"title": "Test Item",
						"inventory": map[string]any{
							"container": "old_container",
						},
					},
					"old_container": {
						"title": "Old Container",
						"inventory": map[string]any{
							"items": []any{"test_item", "other_item"},
						},
					},
					"new_container": {
						"title": "New Container",
						"inventory": map[string]any{
							"items": []any{"existing_item"},
						},
					},
				}
			})

			It("should remove the item from old container's items list", func() {
				oldContainerFm := mockPageReaderMutator.WrittenFrontmatterByID["old_container"]
				Expect(oldContainerFm).NotTo(BeNil())
				inv, ok := oldContainerFm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				items, ok := inv["items"].([]string)
				Expect(ok).To(BeTrue())
				Expect(items).NotTo(ContainElement("test_item"))
				Expect(items).To(ContainElement("other_item"))
			})

			It("should add the item to new container's items list", func() {
				newContainerFm := mockPageReaderMutator.WrittenFrontmatterByID["new_container"]
				Expect(newContainerFm).NotTo(BeNil())
				inv, ok := newContainerFm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				items, ok := inv["items"].([]string)
				Expect(ok).To(BeTrue())
				Expect(items).To(ContainElement("test_item"))
				Expect(items).To(ContainElement("existing_item"))
			})
		})

		When("moving item to container that doesn't have items list yet", func() {
			BeforeEach(func() {
				mockPageReaderMutator.FrontmatterByID = map[string]map[string]any{
					"test_item": {
						"title": "Test Item",
						"inventory": map[string]any{
							"container": "old_container",
						},
					},
					"old_container": {
						"title": "Old Container",
						"inventory": map[string]any{
							"items": []any{"test_item"},
						},
					},
					"new_container": {
						"title": "New Container",
						"inventory": map[string]any{}, // No items list
					},
				}
			})

			It("should create items list in new container", func() {
				newContainerFm := mockPageReaderMutator.WrittenFrontmatterByID["new_container"]
				Expect(newContainerFm).NotTo(BeNil())
				inv, ok := newContainerFm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				items, ok := inv["items"].([]string)
				Expect(ok).To(BeTrue())
				Expect(items).To(ContainElement("test_item"))
			})
		})

		When("moving item with containers having []string items lists", func() {
			BeforeEach(func() {
				mockPageReaderMutator.FrontmatterByID = map[string]map[string]any{
					"test_item": {
						"title": "Test Item",
						"inventory": map[string]any{
							"container": "old_container",
						},
					},
					"old_container": {
						"title": "Old Container",
						"inventory": map[string]any{
							"items": []string{"test_item", "other_item"},
						},
					},
					"new_container": {
						"title": "New Container",
						"inventory": map[string]any{
							"items": []string{"existing_item"},
						},
					},
				}
			})

			It("should handle []string items list correctly", func() {
				Expect(resp.Success).To(BeTrue())
			})

			It("should remove item from old container's []string list", func() {
				oldContainerFm := mockPageReaderMutator.WrittenFrontmatterByID["old_container"]
				Expect(oldContainerFm).NotTo(BeNil())
				inv, ok := oldContainerFm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				items, ok := inv["items"].([]string)
				Expect(ok).To(BeTrue())
				Expect(items).NotTo(ContainElement("test_item"))
			})

			It("should add item to new container's list", func() {
				newContainerFm := mockPageReaderMutator.WrittenFrontmatterByID["new_container"]
				Expect(newContainerFm).NotTo(BeNil())
				inv, ok := newContainerFm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				items, ok := inv["items"].([]string)
				Expect(ok).To(BeTrue())
				Expect(items).To(ContainElement("test_item"))
			})
		})

		When("moving item and container has no inventory section", func() {
			BeforeEach(func() {
				mockPageReaderMutator.FrontmatterByID = map[string]map[string]any{
					"test_item": {
						"title": "Test Item",
						"inventory": map[string]any{
							"container": "old_container",
						},
					},
					"old_container": {
						"title": "Old Container",
						// No inventory section
					},
					"new_container": {
						"title": "New Container",
						// No inventory section
					},
				}
			})

			It("should succeed", func() {
				Expect(resp.Success).To(BeTrue())
			})

			It("should create inventory section in new container", func() {
				newContainerFm := mockPageReaderMutator.WrittenFrontmatterByID["new_container"]
				Expect(newContainerFm).NotTo(BeNil())
				inv, ok := newContainerFm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				items, ok := inv["items"].([]string)
				Expect(ok).To(BeTrue())
				Expect(items).To(ContainElement("test_item"))
			})
		})

		When("moving item from non-existent old container", func() {
			BeforeEach(func() {
				mockPageReaderMutator.FrontmatterByID = map[string]map[string]any{
					"test_item": {
						"title": "Test Item",
						"inventory": map[string]any{
							"container": "nonexistent_container",
						},
					},
					"new_container": {
						"title": "New Container",
						"inventory": map[string]any{},
					},
				}
			})

			It("should succeed even if old container doesn't exist", func() {
				Expect(resp.Success).To(BeTrue())
			})
		})
	})

	Describe("ListContainerContents", func() {
		var (
			req                         *apiv1.ListContainerContentsRequest
			resp                        *apiv1.ListContainerContentsResponse
			err                         error
			mockFrontmatterIndexQueryer *FlexibleMockFrontmatterIndexQueryer
		)

		BeforeEach(func() {
			req = &apiv1.ListContainerContentsRequest{
				ContainerIdentifier: "test-container",
			}
			mockFrontmatterIndexQueryer = &FlexibleMockFrontmatterIndexQueryer{
				ExactMatchResults: make(map[string][]string),
				GetValueResults:   make(map[string]map[string]string),
			}
		})

		JustBeforeEach(func() {
			server = v1.NewServer(
				"commit",
				time.Now(),
				nil,
				nil,
				nil,
				lumber.NewConsoleLogger(lumber.WARN),
				nil,
				nil,
				mockFrontmatterIndexQueryer,
			)
			resp, err = server.ListContainerContents(ctx, req)
		})

		When("the FrontmatterIndexQueryer is not configured", func() {
			BeforeEach(func() {
				mockFrontmatterIndexQueryer = nil
			})

			It("should return an internal error", func() {
				Expect(err).To(HaveGrpcStatus(codes.Internal, "FrontmatterIndexQueryer not available"))
			})
		})

		When("container_identifier is empty", func() {
			BeforeEach(func() {
				req.ContainerIdentifier = ""
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "container_identifier is required"))
			})
		})

		When("container has items", func() {
			BeforeEach(func() {
				mockFrontmatterIndexQueryer.ExactMatchResults["inventory.container:test_container"] = []string{
					"item_1",
					"item_2",
				}
				mockFrontmatterIndexQueryer.GetValueResults["item_1"] = map[string]string{"title": "Item One"}
				mockFrontmatterIndexQueryer.GetValueResults["item_2"] = map[string]string{"title": "Item Two"}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the items", func() {
				Expect(resp.Items).To(HaveLen(2))
			})

			It("should include item details", func() {
				identifiers := []string{resp.Items[0].Identifier, resp.Items[1].Identifier}
				Expect(identifiers).To(ContainElements("item_1", "item_2"))
			})

			It("should return total count", func() {
				Expect(resp.TotalCount).To(Equal(int32(2)))
			})
		})

		When("container is empty", func() {
			It("should return empty items list", func() {
				Expect(resp.Items).To(BeEmpty())
			})

			It("should return summary indicating empty", func() {
				Expect(resp.Summary).To(ContainSubstring("empty"))
			})
		})

		When("container has items in inventory.items array but not indexed yet", func() {
			var mockPageReaderMutator *MockPageReaderMutator

			BeforeEach(func() {
				// Item not in index yet (no inventory.container set)
				mockFrontmatterIndexQueryer.ExactMatchResults["inventory.container:test_container"] = []string{}
				mockPageReaderMutator = &MockPageReaderMutator{
					FrontmatterByID: map[string]map[string]any{
						"test_container": {
							"title": "Test Container",
							"inventory": map[string]any{
								"items": []any{"unindexed_item"},
							},
						},
					},
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
					mockFrontmatterIndexQueryer,
				)
				resp, err = server.ListContainerContents(ctx, req)
			})

			It("should include items from inventory.items array", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Items).To(HaveLen(1))
				Expect(resp.Items[0].Identifier).To(Equal("unindexed_item"))
			})
		})

		When("container has items from both sources (deduplication)", func() {
			var mockPageReaderMutator *MockPageReaderMutator

			BeforeEach(func() {
				// Same item appears in both index and inventory.items array
				mockFrontmatterIndexQueryer.ExactMatchResults["inventory.container:test_container"] = []string{"shared_item"}
				mockPageReaderMutator = &MockPageReaderMutator{
					FrontmatterByID: map[string]map[string]any{
						"test_container": {
							"title": "Test Container",
							"inventory": map[string]any{
								"items": []any{"shared_item", "items_only"},
							},
						},
					},
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
					mockFrontmatterIndexQueryer,
				)
				resp, err = server.ListContainerContents(ctx, req)
			})

			It("should deduplicate items from both sources", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Items).To(HaveLen(2))
				identifiers := []string{}
				for _, item := range resp.Items {
					identifiers = append(identifiers, item.Identifier)
				}
				Expect(identifiers).To(ContainElements("shared_item", "items_only"))
			})
		})
	})

	Describe("FindItemLocation", func() {
		var (
			req                   *apiv1.FindItemLocationRequest
			resp                  *apiv1.FindItemLocationResponse
			err                   error
			mockPageReaderMutator *MockPageReaderMutator
		)

		BeforeEach(func() {
			req = &apiv1.FindItemLocationRequest{
				ItemIdentifier: "test-item",
			}
			mockPageReaderMutator = &MockPageReaderMutator{}
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
			resp, err = server.FindItemLocation(ctx, req)
		})

		When("the item does not exist", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = os.ErrNotExist
			})

			It("should not return a gRPC error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return found=false", func() {
				Expect(resp.Found).To(BeFalse())
			})
		})

		When("item exists with a container", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Frontmatter = map[string]any{
					"title": "Test Item",
					"inventory": map[string]any{
						"container": "my_drawer",
					},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return found=true", func() {
				Expect(resp.Found).To(BeTrue())
			})

			It("should return the container", func() {
				Expect(resp.Locations).To(HaveLen(1))
				Expect(resp.Locations[0].Container).To(Equal("my_drawer"))
			})
		})

		When("item exists without a container", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Frontmatter = map[string]any{
					"title": "Test Item",
				}
			})

			It("should return found=true", func() {
				Expect(resp.Found).To(BeTrue())
			})

			It("should return empty locations", func() {
				Expect(resp.Locations).To(BeEmpty())
			})

			It("should indicate root-level in summary", func() {
				Expect(resp.Summary).To(ContainSubstring("root-level"))
			})
		})

		When("requesting hierarchy for nested item", func() {
			BeforeEach(func() {
				req.IncludeHierarchy = true
				// Set up a nested hierarchy: item -> drawer -> cabinet -> room
				mockPageReaderMutator.FrontmatterByID = map[string]map[string]any{
					"test_item": {
						"title": "Test Item",
						"inventory": map[string]any{
							"container": "drawer",
						},
					},
					"drawer": {
						"title": "Drawer",
						"inventory": map[string]any{
							"container": "cabinet",
						},
					},
					"cabinet": {
						"title": "Cabinet",
						"inventory": map[string]any{
							"container": "room",
						},
					},
					"room": {
						"title": "Room",
					},
				}
			})

			It("should return the full hierarchy path", func() {
				Expect(resp.Locations).To(HaveLen(1))
				Expect(resp.Locations[0].Path).To(Equal([]string{"room", "cabinet", "drawer"}))
			})

			It("should include hierarchy in summary", func() {
				Expect(resp.Summary).To(ContainSubstring("Full path"))
			})
		})

		When("the PageReaderMutator is not configured", func() {
			BeforeEach(func() {
				mockPageReaderMutator = nil
			})

			It("should return an internal error", func() {
				Expect(err).To(HaveGrpcStatus(codes.Internal, "PageReaderMutator not available"))
			})
		})

		When("the item_identifier is empty", func() {
			BeforeEach(func() {
				req.ItemIdentifier = ""
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "item_identifier is required"))
			})
		})

		When("reading frontmatter fails with unexpected error", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = errors.New("database connection failed")
			})

			It("should return an internal error", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})
})

// FlexibleMockFrontmatterIndexQueryer is a more flexible mock for testing
type FlexibleMockFrontmatterIndexQueryer struct {
	ExactMatchResults map[string][]string     // key:value -> results
	GetValueResults   map[string]map[string]string // identifier -> keypath -> value
}

func (m *FlexibleMockFrontmatterIndexQueryer) QueryExactMatch(dottedKeyPath wikipage.DottedKeyPath, value wikipage.Value) []wikipage.PageIdentifier {
	if m == nil || m.ExactMatchResults == nil {
		return nil
	}
	key := string(dottedKeyPath) + ":" + string(value)
	return m.ExactMatchResults[key]
}

func (*FlexibleMockFrontmatterIndexQueryer) QueryKeyExistence(_ wikipage.DottedKeyPath) []wikipage.PageIdentifier {
	return nil
}

func (*FlexibleMockFrontmatterIndexQueryer) QueryPrefixMatch(_ wikipage.DottedKeyPath, _ string) []wikipage.PageIdentifier {
	return nil
}

func (m *FlexibleMockFrontmatterIndexQueryer) GetValue(identifier wikipage.PageIdentifier, dottedKeyPath wikipage.DottedKeyPath) wikipage.Value {
	if m == nil || m.GetValueResults == nil {
		return ""
	}
	if values, ok := m.GetValueResults[string(identifier)]; ok {
		return wikipage.Value(values[string(dottedKeyPath)])
	}
	return ""
}
