//revive:disable:dot-imports
package wikipage_test

import (
	"errors"
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

	Describe("ParseFrontmatterAndMarkdown", func() {
		var (
			content    string
			markdown   []byte
			frontmatter map[string]any
			err        error
		)

		JustBeforeEach(func() {
			markdown, frontmatter, err = wikipage.ParseFrontmatterAndMarkdown(content)
		})

		When("content has valid YAML frontmatter", func() {
			BeforeEach(func() {
				content = `---
title: Test Page
tags: [one, two]
---
This is the markdown content.`
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should parse frontmatter correctly", func() {
				Expect(frontmatter).To(HaveKeyWithValue("title", "Test Page"))
				Expect(frontmatter).To(HaveKey("tags"))
			})

			It("should extract markdown content", func() {
				Expect(string(markdown)).To(Equal("This is the markdown content."))
			})
		})

		When("content has no frontmatter", func() {
			BeforeEach(func() {
				content = "Just plain markdown"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return empty frontmatter", func() {
				Expect(frontmatter).To(BeEmpty())
			})

			It("should return all content as markdown", func() {
				Expect(string(markdown)).To(Equal("Just plain markdown"))
			})
		})

		When("content has invalid YAML frontmatter", func() {
			BeforeEach(func() {
				content = `---
title: Test
tags: [unclosed
---
Content`
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should wrap the error with context", func() {
				Expect(err.Error()).To(ContainSubstring("failed to parse frontmatter"))
			})
		})

		When("content is empty", func() {
			BeforeEach(func() {
				content = ""
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return empty markdown", func() {
				Expect(markdown).To(BeEmpty())
			})

			It("should return empty frontmatter", func() {
				Expect(frontmatter).To(BeEmpty())
			})
		})
	})

	Describe("ExecuteTemplatesOnMarkdown", func() {
		var (
			markdown         []byte
			frontmatter      map[string]any
			reader           wikipage.PageReader
			templateExecutor wikipage.IExecuteTemplate
			query            wikipage.IQueryFrontmatterIndex
			result           []byte
			err              error
		)

		BeforeEach(func() {
			markdown = []byte("# Original Content")
			frontmatter = map[string]any{"title": "Test"}
			reader = &mockPageReader{}
			templateExecutor = &mockTemplateExecutor{}
			query = &mockQueryFrontmatterIndex{}
		})

		JustBeforeEach(func() {
			result, err = wikipage.ExecuteTemplatesOnMarkdown(markdown, frontmatter, reader, templateExecutor, query)
		})

		When("template execution succeeds", func() {
			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return template-expanded markdown", func() {
				Expect(result).NotTo(BeEmpty())
				Expect(string(result)).To(Equal("# Expanded Content"))
			})
		})

		When("template executor fails", func() {
			BeforeEach(func() {
				templateExecutor = &mockTemplateExecutorError{}
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should wrap the error with context", func() {
				Expect(err.Error()).To(ContainSubstring("failed to execute templates"))
			})
		})

		When("template executor is nil", func() {
			BeforeEach(func() {
				templateExecutor = nil
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should indicate template executor is not initialized", func() {
				Expect(err.Error()).To(ContainSubstring("template executor is not initialized"))
			})
		})

		When("query is nil", func() {
			BeforeEach(func() {
				query = nil
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should indicate frontmatter index queryer is not initialized", func() {
				Expect(err.Error()).To(ContainSubstring("frontmatter index queryer is not initialized"))
			})
		})

		When("reader is nil", func() {
			BeforeEach(func() {
				reader = nil
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should indicate page reader is not initialized", func() {
				Expect(err.Error()).To(ContainSubstring("page reader is not initialized"))
			})
		})
	})

	Describe("RenderMarkdownToHTML", func() {
		var (
			markdown []byte
			renderer wikipage.IRenderMarkdownToHTML
			result   []byte
			err      error
		)

		BeforeEach(func() {
			markdown = []byte("# Test Heading")
			renderer = &mockRenderer{}
		})

		JustBeforeEach(func() {
			result, err = wikipage.RenderMarkdownToHTML(markdown, renderer)
		})

		When("renderer is nil", func() {
			BeforeEach(func() {
				renderer = nil
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should indicate renderer is not initialized", func() {
				Expect(err.Error()).To(ContainSubstring("renderer is not initialized"))
			})
		})

		When("rendering succeeds", func() {
			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return HTML content", func() {
				Expect(result).NotTo(BeEmpty())
				Expect(string(result)).To(Equal("<h1>Test Heading</h1>"))
			})
		})

		When("renderer fails", func() {
			BeforeEach(func() {
				renderer = &mockRendererError{}
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should wrap the error with context", func() {
				Expect(err.Error()).To(ContainSubstring("failed to render markdown to HTML"))
			})
		})
	})

	Describe("RenderPageWithTemplates integration", func() {
		var (
			content          string
			reader           wikipage.PageReader
			renderer         wikipage.IRenderMarkdownToHTML
			templateExecutor wikipage.IExecuteTemplate
			query            wikipage.IQueryFrontmatterIndex
			result           wikipage.RenderingResult
			err              error
		)

		BeforeEach(func() {
			content = `---
title: Integration Test
---
# Original Markdown`
			reader = &mockPageReader{}
			renderer = &mockRenderer{}
			templateExecutor = &mockTemplateExecutor{}
			query = &mockQueryFrontmatterIndex{}
		})

		JustBeforeEach(func() {
			result, err = wikipage.RenderPageWithTemplates(content, reader, renderer, templateExecutor, query)
		})

		When("full rendering pipeline succeeds", func() {
			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should populate HTML field", func() {
				Expect(result.HTML).NotTo(BeEmpty())
			})

			It("should populate RenderedMarkdown field with template-expanded markdown", func() {
				Expect(result.RenderedMarkdown).NotTo(BeEmpty())
				Expect(string(result.RenderedMarkdown)).To(Equal("# Expanded Content"))
			})

			It("should populate FrontmatterJSON field", func() {
				Expect(result.FrontmatterJSON).NotTo(BeEmpty())
			})

			It("should have valid frontmatter JSON", func() {
				Expect(string(result.FrontmatterJSON)).To(ContainSubstring("title"))
			})
		})

		When("renderer is nil", func() {
			BeforeEach(func() {
				renderer = nil
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should indicate renderer is not initialized", func() {
				Expect(err.Error()).To(ContainSubstring("renderer is not initialized"))
			})

			It("should not populate HTML field with error message", func() {
				Expect(result.HTML).To(BeEmpty())
			})
		})

		When("reader is nil", func() {
			BeforeEach(func() {
				reader = nil
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should indicate page reader is not initialized", func() {
				Expect(err.Error()).To(ContainSubstring("page reader is not initialized"))
			})
		})

		When("template executor is nil", func() {
			BeforeEach(func() {
				templateExecutor = nil
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should indicate template executor is not initialized", func() {
				Expect(err.Error()).To(ContainSubstring("template executor is not initialized"))
			})
		})

		When("query is nil", func() {
			BeforeEach(func() {
				query = nil
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should indicate frontmatter index queryer is not initialized", func() {
				Expect(err.Error()).To(ContainSubstring("frontmatter index queryer is not initialized"))
			})
		})

		When("frontmatter parsing fails", func() {
			BeforeEach(func() {
				content = `---
invalid: [unclosed
---
Content`
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should not populate HTML field with error message", func() {
				Expect(result.HTML).To(BeEmpty())
			})
		})

		When("template execution fails", func() {
			BeforeEach(func() {
				templateExecutor = &mockTemplateExecutorError{}
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should not populate HTML field with error message", func() {
				Expect(result.HTML).To(BeEmpty())
			})
		})
	})
})

