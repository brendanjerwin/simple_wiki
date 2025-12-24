//revive:disable:dot-imports
package templating_test

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"text/template"
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

	Describe("when container has items from frontmatter index", func() {
		var (
			mockSite  *mockPageReader
			mockIndex *mockFrontmatterIndex
			result    string
		)

		BeforeEach(func() {
			// Arrange - Container with no direct inventory.items but items in index pointing to it
			mockSite = &mockPageReader{
				pages: map[string]wikipage.FrontMatter{
					"test_container": {
						identifierKey: "test_container",
						titleKey:      "Test Container",
						// No inventory.items in direct frontmatter
					},
					"item_from_index": {
						identifierKey: "item_from_index", 
						titleKey:      "Item From Index",
						inventoryKey: map[string]any{
							"container": "test_container", // This item points to test_container
						},
					},
				},
			}

			// Set up index to return item_from_index when querying for inventory.container = test_container
			mockIndex = &mockFrontmatterIndex{
				index: map[string]map[string][]string{
					"inventory.container": {
						"test_container": []string{"item_from_index"}, // This is the key integration
					},
				},
				values: map[string]map[string]string{
					"test_container": {
						titleKey: "Test Container",
					},
					"item_from_index": {
						titleKey: "Item From Index",
					},
				},
			}

			// Act
			showInventoryFunc := templating.BuildShowInventoryContentsOf(mockSite, mockIndex, 0)
			result = showInventoryFunc("test_container")
		})

		It("should include items from index in output", func() {
			Expect(result).To(ContainSubstring("Item From Index"))
		})

		It("should generate proper markdown list format", func() {
			Expect(result).To(ContainSubstring("- [Item From Index]"))
		})

		It("should use proper link format", func() {
			Expect(result).To(ContainSubstring("](/item_from_index)"))
		})

		It("should not be empty", func() {
			Expect(result).NotTo(BeEmpty())
		})
	})

	Describe("when container has mixed inventory sources", func() {
		var (
			isolatedMockSite  *mockPageReader
			isolatedMockIndex *mockFrontmatterIndex
			result            string
		)

		BeforeEach(func() {
			// Arrange - Container with BOTH direct items AND index items - completely isolated
			isolatedMockSite = &mockPageReader{
				pages: map[string]wikipage.FrontMatter{
					"isolated_mixed_container": {
						identifierKey: "isolated_mixed_container",
						titleKey:      "Isolated Mixed Container",
						inventoryKey: map[string]any{
							itemsKey: []string{"isolated_direct_item"}, // Direct inventory item
						},
					},
					"isolated_direct_item": {
						identifierKey: "isolated_direct_item",
						titleKey:      "Isolated Direct Item",
					},
					"isolated_index_item": {
						identifierKey: "isolated_index_item",
						titleKey:      "Isolated Index Item",
						inventoryKey: map[string]any{
							"container": "isolated_mixed_container", // This item points to mixed_container
						},
					},
				},
			}

			// Set up index to return index_item for mixed_container
			isolatedMockIndex = &mockFrontmatterIndex{
				index: map[string]map[string][]string{
					"inventory.container": {
						"isolated_mixed_container": []string{"isolated_index_item"}, 
					},
				},
				values: map[string]map[string]string{
					"isolated_mixed_container": {
						titleKey: "Isolated Mixed Container",
					},
					"isolated_direct_item": {
						titleKey: "Isolated Direct Item",
					},
					"isolated_index_item": {
						titleKey: "Isolated Index Item",
					},
				},
			}

			// Act
			showInventoryFunc := templating.BuildShowInventoryContentsOf(isolatedMockSite, isolatedMockIndex, 0)
			result = showInventoryFunc("isolated_mixed_container")
		})

		It("should include both direct and index items", func() {
			Expect(result).To(ContainSubstring("Isolated Direct Item"))
			Expect(result).To(ContainSubstring("Isolated Index Item"))
		})
	})

	Describe("when testing template execution directly", func() {
		var (
			mockSite  *mockPageReader
			mockIndex *mockFrontmatterIndex
			result    string
		)

		BeforeEach(func() {
			// Arrange - Simplified setup to test template execution  
			mockSite = &mockPageReader{
				pages: map[string]wikipage.FrontMatter{
					"test_container": {
						identifierKey: "test_container",
						titleKey:      "Test Container", 
						inventoryKey: map[string]any{
							itemsKey: []string{"simple_item"}, // Direct inventory item
						},
					},
					"simple_item": { 
						identifierKey: "simple_item",
						titleKey:      "Simple Item",
					},
				},
			}

			mockIndex = &mockFrontmatterIndex{
				index:  map[string]map[string][]string{},
				values: map[string]map[string]string{
					"simple_item": {
						titleKey: "Simple Item",
					},
				},
			}

			// Act
			showInventoryFunc := templating.BuildShowInventoryContentsOf(mockSite, mockIndex, 0)
			result = showInventoryFunc("test_container")
		})

		It("should produce markdown output for direct inventory item", func() {
			Expect(result).To(ContainSubstring("Simple Item"))
		})
	})

	Describe("when testing manual template execution", func() {
		var (
			containerFrontmatter wikipage.FrontMatter
			templateContext      templating.TemplateContext
			tmplString          string
			funcs               map[string]any
			tmpl                *template.Template
			buf                 *bytes.Buffer
			result              string
			err                 error
		)

		BeforeEach(func() {
			// Arrange
			mockSite := &mockPageReader{
				pages: map[string]wikipage.FrontMatter{
					"test_container": {
						identifierKey: "test_container",
						titleKey:      "Test Container", 
						inventoryKey: map[string]any{
							itemsKey: []string{"simple_item"}, // Direct inventory item
						},
					},
					"simple_item": { 
						identifierKey: "simple_item",
						titleKey:      "Simple Item",
					},
				},
			}

			mockIndex := &mockFrontmatterIndex{
				index:  map[string]map[string][]string{},
				values: map[string]map[string]string{
					"simple_item": {
						titleKey: "Simple Item",
					},
				},
			}

			// Act - manually execute the same template that BuildShowInventoryContentsOf uses
			_, containerFrontmatter, err = mockSite.ReadFrontMatter("test_container")
			Expect(err).NotTo(HaveOccurred())
			
			templateContext, err = templating.ConstructTemplateContextFromFrontmatterWithVisited(
				containerFrontmatter, mockIndex, make(map[string]bool))
			Expect(err).NotTo(HaveOccurred())
			
			// Use a simplified template string (without ShowInventoryContentsOf for now)
			tmplString = `
{{ range .Inventory.Items }}
{{ if IsContainer . }}
 - **{{ LinkTo . }}**
{{ else }}
 - {{ LinkTo . }}
{{ end }}
{{ end }}
`
			
			// Build the same functions
			funcs = map[string]any{
				"LinkTo":      templating.BuildLinkTo(mockSite, templateContext, mockIndex), 
				"IsContainer": templating.BuildIsContainer(mockIndex),
			}
			
			tmpl, err = template.New("test").Funcs(funcs).Parse(tmplString)
			Expect(err).NotTo(HaveOccurred())
			
			buf = &bytes.Buffer{}
			err = tmpl.Execute(buf, templateContext)
			Expect(err).NotTo(HaveOccurred())
			
			result = buf.String()
		})

		It("should produce markdown output for direct inventory item", func() {
			Expect(result).To(ContainSubstring("Simple Item"))
		})

		It("should test manual template with mixed inventory", func() {
			// Test the exact template execution with mixed inventory that's failing
			mockSite := &mockPageReader{
				pages: map[string]wikipage.FrontMatter{
					"mixed_container": {
						identifierKey: "mixed_container",
						titleKey:      "Mixed Container",
						inventoryKey: map[string]any{
							itemsKey: []string{"direct_item"}, // Direct inventory item
						},
					},
					"direct_item": {
						identifierKey: "direct_item",
						titleKey:      "Direct Item",
					},
					"index_item": {
						identifierKey: "index_item",
						titleKey:      "Index Item",
					},
				},
			}

			mockIndex := &mockFrontmatterIndex{
				index: map[string]map[string][]string{
					"inventory.container": {
						"mixed_container": []string{"index_item"}, 
					},
				},
				values: map[string]map[string]string{
					"direct_item": {titleKey: "Direct Item"},
					"index_item": {titleKey: "Index Item"},
				},
			}

			_, containerFrontmatter, err := mockSite.ReadFrontMatter("mixed_container")
			Expect(err).NotTo(HaveOccurred())
			
			templateContext, err := templating.ConstructTemplateContextFromFrontmatterWithVisited(
				containerFrontmatter, mockIndex, make(map[string]bool))
			Expect(err).NotTo(HaveOccurred())
			

			// Use the exact same template string from BuildShowInventoryContentsOfWithLimit
			tmplString := `
{{ range .Inventory.Items }}
{{ if IsContainer . }}
 - **{{ LinkTo . }}**
{{ else }}
 - {{ LinkTo . }}
{{ end }}
{{ end }}
`
			
			// Build the same functions exactly as in BuildShowInventoryContentsOfWithLimit
			funcs := map[string]any{
				"LinkTo":      templating.BuildLinkTo(mockSite, templateContext, mockIndex), 
				"IsContainer": templating.BuildIsContainer(mockIndex),
			}
			
			tmpl, err := template.New("test").Funcs(funcs).Parse(tmplString)
			Expect(err).NotTo(HaveOccurred())
			
			buf := &bytes.Buffer{}
			err = tmpl.Execute(buf, templateContext)
			Expect(err).NotTo(HaveOccurred())
			
			result := buf.String()
			
			// This should work if the template execution is correct
			Expect(result).To(ContainSubstring("Direct Item"))
			Expect(result).To(ContainSubstring("Index Item"))
		})

		It("should test exact BuildShowInventoryContentsOfWithLimit setup", func() {
			// Use the EXACT same setup as BuildShowInventoryContentsOfWithLimit
			mockSite := &mockPageReader{
				pages: map[string]wikipage.FrontMatter{
					"mixed_container": {
						identifierKey: "mixed_container",
						titleKey:      "Mixed Container",
						inventoryKey: map[string]any{
							itemsKey: []string{"direct_item"}, 
						},
					},
					"direct_item": {
						identifierKey: "direct_item",
						titleKey:      "Direct Item",
					},
					"index_item": {
						identifierKey: "index_item",
						titleKey:      "Index Item",
					},
				},
			}

			mockIndex := &mockFrontmatterIndex{
				index: map[string]map[string][]string{
					"inventory.container": {
						"mixed_container": []string{"index_item"}, 
					},
				},
				values: map[string]map[string]string{
					"direct_item": {titleKey: "Direct Item"},
					"index_item": {titleKey: "Index Item"},
				},
			}

			_, containerFrontmatter, err := mockSite.ReadFrontMatter("mixed_container")
			Expect(err).NotTo(HaveOccurred())
			
			templateContext, err := templating.ConstructTemplateContextFromFrontmatterWithVisited(
				containerFrontmatter, mockIndex, make(map[string]bool))
			Expect(err).NotTo(HaveOccurred())
			

			// Use the EXACT template and functions from BuildShowInventoryContentsOfWithLimit
			tmplString := `
{{ range .Inventory.Items }}
{{ if IsContainer . }}
{{ __Indent }} - **{{ LinkTo . }}**
{{ ShowInventoryContentsOf . }}
{{ else }}
{{ __Indent }} - {{ LinkTo . }}
{{ end }}
{{ end }}
`
			// EXACT same functions as BuildShowInventoryContentsOf 
			isContainer := templating.BuildIsContainer(mockIndex)
			
			funcs := template.FuncMap{
				"LinkTo":                  templating.BuildLinkTo(mockSite, templateContext, mockIndex),
				"ShowInventoryContentsOf": templating.BuildShowInventoryContentsOf(mockSite, mockIndex, 1),
				"IsContainer":             isContainer,
				"FindBy":                  mockIndex.QueryExactMatch,
				"FindByPrefix":            mockIndex.QueryPrefixMatch,
				"FindByKeyExistence":      mockIndex.QueryKeyExistence,
				"__Indent":                func() string { return strings.Repeat(" ", 0*2) },
			}
			
			tmpl, err := template.New("test").Funcs(funcs).Parse(tmplString)
			Expect(err).NotTo(HaveOccurred())
			
			buf := &bytes.Buffer{}
			err = tmpl.Execute(buf, templateContext)
			Expect(err).NotTo(HaveOccurred())
			
			result := buf.String()
			
			// This should work if the bug is not in the template setup
			Expect(result).To(ContainSubstring("Direct Item"))
			Expect(result).To(ContainSubstring("Index Item"))
		})
	})
})

