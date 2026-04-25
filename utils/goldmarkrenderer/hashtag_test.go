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

		It("should render an anchor with the hashtag-pill class", func() {
			Expect(output).To(ContainSubstring(`class="hashtag-pill"`))
		})

		It("should target the search route with the URL-encoded hashtag", func() {
			Expect(output).To(ContainSubstring(`href="/search?q=%23milk"`))
		})

		It("should render the leading `#` and tag inside the anchor", func() {
			Expect(output).To(ContainSubstring(`>#milk</a>`))
		})
	})

	When("rendering a hashtag at start of paragraph", func() {
		It("should render the pill", func() {
			out := render("#urgent ship it")
			Expect(out).To(ContainSubstring(`href="/search?q=%23urgent"`))
			Expect(out).To(ContainSubstring(`>#urgent</a>`))
		})
	})

	When("rendering multiple hashtags", func() {
		It("should render each as a pill", func() {
			out := render("#alpha and #beta")
			Expect(out).To(ContainSubstring(`href="/search?q=%23alpha"`))
			Expect(out).To(ContainSubstring(`href="/search?q=%23beta"`))
		})
	})

	When("hashtag is mid-word", func() {
		It("should not render a pill", func() {
			out := render("foo#bar baz")
			Expect(out).NotTo(ContainSubstring(`hashtag-pill`))
		})
	})

	When("hashtag is escaped with backslash", func() {
		It("should render literal #tag without a pill", func() {
			out := render(`literal \#hash here`)
			Expect(out).NotTo(ContainSubstring(`hashtag-pill`))
		})
	})

	When("hashtag appears in a markdown link target like [link](#anchor)", func() {
		It("should not render the anchor as a pill", func() {
			out := render("see [link](#section) for more")
			Expect(out).NotTo(ContainSubstring(`hashtag-pill`))
		})
	})

	When("hashtag appears inside an inline code span", func() {
		It("should not render a pill", func() {
			out := render("code: `not a #tag` here")
			Expect(out).NotTo(ContainSubstring(`hashtag-pill`))
		})
	})

	When("hashtag appears inside a fenced code block", func() {
		It("should not render a pill", func() {
			out := render("before\n```\n#fence-tag\n```\nafter")
			Expect(out).NotTo(ContainSubstring(`hashtag-pill`))
		})
	})

	When("hashtag has stylized casing", func() {
		It("should normalize the href but preserve the display text", func() {
			out := render("#Groceries are great")
			Expect(out).To(ContainSubstring(`href="/search?q=%23groceries"`))
			Expect(out).To(ContainSubstring(`>#Groceries</a>`))
		})
	})

	When("hashtag is numeric", func() {
		It("should render a pill", func() {
			Expect(render("year #2026 wraps")).To(ContainSubstring(`href="/search?q=%232026"`))
		})
	})

	When("hashtag includes hyphen and underscore", func() {
		It("should preserve them in href and display", func() {
			out := render("setup #home-lab and #project_alpha")
			Expect(out).To(ContainSubstring(`href="/search?q=%23home-lab"`))
			Expect(out).To(ContainSubstring(`>#home-lab</a>`))
			Expect(out).To(ContainSubstring(`href="/search?q=%23project_alpha"`))
		})
	})

	When("text contains a single `#` followed by space", func() {
		It("should not render a pill", func() {
			out := render("plain # not a tag")
			Expect(out).NotTo(ContainSubstring(`hashtag-pill`))
		})
	})
})
