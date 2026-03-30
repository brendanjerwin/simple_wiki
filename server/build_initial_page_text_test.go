//revive:disable:dot-imports
package server

import (
	"strings"

	"github.com/brendanjerwin/simple_wiki/wikipage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("buildInitialPageText", func() {
	When("frontmatter is empty and template is empty", func() {
		var result string
		var err error

		BeforeEach(func() {
			result, err = buildInitialPageText(wikipage.FrontMatter{}, "")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should include a title heading template", func() {
			Expect(result).To(ContainSubstring("{{or .Title .Identifier}}"))
		})

		It("should not include TOML delimiters", func() {
			Expect(result).NotTo(ContainSubstring(tomlDelimiter))
		})
	})

	When("frontmatter has values", func() {
		var result string
		var err error

		BeforeEach(func() {
			fm := wikipage.FrontMatter{
				"title": "My Test Page",
			}
			result, err = buildInitialPageText(fm, "")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should include TOML delimiters", func() {
			Expect(strings.Count(result, tomlDelimiter)).To(Equal(2))
		})

		It("should include the frontmatter value", func() {
			Expect(result).To(ContainSubstring("title"))
		})

		It("should include the title heading template after the TOML block", func() {
			Expect(result).To(ContainSubstring("{{or .Title .Identifier}}"))
		})
	})

	When("template is inv_item", func() {
		var result string
		var err error

		BeforeEach(func() {
			result, err = buildInitialPageText(wikipage.FrontMatter{}, "inv_item")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should include the inventory contents section", func() {
			Expect(result).To(ContainSubstring("ShowInventoryContentsOf"))
		})

		It("should include the Contents heading", func() {
			Expect(result).To(ContainSubstring("## Contents"))
		})
	})

	When("template is not inv_item", func() {
		var result string
		var err error

		BeforeEach(func() {
			result, err = buildInitialPageText(wikipage.FrontMatter{}, "some_other_template")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not include inventory contents section", func() {
			Expect(result).NotTo(ContainSubstring("ShowInventoryContentsOf"))
		})
	})
})
