//revive:disable:dot-imports
package inventory_test

import (
	"testing"

	"github.com/brendanjerwin/simple_wiki/pkg/inventory"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestInventory(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Inventory Package Suite")
}

var _ = Describe("inventory/markdown.go", func() {
	Describe("BuildItemMarkdown", func() {
		When("building markdown content", func() {
			var result string

			BeforeEach(func() {
				result = inventory.BuildItemMarkdown()
			})

			It("should contain the title template", func() {
				Expect(result).To(ContainSubstring("# {{or .Title .Identifier}}"))
			})

			It("should contain the inventory markdown template", func() {
				Expect(result).To(ContainSubstring("IsContainer"))
			})

			It("should contain the container section", func() {
				Expect(result).To(ContainSubstring("### Goes in: {{LinkTo .Inventory.Container }}"))
			})

			It("should contain the contents section", func() {
				Expect(result).To(ContainSubstring("## Contents"))
			})
		})
	})

	Describe("BuildItemPageText", func() {
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
				pageText, err = inventory.BuildItemPageText(fm)
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
				pageText, err = inventory.BuildItemPageText(fm)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should still contain TOML delimiters for empty frontmatter", func() {
				Expect(pageText).NotTo(BeEmpty())
			})

			It("should contain the markdown template", func() {
				Expect(pageText).To(ContainSubstring("# {{or .Title .Identifier}}"))
			})
		})

		When("frontmatter bytes end with a newline already", func() {
			var (
				pageText string
				err      error
			)

			BeforeEach(func() {
				fm := map[string]any{
					"key": "value",
				}
				pageText, err = inventory.BuildItemPageText(fm)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should produce valid page text", func() {
				Expect(pageText).To(ContainSubstring("+++"))
			})
		})
	})
})
