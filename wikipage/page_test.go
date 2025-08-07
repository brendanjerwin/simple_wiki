//revive:disable:dot-imports
package wikipage_test

import (
	"time"

	"github.com/brendanjerwin/simple_wiki/wikipage"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Page", func() {
	Describe("frontmatter and markdown parsing", func() {
		var (
			p           *wikipage.Page
			frontmatter wikipage.FrontMatter
			markdown    wikipage.Markdown
			err         error
		)

		BeforeEach(func() {
			p = &wikipage.Page{
				Identifier: "testpage",
				Text:       "",
			}
		})

		JustBeforeEach(func() {
			frontmatter, err = p.GetFrontMatter()
			if err == nil {
				markdown, err = p.GetMarkdown()
			}
		})

		When("the page has no frontmatter", func() {
			BeforeEach(func() {
				p.Text = "Just some markdown content."
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
				p.Text = content
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
				p.Text = content
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to parse frontmatter"))
			})
		})

		When("the content is empty", func() {
			BeforeEach(func() {
				p.Text = ""
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
				p.Text = content
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
				p.Text = content
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

	Describe("IsModifiedSince", func() {
		var (
			p              *wikipage.Page
			baseTime       time.Time
			result         bool
		)

		BeforeEach(func() {
			baseTime = time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
			p = &wikipage.Page{
				Identifier: "testpage",
				ModTime:    baseTime,
			}
		})

		When("checking against earlier timestamp", func() {
			BeforeEach(func() {
				earlierTimestamp := baseTime.Add(-1 * time.Hour).Unix()
				result = p.IsModifiedSince(earlierTimestamp)
			})

			It("should return true", func() {
				Expect(result).To(BeTrue())
			})
		})

		When("checking against later timestamp", func() {
			BeforeEach(func() {
				laterTimestamp := baseTime.Add(1 * time.Hour).Unix()
				result = p.IsModifiedSince(laterTimestamp)
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("checking against same timestamp", func() {
			BeforeEach(func() {
				sameTimestamp := baseTime.Unix()
				result = p.IsModifiedSince(sameTimestamp)
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("ModTime is zero", func() {
			BeforeEach(func() {
				p.ModTime = time.Time{}
				result = p.IsModifiedSince(time.Now().Unix())
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})
	})
})