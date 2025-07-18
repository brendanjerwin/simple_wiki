//revive:disable:dot-imports
package goldmarkrenderer_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/utils/goldmarkrenderer"
)

var _ = Describe("GoldmarkRenderer", func() {
	var renderer *goldmarkrenderer.GoldmarkRenderer

	BeforeEach(func() {
		renderer = &goldmarkrenderer.GoldmarkRenderer{}
	})

	It("should exist", func() {
		Expect(renderer).NotTo(BeNil())
	})

	Describe("Render", func() {
		var (
			source []byte
			output []byte
			err    error
		)

		JustBeforeEach(func() {
			output, err = renderer.Render(source)
		})

		When("rendering simple markdown", func() {
			BeforeEach(func() {
				source = []byte("test")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should render a paragraph", func() {
				Expect(string(output)).To(Equal("<p>test</p>\n"))
			})
		})

		When("rendering markdown with checkboxes", func() {
			BeforeEach(func() {
				source = []byte("- [x] Done\n- [ ] Not Done")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should render checkboxes as disabled input elements", func() {
				expected := "<ul>\n<li><input checked=\"\" disabled=\"\" type=\"checkbox\"/> Done</li>\n<li><input disabled=\"\" type=\"checkbox\"/> Not Done</li>\n</ul>\n"
				Expect(string(output)).To(Equal(expected))
			})
		})

		When("rendering markdown with emojis", func() {
			BeforeEach(func() {
				source = []byte("I am so happy :joy:")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should render the emoji", func() {
				expected := "<p>I am so happy 😂</p>\n"
				Expect(string(output)).To(Equal(expected))
			})
		})

		When("rendering markdown with a table", func() {
			BeforeEach(func() {
				source = []byte("| A | B |\n|---|---|\n| 1 | 2 |")
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should render an HTML table", func() {
				expected := "<table>\n<thead>\n<tr>\n<th>A</th>\n<th>B</th>\n</tr>\n</thead>\n<tbody>\n<tr>\n<td>1</td>\n<td>2</td>\n</tr>\n</tbody>\n</table>\n"
				Expect(string(output)).To(Equal(expected))
			})
		})

		When("rendering markdown with strikethrough", func() {
			BeforeEach(func() {
				source = []byte("~~deleted~~")
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should render a <del> tag", func() {
				expected := "<p><del>deleted</del></p>\n"
				Expect(string(output)).To(Equal(expected))
			})
		})

		When("rendering markdown with an autolink", func() {
			BeforeEach(func() {
				source = []byte("https://www.google.com")
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should render an <a> tag", func() {
				expected := "<p><a href=\"https://www.google.com\" rel=\"nofollow\">https://www.google.com</a></p>\n"
				Expect(string(output)).To(Equal(expected))
			})
		})

		When("rendering markdown with headings", func() {
			BeforeEach(func() {
				source = []byte("# heading 1\n## heading 2")
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should render h tags with ids", func() {
				expected := "<h1 id=\"heading-1\">heading 1</h1>\n<h2 id=\"heading-2\">heading 2</h2>\n"
				Expect(string(output)).To(Equal(expected))
			})
		})

		When("rendering markdown with hard wraps", func() {
			BeforeEach(func() {
				source = []byte("hello\nworld")
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should render a <br /> tag", func() {
				expected := "<p>hello<br/>\nworld</p>\n"
				Expect(string(output)).To(Equal(expected))
			})
		})

		When("rendering markdown with raw HTML", func() {
			BeforeEach(func() {
				source = []byte("<div>hello</div>")
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should allow safe HTML", func() {
				expected := "<div>hello</div>"
				Expect(string(output)).To(Equal(expected))
			})
		})

		When("rendering nil source", func() {
			BeforeEach(func() {
				source = nil
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should render empty string", func() {
				Expect(string(output)).To(BeEmpty())
			})
		})
	})
})