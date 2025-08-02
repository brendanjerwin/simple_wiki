//revive:disable:dot-imports
package templating_test

import (
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/templating"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

const (
	containerA     = "container_a"
	containerB     = "container_b"
	titleKey       = "title"
	identifierKey  = "identifier"
	inventoryKey   = "inventory"
	itemsKey       = "items"
	maxTestLevels  = 12
	levelTemplate  = "level_%d"
)

// Mock implementations for testing
type mockPageReader struct {
	pages map[string]wikipage.FrontMatter
}

func (m *mockPageReader) ReadFrontMatter(identifier string) (string, wikipage.FrontMatter, error) {
	if fm, exists := m.pages[identifier]; exists {
		return identifier, fm, nil
	}
	return identifier, nil, errors.New("page not found")
}

func (*mockPageReader) ReadMarkdown(identifier string) (string, wikipage.Markdown, error) {
	return identifier, "", nil // Not needed for this test
}

type mockFrontmatterIndex struct {
	index  map[string]map[string][]string
	values map[string]map[string]string
}

func (m *mockFrontmatterIndex) QueryExactMatch(key, value string) []string {
	if keyMap, exists := m.index[key]; exists {
		if results, exists := keyMap[value]; exists {
			return results
		}
	}
	return []string{}
}

func (*mockFrontmatterIndex) QueryPrefixMatch(key, prefix string) []string {
	return []string{}
}

func (*mockFrontmatterIndex) QueryKeyExistence(key string) []string {
	return []string{}
}

func (m *mockFrontmatterIndex) GetValue(identifier, key string) string {
	if idMap, exists := m.values[identifier]; exists {
		if value, exists := idMap[key]; exists {
			return value
		}
	}
	return ""
}

var _ = Describe("BuildShowInventoryContentsOf", func() {
	Describe("when handling circular references", func() {
		var (
			mockSite  *mockPageReader
			mockIndex *mockFrontmatterIndex
			result    string
		)

		BeforeEach(func() {
			// Arrange - Create circular container references
			// Container A contains Container B, Container B contains Container A
			mockSite = &mockPageReader{
				pages: map[string]wikipage.FrontMatter{
					containerA: {
						identifierKey: containerA,
						titleKey:      "Container A",
						inventoryKey: map[string]any{
							itemsKey: []string{containerB},
						},
					},
					containerB: {
						identifierKey: containerB,
						titleKey:      "Container B",
						inventoryKey: map[string]any{
							itemsKey: []string{containerA},
						},
					},
				},
			}

			mockIndex = &mockFrontmatterIndex{
				index: map[string]map[string][]string{},
				values: map[string]map[string]string{
					containerA: {
						"inventory.items": containerB,
						titleKey:          "Container A",
					},
					containerB: {
						"inventory.items": containerA,
						titleKey:          "Container B",
					},
				},
			}

			// Act - Execute the template function that should prevent infinite recursion
			// We use a timeout to ensure the test doesn't hang forever
			done := make(chan string, 1)
			go func() {
				showInventoryFunc := templating.BuildShowInventoryContentsOf(mockSite, mockIndex, 0)
				result := showInventoryFunc(containerA)
				done <- result
			}()

			select {
			case result = <-done:
				// Function returned without hanging
			case <-time.After(5 * time.Second):
				Fail("Test timed out - function likely has infinite recursion")
			}
		})

		It("should return non-empty result", func() {
			Expect(result).NotTo(BeEmpty())
		})

		It("should not contain stack overflow indication", func() {
			Expect(result).NotTo(ContainSubstring("stack overflow"))
		})
	})

	Describe("when handling deep nesting", func() {
		var (
			mockSite  *mockPageReader
			mockIndex *mockFrontmatterIndex
			result    string
		)

		BeforeEach(func() {
			// Arrange - Create a deep nested container structure (deeper than max depth of 10)
			mockSite = &mockPageReader{pages: createDeepNestedPages()}
			mockIndex = createDeepNestedIndex()

			// Act
			showInventoryFunc := templating.BuildShowInventoryContentsOf(mockSite, mockIndex, 0)
			result = showInventoryFunc("level_0")
		})

		It("should contain max depth message", func() {
			Expect(result).To(ContainSubstring("[Maximum depth reached]"))
		})
	})
})

func createDeepNestedPages() map[string]wikipage.FrontMatter {
	pages := make(map[string]wikipage.FrontMatter)
	for i := 0; i <= maxTestLevels; i++ {
		levelKey := fmt.Sprintf(levelTemplate, i)
		nextItems := []string{}
		if i < maxTestLevels {
			nextItems = []string{fmt.Sprintf(levelTemplate, i+1)}
		}
		
		pages[levelKey] = wikipage.FrontMatter{
			identifierKey: levelKey,
			titleKey:      fmt.Sprintf("Level %d", i),
			inventoryKey: map[string]any{
				itemsKey: nextItems,
			},
		}
	}
	return pages
}

func createDeepNestedIndex() *mockFrontmatterIndex {
	mockIndex := &mockFrontmatterIndex{
		index:  map[string]map[string][]string{},
		values: make(map[string]map[string]string),
	}
	
	// Set up values for all levels
	for i := 0; i <= maxTestLevels; i++ {
		levelKey := fmt.Sprintf(levelTemplate, i)
		mockIndex.values[levelKey] = map[string]string{
			titleKey: fmt.Sprintf("Level %d", i),
		}
		if i < maxTestLevels {
			nextLevel := fmt.Sprintf(levelTemplate, i+1)
			mockIndex.values[levelKey]["inventory.items"] = nextLevel
		}
	}
	return mockIndex
}

var _ = Describe("BuildLinkTo", func() {
	var (
		mockSite                   *mockPageReader
		mockIndex                  *mockFrontmatterIndex
		currentPageTemplateContext templating.TemplateContext
		linkToFunc                 func(string) string
		result                     string
	)

	BeforeEach(func() {
		mockSite = &mockPageReader{
			pages: map[string]wikipage.FrontMatter{},
		}
		mockIndex = &mockFrontmatterIndex{
			index:  map[string]map[string][]string{},
			values: map[string]map[string]string{},
		}
		currentPageTemplateContext = templating.TemplateContext{
			Identifier: "current_page",
			Title:      "Current Page",
		}
		linkToFunc = templating.BuildLinkTo(mockSite, currentPageTemplateContext, mockIndex)
	})

	Describe("when linking to identifier with spaces", func() {
		BeforeEach(func() {
			// Act - Try to link to an identifier with spaces
			result = linkToFunc("Kinect to Windows adapter")
		})

		It("should use munged identifier in URL path", func() {
			// This test will fail until we fix the LinkTo function
			// The URL should contain 'kinect_to_windows_adapter' not 'Kinect to Windows adapter'
			Expect(result).To(ContainSubstring("/kinect_to_windows_adapter?"))
		})

		It("should preserve original title in link text", func() {
			// The link text should still show the human-readable title
			Expect(result).To(ContainSubstring("[Kinect To Windows Adapter]"))
		})

		It("should properly encode title parameter", func() {
			// The title parameter should be URL encoded
			Expect(result).To(ContainSubstring("title=Kinect+To+Windows+Adapter"))
		})
	})

	Describe("when linking to existing page with spaces in identifier", func() {
		BeforeEach(func() {
			// Setup an existing page with spaces in identifier
			mockSite.pages["kinect_to_windows_adapter"] = wikipage.FrontMatter{
				identifierKey: "kinect_to_windows_adapter",
				titleKey:      "Kinect To Windows Adapter",
			}

			// Act
			result = linkToFunc("Kinect to Windows adapter")
		})

		It("should use munged identifier in URL path", func() {
			// Should use the munged version in URL (without the closing paren since there may be query params)
			Expect(result).To(ContainSubstring("/kinect_to_windows_adapter"))
		})

		It("should use existing page title", func() {
			// Should use the title from the existing page
			Expect(result).To(ContainSubstring("[Kinect To Windows Adapter]"))
		})
	})

	Describe("when current page is container and linking to inventory item with spaces", func() {
		BeforeEach(func() {
			// Setup current page as container
			mockIndex.values["current_page"] = map[string]string{
				"inventory.items": "some_item",
			}

			// Act
			result = linkToFunc("Kinect to Windows adapter")
		})

		It("should use munged identifier in inventory item URL", func() {
			// Should use munged identifier in the special inventory item link
			Expect(result).To(ContainSubstring("/kinect_to_windows_adapter?tmpl=inv_item"))
		})

		It("should include container reference", func() {
			// Should include the container reference
			Expect(result).To(ContainSubstring("inventory.container=current_page"))
		})
	})
})