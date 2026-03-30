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
				Expect(results).To(ContainElement(wikipage.PageIdentifier("test_page")))
			})

			It("should allow querying by key existence", func() {
				results := index.QueryKeyExistence("category")
				Expect(results).To(ContainElement(wikipage.PageIdentifier("test_page")))
			})

			It("should allow getting values", func() {
				value := index.GetValue("test-page", "title")
				Expect(value).To(Equal("Test Page"))
			})
		})

		Describe("when adding a page with an empty-string key containing a nested map", func() {
			var err error

			BeforeEach(func() {
				// An empty-string key with a map value exercises the buildKeyPath("", key) branch
				// in indexMap, where the empty current path is replaced by the child key directly.
				mockReader.AddPage("empty-key-page", wikipage.FrontMatter{
					"identifier": "empty-key-page",
					"": map[string]any{
						"nested": "value",
					},
				})
				err = index.AddPageToIndex("empty-key-page")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should index the nested value under its key", func() {
				results := index.QueryExactMatch("nested", "value")
				Expect(results).To(ContainElement(wikipage.PageIdentifier("empty_key_page")))
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
				Expect(results).To(ContainElement(wikipage.PageIdentifier("inventory_item")))
			})

			It("should index nested category keys", func() {
				results := index.QueryExactMatch("inventory.category", "tools")
				Expect(results).To(ContainElement(wikipage.PageIdentifier("inventory_item")))
			})

			It("should allow key existence queries for nested paths", func() {
				results := index.QueryKeyExistence("inventory.container")
				Expect(results).To(ContainElement(wikipage.PageIdentifier("inventory_item")))
			})

			It("should allow key existence queries for parent paths", func() {
				results := index.QueryKeyExistence("inventory")
				Expect(results).To(ContainElement(wikipage.PageIdentifier("inventory_item")))
			})

			It("should get nested values using dotted notation", func() {
				value := index.GetValue("inventory-item", "inventory.container")
				Expect(value).To(Equal("GarageInventory"))
			})
		})
	})

	Describe("QueryExactMatch with inventory.container scenario", func() {
		Describe("when multiple pages have inventory.container frontmatter", func() {
			var results []wikipage.PageIdentifier

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
				Expect(results).To(ContainElement(wikipage.PageIdentifier("hammer")))
				Expect(results).To(ContainElement(wikipage.PageIdentifier("screwdriver")))
			})

			It("should not return items with different container", func() {
				Expect(results).NotTo(ContainElement(wikipage.PageIdentifier("cookbook")))
			})

			It("should return exactly 2 items for garage_inventory", func() {
				Expect(results).To(HaveLen(2))
			})
		})

		Describe("when querying for a specific container identifier", func() {
			var templateContext struct {
				Identifier string
			}
			var itemsFromIndex []wikipage.PageIdentifier

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
				Expect(itemsFromIndex).To(ContainElement(wikipage.PageIdentifier("tool1")))
				Expect(itemsFromIndex).To(ContainElement(wikipage.PageIdentifier("tool2")))
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
				Expect(results).NotTo(ContainElement(wikipage.PageIdentifier("temp-page")))
			})

			It("should return empty value for removed page", func() {
				value := index.GetValue("temp-page", "title")
				Expect(value).To(Equal(""))
			})
		})
	})

	Describe("QueryPrefixMatch", func() {
		Describe("when pages have values with common prefixes", func() {
			var results []wikipage.PageIdentifier

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
				Expect(results).To(ContainElement(wikipage.PageIdentifier("garage_tool")))
				Expect(results).To(ContainElement(wikipage.PageIdentifier("garage_item")))
			})

			It("should not return items without matching prefix", func() {
				Expect(results).NotTo(ContainElement(wikipage.PageIdentifier("kitchen_item")))
			})
		})

		Describe("when a page has multiple values that match the prefix", func() {
			var (
				err     error
				results []wikipage.PageIdentifier
			)

			BeforeEach(func() {
				mockReader.AddPage("multi-location-item", wikipage.FrontMatter{
					"identifier": "multi-location-item",
					"inventory": map[string]any{
						"containers": []any{"Garage-Section-A", "Garage-Section-B"},
					},
				})
				err = index.AddPageToIndex("multi-location-item")

				results = index.QueryPrefixMatch("inventory.containers", "Garage")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the page only once despite multiple matching values", func() {
				Expect(results).To(ContainElement(wikipage.PageIdentifier("multi_location_item")))
				count := 0
				for _, r := range results {
					if r == wikipage.PageIdentifier("multi_location_item") {
						count++
					}
				}
				Expect(count).To(Equal(1))
			})
		})
	})

	Describe("QueryExactMatchSortedBy", func() {
		Describe("when multiple pages match with different sort values", func() {
			var results []wikipage.PageIdentifier

			BeforeEach(func() {
				mockReader.AddPage("post-2026-03-01", wikipage.FrontMatter{
					"identifier": "post-2026-03-01",
					"title":      "March Post",
					"blog": map[string]any{
						"identifier":     "my-blog",
						"published-date": "2026-03-01",
					},
				})
				mockReader.AddPage("post-2026-01-15", wikipage.FrontMatter{
					"identifier": "post-2026-01-15",
					"title":      "January Post",
					"blog": map[string]any{
						"identifier":     "my-blog",
						"published-date": "2026-01-15",
					},
				})
				mockReader.AddPage("post-2026-02-10", wikipage.FrontMatter{
					"identifier": "post-2026-02-10",
					"title":      "February Post",
					"blog": map[string]any{
						"identifier":     "my-blog",
						"published-date": "2026-02-10",
					},
				})

				_ = index.AddPageToIndex("post-2026-03-01")
				_ = index.AddPageToIndex("post-2026-01-15")
				_ = index.AddPageToIndex("post-2026-02-10")
			})

			Describe("when sorting descending", func() {
				BeforeEach(func() {
					results = index.QueryExactMatchSortedBy("blog.identifier", "my-blog", "blog.published-date", false, 0)
				})

				It("should return all matching pages", func() {
					Expect(results).To(HaveLen(3))
				})

				It("should return results sorted by date descending", func() {
					Expect(results[0]).To(Equal(wikipage.PageIdentifier("post_2026_03_01")))
					Expect(results[1]).To(Equal(wikipage.PageIdentifier("post_2026_02_10")))
					Expect(results[2]).To(Equal(wikipage.PageIdentifier("post_2026_01_15")))
				})
			})

			Describe("when sorting ascending", func() {
				BeforeEach(func() {
					results = index.QueryExactMatchSortedBy("blog.identifier", "my-blog", "blog.published-date", true, 0)
				})

				It("should return results sorted by date ascending", func() {
					Expect(results[0]).To(Equal(wikipage.PageIdentifier("post_2026_01_15")))
					Expect(results[1]).To(Equal(wikipage.PageIdentifier("post_2026_02_10")))
					Expect(results[2]).To(Equal(wikipage.PageIdentifier("post_2026_03_01")))
				})
			})

			Describe("when limiting results", func() {
				BeforeEach(func() {
					results = index.QueryExactMatchSortedBy("blog.identifier", "my-blog", "blog.published-date", false, 2)
				})

				It("should return only the requested number of results", func() {
					Expect(results).To(HaveLen(2))
				})

				It("should return the top N sorted results", func() {
					Expect(results[0]).To(Equal(wikipage.PageIdentifier("post_2026_03_01")))
					Expect(results[1]).To(Equal(wikipage.PageIdentifier("post_2026_02_10")))
				})
			})
		})

		Describe("when no pages match", func() {
			var results []wikipage.PageIdentifier

			BeforeEach(func() {
				results = index.QueryExactMatchSortedBy("blog.identifier", "nonexistent", "blog.published-date", false, 10)
			})

			It("should return nil", func() {
				Expect(results).To(BeNil())
			})
		})

		Describe("when pages have no sort key value", func() {
			var results []wikipage.PageIdentifier

			BeforeEach(func() {
				mockReader.AddPage("post-with-date", wikipage.FrontMatter{
					"identifier": "post-with-date",
					"blog": map[string]any{
						"identifier":     "test-blog",
						"published-date": "2026-05-01",
					},
				})
				mockReader.AddPage("post-without-date", wikipage.FrontMatter{
					"identifier": "post-without-date",
					"blog": map[string]any{
						"identifier": "test-blog",
					},
				})

				_ = index.AddPageToIndex("post-with-date")
				_ = index.AddPageToIndex("post-without-date")

				results = index.QueryExactMatchSortedBy("blog.identifier", "test-blog", "blog.published-date", false, 0)
			})

			It("should return all matching pages", func() {
				Expect(results).To(HaveLen(2))
			})

			It("should sort pages with values before pages without", func() {
				Expect(results[0]).To(Equal(wikipage.PageIdentifier("post_with_date")))
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
				Expect(results).To(ContainElement(wikipage.PageIdentifier("deep_page")))
			})

			It("should index intermediate levels", func() {
				results := index.QueryKeyExistence("level1.level2")
				Expect(results).To(ContainElement(wikipage.PageIdentifier("deep_page")))
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
				Expect(results).To(ContainElement(wikipage.PageIdentifier("array_page")))
			})

			It("should index nested array elements", func() {
				results := index.QueryExactMatch("inventory.items", "item1")
				Expect(results).To(ContainElement(wikipage.PageIdentifier("array_page")))
			})
		})

		Describe("when frontmatter has arrays of maps (checklist data)", func() {
			var err error

			BeforeEach(func() {
				mockReader.AddPage("checklist-page", wikipage.FrontMatter{
					"identifier": "checklist-page",
					"title":      "Checklist Page",
					"checklists": map[string]any{
						"grocery_list": map[string]any{
							"name": "Grocery List",
							"items": []any{
								map[string]any{"text": "Milk", "checked": false},
								map[string]any{"text": "Eggs", "checked": true},
							},
						},
					},
				})
				err = index.AddPageToIndex("checklist-page")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should index parent keys for existence queries", func() {
				results := index.QueryKeyExistence("checklists.grocery_list")
				Expect(results).To(ContainElement(wikipage.PageIdentifier("checklist_page")))
			})

			It("should index the name field within the checklist", func() {
				results := index.QueryExactMatch("checklists.grocery_list.name", "Grocery List")
				Expect(results).To(ContainElement(wikipage.PageIdentifier("checklist_page")))
			})
		})

		Describe("when a page has multiple values for the queried key", func() {
			var (
				err     error
				results []wikipage.PageIdentifier
			)

			BeforeEach(func() {
				mockReader.AddPage("multi-value-page", wikipage.FrontMatter{
					"identifier": "multi-value-page",
					"tags":       []any{"go", "testing", "wiki"},
				})
				err = index.AddPageToIndex("multi-value-page")

				results = index.QueryKeyExistence("tags")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the page only once despite multiple indexed values", func() {
				Expect(results).To(ContainElement(wikipage.PageIdentifier("multi_value_page")))
				count := 0
				for _, r := range results {
					if r == wikipage.PageIdentifier("multi_value_page") {
						count++
					}
				}
				Expect(count).To(Equal(1))
			})
		})

		Describe("when frontmatter has empty array values", func() {
			var err error

			BeforeEach(func() {
				mockReader.AddPage("container-page", wikipage.FrontMatter{
					"identifier": "container-page",
					"title":      "Empty Container",
					"inventory": map[string]any{
						"items": []any{},
					},
				})
				err = index.AddPageToIndex("container-page")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should allow key existence queries for empty arrays", func() {
				results := index.QueryKeyExistence("inventory.items")
				Expect(results).To(ContainElement(wikipage.PageIdentifier("container_page")))
			})
		})
	})
})

