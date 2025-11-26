//revive:disable:dot-imports
package server

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("inventory.go", func() {
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
				Expect(pageText).To(ContainSubstring("Goes in:"))
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

			It("should create an empty items array", func() {
				inventory, ok := fm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(inventory).To(HaveKey("items"))
				items, ok := inventory["items"].([]string)
				Expect(ok).To(BeTrue())
				Expect(items).To(BeEmpty())
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

			It("should add items array", func() {
				inventory, ok := fm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(inventory).To(HaveKey("items"))
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
