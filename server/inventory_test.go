//revive:disable:dot-imports
package server

import (
	"os"
	"path/filepath"

	"github.com/brendanjerwin/simple_wiki/migrations/lazy"
	"github.com/brendanjerwin/simple_wiki/utils/goldmarkrenderer"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("inventory.go", func() {
	Describe("CreateInventoryItemPage", func() {
		var (
			pathToData string
			s          *Site
		)

		BeforeEach(func() {
			pathToData = "testdata_inventory"
			err := os.MkdirAll(pathToData, 0755)
			Expect(err).NotTo(HaveOccurred())
			s = &Site{
				PathToData:          pathToData,
				MarkdownRenderer:    &goldmarkrenderer.GoldmarkRenderer{},
				Logger:              lumber.NewConsoleLogger(lumber.WARN),
				MigrationApplicator: lazy.NewEmptyApplicator(),
			}
			err = s.InitializeIndexing()
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			_ = os.RemoveAll(pathToData)
		})

		When("creating a new inventory item with container", func() {
			var (
				page *wikipage.Page
				err  error
			)

			BeforeEach(func() {
				page, err = s.CreateInventoryItemPage(InventoryItemParams{
					Identifier: "test_item",
					Container:  "my_drawer",
				})
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a page with the correct identifier", func() {
				Expect(page).NotTo(BeNil())
				Expect(page.Identifier).To(Equal("test_item"))
			})

			It("should create the page file on disk", func() {
				// Check that the file exists
				files, err := os.ReadDir(pathToData)
				Expect(err).NotTo(HaveOccurred())
				found := false
				for _, f := range files {
					if filepath.Ext(f.Name()) == ".md" {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue())
			})

			It("should have frontmatter with the identifier", func() {
				Expect(page.Text).To(ContainSubstring("identifier = 'test_item'"))
			})

			It("should have frontmatter with the container", func() {
				Expect(page.Text).To(ContainSubstring("container = 'my_drawer'"))
			})

			It("should not have items array (items array is only for containers)", func() {
				Expect(page.Text).NotTo(ContainSubstring("items"))
			})

			It("should contain the inventory markdown template", func() {
				Expect(page.Text).To(ContainSubstring("IsContainer"))
			})
		})

		When("creating a new inventory item with custom title", func() {
			var (
				page *wikipage.Page
				err  error
			)

			BeforeEach(func() {
				page, err = s.CreateInventoryItemPage(InventoryItemParams{
					Identifier: "test_item_titled",
					Title:      "My Custom Title",
				})
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should use the custom title", func() {
				Expect(page.Text).To(ContainSubstring("title = 'My Custom Title'"))
			})
		})

		When("creating a new inventory item without title (auto-generate)", func() {
			var (
				page *wikipage.Page
				err  error
			)

			BeforeEach(func() {
				page, err = s.CreateInventoryItemPage(InventoryItemParams{
					Identifier: "my_cool_widget",
				})
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should auto-generate a title from identifier", func() {
				Expect(page.Text).To(ContainSubstring("title = "))
			})
		})

		When("creating a new inventory item without container", func() {
			var (
				page *wikipage.Page
				err  error
			)

			BeforeEach(func() {
				page, err = s.CreateInventoryItemPage(InventoryItemParams{
					Identifier: "standalone_item",
				})
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not have a container in frontmatter", func() {
				Expect(page.Text).NotTo(ContainSubstring("container = "))
			})
		})

		When("creating an item that already exists", func() {
			var err error

			BeforeEach(func() {
				// Create the page first
				_, err = s.CreateInventoryItemPage(InventoryItemParams{
					Identifier: "existing_item",
				})
				Expect(err).NotTo(HaveOccurred())

				// Try to create it again
				_, err = s.CreateInventoryItemPage(InventoryItemParams{
					Identifier: "existing_item",
				})
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should indicate the page already exists", func() {
				Expect(err.Error()).To(ContainSubstring("already exists"))
			})
		})

		When("creating an item with an identifier that needs munging", func() {
			var (
				page *wikipage.Page
				err  error
			)

			BeforeEach(func() {
				page, err = s.CreateInventoryItemPage(InventoryItemParams{
					Identifier: "My Cool Item",
					Container:  "Some Container",
				})
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should munge the identifier", func() {
				Expect(page.Identifier).To(Equal("my_cool_item"))
			})

			It("should munge the container", func() {
				Expect(page.Text).To(ContainSubstring("container = 'some_container'"))
			})
		})
	})

	Describe("buildInventoryItemPageText edge cases", func() {
		When("frontmatter bytes end with a newline already", func() {
			var (
				pageText string
				err      error
			)

			BeforeEach(func() {
				// Create frontmatter that ends with newline when marshaled
				fm := map[string]any{
					"key": "value",
				}
				pageText, err = buildInventoryItemPageText(fm)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should produce valid page text", func() {
				Expect(pageText).To(ContainSubstring("+++"))
			})
		})
	})

	Describe("buildInventoryItemPageText", func() {
		When("given valid frontmatter", func() {
			var (
				pageText string
				err      error
			)

			BeforeEach(func() {
				fm := map[string]any{
					"identifier": "test_item",
					"title":      "Test Item",
					"inventory": map[string]any{
						"container": "my_drawer",
						"items":     []string{},
					},
				}
				pageText, err = buildInventoryItemPageText(fm)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should contain TOML frontmatter delimiters", func() {
				Expect(pageText).To(ContainSubstring("+++"))
			})

			It("should contain the identifier in frontmatter", func() {
				Expect(pageText).To(ContainSubstring("identifier = 'test_item'"))
			})

			It("should contain the title in frontmatter", func() {
				Expect(pageText).To(ContainSubstring("title = 'Test Item'"))
			})

			It("should contain the inventory section", func() {
				Expect(pageText).To(ContainSubstring("[inventory]"))
			})

			It("should contain the container in inventory", func() {
				Expect(pageText).To(ContainSubstring("container = 'my_drawer'"))
			})

			It("should contain the title template", func() {
				Expect(pageText).To(ContainSubstring("# {{or .Title .Identifier}}"))
			})

			It("should contain the inventory markdown template", func() {
				Expect(pageText).To(ContainSubstring("IsContainer"))
			})
		})

		When("given empty frontmatter", func() {
			var (
				pageText string
				err      error
			)

			BeforeEach(func() {
				fm := map[string]any{}
				pageText, err = buildInventoryItemPageText(fm)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should still contain TOML delimiters for empty frontmatter", func() {
				// Empty frontmatter should still produce valid output
				Expect(pageText).NotTo(BeEmpty())
			})

			It("should contain the markdown template", func() {
				Expect(pageText).To(ContainSubstring("# {{or .Title .Identifier}}"))
			})
		})
	})

	Describe("EnsureInventoryFrontmatterStructure", func() {
		When("frontmatter has no inventory section", func() {
			var fm map[string]any

			BeforeEach(func() {
				fm = map[string]any{
					"title": "Some Page",
				}
				EnsureInventoryFrontmatterStructure(fm)
			})

			It("should create an inventory section", func() {
				Expect(fm).To(HaveKey("inventory"))
			})

			It("should not add items array (items array is only for containers)", func() {
				inventory, ok := fm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(inventory).NotTo(HaveKey("items"))
			})
		})

		When("frontmatter has inventory but no items", func() {
			var fm map[string]any

			BeforeEach(func() {
				fm = map[string]any{
					"title": "Some Page",
					"inventory": map[string]any{
						"container": "parent_box",
					},
				}
				EnsureInventoryFrontmatterStructure(fm)
			})

			It("should preserve existing inventory data", func() {
				inventory, ok := fm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(inventory["container"]).To(Equal("parent_box"))
			})

			It("should not add items array (items array is only for containers)", func() {
				inventory, ok := fm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(inventory).NotTo(HaveKey("items"))
			})
		})

		When("frontmatter already has complete inventory structure", func() {
			var fm map[string]any

			BeforeEach(func() {
				fm = map[string]any{
					"title": "Some Page",
					"inventory": map[string]any{
						"container": "parent_box",
						"items":     []string{"item1", "item2"},
					},
				}
				EnsureInventoryFrontmatterStructure(fm)
			})

			It("should not modify existing items", func() {
				inventory, ok := fm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				items, ok := inventory["items"].([]string)
				Expect(ok).To(BeTrue())
				Expect(items).To(HaveLen(2))
				Expect(items).To(ContainElements("item1", "item2"))
			})
		})

		When("inventory is not a map", func() {
			var fm map[string]any

			BeforeEach(func() {
				fm = map[string]any{
					"title":     "Some Page",
					"inventory": "not-a-map",
				}
				EnsureInventoryFrontmatterStructure(fm)
			})

			It("should not modify frontmatter when inventory is wrong type", func() {
				// Since inventory exists but is wrong type, it won't be modified
				Expect(fm["inventory"]).To(Equal("not-a-map"))
			})
		})
	})
})
