//revive:disable:dot-imports
package goldmarkrenderer_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/utils/goldmarkrenderer"
)

var _ = Describe("Hashtag rendering", func() {
	var renderer *goldmarkrenderer.GoldmarkRenderer

	BeforeEach(func() {
		renderer = &goldmarkrenderer.GoldmarkRenderer{}
	})

	render := func(source string) string {
		out, err := renderer.Render([]byte(source))
		Expect(err).NotTo(HaveOccurred())
		return string(out)
	}

	When("rendering a single hashtag in a paragraph", func() {
		var output string

		BeforeEach(func() {
			output = render("buy #milk today")
		})

		It("should render a wiki-hashtag custom element", func() {
			Expect(output).To(ContainSubstring(`<wiki-hashtag`))
		})

		It("should set the tag attribute to the normalized form", func() {
			Expect(output).To(ContainSubstring(`tag="milk"`))
		})

		It("should slot the leading `#` and original tag spelling as content", func() {
			Expect(output).To(ContainSubstring(`>#milk</wiki-hashtag>`))
		})
	})

	When("rendering a hashtag at start of paragraph", func() {
		It("should render the wiki-hashtag element", func() {
			out := render("#urgent ship it")
			Expect(out).To(ContainSubstring(`tag="urgent"`))
			Expect(out).To(ContainSubstring(`>#urgent</wiki-hashtag>`))
		})
	})

	When("rendering multiple hashtags", func() {
		It("should render each as a wiki-hashtag element", func() {
			out := render("#alpha and #beta")
			Expect(out).To(ContainSubstring(`tag="alpha"`))
			Expect(out).To(ContainSubstring(`tag="beta"`))
		})
	})

	When("hashtag is mid-word", func() {
		It("should not render a wiki-hashtag element", func() {
			out := render("foo#bar baz")
			Expect(out).NotTo(ContainSubstring(`wiki-hashtag`))
		})
	})

	When("hashtag is escaped with backslash", func() {
		It("should render literal #tag without a wiki-hashtag element", func() {
			out := render(`literal \#hash here`)
			Expect(out).NotTo(ContainSubstring(`wiki-hashtag`))
		})
	})

	When("hashtag appears in a markdown link target like [link](#anchor)", func() {
		It("should not render the anchor as a wiki-hashtag element", func() {
			out := render("see [link](#section) for more")
			Expect(out).NotTo(ContainSubstring(`wiki-hashtag`))
		})
	})

	When("hashtag appears inside an inline code span", func() {
		It("should not render a wiki-hashtag element", func() {
			out := render("code: `not a #tag` here")
			Expect(out).NotTo(ContainSubstring(`wiki-hashtag`))
		})
	})

	When("hashtag appears inside a fenced code block", func() {
		It("should not render a wiki-hashtag element", func() {
			out := render("before\n```\n#fence-tag\n```\nafter")
			Expect(out).NotTo(ContainSubstring(`wiki-hashtag`))
		})
	})

	When("hashtag has stylized casing", func() {
		It("should normalize the tag attribute but preserve the display text", func() {
			out := render("#Groceries are great")
			Expect(out).To(ContainSubstring(`tag="groceries"`))
			Expect(out).To(ContainSubstring(`>#Groceries</wiki-hashtag>`))
		})
	})

	When("hashtag is numeric", func() {
		It("should render a wiki-hashtag element", func() {
			Expect(render("year #2026 wraps")).To(ContainSubstring(`tag="2026"`))
		})
	})

	When("hashtag includes hyphen and underscore", func() {
		It("should preserve them in the tag attribute and display", func() {
			out := render("setup #home-lab and #project_alpha")
			Expect(out).To(ContainSubstring(`tag="home-lab"`))
			Expect(out).To(ContainSubstring(`>#home-lab</wiki-hashtag>`))
			Expect(out).To(ContainSubstring(`tag="project_alpha"`))
		})
	})

	When("text contains a single `#` followed by space", func() {
		It("should not render a wiki-hashtag element", func() {
			out := render("plain # not a tag")
			Expect(out).NotTo(ContainSubstring(`wiki-hashtag`))
		})
	})
})
