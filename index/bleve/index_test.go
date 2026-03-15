//revive:disable:dot-imports
package bleve_test

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/blevesearch/bleve/search"
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

	Describe("RemovePageFromIndex", func() {
		var err error

		BeforeEach(func() {
			// Add a page
			mockReader.AddPage("test-page", wikipage.FrontMatter{
				"identifier": "test-page",
				"title":      "Test Page",
			})
			Expect(frontmatterIndex.AddPageToIndex("test-page")).To(Succeed())
			Expect(index.AddPageToIndex("test-page")).To(Succeed())

			// Verify it's there first
			results, queryErr := index.Query("Test Page")
			Expect(queryErr).NotTo(HaveOccurred())
			Expect(results).NotTo(BeEmpty())

			// Remove it
			err = index.RemovePageFromIndex("test-page")
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should remove from the index", func() {
			results, err := index.Query("Test Page")
			Expect(err).NotTo(HaveOccurred())
			Expect(results).To(BeEmpty())
		})
	})

	Describe("calculateFragmentWindow", func() {
		var idx *bleve.Index

		BeforeEach(func() {
			var err error
			idx, err = bleve.NewIndex(mockReader, frontmatterIndex)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there are no locations", func() {
			var start, end int
			var contentText string

			BeforeEach(func() {
				contentText = "This is a test content that is longer than the max fragment length"
				start, end = bleve.CalculateFragmentWindowForTest(idx, contentText, []*search.Location{})
			})

			It("should start at zero", func() {
				Expect(start).To(Equal(0))
			})

			It("should end at max fragment length", func() {
				Expect(end).To(Equal(min(len(contentText), 200))) // maxFragmentLength is 200
			})
		})

		Context("when matches fit within fragment with context", func() {
			var start, end int
			var contentText string

			BeforeEach(func() {
				contentText = strings.Repeat("x", 500)
				locations := []*search.Location{
					{Start: 100, End: 110},
					{Start: 120, End: 130},
				}
				start, end = bleve.CalculateFragmentWindowForTest(idx, contentText, locations)
			})

			It("should have valid boundaries", func() {
				Expect(start).To(BeNumerically(">=", 0))
				Expect(end).To(BeNumerically("<=", len(contentText)))
			})

			It("should have max fragment length", func() {
				Expect(end - start).To(Equal(200)) // maxFragmentLength
			})

			It("should include both matches", func() {
				// Should include both matches
				Expect(start).To(BeNumerically("<=", 100))
				Expect(end).To(BeNumerically(">=", 130))
			})
		})

		Context("when matches span too wide", func() {
			var start, end int
			var contentText string

			BeforeEach(func() {
				contentText = strings.Repeat("x", 500)
				locations := []*search.Location{
					{Start: 100, End: 110},
					{Start: 400, End: 410}, // Too far from first match
				}
				start, end = bleve.CalculateFragmentWindowForTest(idx, contentText, locations)
			})

			It("should have valid start boundary", func() {
				Expect(start).To(BeNumerically(">=", 0))
			})

			It("should have max fragment length", func() {
				Expect(end - start).To(Equal(200)) // maxFragmentLength
			})

			It("should focus on first match", func() {
				// Should be near first match
				Expect(start).To(BeNumerically("<=", 100))
				Expect(start).To(BeNumerically(">=", 50)) // contextPadding is 50
			})
		})

		Context("when content is shorter than max fragment", func() {
			var start, end int
			var contentText string

			BeforeEach(func() {
				contentText = "Short content"
				locations := []*search.Location{{Start: 0, End: 5}}
				start, end = bleve.CalculateFragmentWindowForTest(idx, contentText, locations)
			})

			It("should start at zero", func() {
				Expect(start).To(Equal(0))
			})

			It("should end at content length", func() {
				Expect(end).To(Equal(len(contentText)))
			})
		})

		Context("when location data points beyond content (stale index)", func() {
			var start, end int

			BeforeEach(func() {
				// Simulate stale index: locations point to position 101 but content is empty
				contentText := ""
				locations := []*search.Location{{Start: 101, End: 110}}
				start, end = bleve.CalculateFragmentWindowForTest(idx, contentText, locations)
			})

			It("should have start <= end (no panic)", func() {
				Expect(start).To(BeNumerically("<=", end))
			})

			It("should have valid boundaries for empty content", func() {
				Expect(start).To(Equal(0))
				Expect(end).To(Equal(0))
			})
		})

		Context("when location data spans wide and points beyond content (stale index)", func() {
			var start, end int

			BeforeEach(func() {
				// This triggers the else branch where span is too wide
				// firstMatch=500, lastMatch=1000, matchSpan=500, totalNeeded=600 > 200
				// fragmentStart = max(0, 500-50) = 450
				// fragmentEnd = min(0, 450+200) = 0
				// Without fix: [450:0] would panic!
				contentText := ""
				locations := []*search.Location{
					{Start: 500, End: 510},
					{Start: 990, End: 1000}, // Wide span to trigger else branch
				}
				start, end = bleve.CalculateFragmentWindowForTest(idx, contentText, locations)
			})

			It("should have start <= end (no panic)", func() {
				Expect(start).To(BeNumerically("<=", end))
			})

			It("should have valid boundaries for empty content", func() {
				Expect(start).To(Equal(0))
				Expect(end).To(Equal(0))
			})
		})

		Context("when location data points beyond short content", func() {
			var start, end int
			var contentText string

			BeforeEach(func() {
				// Simulate stale index: locations point to position 500 but content is only 10 chars
				contentText = "short text"
				locations := []*search.Location{{Start: 500, End: 510}}
				start, end = bleve.CalculateFragmentWindowForTest(idx, contentText, locations)
			})

			It("should have start <= end (no panic)", func() {
				Expect(start).To(BeNumerically("<=", end))
			})

			It("should clamp to content boundaries", func() {
				Expect(start).To(BeNumerically(">=", 0))
				Expect(end).To(BeNumerically("<=", len(contentText)))
			})
		})

		Context("when the calculated boundary falls mid-rune in multi-byte UTF-8 content", func() {
			var start, end int
			// Build a string where a 3-byte rune straddles a fragment boundary.
			// Each Japanese character ('日', '本', '語') is 3 bytes.
			// We construct content so that adding contextPadding to a match position
			// would naturally land on a continuation byte of one of these runes.
			var contentText string

			BeforeEach(func() {
				// '日' is 3 bytes (0xE6 0x97 0xA5), placed at bytes [198, 201).
				// A match at byte 50 with contextPadding=50 => fragmentStart=0, fragmentEnd=200.
				// fragmentEnd=200 lands on the third byte (0xA5) of '日', which is a continuation byte.
				// Without the fix, slicing contentText[0:200] would produce invalid UTF-8.
				contentText = strings.Repeat("a", 198) + "日本語" + strings.Repeat("b", 300)
				locations := []*search.Location{{Start: 50, End: 55}}
				start, end = bleve.CalculateFragmentWindowForTest(idx, contentText, locations)
			})

			It("should produce boundaries at valid rune starts", func() {
				// start=0 is implicitly valid (start of string is always a rune start).
				// For non-zero start, verify it lands on a rune start byte.
				if start > 0 {
					Expect(utf8.RuneStart(contentText[start])).To(BeTrue(), "fragmentStart should be at a rune start")
				}
				// end=len(contentText) is implicitly valid (end of string, no byte to check).
				// For end within the string, verify it lands on a rune start byte.
				if end < len(contentText) {
					Expect(utf8.RuneStart(contentText[end])).To(BeTrue(), "fragmentEnd should be at a rune start")
				}
			})

			It("should produce a valid UTF-8 slice", func() {
				Expect(utf8.ValidString(contentText[start:end])).To(BeTrue())
			})
		})
	})

	Describe("extractFragmentFromLocations", func() {
		var idx *bleve.Index

		BeforeEach(func() {
			var err error
			idx, err = bleve.NewIndex(mockReader, frontmatterIndex)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when locations is nil", func() {
			var fragment string
			var highlights []bleve.HighlightSpan

			BeforeEach(func() {
				fragment, highlights = bleve.ExtractFragmentFromLocationsForTest(idx, "test content", nil)
			})

			It("should return empty fragment", func() {
				Expect(fragment).To(BeEmpty())
			})

			It("should return nil highlights", func() {
				Expect(highlights).To(BeNil())
			})
		})

		Context("when content field is nil", func() {
			var fragment string
			var highlights []bleve.HighlightSpan

			BeforeEach(func() {
				locations := search.FieldTermLocationMap{
					"other_field": map[string]search.Locations{},
				}
				fragment, highlights = bleve.ExtractFragmentFromLocationsForTest(idx, "test content", locations)
			})

			It("should return empty fragment", func() {
				Expect(fragment).To(BeEmpty())
			})

			It("should return nil highlights", func() {
				Expect(highlights).To(BeNil())
			})
		})

		Context("when there are no term locations", func() {
			var fragment string
			var highlights []bleve.HighlightSpan

			BeforeEach(func() {
				locations := search.FieldTermLocationMap{
					"content": map[string]search.Locations{},
				}
				fragment, highlights = bleve.ExtractFragmentFromLocationsForTest(idx, "test content", locations)
			})

			It("should return empty fragment", func() {
				Expect(fragment).To(BeEmpty())
			})

			It("should return nil highlights", func() {
				Expect(highlights).To(BeNil())
			})
		})

		Context("when there are valid locations", func() {
			var fragment string
			var highlights []bleve.HighlightSpan

			BeforeEach(func() {
				contentText := "The quick brown fox jumps over the lazy dog"
				locations := search.FieldTermLocationMap{
					"content": map[string]search.Locations{
						"quick": []*search.Location{{Start: 4, End: 9}},
						"fox":   []*search.Location{{Start: 16, End: 19}},
					},
				}
				fragment, highlights = bleve.ExtractFragmentFromLocationsForTest(idx, contentText, locations)
			})

			It("should extract non-empty fragment", func() {
				Expect(fragment).NotTo(BeEmpty())
			})

			It("should have two highlights", func() {
				Expect(highlights).To(HaveLen(2))
			})

			It("should have relative highlight positions", func() {
				// Highlights should be relative to fragment start
				Expect(highlights[0].Start).To(BeNumerically(">=", 0))
				Expect(highlights[1].Start).To(BeNumerically(">=", 0))
			})
		})

		Context("when locations are outside fragment window", func() {
			var fragment string
			var highlights []bleve.HighlightSpan

			BeforeEach(func() {
				contentText := strings.Repeat("x", 500)
				// Create locations where some are outside the window
				locations := search.FieldTermLocationMap{
					"content": map[string]search.Locations{
						"term": []*search.Location{
							{Start: 100, End: 105},
							{Start: 450, End: 455}, // This might be outside the window
						},
					},
				}
				fragment, highlights = bleve.ExtractFragmentFromLocationsForTest(idx, contentText, locations)
			})

			It("should extract non-empty fragment", func() {
				Expect(fragment).NotTo(BeEmpty())
			})

			It("should have at least one highlight", func() {
				// Should have at least one highlight
				Expect(len(highlights)).To(BeNumerically(">=", 1))
			})
		})

		Context("when content has multi-byte UTF-8 characters near fragment boundary", func() {
			var fragment string
			var highlights []bleve.HighlightSpan

			BeforeEach(func() {
				// 198 ASCII bytes + '日' (3 bytes) + remainder; match at byte 50 causes
				// fragmentEnd=200 which lands on a continuation byte of '日'.
				contentText := strings.Repeat("a", 198) + "日本語" + strings.Repeat("b", 300)
				locations := search.FieldTermLocationMap{
					"content": map[string]search.Locations{
						"term": []*search.Location{{Start: 50, End: 55}},
					},
				}
				fragment, highlights = bleve.ExtractFragmentFromLocationsForTest(idx, contentText, locations)
			})

			It("should return a valid UTF-8 fragment", func() {
				Expect(utf8.ValidString(fragment)).To(BeTrue())
			})

			It("should have at least one highlight", func() {
				Expect(highlights).To(HaveLen(1))
			})
		})

		Context("when content itself contains invalid UTF-8 bytes", func() {
			var fragment string

			BeforeEach(func() {
				// Construct a string with an invalid UTF-8 sequence embedded in it.
				// 0xFF is never valid in UTF-8.
				contentText := "valid prefix " + string([]byte{0xFF, 0xFE}) + " valid suffix with more text to fill"
				locations := search.FieldTermLocationMap{
					"content": map[string]search.Locations{
						"term": []*search.Location{{Start: 0, End: 5}},
					},
				}
				fragment, _ = bleve.ExtractFragmentFromLocationsForTest(idx, contentText, locations)
			})

			It("should return a valid UTF-8 fragment", func() {
				Expect(utf8.ValidString(fragment)).To(BeTrue())
			})
		})
	})

	Describe("AddPageToIndex", func() {
		When("the identifier fails munging", func() {
			var err error

			BeforeEach(func() {
				// "///" will fail MungeIdentifier because it becomes empty after sanitization
				err = index.AddPageToIndex("///")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should include invalid identifier context", func() {
				Expect(err.Error()).To(ContainSubstring("invalid identifier"))
			})
		})
	})
})
