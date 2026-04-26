//revive:disable:dot-imports
package wikipage_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

var _ = Describe("NormalizeListName", func() {
	When("the name has no unsafe characters", func() {
		It("should return the name unchanged", func() {
			Expect(wikipage.NormalizeListName("groceries")).To(Equal("groceries"))
		})

		It("should preserve hyphens already present", func() {
			Expect(wikipage.NormalizeListName("this-week")).To(Equal("this-week"))
		})

		It("should preserve unicode letters", func() {
			Expect(wikipage.NormalizeListName("Schöne-Liste")).To(Equal("Schöne-Liste"))
		})
	})

	When("the name contains a forward slash", func() {
		It("should replace the slash with a hyphen", func() {
			Expect(wikipage.NormalizeListName("Groceries/Household")).To(Equal("Groceries-Household"))
		})
	})

	When("the name contains a backslash", func() {
		It("should replace the backslash with a hyphen", func() {
			Expect(wikipage.NormalizeListName(`a\b`)).To(Equal("a-b"))
		})
	})

	When("the name contains URL-reserved punctuation", func() {
		It("should replace ? with a hyphen", func() {
			Expect(wikipage.NormalizeListName("foo?bar")).To(Equal("foo-bar"))
		})

		It("should replace # with a hyphen", func() {
			Expect(wikipage.NormalizeListName("foo#bar")).To(Equal("foo-bar"))
		})

		It("should replace % with a hyphen", func() {
			Expect(wikipage.NormalizeListName("foo%bar")).To(Equal("foo-bar"))
		})
	})

	When("the name contains whitespace", func() {
		It("should replace a single space with a hyphen", func() {
			Expect(wikipage.NormalizeListName("two words")).To(Equal("two-words"))
		})

		It("should collapse runs of whitespace to one hyphen", func() {
			Expect(wikipage.NormalizeListName("two  \t  words")).To(Equal("two-words"))
		})
	})

	When("the name has unsafe characters at the boundaries", func() {
		It("should strip a trailing slash", func() {
			Expect(wikipage.NormalizeListName("groceries/")).To(Equal("groceries"))
		})

		It("should strip a leading slash", func() {
			Expect(wikipage.NormalizeListName("/groceries")).To(Equal("groceries"))
		})

		It("should strip leading and trailing whitespace", func() {
			Expect(wikipage.NormalizeListName("  groceries  ")).To(Equal("groceries"))
		})
	})

	When("the name collapses runs of unsafe characters", func() {
		It("should produce a single hyphen for adjacent unsafe runs", func() {
			Expect(wikipage.NormalizeListName("a / b")).To(Equal("a-b"))
		})
	})

	When("the name is the empty string", func() {
		It("should return the empty string", func() {
			Expect(wikipage.NormalizeListName("")).To(Equal(""))
		})
	})

	When("the name is entirely unsafe", func() {
		It("should return the empty string after trim", func() {
			Expect(wikipage.NormalizeListName("///")).To(Equal(""))
		})
	})

	When("two distinct names normalize to the same value", func() {
		It("should treat them as colliding (caller's responsibility to reject)", func() {
			Expect(wikipage.NormalizeListName("a/b")).To(Equal(wikipage.NormalizeListName("a-b")))
		})
	})
})
