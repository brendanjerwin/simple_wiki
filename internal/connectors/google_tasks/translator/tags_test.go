//revive:disable:dot-imports
package translator_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/translator"
)

var _ = Describe("EncodeTitleWithTags", func() {
	When("there are no tags", func() {
		var encoded string

		BeforeEach(func() {
			encoded = translator.EncodeTitleWithTags("Buy milk", nil)
		})

		It("should return the title unchanged", func() {
			Expect(encoded).To(Equal("Buy milk"))
		})
	})

	When("tags are not already in the title", func() {
		var encoded string

		BeforeEach(func() {
			encoded = translator.EncodeTitleWithTags("Buy milk", []string{"urgent", "kirsten"})
		})

		It("should append #urgent", func() {
			Expect(encoded).To(ContainSubstring("#urgent"))
		})

		It("should append #kirsten", func() {
			Expect(encoded).To(ContainSubstring("#kirsten"))
		})

		It("should preserve the original title prefix", func() {
			Expect(encoded).To(HavePrefix("Buy milk"))
		})
	})

	When("a tag already appears inline in the title", func() {
		var encoded string

		BeforeEach(func() {
			encoded = translator.EncodeTitleWithTags("Buy milk #urgent", []string{"urgent"})
		})

		It("should not duplicate the tag", func() {
			matches := 0
			for i := 0; i+len("#urgent") <= len(encoded); i++ {
				if encoded[i:i+len("#urgent")] == "#urgent" {
					matches++
				}
			}
			Expect(matches).To(Equal(1))
		})
	})
})

var _ = Describe("TitleAndTagsFromText", func() {
	When("text has trailing #tag tokens", func() {
		var (
			title string
			tags  []string
		)

		BeforeEach(func() {
			title, tags = translator.TitleAndTagsFromText("Buy oat milk #urgent #kirsten")
		})

		It("should strip the #tag tokens from the title", func() {
			Expect(title).To(Equal("Buy oat milk"))
		})

		It("should extract urgent", func() {
			Expect(tags).To(ContainElement("urgent"))
		})

		It("should extract kirsten", func() {
			Expect(tags).To(ContainElement("kirsten"))
		})
	})

	When("text has no tags", func() {
		var (
			title string
			tags  []string
		)

		BeforeEach(func() {
			title, tags = translator.TitleAndTagsFromText("Just text")
		})

		It("should return the original text as title", func() {
			Expect(title).To(Equal("Just text"))
		})

		It("should return an empty tag list", func() {
			Expect(tags).To(BeEmpty())
		})
	})

	When("text is empty", func() {
		var (
			title string
			tags  []string
		)

		BeforeEach(func() {
			title, tags = translator.TitleAndTagsFromText("")
		})

		It("should return empty title", func() {
			Expect(title).To(Equal(""))
		})

		It("should return empty tags", func() {
			Expect(tags).To(BeEmpty())
		})
	})

	When("a tag is interleaved between words", func() {
		var (
			title string
			tags  []string
		)

		BeforeEach(func() {
			title, tags = translator.TitleAndTagsFromText("Buy #urgent oat milk")
		})

		It("should strip the #tag and keep the surrounding words", func() {
			Expect(title).To(Equal("Buy oat milk"))
		})

		It("should extract the tag", func() {
			Expect(tags).To(ContainElement("urgent"))
		})
	})
})

var _ = Describe("EncodeTitleWithTags ↔ TitleAndTagsFromText round trip", func() {
	When("a clean title and tag list is encoded then decoded", func() {
		var (
			roundTitle string
			roundTags  []string
		)

		BeforeEach(func() {
			encoded := translator.EncodeTitleWithTags("Buy oat milk", []string{"urgent", "kirsten"})
			roundTitle, roundTags = translator.TitleAndTagsFromText(encoded)
		})

		It("should preserve the title", func() {
			Expect(roundTitle).To(Equal("Buy oat milk"))
		})

		It("should preserve the urgent tag", func() {
			Expect(roundTags).To(ContainElement("urgent"))
		})

		It("should preserve the kirsten tag", func() {
			Expect(roundTags).To(ContainElement("kirsten"))
		})
	})
})
