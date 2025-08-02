package frontmatter

import (
	"errors"
	"fmt"
	"testing"

	"github.com/brendanjerwin/simple_wiki/wikipage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestFrontmatterIndex(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Frontmatter Index Suite")
}

// MockPageReader for testing
type MockPageReader struct {
	pages map[wikipage.PageIdentifier]map[string]any
	err   error
}

func NewMockPageReader() *MockPageReader {
	return &MockPageReader{
		pages: make(map[wikipage.PageIdentifier]map[string]any),
	}
}

func (m *MockPageReader) SetPageFrontmatter(identifier wikipage.PageIdentifier, frontmatter map[string]any) {
	m.pages[identifier] = frontmatter
}

func (m *MockPageReader) SetError(err error) {
	m.err = err
}

func (m *MockPageReader) ReadFrontMatter(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, map[string]any, error) {
	if m.err != nil {
		return identifier, nil, m.err
	}
	
	frontmatter, exists := m.pages[identifier]
	if !exists {
		return identifier, nil, errors.New("page not found")
	}
	
	return identifier, frontmatter, nil
}

func (*MockPageReader) ReadMarkdown(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	// Not used by frontmatter indexing, but required by interface
	return identifier, "", nil
}

var _ = Describe("Index", func() {
	var (
		index      *Index
		pageReader *MockPageReader
	)

	BeforeEach(func() {
		pageReader = NewMockPageReader()
		index = NewIndex(pageReader)
	})

	Describe("NewIndex", func() {
		It("should create a new index", func() {
			Expect(index).NotTo(BeNil())
			Expect(index.InvertedIndex).NotTo(BeNil())
			Expect(index.PageKeyMap).NotTo(BeNil())
			Expect(index.pageReader).To(Equal(pageReader))
		})
	})

	Describe("GetIndexName", func() {
		It("should return 'frontmatter'", func() {
			Expect(index.GetIndexName()).To(Equal("frontmatter"))
		})
	})

	Describe("AddPageToIndex", func() {
		When("adding a page with simple frontmatter", func() {
			var err error

			BeforeEach(func() {
				pageReader.SetPageFrontmatter("test-page", map[string]any{
					"title": "Test Page",
					"tags":  []any{"tag1", "tag2"},
				})
				err = index.AddPageToIndex("test-page")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should index the title", func() {
				matches := index.QueryExactMatch("title", "Test Page")
				Expect(matches).To(ContainElement("test_page"))
			})

			It("should index the tags", func() {
				matches := index.QueryExactMatch("tags", "tag1")
				Expect(matches).To(ContainElement("test_page"))
				matches = index.QueryExactMatch("tags", "tag2")
				Expect(matches).To(ContainElement("test_page"))
			})
		})

		When("adding a page with nested frontmatter", func() {
			var err error

			BeforeEach(func() {
				pageReader.SetPageFrontmatter("nested-page", map[string]any{
					"meta": map[string]any{
						"author": "John Doe",
						"date":   "2023-01-01",
					},
					"config": map[string]any{
						"settings": map[string]any{
							"theme": "dark",
						},
					},
				})
				err = index.AddPageToIndex("nested-page")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should index nested keys", func() {
				matches := index.QueryExactMatch("meta.author", "John Doe")
				Expect(matches).To(ContainElement("nested_page"))
				matches = index.QueryExactMatch("meta.date", "2023-01-01")
				Expect(matches).To(ContainElement("nested_page"))
				matches = index.QueryExactMatch("config.settings.theme", "dark")
				Expect(matches).To(ContainElement("nested_page"))
			})

			It("should support key existence queries for intermediate paths", func() {
				matches := index.QueryKeyExistence("meta")
				Expect(matches).To(ContainElement("nested_page"))
				matches = index.QueryKeyExistence("config.settings")
				Expect(matches).To(ContainElement("nested_page"))
			})
		})

		When("frontmatter contains circular references", func() {
			var err error

			BeforeEach(func() {
				// Create a circular reference in the frontmatter
				circularMap := make(map[string]any)
				innerMap := make(map[string]any)
				circularMap["inner"] = innerMap
				innerMap["outer"] = circularMap // This creates a circular reference

				pageReader.SetPageFrontmatter("circular-page", circularMap)
				err = index.AddPageToIndex("circular-page")
			})

			It("should not cause infinite recursion", func() {
				// This test should not hang - if it does, we have infinite recursion
				Expect(err).NotTo(HaveOccurred())
			})

			It("should still index non-circular parts", func() {
				// Add some non-circular data to verify partial indexing works
				mixedMap := map[string]any{
					"title": "Mixed Page",
					"meta": map[string]any{
						"author": "Test Author",
					},
				}
				
				// Create circular reference in a nested part
				circularPart := make(map[string]any)
				innerPart := make(map[string]any)
				circularPart["inner"] = innerPart
				innerPart["outer"] = circularPart
				mixedMap["circular"] = circularPart

				pageReader.SetPageFrontmatter("mixed-page", mixedMap)
				err := index.AddPageToIndex("mixed-page")
				
				Expect(err).NotTo(HaveOccurred())
				
				// Should still index the non-circular parts
				matches := index.QueryExactMatch("title", "Mixed Page")
				Expect(matches).To(ContainElement("mixed_page"))
				matches = index.QueryExactMatch("meta.author", "Test Author")
				Expect(matches).To(ContainElement("mixed_page"))
			})
		})

		When("frontmatter contains deeply nested structures", func() {
			var err error

			BeforeEach(func() {
				// Create a very deep nesting structure
				deepMap := make(map[string]any)
				current := deepMap
				
				// Create 20 levels of nesting
				for i := 0; i < 20; i++ {
					nested := make(map[string]any)
					current[fmt.Sprintf("level%d", i)] = nested
					current = nested
				}
				current["final"] = "deep_value"

				pageReader.SetPageFrontmatter("deep-page", deepMap)
				err = index.AddPageToIndex("deep-page")
			})

			It("should handle deep nesting without stack overflow", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should index the deeply nested value", func() {
				expectedPath := "level0.level1.level2.level3.level4.level5.level6.level7.level8.level9.level10.level11.level12.level13.level14.level15.level16.level17.level18.level19.final"
				matches := index.QueryExactMatch(expectedPath, "deep_value")
				Expect(matches).To(ContainElement("deep_page"))
			})
		})

		When("page reader returns an error", func() {
			var err error

			BeforeEach(func() {
				pageReader.SetError(errors.New("read error"))
				err = index.AddPageToIndex("error-page")
			})

			It("should return the error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("read error"))
			})
		})
	})

	Describe("RemovePageFromIndex", func() {
		When("removing an indexed page", func() {
			var err error

			BeforeEach(func() {
				pageReader.SetPageFrontmatter("remove-page", map[string]any{
					"title": "Remove Me",
					"category": "test",
				})
				index.AddPageToIndex("remove-page")
				err = index.RemovePageFromIndex("remove-page")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should remove the page from query results", func() {
				matches := index.QueryExactMatch("title", "Remove Me")
				Expect(matches).NotTo(ContainElement("remove_page"))
				matches = index.QueryExactMatch("category", "test")
				Expect(matches).NotTo(ContainElement("remove_page"))
			})
		})
	})

	Describe("Query methods", func() {
		BeforeEach(func() {
			// Set up test data
			pageReader.SetPageFrontmatter("page1", map[string]any{
				"title": "First Page",
				"category": "docs",
				"tags": []any{"important", "guide"},
			})
			pageReader.SetPageFrontmatter("page2", map[string]any{
				"title": "Second Page", 
				"category": "docs",
				"tags": []any{"tutorial"},
			})
			pageReader.SetPageFrontmatter("page3", map[string]any{
				"title": "Third Page",
				"category": "blog",
			})
			
			index.AddPageToIndex("page1")
			index.AddPageToIndex("page2")
			index.AddPageToIndex("page3")
		})

		Describe("QueryExactMatch", func() {
			It("should return pages with exact value matches", func() {
				matches := index.QueryExactMatch("category", "docs")
				Expect(matches).To(ConsistOf("page1", "page2"))
				
				matches = index.QueryExactMatch("category", "blog")
				Expect(matches).To(ConsistOf("page3"))
				
				matches = index.QueryExactMatch("tags", "important")
				Expect(matches).To(ConsistOf("page1"))
			})
		})

		Describe("QueryKeyExistence", func() {
			It("should return pages that have the specified key", func() {
				matches := index.QueryKeyExistence("category")
				Expect(matches).To(ConsistOf("page1", "page2", "page3"))
				
				matches = index.QueryKeyExistence("tags")
				Expect(matches).To(ConsistOf("page1", "page2"))
			})
		})

		Describe("QueryPrefixMatch", func() {
			It("should return pages with values matching the prefix", func() {
				matches := index.QueryPrefixMatch("title", "First")
				Expect(matches).To(ConsistOf("page1"))
				
				matches = index.QueryPrefixMatch("title", "Page")
				Expect(matches).To(BeEmpty()) // No titles start with "Page"
				
				matches = index.QueryPrefixMatch("category", "do")
				Expect(matches).To(ConsistOf("page1", "page2"))
			})
		})

		Describe("GetValue", func() {
			It("should return the value for a given page and key", func() {
				value := index.GetValue("page1", "title")
				Expect(value).To(Equal("First Page"))
				
				value = index.GetValue("page2", "category")
				Expect(value).To(Equal("docs"))
				
				value = index.GetValue("page3", "nonexistent")
				Expect(value).To(Equal(""))
			})
		})
	})
})