// Mock implementations for testing

type mockRenderer struct{}

func (*mockRenderer) Render(input []byte) ([]byte, error) {
	return []byte("<h1>Test Heading</h1>"), nil
}

type mockRendererError struct{}

func (*mockRendererError) Render(input []byte) ([]byte, error) {
	return nil, errors.New("render error")
}

type mockTemplateExecutor struct{}

func (*mockTemplateExecutor) ExecuteTemplate(templateString string, fm wikipage.FrontMatter, reader wikipage.PageReader, query wikipage.IQueryFrontmatterIndex) ([]byte, error) {
	return []byte("# Expanded Content"), nil
}

type mockTemplateExecutorError struct{}

func (*mockTemplateExecutorError) ExecuteTemplate(templateString string, fm wikipage.FrontMatter, reader wikipage.PageReader, query wikipage.IQueryFrontmatterIndex) ([]byte, error) {
	return nil, errors.New("template execution error")
}

type mockPageReader struct{}

func (*mockPageReader) ReadFrontMatter(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	return identifier, wikipage.FrontMatter{"title": "Mock Page"}, nil
}

func (*mockPageReader) ReadMarkdown(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	return identifier, "# Mock Markdown", nil
}

type mockQueryFrontmatterIndex struct{}

func (*mockQueryFrontmatterIndex) QueryExactMatch(dottedKeyPath wikipage.DottedKeyPath, value wikipage.Value) []wikipage.PageIdentifier {
	return []wikipage.PageIdentifier{}
}

func (*mockQueryFrontmatterIndex) QueryKeyExistence(dottedKeyPath wikipage.DottedKeyPath) []wikipage.PageIdentifier {
	return []wikipage.PageIdentifier{}
}

func (*mockQueryFrontmatterIndex) QueryPrefixMatch(dottedKeyPath wikipage.DottedKeyPath, valuePrefix string) []wikipage.PageIdentifier {
	return []wikipage.PageIdentifier{}
}

func (*mockQueryFrontmatterIndex) GetValue(identifier wikipage.PageIdentifier, dottedKeyPath wikipage.DottedKeyPath) wikipage.Value {
	return ""
}