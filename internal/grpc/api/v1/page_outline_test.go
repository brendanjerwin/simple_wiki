// Internal tests for parseHeadings and headingSlugger — package v1 for access
// to unexported functions.
package v1

import (
	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// parseHeadingsResult is a type alias for test readability.
type parseHeadingsResult = []*apiv1.PageHeading

var _ = Describe("goldmarkSlugify", func() {
	It("lowercases ASCII letters and converts spaces to hyphens", func() {
		Expect(goldmarkSlugify("Hello World")).To(Equal("hello-world"))
	})

	It("converts hyphens and underscores to hyphens", func() {
		Expect(goldmarkSlugify("foo-bar_baz")).To(Equal("foo-bar-baz"))
	})

	It("drops non-alphanumeric non-separator ASCII bytes", func() {
		Expect(goldmarkSlugify("Hello, World!")).To(Equal("hello-world"))
	})

	It("drops multi-byte UTF-8 characters", func() {
		Expect(goldmarkSlugify("Héllo")).To(Equal("hllo"))
	})

	It("returns 'heading' for all-punctuation input", func() {
		Expect(goldmarkSlugify("!!!")).To(Equal("heading"))
	})

	It("returns 'heading' for empty input", func() {
		Expect(goldmarkSlugify("")).To(Equal("heading"))
	})
})

var _ = Describe("headingSlugger", func() {
	var slugger *headingSlugger

	BeforeEach(func() {
		slugger = newHeadingSlugger()
	})

	When("the same text appears once", func() {
		var result string

		BeforeEach(func() {
			result = slugger.slug("Hello World")
		})

		It("returns the base slug", func() {
			Expect(result).To(Equal("hello-world"))
		})
	})

	When("the same text appears twice", func() {
		var first, second string

		BeforeEach(func() {
			first = slugger.slug("Hello World")
			second = slugger.slug("Hello World")
		})

		It("returns the base slug for the first occurrence", func() {
			Expect(first).To(Equal("hello-world"))
		})

		It("appends -1 suffix for the second occurrence", func() {
			Expect(second).To(Equal("hello-world-1"))
		})
	})

	When("the same text appears three times", func() {
		var first, second, third string

		BeforeEach(func() {
			first = slugger.slug("Dup")
			second = slugger.slug("Dup")
			third = slugger.slug("Dup")
		})

		It("returns base slug first", func() {
			Expect(first).To(Equal("dup"))
		})

		It("returns -1 suffix second", func() {
			Expect(second).To(Equal("dup-1"))
		})

		It("returns -2 suffix third", func() {
			Expect(third).To(Equal("dup-2"))
		})
	})
})

var _ = Describe("parseHeadings", func() {
	When("given an empty markdown string", func() {
		BeforeEach(func() {})

		It("returns nil", func() {
			Expect(parseHeadings("")).To(BeNil())
		})
	})

	When("given markdown with no headings", func() {
		It("returns nil", func() {
			Expect(parseHeadings("Just some prose.\n\nNo headings here.\n")).To(BeNil())
		})
	})

	When("given a single H1 heading with body", func() {
		// "# Hello\n" = 8 bytes
		const markdown = "# Hello\n\nsome body text\n"
		var headings parseHeadingsResult

		BeforeEach(func() {
			headings = parseHeadings(markdown)
		})

		It("returns one heading", func() {
			Expect(headings).To(HaveLen(1))
		})

		It("has level 1", func() {
			Expect(headings[0].Level).To(Equal(int32(1)))
		})

		It("has text 'Hello'", func() {
			Expect(headings[0].Text).To(Equal("Hello"))
		})

		It("has slug 'hello'", func() {
			Expect(headings[0].Slug).To(Equal("hello"))
		})

		It("has byte_offset pointing after the heading line", func() {
			Expect(headings[0].ByteOffset).To(Equal(int64(8)))
		})

		It("has byte_length covering the body to EOF", func() {
			Expect(headings[0].ByteLength).To(Equal(int64(len(markdown) - 8)))
		})
	})

	When("given deeply-nested headings", func() {
		const markdown = "# H1\n\nbody1\n\n## H2\n\nbody2\n\n### H3\n\nbody3\n"
		var headings parseHeadingsResult

		BeforeEach(func() {
			headings = parseHeadings(markdown)
		})

		It("returns three headings", func() {
			Expect(headings).To(HaveLen(3))
		})

		It("H1 section spans until EOF", func() {
			h1BodyStart := int64(len("# H1\n"))
			totalBytes := int64(len(markdown))
			Expect(headings[0].ByteOffset).To(Equal(h1BodyStart))
			Expect(headings[0].ByteLength).To(Equal(totalBytes - h1BodyStart))
		})

		It("H2 section extends to EOF because H3 is nested inside it", func() {
			// H3 (level=3) does not end the H2 (level=2) section because it is
			// at a deeper level. Only a heading at level ≤ 2 would end this section.
			h2BodyStart := int64(len("# H1\n\nbody1\n\n## H2\n"))
			totalBytes := int64(len(markdown))
			Expect(headings[1].ByteOffset).To(Equal(h2BodyStart))
			Expect(headings[1].ByteLength).To(Equal(totalBytes - h2BodyStart))
		})

		It("H3 section spans to EOF", func() {
			h3BodyStart := int64(len("# H1\n\nbody1\n\n## H2\n\nbody2\n\n### H3\n"))
			totalBytes := int64(len(markdown))
			Expect(headings[2].ByteOffset).To(Equal(h3BodyStart))
			Expect(headings[2].ByteLength).To(Equal(totalBytes - h3BodyStart))
		})
	})

	When("H2 is followed by H1 (H1 terminates the H2 section)", func() {
		const markdown = "## Section A\n\nbody A\n\n# Back to Top\n\nbody top\n"
		var headings parseHeadingsResult

		BeforeEach(func() {
			headings = parseHeadings(markdown)
		})

		It("returns two headings", func() {
			Expect(headings).To(HaveLen(2))
		})

		It("H2 section ends at H1 start", func() {
			h2BodyStart := int64(len("## Section A\n"))
			h1Start := int64(len("## Section A\n\nbody A\n\n"))
			Expect(headings[0].ByteOffset).To(Equal(h2BodyStart))
			Expect(headings[0].ByteLength).To(Equal(h1Start - h2BodyStart))
		})
	})

	When("headings appear inside a backtick fenced code block", func() {
		const markdown = "# Real Heading\n\n```\n# Not A Heading\n```\n\nsome text\n"
		var headings parseHeadingsResult

		BeforeEach(func() {
			headings = parseHeadings(markdown)
		})

		It("returns only the real heading", func() {
			Expect(headings).To(HaveLen(1))
		})

		It("has text 'Real Heading'", func() {
			Expect(headings[0].Text).To(Equal("Real Heading"))
		})
	})

	When("headings appear inside a tilde fenced code block", func() {
		const markdown = "# Outside\n\n~~~\n# Inside\n~~~\n\nafter\n"
		var headings parseHeadingsResult

		BeforeEach(func() {
			headings = parseHeadings(markdown)
		})

		It("excludes the heading inside the tilde fence", func() {
			Expect(headings).To(HaveLen(1))
		})

		It("has text 'Outside'", func() {
			Expect(headings[0].Text).To(Equal("Outside"))
		})
	})

	When("heading text contains multi-byte UTF-8 characters", func() {
		// "# Héllo\n": '#'(1) ' '(1) 'H'(1) 'é'(2) 'l'(1) 'l'(1) 'o'(1) '\n'(1) = 9 bytes
		const markdown = "# Héllo\n\nbody\n"
		var headings parseHeadingsResult

		BeforeEach(func() {
			headings = parseHeadings(markdown)
		})

		It("preserves the full heading text including multi-byte chars", func() {
			Expect(headings[0].Text).To(Equal("Héllo"))
		})

		It("drops multi-byte chars in the slug", func() {
			Expect(headings[0].Slug).To(Equal("hllo"))
		})

		It("byte_offset reflects the byte length of the heading line", func() {
			Expect(headings[0].ByteOffset).To(Equal(int64(9)))
		})
	})

	When("there are duplicate heading texts", func() {
		const markdown = "# Same\n\n## Same\n\n"
		var headings parseHeadingsResult

		BeforeEach(func() {
			headings = parseHeadings(markdown)
		})

		It("first occurrence has base slug", func() {
			Expect(headings[0].Slug).To(Equal("same"))
		})

		It("second occurrence has -1 suffix", func() {
			Expect(headings[1].Slug).To(Equal("same-1"))
		})
	})
})
