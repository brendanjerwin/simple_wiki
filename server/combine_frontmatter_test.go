//revive:disable:dot-imports
package server

import (
	"github.com/brendanjerwin/simple_wiki/wikipage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("combineFrontmatterAndMarkdown", func() {
	When("both frontmatter and markdown are empty", func() {
		var result string
		var err error

		BeforeEach(func() {
			result, err = combineFrontmatterAndMarkdown(wikipage.FrontMatter{}, "")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an empty string", func() {
			Expect(result).To(Equal(""))
		})
	})

	When("only frontmatter is present", func() {
		var result string
		var err error

		BeforeEach(func() {
			fm := wikipage.FrontMatter{"title": "Test Page"}
			result, err = combineFrontmatterAndMarkdown(fm, "")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should include TOML delimiters", func() {
			Expect(result).To(ContainSubstring(tomlDelimiter))
		})

		It("should include the frontmatter key", func() {
			Expect(result).To(ContainSubstring("title"))
		})
	})

	When("only markdown is present", func() {
		var result string
		var err error

		BeforeEach(func() {
			result, err = combineFrontmatterAndMarkdown(wikipage.FrontMatter{}, "# Hello\n")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should include the markdown content", func() {
			Expect(result).To(ContainSubstring("# Hello"))
		})

		It("should not include TOML delimiters", func() {
			Expect(result).NotTo(ContainSubstring(tomlDelimiter))
		})
	})

	When("both frontmatter and markdown are present", func() {
		var result string
		var err error

		BeforeEach(func() {
			fm := wikipage.FrontMatter{"title": "My Page"}
			result, err = combineFrontmatterAndMarkdown(fm, "# My Page\n\nSome content.")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should include both TOML delimiters and markdown content", func() {
			Expect(result).To(ContainSubstring(tomlDelimiter))
			Expect(result).To(ContainSubstring("# My Page"))
		})
	})
})
