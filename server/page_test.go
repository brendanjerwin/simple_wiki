//revive:disable:dot-imports
package server

import (
	"net/http"
	"os"
	"time"

	"github.com/brendanjerwin/simple_wiki/common"

	"github.com/brendanjerwin/simple_wiki/utils"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/schollz/versionedtext"
)

var _ = Describe("Page Functions", func() {
	var (
		pathToData string
		s          *Site
	)

	BeforeEach(func() {
		pathToData = "testdata_page"
		err := os.MkdirAll(pathToData, 0755)
		Expect(err).NotTo(HaveOccurred())
		s = &Site{
			PathToData:       pathToData,
			MarkdownRenderer: &utils.GoldmarkRenderer{},
			Logger:           lumber.NewConsoleLogger(lumber.INFO),
		}
		err = s.InitializeIndexing()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_ = os.RemoveAll(pathToData)
	})

	Describe("Site.DirectoryList", func() {
		When("there are pages", func() {
			var pages []os.FileInfo

			BeforeEach(func() {
				req, _ := http.NewRequest("GET", "/", nil)
				p := s.OpenOrInit("testpage", req)
				err := p.Update("Some data")
				Expect(err).ToNot(HaveOccurred())
				time.Sleep(10 * time.Millisecond)

				p = s.OpenOrInit("testpage2", req)
				err = p.Update("A different bunch of data")
				Expect(err).ToNot(HaveOccurred())
				time.Sleep(10 * time.Millisecond)

				p = s.OpenOrInit("testpage3", req)
				err = p.Update("Not much else")
				Expect(err).ToNot(HaveOccurred())

				pages = s.DirectoryList()
			})

			It("should return all the pages", func() {
				Expect(pages).To(HaveLen(3))
			})

			It("should sort pages by modification time (oldest first)", func() {
				// Aider: don't change the order here. 3 should be _last_
				Expect(pages[0].Name()).To(Equal("testpage"))
				Expect(pages[1].Name()).To(Equal("testpage2"))
				Expect(pages[2].Name()).To(Equal("testpage3"))
			})
		})
	})

	Describe("Page update and render", func() {
		var p *Page

		BeforeEach(func() {
			req, _ := http.NewRequest("GET", "/", nil)
			p = s.OpenOrInit("testpage", req)
		})

		When("A page is updated", func() {
			BeforeEach(func() {
				err := p.Update("**bold**")
				Expect(err).ToNot(HaveOccurred())
			})

			It("should render correctly", func() {
				Expect(string(p.RenderedPage)).To(ContainSubstring("<p><strong>bold</strong></p>"))
			})

			When("the page is updated again", func() {
				BeforeEach(func() {
					err := p.Update("**bold** and *italic*")
					Expect(err).ToNot(HaveOccurred())
					err = p.Save()
					Expect(err).ToNot(HaveOccurred())
				})

				It("should render the new content", func() {
					Expect(string(p.RenderedPage)).To(ContainSubstring("<p><strong>bold</strong> and <em>italic</em></p>"))
				})

				When("the page is retrieved from disk", func() {
					var p2 *Page

					BeforeEach(func() {
						p2 = s.Open("testpage")
					})

					It("should have its content preserved", func() {
						Expect(p2.Text.GetCurrent()).To(Equal("**bold** and *italic*"))
					})

					When("the retrieved page is rendered", func() {
						BeforeEach(func() {
							p2.Render()
						})

						It("should render correctly", func() {
							Expect(string(p2.RenderedPage)).To(ContainSubstring("<p><strong>bold</strong> and <em>italic</em></p>"))
						})
					})
				})
			})
		})
	})

	Describe("Page.parse", func() {
		var (
			p           *Page
			frontmatter common.FrontMatter
			markdown    common.Markdown
			err         error
		)

		BeforeEach(func() {
			p = &Page{
				Site:       s,
				Identifier: "testpage",
				Text:       versionedtext.NewVersionedText(""),
			}
		})

		JustBeforeEach(func() {
			frontmatter, markdown, err = p.parse()
		})

		When("the page has no frontmatter", func() {
			BeforeEach(func() {
				p.Text.Update("Just some markdown content.")
			})

			It("should return empty frontmatter", func() {
				Expect(frontmatter).To(BeEmpty())
			})

			It("should return the full text as markdown", func() {
				Expect(string(markdown)).To(Equal("Just some markdown content."))
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the page has valid frontmatter", func() {
			BeforeEach(func() {
				content := `---
title: Test Page
tags: [one, two]
---
This is the markdown content.`
				p.Text.Update(content)
			})

			It("should correctly parse the frontmatter", func() {
				Expect(frontmatter).To(HaveKeyWithValue("title", "Test Page"))
				Expect(frontmatter).To(HaveKey("tags"))
				Expect(frontmatter["tags"]).To(BeEquivalentTo([]any{"one", "two"}))
			})

			It("should return the content after the frontmatter as markdown", func() {
				Expect(string(markdown)).To(Equal("This is the markdown content."))
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the page has invalid YAML in frontmatter", func() {
			BeforeEach(func() {
				content := `---
title: Test Page
tags: [one, two
---
This is the markdown content.`
				p.Text.Update(content)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to unmarshal frontmatter for testpage"))
			})
		})

		When("the content is empty", func() {
			BeforeEach(func() {
				p.Text.Update("")
			})

			It("should return empty frontmatter and markdown", func() {
				Expect(frontmatter).To(BeEmpty())
				Expect(string(markdown)).To(BeEmpty())
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("there is only frontmatter", func() {
			BeforeEach(func() {
				content := `---
title: Only Frontmatter
---
`
				p.Text.Update(content)
			})

			It("should parse the frontmatter", func() {
				Expect(frontmatter).To(HaveKeyWithValue("title", "Only Frontmatter"))
			})

			It("should return empty markdown", func() {
				Expect(string(markdown)).To(BeEmpty())
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the text contains '---' but not as a separator", func() {
			var content string
			BeforeEach(func() {
				content = `Here is some text.
---
And some more text. But this is not frontmatter.`
				p.Text.Update(content)
			})

			It("should return empty frontmatter", func() {
				Expect(frontmatter).To(BeEmpty())
			})

			It("should return the full text as markdown", func() {
				Expect(string(markdown)).To(Equal(content))
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