var _ = Describe("ConstructTemplateContextFromFrontmatterWithVisited", func() {
	Describe("when frontmatter index contains items for container", func() {
		var (
			templateContext templating.TemplateContext
			err            error
			mockIndex      *mockFrontmatterIndex
			frontmatter    wikipage.FrontMatter
		)

		BeforeEach(func() {
			// Arrange - Set up realistic frontmatter and index
			frontmatter = wikipage.FrontMatter{
				identifierKey: "test_container",
				titleKey:      "Test Container",
				// No direct inventory.items
			}

			mockIndex = &mockFrontmatterIndex{
				index: map[string]map[string][]string{
					"inventory.container": {
						"test_container": []string{"item_from_index"},
					},
				},
				values: map[string]map[string]string{
					"item_from_index": {
						titleKey: "Item From Index",
					},
				},
			}

			// Act
			templateContext, err = templating.ConstructTemplateContextFromFrontmatterWithVisited(
				frontmatter, mockIndex, make(map[string]bool))
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should include items from index in inventory", func() {
			Expect(templateContext.Inventory.Items).To(ContainElement("item_from_index"))
		})

		It("should have non-empty inventory items", func() {
			Expect(len(templateContext.Inventory.Items)).To(BeNumerically(">", 0))
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
			// Mark all levels as containers so they get processed recursively
			"inventory.is_container": "true",
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
			// Setup current page as container (using is_container flag)
			mockIndex.values["current_page"] = map[string]string{
				"inventory.is_container": "true",
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

var _ = Describe("BuildIsContainer edge cases", func() {
	var (
		mockIndex      *mockFrontmatterIndex
		isContainerFunc func(string) bool
	)

	BeforeEach(func() {
		mockIndex = &mockFrontmatterIndex{
			index:  map[string]map[string][]string{},
			values: map[string]map[string]string{},
		}
		isContainerFunc = templating.BuildIsContainer(mockIndex)
	})

	Context("with empty identifier", func() {
		It("should return false", func() {
			Expect(isContainerFunc("")).To(BeFalse())
		})
	})

	Context("with container marked by is_container field", func() {
		BeforeEach(func() {
			mockIndex.values["explicit_container"] = map[string]string{
				"inventory.is_container": "true",
			}
		})

		It("should return true", func() {
			Expect(isContainerFunc("explicit_container")).To(BeTrue())
		})
	})

	Context("with legacy container (items reference it)", func() {
		BeforeEach(func() {
			mockIndex.index["inventory.container"] = map[string][]string{
				"legacy_container": []string{"item1", "item2"},
			}
		})

		It("should return true", func() {
			Expect(isContainerFunc("legacy_container")).To(BeTrue())
		})
	})

	Context("with non-container identifier", func() {
		It("should return false", func() {
			Expect(isContainerFunc("regular_page")).To(BeFalse())
		})
	})
})

var _ = Describe("ExecuteTemplate", func() {
	var (
		mockSite  *mockPageReader
		mockIndex *mockFrontmatterIndex
	)

	BeforeEach(func() {
		mockSite = &mockPageReader{
			pages: map[string]wikipage.FrontMatter{
				"test_page": {
					identifierKey: "test_page",
					titleKey:      "Test Page",
					"description": "Test Description",
				},
			},
		}

		mockIndex = &mockFrontmatterIndex{
			index:  map[string]map[string][]string{},
			values: map[string]map[string]string{},
		}
	})

	Context("with simple template", func() {
		It("should execute template with frontmatter data", func() {
			templateString := "Title: {{ .Title }}, Description: {{ .Description }}"
			frontmatter := wikipage.FrontMatter{
				identifierKey: "test_page",
				titleKey:      "My Page",
				"description": "My Description",
			}

			result, err := templating.ExecuteTemplate(templateString, frontmatter, mockSite, mockIndex)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(result)).To(ContainSubstring("Title: My Page"))
			Expect(string(result)).To(ContainSubstring("Description: My Description"))
		})
	})

	Context("with LinkTo function", func() {
		It("should execute template with LinkTo function", func() {
			mockSite.pages["linked_page"] = wikipage.FrontMatter{
				identifierKey: "linked_page",
				titleKey:      "Linked Page",
			}

			templateString := "Link: {{ LinkTo \"linked_page\" }}"
			frontmatter := wikipage.FrontMatter{
				identifierKey: "test_page",
				titleKey:      "Test Page",
			}

			result, err := templating.ExecuteTemplate(templateString, frontmatter, mockSite, mockIndex)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(result)).To(ContainSubstring("Linked Page"))
		})
	})

	Context("with ShowInventoryContentsOf function", func() {
		It("should execute template with ShowInventoryContentsOf function", func() {
			mockSite.pages["container"] = wikipage.FrontMatter{
				identifierKey: "container",
				titleKey:      "Container",
				inventoryKey: map[string]any{
					itemsKey: []string{"item1"},
				},
			}
			mockSite.pages["item1"] = wikipage.FrontMatter{
				identifierKey: "item1",
				titleKey:      "Item 1",
			}
			mockIndex.values["item1"] = map[string]string{
				titleKey: "Item 1",
			}

			templateString := "{{ ShowInventoryContentsOf \"container\" }}"
			frontmatter := wikipage.FrontMatter{
				identifierKey: "test_page",
				titleKey:      "Test Page",
			}

			result, err := templating.ExecuteTemplate(templateString, frontmatter, mockSite, mockIndex)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(result)).To(ContainSubstring("Item 1"))
		})
	})

	Context("with IsContainer function", func() {
		It("should execute template with IsContainer function", func() {
			mockIndex.values["test_container"] = map[string]string{
				"inventory.is_container": "true",
			}

			templateString := "{{ if IsContainer \"test_container\" }}Is Container{{ else }}Not Container{{ end }}"
			frontmatter := wikipage.FrontMatter{
				identifierKey: "test_page",
				titleKey:      "Test Page",
			}

			result, err := templating.ExecuteTemplate(templateString, frontmatter, mockSite, mockIndex)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(result)).To(ContainSubstring("Is Container"))
		})
	})

	Context("with FindBy function", func() {
		It("should execute template with FindBy function", func() {
			mockIndex.index["tag"] = map[string][]string{
				"electronics": []string{"item1", "item2"},
			}

			templateString := "{{ $items := FindBy \"tag\" \"electronics\" }}Count: {{ len $items }}"
			frontmatter := wikipage.FrontMatter{
				identifierKey: "test_page",
				titleKey:      "Test Page",
			}

			result, err := templating.ExecuteTemplate(templateString, frontmatter, mockSite, mockIndex)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(result)).To(ContainSubstring("Count: 2"))
		})
	})

	Context("with invalid template", func() {
		It("should return error for invalid template syntax", func() {
			templateString := "{{ .Title"
			frontmatter := wikipage.FrontMatter{
				identifierKey: "test_page",
				titleKey:      "Test Page",
			}

			_, err := templating.ExecuteTemplate(templateString, frontmatter, mockSite, mockIndex)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("with circular reference in template", func() {
		It("should handle circular references gracefully", func() {
			mockSite.pages["container_a"] = wikipage.FrontMatter{
				identifierKey: "container_a",
				titleKey:      "Container A",
				inventoryKey: map[string]any{
					itemsKey: []string{"container_b"},
				},
			}
			mockSite.pages["container_b"] = wikipage.FrontMatter{
				identifierKey: "container_b",
				titleKey:      "Container B",
				inventoryKey: map[string]any{
					itemsKey: []string{"container_a"},
				},
			}
			mockIndex.values["container_a"] = map[string]string{
				"inventory.is_container": "true",
			}
			mockIndex.values["container_b"] = map[string]string{
				"inventory.is_container": "true",
			}

			templateString := "{{ ShowInventoryContentsOf \"container_a\" }}"
			frontmatter := wikipage.FrontMatter{
				identifierKey: "test_page",
				titleKey:      "Test Page",
			}

			// Should complete without hanging due to timeout protection
			done := make(chan struct{})
			go func() {
				defer close(done)
				_, _ = templating.ExecuteTemplate(templateString, frontmatter, mockSite, mockIndex)
			}()

			select {
			case <-done:
				// Success - function completed
			case <-time.After(5 * time.Second):
				Fail("ExecuteTemplate timed out with circular reference")
			}
		})
	})

	Context("with FindByPrefix function", func() {
		It("should execute template with FindByPrefix function", func() {
			mockIndex.index["tag"] = map[string][]string{
				"electronics": []string{"item1", "item2"},
			}

			templateString := "{{ $items := FindByPrefix \"tag\" \"elect\" }}Count: {{ len $items }}"
			frontmatter := wikipage.FrontMatter{
				identifierKey: "test_page",
				titleKey:      "Test Page",
			}

			result, err := templating.ExecuteTemplate(templateString, frontmatter, mockSite, mockIndex)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(result)).To(ContainSubstring("Count:"))
		})
	})

	Context("with FindByKeyExistence function", func() {
		It("should execute template with FindByKeyExistence function", func() {
			mockIndex.index["has_serial"] = map[string][]string{
				"": []string{"item1", "item2"},
			}

			templateString := "{{ $items := FindByKeyExistence \"has_serial\" }}Count: {{ len $items }}"
			frontmatter := wikipage.FrontMatter{
				identifierKey: "test_page",
				titleKey:      "Test Page",
			}

			result, err := templating.ExecuteTemplate(templateString, frontmatter, mockSite, mockIndex)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(result)).To(ContainSubstring("Count:"))
		})
	})
})