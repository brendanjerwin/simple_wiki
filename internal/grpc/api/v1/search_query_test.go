package v1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("parseUserSearchQuery", func() {
	Describe("when query has no #tag tokens", func() {
		var parsed parsedSearchQuery

		BeforeEach(func() {
			parsed = parseUserSearchQuery("home lab setup")
		})

		It("should not produce required tags", func() {
			Expect(parsed.requiredTags).To(BeEmpty())
		})

		It("should produce one free-text token per word", func() {
			Expect(parsed.freeTextTokens).To(Equal([]string{"home", "lab", "setup"}))
		})
	})

	Describe("when query is a single #tag", func() {
		var parsed parsedSearchQuery

		BeforeEach(func() {
			parsed = parseUserSearchQuery("#groceries")
		})

		It("should produce one required tag", func() {
			Expect(parsed.requiredTags).To(Equal([]string{"groceries"}))
		})

		It("should produce no free-text tokens", func() {
			Expect(parsed.freeTextTokens).To(BeEmpty())
		})
	})

	Describe("when query mixes #tag tokens with free text", func() {
		var parsed parsedSearchQuery

		BeforeEach(func() {
			parsed = parseUserSearchQuery("#groceries milk")
		})

		It("should produce one required tag", func() {
			Expect(parsed.requiredTags).To(Equal([]string{"groceries"}))
		})

		It("should produce the free-text token", func() {
			Expect(parsed.freeTextTokens).To(Equal([]string{"milk"}))
		})
	})

	Describe("when query has multiple #tag tokens", func() {
		var parsed parsedSearchQuery

		BeforeEach(func() {
			parsed = parseUserSearchQuery("#groceries #urgent")
		})

		It("should produce both as required tags (AND semantics)", func() {
			Expect(parsed.requiredTags).To(ConsistOf("groceries", "urgent"))
		})
	})

	Describe("when query contains a #tag with stylized casing", func() {
		var parsed parsedSearchQuery

		BeforeEach(func() {
			parsed = parseUserSearchQuery("#Groceries milk")
		})

		It("should normalize the tag to lowercase", func() {
			Expect(parsed.requiredTags).To(Equal([]string{"groceries"}))
		})
	})

	Describe("when query has just `#` (empty tag)", func() {
		var parsed parsedSearchQuery

		BeforeEach(func() {
			parsed = parseUserSearchQuery("# lone hash")
		})

		It("should not produce any required tags", func() {
			Expect(parsed.requiredTags).To(BeEmpty())
		})

		It("should drop the bare `#` from free text", func() {
			Expect(parsed.freeTextTokens).To(Equal([]string{"lone", "hash"}))
		})
	})
})

var _ = Describe("buildBleveQueryString", func() {
	Describe("when there are required tags only", func() {
		It("should AND the tag clauses", func() {
			parsed := parsedSearchQuery{requiredTags: []string{"groceries", "urgent"}}
			Expect(buildBleveQueryString(parsed)).To(Equal("+tags:groceries +tags:urgent"))
		})
	})

	Describe("when there is free text only", func() {
		It("should include the free-text tokens with tag-boost should-clauses", func() {
			parsed := parsedSearchQuery{freeTextTokens: []string{"home", "lab"}}
			result := buildBleveQueryString(parsed)
			Expect(result).To(ContainSubstring("home"))
			Expect(result).To(ContainSubstring("lab"))
			Expect(result).To(ContainSubstring("tags:home^2"))
			Expect(result).To(ContainSubstring("tags:lab^2"))
		})
	})

	Describe("when there is a mix of tags and free text", func() {
		It("should AND the tags and OR the free-text tokens with boosts", func() {
			parsed := parsedSearchQuery{
				requiredTags:   []string{"groceries"},
				freeTextTokens: []string{"milk"},
			}
			result := buildBleveQueryString(parsed)
			Expect(result).To(ContainSubstring("+tags:groceries"))
			Expect(result).To(ContainSubstring("milk"))
			Expect(result).To(ContainSubstring("tags:milk^2"))
		})
	})

	Describe("when both required tags and free text are empty", func() {
		It("should return an empty string", func() {
			Expect(buildBleveQueryString(parsedSearchQuery{})).To(Equal(""))
		})
	})
})
