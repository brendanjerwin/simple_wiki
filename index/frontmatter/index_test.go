//revive:disable:dot-imports
package frontmatter_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// MockPageReader is a test implementation of wikipage.PageReader for testing the frontmatter index.
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
	// For frontmatter index testing, we don't need actual markdown content
	_, exists := m.pages[string(identifier)]
	if !exists {
		return identifier, "", errors.New("page not found")
	}
	return identifier, "Mock markdown content", nil
}

var _ = Describe("Index", func() {
	var (
		index      *frontmatter.Index
		mockReader *MockPageReader
	)

	BeforeEach(func() {
		mockReader = NewMockPageReader()
		index = frontmatter.NewIndex(mockReader)
	})

	It("should exist", func() {
		Expect(index).NotTo(BeNil())
	})

	Describe("GetIndexName", func() {
		var indexName string

		BeforeEach(func() {
			indexName = index.GetIndexName()
		})

		It("should return frontmatter", func() {
			Expect(indexName).To(Equal("frontmatter"))
		})
	})

	Describe("AddPageToIndex", func() {
		Describe("when adding a page with simple string frontmatter", func() {
			var err error

			BeforeEach(func() {
				mockReader.AddPage("test-page", wikipage.FrontMatter{
					"identifier": "test-page",
					"title":      "Test Page",
					"category":   "testing",
				})
				err = index.AddPageToIndex("test-page")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should allow querying by exact match", func() {
				results := index.QueryExactMatch("title", "Test Page")
				Expect(results).To(ContainElement("test_page"))
			})

			It("should allow querying by key existence", func() {
				results := index.QueryKeyExistence("category")
				Expect(results).To(ContainElement("test_page"))
			})

			It("should allow getting values", func() {
				value := index.GetValue("test-page", "title")
				Expect(value).To(Equal("Test Page"))
			})
		})

		Describe("when adding a page with nested TOML frontmatter", func() {
			var err error

			BeforeEach(func() {
				mockReader.AddPage("inventory-item", wikipage.FrontMatter{
					"identifier": "inventory-item",
					"title":      "My Inventory Item",
					"inventory": map[string]any{
						"container": "GarageInventory",
						"category":  "tools",
					},
				})
				err = index.AddPageToIndex("inventory-item")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should index nested keys with dotted notation", func() {
				results := index.QueryExactMatch("inventory.container", "GarageInventory")
				Expect(results).To(ContainElement("inventory_item"))
			})

			It("should index nested category keys", func() {
				results := index.QueryExactMatch("inventory.category", "tools")
				Expect(results).To(ContainElement("inventory_item"))
			})

			It("should allow key existence queries for nested paths", func() {
				results := index.QueryKeyExistence("inventory.container")
				Expect(results).To(ContainElement("inventory_item"))
			})

			It("should allow key existence queries for parent paths", func() {
				results := index.QueryKeyExistence("inventory")
				Expect(results).To(ContainElement("inventory_item"))
			})

			It("should get nested values using dotted notation", func() {
				value := index.GetValue("inventory-item", "inventory.container")
				Expect(value).To(Equal("GarageInventory"))
			})
		})
	})

	Describe("QueryExactMatch with inventory.container scenario", func() {
		Describe("when multiple pages have inventory.container frontmatter", func() {
			var results []string

			BeforeEach(func() {
				// Add multiple pages with different container values
				mockReader.AddPage("hammer", wikipage.FrontMatter{
					"identifier": "hammer",
					"title":      "Hammer",
					"inventory": map[string]any{
						"container": "garage_inventory",
					},
				})
				mockReader.AddPage("screwdriver", wikipage.FrontMatter{
					"identifier": "screwdriver",
					"title":      "Screwdriver",
					"inventory": map[string]any{
						"container": "garage_inventory",
					},
				})
				mockReader.AddPage("cookbook", wikipage.FrontMatter{
					"identifier": "cookbook",
					"title":      "Cookbook",
					"inventory": map[string]any{
						"container": "kitchen_inventory",
					},
				})

				// Add all pages to index
				_ = index.AddPageToIndex("hammer")
				_ = index.AddPageToIndex("screwdriver")
				_ = index.AddPageToIndex("cookbook")

				// Perform the query that the templating system uses
				results = index.QueryExactMatch("inventory.container", "garage_inventory")
			})

			It("should return all items with matching container", func() {
				Expect(results).To(ContainElement("hammer"))
				Expect(results).To(ContainElement("screwdriver"))
			})

			It("should not return items with different container", func() {
				Expect(results).NotTo(ContainElement("cookbook"))
			})

			It("should return exactly 2 items for garage_inventory", func() {
				Expect(results).To(HaveLen(2))
			})
		})

		Describe("when querying for a specific container identifier", func() {
			var templateContext struct {
				Identifier string
			}
			var itemsFromIndex []string

			BeforeEach(func() {
				templateContext.Identifier = "GarageInventory"

				// Simulate the exact TOML structure: [inventory] container = "GarageInventory"
				mockReader.AddPage("tool1", wikipage.FrontMatter{
					"identifier": "tool1",
					"title":      "Tool 1",
					"inventory": map[string]any{
						"container": "GarageInventory",
					},
				})
				mockReader.AddPage("tool2", wikipage.FrontMatter{
					"identifier": "tool2",
					"title":      "Tool 2",
					"inventory": map[string]any{
						"container": "GarageInventory",
					},
				})

				_ = index.AddPageToIndex("tool1")
				_ = index.AddPageToIndex("tool2")

				// This is the exact query from templating/templating.go:91
				itemsFromIndex = index.QueryExactMatch("inventory.container", templateContext.Identifier)
			})

			It("should find items that reference the container", func() {
				Expect(itemsFromIndex).To(ContainElement("tool1"))
				Expect(itemsFromIndex).To(ContainElement("tool2"))
			})

			It("should return the correct number of items", func() {
				Expect(itemsFromIndex).To(HaveLen(2))
			})
		})
	})

	Describe("RemovePageFromIndex", func() {
		Describe("when removing a page that was previously indexed", func() {
			var err error

			BeforeEach(func() {
				mockReader.AddPage("temp-page", wikipage.FrontMatter{
					"identifier": "temp-page",
					"title":      "Temporary Page",
					"inventory": map[string]any{
						"container": "TestContainer",
					},
				})
				_ = index.AddPageToIndex("temp-page")
				err = index.RemovePageFromIndex("temp-page")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should no longer find the page in queries", func() {
				results := index.QueryExactMatch("inventory.container", "TestContainer")
				Expect(results).NotTo(ContainElement("temp-page"))
			})

			It("should return empty value for removed page", func() {
				value := index.GetValue("temp-page", "title")
				Expect(value).To(Equal(""))
			})
		})
	})

	Describe("QueryPrefixMatch", func() {
		Describe("when pages have values with common prefixes", func() {
			var results []string

			BeforeEach(func() {
				mockReader.AddPage("garage-tool", wikipage.FrontMatter{
					"identifier": "garage-tool",
					"inventory": map[string]any{
						"container": "Garage-Section-A",
					},
				})
				mockReader.AddPage("garage-item", wikipage.FrontMatter{
					"identifier": "garage-item",
					"inventory": map[string]any{
						"container": "Garage-Section-B",
					},
				})
				mockReader.AddPage("kitchen-item", wikipage.FrontMatter{
					"identifier": "kitchen-item",
					"inventory": map[string]any{
						"container": "Kitchen-Cabinet",
					},
				})

				_ = index.AddPageToIndex("garage-tool")
				_ = index.AddPageToIndex("garage-item")
				_ = index.AddPageToIndex("kitchen-item")

				results = index.QueryPrefixMatch("inventory.container", "Garage")
			})

			It("should return items with matching prefix", func() {
				Expect(results).To(ContainElement("garage_tool"))
				Expect(results).To(ContainElement("garage_item"))
			})

			It("should not return items without matching prefix", func() {
				Expect(results).NotTo(ContainElement("kitchen_item"))
			})
		})
	})

	Describe("edge cases and error handling", func() {
		Describe("when page does not exist", func() {
			var err error

			BeforeEach(func() {
				err = index.AddPageToIndex("nonexistent-page")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("page not found"))
			})
		})

		Describe("when frontmatter has deeply nested structures", func() {
			var err error

			BeforeEach(func() {
				mockReader.AddPage("deep-page", wikipage.FrontMatter{
					"identifier": "deep-page",
					"level1": map[string]any{
						"level2": map[string]any{
							"level3": map[string]any{
								"deepvalue": "found",
							},
						},
					},
				})
				err = index.AddPageToIndex("deep-page")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should index deeply nested keys", func() {
				results := index.QueryExactMatch("level1.level2.level3.deepvalue", "found")
				Expect(results).To(ContainElement("deep_page"))
			})

			It("should index intermediate levels", func() {
				results := index.QueryKeyExistence("level1.level2")
				Expect(results).To(ContainElement("deep_page"))
			})
		})

		Describe("when frontmatter has array values", func() {
			var err error

			BeforeEach(func() {
				mockReader.AddPage("array-page", wikipage.FrontMatter{
					"identifier": "array-page",
					"tags":       []any{"tag1", "tag2", "tag3"},
					"inventory": map[string]any{
						"items": []any{"item1", "item2"},
					},
				})
				err = index.AddPageToIndex("array-page")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should index array elements individually", func() {
				results := index.QueryExactMatch("tags", "tag2")
				Expect(results).To(ContainElement("array_page"))
			})

			It("should index nested array elements", func() {
				results := index.QueryExactMatch("inventory.items", "item1")
				Expect(results).To(ContainElement("array_page"))
			})
		})
	})
})

