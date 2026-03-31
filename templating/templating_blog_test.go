//revive:disable:dot-imports
package templating_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/templating"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

var _ = Describe("BuildBlog", func() {
	var (
		mockSite  *mockPageReader
		mockIndex *mockFrontmatterIndex
		blogFunc  func(string, int) string
		result    string
	)

	BeforeEach(func() {
		mockSite = &mockPageReader{
			pages:    map[string]wikipage.FrontMatter{},
			markdown: map[string]string{},
		}
		mockIndex = &mockFrontmatterIndex{
			index:  map[string]map[string][]wikipage.PageIdentifier{},
			values: map[string]map[string]string{},
		}
		blogFunc = templating.BuildBlog(templating.TemplateContext{
			Identifier: "my_blog",
			Map:        map[string]any{},
		}, mockIndex, mockSite)
	})

	Describe("when there are no blog posts", func() {
		BeforeEach(func() {
			result = blogFunc("my_blog", 10)
		})

		It("should return a wiki-blog element", func() {
			Expect(result).To(ContainSubstring("<wiki-blog"))
		})

		It("should include the blog-id attribute", func() {
			Expect(result).To(ContainSubstring(`blog-id="my_blog"`))
		})

		It("should include the max-articles attribute", func() {
			Expect(result).To(ContainSubstring(`max-articles="10"`))
		})

		It("should not contain any blog article spans", func() {
			Expect(result).NotTo(ContainSubstring(`class="blog-article"`))
		})
	})

	Describe("when a post has no title", func() {
		BeforeEach(func() {
			mockIndex.index["blog.identifier"] = map[string][]wikipage.PageIdentifier{
				"my_blog": {"post_without_title"},
			}
			mockIndex.values["post_without_title"] = map[string]string{}

			result = blogFunc("my_blog", 10)
		})

		It("should fall back to the page identifier as title", func() {
			Expect(result).To(ContainSubstring("post_without_title"))
		})
	})

	Describe("when a post has an empty summary and no markdown", func() {
		BeforeEach(func() {
			mockIndex.index["blog.identifier"] = map[string][]wikipage.PageIdentifier{
				"my_blog": {"post_a"},
			}
			mockIndex.values["post_a"] = map[string]string{
				"title":                 "Post A",
				"blog.summary_markdown": "",
			}

			result = blogFunc("my_blog", 10)
		})

		It("should not include a snippet element", func() {
			Expect(result).NotTo(ContainSubstring(`class="snippet"`))
		})
	})

	Describe("when a post has markdown content as its body", func() {
		BeforeEach(func() {
			mockSite.markdown["post_with_markdown"] = "## My Header\n\nSome **bold** and _italic_ text"
			mockIndex.index["blog.identifier"] = map[string][]wikipage.PageIdentifier{
				"my_blog": {"post_with_markdown"},
			}
			mockIndex.values["post_with_markdown"] = map[string]string{
				"title": "Post With Markdown",
			}

			result = blogFunc("my_blog", 10)
		})

		It("should include a snippet", func() {
			Expect(result).To(ContainSubstring(`class="snippet"`))
		})

		It("should strip heading markdown syntax from snippet", func() {
			Expect(result).NotTo(ContainSubstring("##"))
		})

		It("should strip bold markdown syntax from snippet", func() {
			Expect(result).NotTo(ContainSubstring("**"))
		})

		It("should strip italic markdown syntax from snippet", func() {
			Expect(result).NotTo(ContainSubstring("_italic_"))
		})
	})

	Describe("when a post has a very long summary", func() {
		var longSummary string

		BeforeEach(func() {
			longSummary = strings.Repeat("a", 300)
			mockIndex.index["blog.identifier"] = map[string][]wikipage.PageIdentifier{
				"my_blog": {"post_long"},
			}
			mockIndex.values["post_long"] = map[string]string{
				"title":                 "Long Post",
				"blog.summary_markdown": longSummary,
			}

			result = blogFunc("my_blog", 10)
		})

		It("should include a snippet", func() {
			Expect(result).To(ContainSubstring(`class="snippet"`))
		})

		It("should truncate the snippet to at most 200 characters", func() {
			// Extract the snippet content from the result
			snippetStart := strings.Index(result, `class="snippet">`) + len(`class="snippet">`)
			snippetEnd := strings.Index(result[snippetStart:], "</span>")
			Expect(snippetEnd).To(BeNumerically(">", 0))
			snippet := result[snippetStart : snippetStart+snippetEnd]
			Expect(len([]rune(snippet))).To(BeNumerically("<=", 200))
		})
	})

	Describe("when a post has a non-safe external URL", func() {
		BeforeEach(func() {
			mockIndex.index["blog.identifier"] = map[string][]wikipage.PageIdentifier{
				"my_blog": {"post_xss"},
			}
			mockIndex.values["post_xss"] = map[string]string{
				"title":               "XSS Post",
				"blog.external_url":   "javascript:alert(1)",
			}

			result = blogFunc("my_blog", 10)
		})

		It("should not use the javascript: URL as the link href", func() {
			Expect(result).NotTo(ContainSubstring(`href="javascript:`))
		})

		It("should link to the wiki page instead", func() {
			Expect(result).To(ContainSubstring(`href="/post_xss"`))
		})
	})

	Describe("when a post has a data: scheme URL", func() {
		BeforeEach(func() {
			mockIndex.index["blog.identifier"] = map[string][]wikipage.PageIdentifier{
				"my_blog": {"post_data"},
			}
			mockIndex.values["post_data"] = map[string]string{
				"title":             "Data Post",
				"blog.external_url": "data:text/html,<h1>hello</h1>",
			}

			result = blogFunc("my_blog", 10)
		})

		It("should not use the data: URL as the link href", func() {
			Expect(result).NotTo(ContainSubstring(`href="data:`))
		})

		It("should link to the wiki page instead", func() {
			Expect(result).To(ContainSubstring(`href="/post_data"`))
		})
	})

	Describe("when a post has a safe https external URL", func() {
		BeforeEach(func() {
			mockIndex.index["blog.identifier"] = map[string][]wikipage.PageIdentifier{
				"my_blog": {"post_external"},
			}
			mockIndex.values["post_external"] = map[string]string{
				"title":             "External Post",
				"blog.external_url": "https://example.com/article",
			}

			result = blogFunc("my_blog", 10)
		})

		It("should use the external URL as the link href", func() {
			Expect(result).To(ContainSubstring(`href="https://example.com/article"`))
		})

		It("should include a wiki link alongside the external link", func() {
			Expect(result).To(ContainSubstring(`class="wiki-link"`))
		})
	})

	Describe("when a post has a subtitle", func() {
		BeforeEach(func() {
			mockIndex.index["blog.identifier"] = map[string][]wikipage.PageIdentifier{
				"my_blog": {"post_sub"},
			}
			mockIndex.values["post_sub"] = map[string]string{
				"title":          "Post With Subtitle",
				"blog.subtitle":  "A helpful subtitle",
			}

			result = blogFunc("my_blog", 10)
		})

		It("should include the subtitle", func() {
			Expect(result).To(ContainSubstring("A helpful subtitle"))
		})

		It("should wrap the subtitle in a subtitle span", func() {
			Expect(result).To(ContainSubstring(`class="subtitle"`))
		})
	})

	Describe("when a post has a published date", func() {
		BeforeEach(func() {
			mockIndex.index["blog.identifier"] = map[string][]wikipage.PageIdentifier{
				"my_blog": {"post_dated"},
			}
			mockIndex.values["post_dated"] = map[string]string{
				"title":                "Dated Post",
				"blog.published-date":  "2024-01-15",
			}

			result = blogFunc("my_blog", 10)
		})

		It("should include the published date", func() {
			Expect(result).To(ContainSubstring("2024-01-15"))
		})

		It("should wrap the date in a date span", func() {
			Expect(result).To(ContainSubstring(`class="date"`))
		})
	})

	Describe("when hide-new-post is configured as a bool", func() {
		BeforeEach(func() {
			blogFunc = templating.BuildBlog(templating.TemplateContext{
				Identifier: "my_blog",
				Map: map[string]any{
					"blog": map[string]any{
						"hide-new-post": true,
					},
				},
			}, mockIndex, mockSite)

			result = blogFunc("my_blog", 10)
		})

		It("should include hide-new-post attribute on wiki-blog element", func() {
			Expect(result).To(ContainSubstring(" hide-new-post"))
		})
	})

	Describe("when hide-new-post is configured as string \"true\"", func() {
		BeforeEach(func() {
			blogFunc = templating.BuildBlog(templating.TemplateContext{
				Identifier: "my_blog",
				Map: map[string]any{
					"blog": map[string]any{
						"hide-new-post": "true",
					},
				},
			}, mockIndex, mockSite)

			result = blogFunc("my_blog", 10)
		})

		It("should include hide-new-post attribute on wiki-blog element", func() {
			Expect(result).To(ContainSubstring(" hide-new-post"))
		})
	})

	Describe("when hide-new-post is false", func() {
		BeforeEach(func() {
			blogFunc = templating.BuildBlog(templating.TemplateContext{
				Identifier: "my_blog",
				Map: map[string]any{
					"blog": map[string]any{
						"hide-new-post": false,
					},
				},
			}, mockIndex, mockSite)

			result = blogFunc("my_blog", 10)
		})

		It("should not include hide-new-post attribute on wiki-blog element", func() {
			Expect(result).NotTo(ContainSubstring(" hide-new-post"))
		})
	})

	Describe("when a snippet has markdown heading syntax", func() {
		BeforeEach(func() {
			mockIndex.index["blog.identifier"] = map[string][]wikipage.PageIdentifier{
				"my_blog": {"post_heading"},
			}
			mockIndex.values["post_heading"] = map[string]string{
				"title":                 "Heading Post",
				"blog.summary_markdown": "## Section Title\n\nSome content here",
			}

			result = blogFunc("my_blog", 10)
		})

		It("should strip heading syntax from snippet", func() {
			Expect(result).NotTo(ContainSubstring("##"))
		})

		It("should preserve the heading text content", func() {
			Expect(result).To(ContainSubstring("Section Title"))
		})
	})
})
