//revive:disable:dot-imports
package bleve_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/index/bleve"
	"github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// MockPageReader is a test implementation of wikipage.PageReader.
type MockPageReader struct {
	pages map[string]wikipage.FrontMatter
}

func NewMockPageReader() *MockPageReader {
	return &MockPageReader{
		pages: make(map[string]wikipage.FrontMatter),
	}
}

func (m *MockPageReader) AddPage(identifier string, fm wikipage.FrontMatter) {
	m.pages[identifier] = fm
}

func (m *MockPageReader) ReadFrontMatter(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	fm, exists := m.pages[string(identifier)]
	if !exists {
		return identifier, nil, errors.New("page not found")
	}
	return identifier, fm, nil
}

func (m *MockPageReader) ReadMarkdown(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	_, exists := m.pages[string(identifier)]
	if !exists {
		return identifier, "", errors.New("page not found")
	}
	return identifier, "Mock markdown content", nil
}

var _ = Describe("Index", func() {
	var (
		index               *bleve.Index
		mockReader          *MockPageReader
		frontmatterIndex    *frontmatter.Index
	)

	BeforeEach(func() {
		mockReader = NewMockPageReader()
		frontmatterIndex = frontmatter.NewIndex(mockReader)
		var err error
		index, err = bleve.NewIndex(mockReader, frontmatterIndex)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should exist", func() {
		Expect(index).NotTo(BeNil())
	})

	Describe("Query", func() {
		Describe("when searching with a partial title prefix", func() {
			var results []bleve.SearchResult
			var err error

			BeforeEach(func() {
				// Add page with title "container_2"
				mockReader.AddPage("container-2", wikipage.FrontMatter{
					"identifier": "container-2",
					"title":      "container_2",
				})
				Expect(frontmatterIndex.AddPageToIndex("container-2")).To(Succeed())
				Expect(index.AddPageToIndex("container-2")).To(Succeed())

				results, err = index.Query("container")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should find the page with the prefix match", func() {
				Expect(results).To(HaveLen(1))
				Expect(results[0].Identifier).To(Equal(wikipage.PageIdentifier("container-2")))
			})
		})

		Describe("when searching with case-insensitive prefix", func() {
			var results []bleve.SearchResult
			var err error

			BeforeEach(func() {
				// Add page with title "Container_2" (capital C)
				mockReader.AddPage("container-2", wikipage.FrontMatter{
					"identifier": "container-2",
					"title":      "Container_2",
				})
				Expect(frontmatterIndex.AddPageToIndex("container-2")).To(Succeed())
				Expect(index.AddPageToIndex("container-2")).To(Succeed())

				// Search with lowercase
				results, err = index.Query("container")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should find the page case-insensitively", func() {
				Expect(results).To(HaveLen(1))
				Expect(results[0].Identifier).To(Equal(wikipage.PageIdentifier("container-2")))
			})
		})

		Describe("when searching for a prefix of a single-word title", func() {
			var results []bleve.SearchResult
			var err error

			BeforeEach(func() {
				// Add page with title "ContainerFoo" - a single word that won't be tokenized
				mockReader.AddPage("containerfoo", wikipage.FrontMatter{
					"identifier": "containerfoo",
					"title":      "ContainerFoo",
				})
				Expect(frontmatterIndex.AddPageToIndex("containerfoo")).To(Succeed())
				Expect(index.AddPageToIndex("containerfoo")).To(Succeed())

				// Search for just "Container" - needs prefix matching to work
				results, err = index.Query("Container")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should find the page via prefix matching", func() {
				Expect(results).To(HaveLen(1))
				Expect(results[0].Identifier).To(Equal(wikipage.PageIdentifier("containerfoo")))
			})
		})

		Describe("when searching with exact title match", func() {
			var results []bleve.SearchResult
			var err error

			BeforeEach(func() {
				mockReader.AddPage("my-page", wikipage.FrontMatter{
					"identifier": "my-page",
					"title":      "My Test Page",
				})
				Expect(frontmatterIndex.AddPageToIndex("my-page")).To(Succeed())
				Expect(index.AddPageToIndex("my-page")).To(Succeed())

				results, err = index.Query("My Test Page")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should find the page", func() {
				Expect(results).To(HaveLen(1))
				Expect(results[0].Identifier).To(Equal(wikipage.PageIdentifier("my-page")))
			})
		})
	})
})
