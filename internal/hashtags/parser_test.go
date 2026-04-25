//revive:disable:dot-imports
package hashtags

import (
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestHashtags(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Hashtags Suite")
}

var _ = Describe("Normalize", func() {
	Describe("when input is plain ASCII", func() {
		var result string

		BeforeEach(func() {
			result = Normalize("Groceries")
		})

		It("should lowercase the tag", func() {
			Expect(result).To(Equal("groceries"))
		})
	})

	Describe("when input contains hyphen and underscore", func() {
		It("should preserve hyphens", func() {
			Expect(Normalize("home-lab")).To(Equal("home-lab"))
		})

		It("should preserve underscores", func() {
			Expect(Normalize("home_lab")).To(Equal("home_lab"))
		})

		It("should keep hyphens and underscores distinct", func() {
			Expect(Normalize("home-lab")).NotTo(Equal(Normalize("home_lab")))
		})
	})

	Describe("when input contains stylized Unicode", func() {
		It("should NFKC-fold compatibility forms", func() {
			// `ＡＢＣ` (fullwidth) -> `ABC` -> `abc`
			Expect(Normalize("ＡＢＣ")).To(Equal("abc"))
		})
	})

	Describe("when input contains disallowed characters", func() {
		It("should drop punctuation", func() {
			Expect(Normalize("foo!bar")).To(Equal("foobar"))
		})
	})

	Describe("when input is longer than the cap", func() {
		var (
			input  string
			result string
		)

		BeforeEach(func() {
			input = strings.Repeat("a", maxTagLen+10)
			result = Normalize(input)
		})

		It("should truncate to maxTagLen runes", func() {
			Expect(len([]rune(result))).To(Equal(maxTagLen))
		})
	})
})

var _ = Describe("Extract", func() {
	Describe("when body has no hashtags", func() {
		It("should return an empty slice", func() {
			Expect(Extract("just plain text")).To(BeEmpty())
		})
	})

	Describe("when body has a single hashtag", func() {
		It("should return the normalized tag", func() {
			Expect(Extract("buy #milk today")).To(Equal([]string{"milk"}))
		})
	})

	Describe("when hashtag begins the body", func() {
		It("should match at start of string", func() {
			Expect(Extract("#urgent: ship it")).To(Equal([]string{"urgent"}))
		})
	})

	Describe("when hashtag follows whitespace", func() {
		It("should match after a space", func() {
			Expect(Extract("a #bee c")).To(Equal([]string{"bee"}))
		})
	})

	Describe("when body has multiple distinct hashtags", func() {
		It("should return them in first-occurrence order", func() {
			Expect(Extract("#alpha and #beta and #gamma")).To(Equal([]string{"alpha", "beta", "gamma"}))
		})
	})

	Describe("when body has duplicate hashtags", func() {
		It("should deduplicate to first occurrence", func() {
			Expect(Extract("#dup once #dup twice")).To(Equal([]string{"dup"}))
		})
	})

	Describe("when hashtag is mid-word", func() {
		It("should not extract", func() {
			Expect(Extract("foo#bar")).To(BeEmpty())
		})
	})

	Describe("when content is a markdown anchor link", func() {
		It("should not extract the anchor as a tag", func() {
			Expect(Extract("see [link](#section) for more")).To(BeEmpty())
		})
	})

	Describe("when hashtag is escaped", func() {
		It(`should not extract \#tag`, func() {
			Expect(Extract(`literal \#hash here`)).To(BeEmpty())
		})
	})

	Describe("when hashtag is inside an inline code span", func() {
		It("should not extract", func() {
			Expect(Extract("code: `not a #tag` here")).To(BeEmpty())
		})
	})

	Describe("when hashtag is inside a fenced code block", func() {
		It("should not extract", func() {
			body := "before\n```\n#fence-tag\n```\nafter #real"
			Expect(Extract(body)).To(Equal([]string{"real"}))
		})
	})

	Describe("when hashtag normalization differs from raw spelling", func() {
		It("should return the normalized form", func() {
			Expect(Extract("#Groceries")).To(Equal([]string{"groceries"}))
		})
	})

	Describe("when tag is numeric only", func() {
		It("should accept numeric tags", func() {
			Expect(Extract("year #2026 wraps up")).To(Equal([]string{"2026"}))
		})
	})

	Describe("when tag contains hyphen and underscore", func() {
		It("should preserve them", func() {
			Expect(Extract("setup #home-lab and #project_alpha")).To(Equal([]string{"home-lab", "project_alpha"}))
		})
	})

	Describe("when hashtag is just `#` followed by space", func() {
		It("should not extract an empty tag", func() {
			Expect(Extract("# header-ish")).To(BeEmpty())
		})
	})

	Describe("when extraction is run twice", func() {
		It("should be idempotent for repeated calls", func() {
			body := "a #foo b #bar"
			first := Extract(body)
			second := Extract(body)
			Expect(second).To(Equal(first))
		})
	})
})

var _ = Describe("IsTagBoundary", func() {
	Describe("when previous rune is whitespace", func() {
		It("returns true for space", func() {
			Expect(IsTagBoundary(' ')).To(BeTrue())
		})

		It("returns true for newline", func() {
			Expect(IsTagBoundary('\n')).To(BeTrue())
		})

		It("returns true for tab", func() {
			Expect(IsTagBoundary('\t')).To(BeTrue())
		})
	})

	Describe("when previous rune is a letter or digit", func() {
		It("returns false for letter", func() {
			Expect(IsTagBoundary('a')).To(BeFalse())
		})

		It("returns false for digit", func() {
			Expect(IsTagBoundary('5')).To(BeFalse())
		})
	})

	Describe("when previous rune is bracket-like punctuation", func() {
		It("returns false for `[`", func() {
			Expect(IsTagBoundary('[')).To(BeFalse())
		})

		It("returns false for `(`", func() {
			Expect(IsTagBoundary('(')).To(BeFalse())
		})
	})

	Describe("when previous rune is other punctuation", func() {
		It("returns true for comma", func() {
			Expect(IsTagBoundary(',')).To(BeTrue())
		})

		It("returns true for period", func() {
			Expect(IsTagBoundary('.')).To(BeTrue())
		})
	})
})
