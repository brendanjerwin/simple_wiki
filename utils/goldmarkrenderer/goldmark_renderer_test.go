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

			It("should render an HTML table wrapped in wiki-table", func() {
				expected := "<wiki-table><table>\n<thead>\n<tr>\n<th>A</th>\n<th>B</th>\n</tr>\n</thead>\n<tbody>\n<tr>\n<td>1</td>\n<td>2</td>\n</tr>\n</tbody>\n</table>\n</wiki-table>\n"
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

		When("rendering HTML with a wiki-checklist element", func() {
			BeforeEach(func() {
				source = []byte(`<wiki-checklist list-name="my-list" page="my-page"></wiki-checklist>`)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should preserve the wiki-checklist element and its attributes", func() {
				Expect(string(output)).To(ContainSubstring("<wiki-checklist"))
				Expect(string(output)).To(ContainSubstring(`list-name="my-list"`))
				Expect(string(output)).To(ContainSubstring(`page="my-page"`))
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

		When("rendering markdown with a collapsible heading (#^ syntax)", func() {
			BeforeEach(func() {
				source = []byte("#^ My Section")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should render a collapsible-heading element", func() {
				Expect(string(output)).To(ContainSubstring("<collapsible-heading heading-level=\"1\">"))
			})

			It("should render the heading with slot attribute", func() {
				Expect(string(output)).To(ContainSubstring(`slot="heading"`))
			})

			It("should render the heading with auto-generated id", func() {
				Expect(string(output)).To(ContainSubstring(`id="my-section"`))
			})

			It("should render the heading text", func() {
				Expect(string(output)).To(ContainSubstring("My Section"))
			})

			It("should close the collapsible-heading element", func() {
				Expect(string(output)).To(ContainSubstring("</collapsible-heading>"))
			})
		})

		When("rendering markdown with a level-2 collapsible heading (##^ syntax)", func() {
			BeforeEach(func() {
				source = []byte("##^ Subsection")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should render a collapsible-heading element with level 2", func() {
				Expect(string(output)).To(ContainSubstring("<collapsible-heading heading-level=\"2\">"))
			})

			It("should render an h2 element with slot attribute", func() {
				Expect(string(output)).To(ContainSubstring("<h2 slot=\"heading\""))
			})
		})

		When("rendering markdown with a collapsible heading and following content", func() {
			BeforeEach(func() {
				source = []byte("#^ My Section\n\nSome content here.\n\nMore content.")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should wrap the content inside the collapsible-heading element", func() {
				Expect(string(output)).To(ContainSubstring("<collapsible-heading heading-level=\"1\">"))
				Expect(string(output)).To(ContainSubstring("Some content here."))
				Expect(string(output)).To(ContainSubstring("More content."))
				Expect(string(output)).To(ContainSubstring("</collapsible-heading>"))
			})

			It("should put the content before the closing tag", func() {
				result := string(output)
				closingIdx := len(result) - len("</collapsible-heading>\n")
				Expect(result[closingIdx:]).To(Equal("</collapsible-heading>\n"))
				Expect(result[:closingIdx]).To(ContainSubstring("Some content here."))
			})
		})

		When("rendering markdown with a collapsible heading followed by a same-level heading", func() {
			BeforeEach(func() {
				source = []byte("#^ Section A\n\nContent A.\n\n# Section B\n\nContent B.")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should wrap only Section A content in the collapsible element", func() {
				result := string(output)
				Expect(result).To(ContainSubstring("Content A."))
				Expect(result).To(ContainSubstring("<h1 id=\"section-b\">Section B</h1>"))
				// Section B should be outside the collapsible-heading element
				closingIdx := len(result) - len("<h1 id=\"section-b\">Section B</h1>\n<p>Content B.</p>\n")
				Expect(result[closingIdx:]).To(ContainSubstring("Section B"))
				Expect(result[closingIdx:]).NotTo(ContainSubstring("collapsible-heading"))
			})
		})

		When("rendering markdown with a regular heading (no ^ marker)", func() {
			BeforeEach(func() {
				source = []byte("# Regular Heading")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should render as a normal heading without collapsible wrapper", func() {
				Expect(string(output)).NotTo(ContainSubstring("collapsible-heading"))
				Expect(string(output)).To(ContainSubstring("<h1"))
			})
		})

		When("rendering markdown with all collapsible heading levels", func() {
			BeforeEach(func() {
				source = []byte("######^ Deep Section")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should render a level-6 collapsible heading", func() {
				Expect(string(output)).To(ContainSubstring("<collapsible-heading heading-level=\"6\">"))
				Expect(string(output)).To(ContainSubstring("<h6 slot=\"heading\""))
			})
		})

		When("rendering a NOTE alert block", func() {
			BeforeEach(func() {
				source = []byte("> [!NOTE]\n> Useful information.")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should render a markdown-alert div", func() {
				Expect(string(output)).To(ContainSubstring(`class="markdown-alert markdown-alert-note"`))
			})

			It("should include role=note for accessibility", func() {
				Expect(string(output)).To(ContainSubstring(`role="note"`))
			})

			It("should render the alert title paragraph", func() {
				Expect(string(output)).To(ContainSubstring(`class="markdown-alert-title"`))
			})

			It("should render the Note label", func() {
				Expect(string(output)).To(ContainSubstring("Note"))
			})

			It("should render the alert content", func() {
				Expect(string(output)).To(ContainSubstring("Useful information."))
			})

			It("should not render a plain blockquote", func() {
				Expect(string(output)).NotTo(ContainSubstring("<blockquote>"))
			})
		})

		When("rendering a TIP alert block", func() {
			BeforeEach(func() {
				source = []byte("> [!TIP]\n> Do it this way.")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should render a markdown-alert-tip div", func() {
				Expect(string(output)).To(ContainSubstring(`class="markdown-alert markdown-alert-tip"`))
			})

			It("should render the Tip label", func() {
				Expect(string(output)).To(ContainSubstring("Tip"))
			})
		})

		When("rendering an IMPORTANT alert block", func() {
			BeforeEach(func() {
				source = []byte("> [!IMPORTANT]\n> Key information.")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should render a markdown-alert-important div", func() {
				Expect(string(output)).To(ContainSubstring(`class="markdown-alert markdown-alert-important"`))
			})

			It("should render the Important label", func() {
				Expect(string(output)).To(ContainSubstring("Important"))
			})
		})

		When("rendering a WARNING alert block", func() {
			BeforeEach(func() {
				source = []byte("> [!WARNING]\n> Watch out!")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should render a markdown-alert-warning div", func() {
				Expect(string(output)).To(ContainSubstring(`class="markdown-alert markdown-alert-warning"`))
			})

			It("should render the Warning label", func() {
				Expect(string(output)).To(ContainSubstring("Warning"))
			})
		})

		When("rendering a CAUTION alert block", func() {
			BeforeEach(func() {
				source = []byte("> [!CAUTION]\n> Risky action.")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should render a markdown-alert-caution div", func() {
				Expect(string(output)).To(ContainSubstring(`class="markdown-alert markdown-alert-caution"`))
			})

			It("should render the Caution label", func() {
				Expect(string(output)).To(ContainSubstring("Caution"))
			})
		})

		When("rendering an alert block with lowercase type marker", func() {
			BeforeEach(func() {
				source = []byte("> [!note]\n> Content.")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should still render as a note alert", func() {
				Expect(string(output)).To(ContainSubstring(`class="markdown-alert markdown-alert-note"`))
			})
		})

		When("rendering an alert block with multi-paragraph content", func() {
			BeforeEach(func() {
				source = []byte("> [!NOTE]\n> First paragraph.\n>\n> Second paragraph.")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should render both paragraphs inside the alert", func() {
				Expect(string(output)).To(ContainSubstring("First paragraph."))
				Expect(string(output)).To(ContainSubstring("Second paragraph."))
			})

			It("should not include the type marker as content", func() {
				Expect(string(output)).NotTo(ContainSubstring("[!NOTE]"))
			})
		})

		When("rendering a blockquote without an alert marker", func() {
			BeforeEach(func() {
				source = []byte("> Regular blockquote content.")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should render as a plain blockquote", func() {
				Expect(string(output)).To(ContainSubstring("<blockquote>"))
			})

			It("should not render as an alert", func() {
				Expect(string(output)).NotTo(ContainSubstring("markdown-alert"))
			})
		})

		When("rendering a blockquote with an unknown alert type", func() {
			BeforeEach(func() {
				source = []byte("> [!UNKNOWN]\n> Content.")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should render as a plain blockquote", func() {
				Expect(string(output)).To(ContainSubstring("<blockquote>"))
			})

			It("should not render as an alert", func() {
				Expect(string(output)).NotTo(ContainSubstring("markdown-alert"))
			})
		})

		When("rendering an alert where [!TYPE] has trailing text on the same line", func() {
			BeforeEach(func() {
				// "[!NOTE] extra" has content after the closing bracket — not a valid marker
				source = []byte("> [!NOTE] extra text\n> Content.")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should render as a plain blockquote", func() {
				Expect(string(output)).To(ContainSubstring("<blockquote>"))
			})
		})

		When("rendering an alert with the icon aria-hidden attribute", func() {
			BeforeEach(func() {
				source = []byte("> [!WARNING]\n> Be careful.")
			})

			It("should render the icon span with aria-hidden", func() {
				Expect(string(output)).To(ContainSubstring(`aria-hidden="true"`))
			})
		})
	})
})