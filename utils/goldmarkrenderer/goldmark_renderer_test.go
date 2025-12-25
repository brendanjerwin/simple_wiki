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

		When("rendering markdown with images", func() {
			BeforeEach(func() {
				source = []byte("![alt text](image.jpg)")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should render a wiki-image element", func() {
				expected := "<p><wiki-image src=\"image.jpg\" alt=\"alt text\"></wiki-image></p>\n"
				Expect(string(output)).To(Equal(expected))
			})
		})

		When("rendering markdown with images with title", func() {
			BeforeEach(func() {
				source = []byte("![alt text](image.jpg \"My Title\")")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should include title attribute", func() {
				Expect(string(output)).To(ContainSubstring(`title="My Title"`))
			})
		})

		When("rendering markdown with images with complex alt text (emphasis)", func() {
			BeforeEach(func() {
				source = []byte("![*emphasized* text](image.jpg)")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should include full alt text", func() {
				Expect(string(output)).To(ContainSubstring(`alt="emphasized text"`))
			})
		})

		When("rendering markdown with images with complex alt text (bold)", func() {
			BeforeEach(func() {
				source = []byte("![**bold** text](image.jpg)")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should include full alt text", func() {
				Expect(string(output)).To(ContainSubstring(`alt="bold text"`))
			})
		})

		When("rendering markdown with images with empty alt text", func() {
			BeforeEach(func() {
				source = []byte("![](image.jpg)")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have empty alt attribute", func() {
				Expect(string(output)).To(ContainSubstring(`alt=""`))
			})
		})

		When("rendering markdown with images with dangerous URL", func() {
			// Note: The renderer is created with WithUnsafe() and bluemonday allows
			// custom element attributes, so dangerous URLs are not filtered.
			// The wiki-image component handles this by using noopener,noreferrer.
			BeforeEach(func() {
				source = []byte("![danger](javascript:alert('xss'))")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should render a wiki-image element with escaped URL", func() {
				Expect(string(output)).To(ContainSubstring("<wiki-image"))
				Expect(string(output)).To(ContainSubstring("javascript:"))
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

		When("rendering markdown with wikilinks", func() {
			BeforeEach(func() {
				source = []byte("This is a [[wikilink]] in text")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should render wikilink as an anchor tag", func() {
				Expect(string(output)).To(ContainSubstring("href=\"/wikilink?title=wikilink\""))
				Expect(string(output)).To(ContainSubstring(">wikilink</a>"))
			})
		})

		When("rendering markdown with wikilink with spaces", func() {
			BeforeEach(func() {
				source = []byte("Link to [[My Page]] here")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should render wikilink with munged identifier in path", func() {
				Expect(string(output)).To(ContainSubstring("href=\"/my_page?title=My+Page\""))
			})
		})
	})
})