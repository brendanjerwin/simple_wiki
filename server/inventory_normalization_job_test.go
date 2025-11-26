//revive:disable:dot-imports
package server

import (
	"os"

	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// mockNormalizationDeps implements InventoryNormalizationDependencies for testing.
type mockNormalizationDeps struct {
	pages             map[string]*mockPageData
	writtenPages      map[string]*mockPageData
	deletedPages      []string
	readFrontmatterCalls []string
}

type mockPageData struct {
	frontmatter map[string]any
	markdown    string
	err         error
}

func newMockNormalizationDeps() *mockNormalizationDeps {
	return &mockNormalizationDeps{
		pages:        make(map[string]*mockPageData),
		writtenPages: make(map[string]*mockPageData),
	}
}

func (m *mockNormalizationDeps) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	m.readFrontmatterCalls = append(m.readFrontmatterCalls, id)
	if page, ok := m.pages[id]; ok {
		if page.err != nil {
			return "", nil, page.err
		}
		return id, page.frontmatter, nil
	}
	return "", nil, os.ErrNotExist
}

func (m *mockNormalizationDeps) ReadMarkdown(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	if page, ok := m.pages[id]; ok {
		if page.err != nil {
			return "", "", page.err
		}
		return id, page.markdown, nil
	}
	return "", "", os.ErrNotExist
}

func (m *mockNormalizationDeps) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	if m.writtenPages[id] == nil {
		m.writtenPages[id] = &mockPageData{}
	}
	m.writtenPages[id].frontmatter = fm
	return nil
}

func (m *mockNormalizationDeps) WriteMarkdown(id wikipage.PageIdentifier, md wikipage.Markdown) error {
	if m.writtenPages[id] == nil {
		m.writtenPages[id] = &mockPageData{}
	}
	m.writtenPages[id].markdown = md
	return nil
}

func (m *mockNormalizationDeps) DeletePage(id wikipage.PageIdentifier) error {
	m.deletedPages = append(m.deletedPages, id)
	return nil
}

func (m *mockNormalizationDeps) ReadPage(id wikipage.PageIdentifier) (*wikipage.Page, error) {
	if page, ok := m.pages[id]; ok {
		if page.err != nil {
			return nil, page.err
		}
		return &wikipage.Page{
			Identifier: id,
			Text:       page.markdown,
		}, nil
	}
	return nil, os.ErrNotExist
}

var _ = Describe("InventoryNormalizationJob", func() {
	var (
		job      *InventoryNormalizationJob
		mockDeps *mockNormalizationDeps
		mockFmIndex *mockFrontmatterIndexQueryer
		logger   lumber.Logger
	)

	BeforeEach(func() {
		mockDeps = newMockNormalizationDeps()
		mockFmIndex = &mockFrontmatterIndexQueryer{
			data: make(map[string]map[string]string),
		}
		logger = lumber.NewConsoleLogger(lumber.WARN)
	})

	Describe("findAllContainers", func() {
		BeforeEach(func() {
			job = NewInventoryNormalizationJob(mockDeps, mockFmIndex, logger, nil)
		})

		When("there are pages with inventory.items", func() {
			var containers []string

			BeforeEach(func() {
				mockFmIndex.QueryKeyExistenceFunc = func(key string) []string {
					if key == "inventory.items" {
						return []string{"drawer", "box"}
					}
					if key == "inventory.container" {
						return []string{}
					}
					return nil
				}

				containers = job.findAllContainers()
			})

			It("should return those pages as containers", func() {
				Expect(containers).To(ContainElements("drawer", "box"))
			})
		})

		When("there are pages referencing containers", func() {
			var containers []string

			BeforeEach(func() {
				mockFmIndex.QueryKeyExistenceFunc = func(key string) []string {
					if key == "inventory.items" {
						return []string{}
					}
					if key == "inventory.container" {
						return []string{"item1", "item2"}
					}
					return nil
				}
				mockFmIndex.data["item1"] = map[string]string{"inventory.container": "cabinet"}
				mockFmIndex.data["item2"] = map[string]string{"inventory.container": "cabinet"}

				containers = job.findAllContainers()
			})

			It("should include the referenced containers", func() {
				Expect(containers).To(ContainElement("cabinet"))
			})
		})
	})

	Describe("getContainerItems", func() {
		BeforeEach(func() {
			job = NewInventoryNormalizationJob(mockDeps, mockFmIndex, logger, nil)
		})

		When("container has items in inventory.items array", func() {
			var items []string

			BeforeEach(func() {
				mockDeps.pages["drawer"] = &mockPageData{
					frontmatter: map[string]any{
						"title": "My Drawer",
						"inventory": map[string]any{
							"items": []any{"screwdriver", "hammer", "nails"},
						},
					},
				}

				items = job.getContainerItems("drawer")
			})

			It("should return all items", func() {
				Expect(items).To(HaveLen(3))
				Expect(items).To(ContainElements("screwdriver", "hammer", "nails"))
			})
		})

		When("container does not exist", func() {
			var items []string

			BeforeEach(func() {
				items = job.getContainerItems("nonexistent")
			})

			It("should return nil", func() {
				Expect(items).To(BeNil())
			})
		})

		When("container has no inventory section", func() {
			var items []string

			BeforeEach(func() {
				mockDeps.pages["page"] = &mockPageData{
					frontmatter: map[string]any{
						"title": "Just a page",
					},
				}

				items = job.getContainerItems("page")
			})

			It("should return nil", func() {
				Expect(items).To(BeNil())
			})
		})
	})

	Describe("createItemPage", func() {
		BeforeEach(func() {
			job = NewInventoryNormalizationJob(mockDeps, mockFmIndex, logger, nil)
		})

		When("creating a new item page", func() {
			var err error

			BeforeEach(func() {
				err = job.createItemPage("my-screwdriver", "tool_drawer")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write the frontmatter with identifier", func() {
				Expect(mockDeps.writtenPages).To(HaveKey("my_screwdriver"))
				fm := mockDeps.writtenPages["my_screwdriver"].frontmatter
				Expect(fm["identifier"]).To(Equal("my_screwdriver"))
			})

			It("should set the title", func() {
				fm := mockDeps.writtenPages["my_screwdriver"].frontmatter
				Expect(fm["title"]).NotTo(BeEmpty())
			})

			It("should set the container", func() {
				fm := mockDeps.writtenPages["my_screwdriver"].frontmatter
				inventory, ok := fm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue(), "expected inventory to be map[string]any")
				Expect(inventory["container"]).To(Equal("tool_drawer"))
			})

			It("should write markdown content", func() {
				Expect(mockDeps.writtenPages["my_screwdriver"].markdown).NotTo(BeEmpty())
			})
		})

		When("creating an item without a container", func() {
			var err error

			BeforeEach(func() {
				err = job.createItemPage("standalone-item", "")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not set a container in frontmatter", func() {
				fm := mockDeps.writtenPages["standalone_item"].frontmatter
				inventory, ok := fm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue(), "expected inventory to be map[string]any")
				_, hasContainer := inventory["container"]
				Expect(hasContainer).To(BeFalse())
			})
		})
	})

	Describe("detectCircularReferences", func() {
		BeforeEach(func() {
			job = NewInventoryNormalizationJob(mockDeps, mockFmIndex, logger, nil)
		})

		When("there are no circular references", func() {
			var anomalies []InventoryAnomaly

			BeforeEach(func() {
				mockFmIndex.data["container_a"] = map[string]string{"inventory.container": ""}
				mockFmIndex.data["container_b"] = map[string]string{"inventory.container": "container_a"}

				anomalies = job.detectCircularReferences([]string{"container_a", "container_b"})
			})

			It("should return no anomalies", func() {
				Expect(anomalies).To(BeEmpty())
			})
		})

		When("there is a circular reference", func() {
			var anomalies []InventoryAnomaly

			BeforeEach(func() {
				// A -> B -> C -> A (circular)
				mockFmIndex.data["container_a"] = map[string]string{"inventory.container": "container_c"}
				mockFmIndex.data["container_b"] = map[string]string{"inventory.container": "container_a"}
				mockFmIndex.data["container_c"] = map[string]string{"inventory.container": "container_b"}

				anomalies = job.detectCircularReferences([]string{"container_a", "container_b", "container_c"})
			})

			It("should detect the circular reference", func() {
				Expect(anomalies).NotTo(BeEmpty())
			})

			It("should have type circular_reference", func() {
				Expect(anomalies[0].Type).To(Equal("circular_reference"))
			})

			It("should have error severity", func() {
				Expect(anomalies[0].Severity).To(Equal("error"))
			})
		})
	})

	Describe("Execute", func() {
		BeforeEach(func() {
			job = NewInventoryNormalizationJob(mockDeps, mockFmIndex, logger, nil)
		})

		When("container has items without pages", func() {
			var err error

			BeforeEach(func() {
				// Set up a container with items
				mockDeps.pages["tool_box"] = &mockPageData{
					frontmatter: map[string]any{
						"title": "Tool Box",
						"inventory": map[string]any{
							"items": []any{"hammer", "screwdriver"},
						},
					},
				}

				mockFmIndex.QueryKeyExistenceFunc = func(key string) []string {
					if key == "inventory.items" {
						return []string{"tool_box"}
					}
					return []string{}
				}

				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create pages for missing items", func() {
				Expect(mockDeps.writtenPages).To(HaveKey("hammer"))
				Expect(mockDeps.writtenPages).To(HaveKey("screwdriver"))
			})

			It("should set the container in created pages", func() {
				hammerFm := mockDeps.writtenPages["hammer"].frontmatter
				inventory, ok := hammerFm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue(), "expected inventory to be map[string]any")
				Expect(inventory["container"]).To(Equal("tool_box"))
			})

			It("should generate an audit report", func() {
				Expect(mockDeps.writtenPages).To(HaveKey(AuditReportPage))
			})
		})

		When("items already have pages", func() {
			var err error

			BeforeEach(func() {
				// Container with items
				mockDeps.pages["tool_box"] = &mockPageData{
					frontmatter: map[string]any{
						"title": "Tool Box",
						"inventory": map[string]any{
							"items": []any{"hammer"},
						},
					},
				}
				// Existing page for the item
				mockDeps.pages["hammer"] = &mockPageData{
					frontmatter: map[string]any{
						"title": "Hammer",
						"identifier": "hammer",
					},
				}

				mockFmIndex.QueryKeyExistenceFunc = func(key string) []string {
					if key == "inventory.items" {
						return []string{"tool_box"}
					}
					return []string{}
				}

				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not recreate existing pages", func() {
				// hammer should not be in writtenPages (only audit report should be written)
				_, hammerWritten := mockDeps.writtenPages["hammer"]
				Expect(hammerWritten).To(BeFalse())
			})
		})

		When("item references non-existent container", func() {
			var err error

			BeforeEach(func() {
				// Item referencing a container that doesn't exist
				mockDeps.pages["orphan_item"] = &mockPageData{
					frontmatter: map[string]any{
						"title": "Orphan Item",
						"inventory": map[string]any{
							"container": "nonexistent_container",
						},
					},
				}

				mockFmIndex.QueryKeyExistenceFunc = func(key string) []string {
					if key == "inventory.container" {
						return []string{"orphan_item"}
					}
					return []string{}
				}
				mockFmIndex.data["orphan_item"] = map[string]string{
					"inventory.container": "nonexistent_container",
				}

				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create an audit report with orphan anomaly", func() {
				Expect(mockDeps.writtenPages).To(HaveKey(AuditReportPage))
				report := mockDeps.writtenPages[AuditReportPage].markdown
				Expect(report).To(ContainSubstring("orphan_item"))
			})
		})

		When("item is in multiple containers", func() {
			var err error

			BeforeEach(func() {
				mockFmIndex.QueryKeyExistenceFunc = func(key string) []string {
					if key == "inventory.items" {
						return []string{}
					}
					if key == "inventory.container" {
						return []string{}
					}
					return []string{}
				}
				mockFmIndex.QueryExactMatchFunc = func(key, value string) []string {
					if key == "inventory.container" {
						// Simulate an item being in multiple containers
						if value == "drawer_1" {
							return []string{"multi_item"}
						}
						if value == "drawer_2" {
							return []string{"multi_item"}
						}
					}
					return nil
				}

				// Mock containers - provide pages for the containers
				mockDeps.pages["drawer_1"] = &mockPageData{
					frontmatter: map[string]any{
						"title": "Drawer 1",
					},
				}
				mockDeps.pages["drawer_2"] = &mockPageData{
					frontmatter: map[string]any{
						"title": "Drawer 2",
					},
				}

				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("generateAuditReport", func() {
		BeforeEach(func() {
			job = NewInventoryNormalizationJob(mockDeps, mockFmIndex, logger, nil)
		})

		When("there are anomalies and created pages", func() {
			var err error

			BeforeEach(func() {
				anomalies := []InventoryAnomaly{
					{
						Type:        "orphan",
						ItemID:      "lost_item",
						Description: "Item 'lost_item' references non-existent container",
						Severity:    "warning",
					},
				}
				createdPages := []string{"new_item_1", "new_item_2"}

				err = job.generateAuditReport(anomalies, createdPages)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create the audit report page", func() {
				Expect(mockDeps.writtenPages).To(HaveKey(AuditReportPage))
			})

			It("should include created pages in the report", func() {
				report := mockDeps.writtenPages[AuditReportPage].markdown
				Expect(report).To(ContainSubstring("new_item_1"))
				Expect(report).To(ContainSubstring("new_item_2"))
			})

			It("should include anomalies in the report", func() {
				report := mockDeps.writtenPages[AuditReportPage].markdown
				Expect(report).To(ContainSubstring("lost_item"))
				Expect(report).To(ContainSubstring("Orphaned Items"))
			})
		})

		When("there are no anomalies", func() {
			var err error

			BeforeEach(func() {
				err = job.generateAuditReport([]InventoryAnomaly{}, []string{})
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should indicate no anomalies", func() {
				report := mockDeps.writtenPages[AuditReportPage].markdown
				Expect(report).To(ContainSubstring("No anomalies detected"))
			})
		})
	})

	Describe("formatAnomalyType", func() {
		It("should format orphan correctly", func() {
			Expect(formatAnomalyType("orphan")).To(Equal("Orphaned Items"))
		})

		It("should format multiple_containers correctly", func() {
			Expect(formatAnomalyType("multiple_containers")).To(Equal("Items in Multiple Containers"))
		})

		It("should format circular_reference correctly", func() {
			Expect(formatAnomalyType("circular_reference")).To(Equal("Circular References"))
		})

		It("should format missing_page correctly", func() {
			Expect(formatAnomalyType("missing_page")).To(Equal("Missing Pages"))
		})

		It("should format page_creation_failed correctly", func() {
			Expect(formatAnomalyType("page_creation_failed")).To(Equal("Page Creation Failures"))
		})

		It("should handle unknown types", func() {
			result := formatAnomalyType("unknown_type")
			Expect(result).To(Equal("Unknown Type"))
		})
	})

	Describe("getContainerItems edge cases", func() {
		BeforeEach(func() {
			job = NewInventoryNormalizationJob(mockDeps, mockFmIndex, logger, nil)
		})

		When("container has items as []string type", func() {
			var items []string

			BeforeEach(func() {
				mockDeps.pages["drawer"] = &mockPageData{
					frontmatter: map[string]any{
						"title": "My Drawer",
						"inventory": map[string]any{
							"items": []string{"screwdriver", "hammer"},
						},
					},
				}

				items = job.getContainerItems("drawer")
			})

			It("should return all items", func() {
				Expect(items).To(HaveLen(2))
				Expect(items).To(ContainElements("screwdriver", "hammer"))
			})
		})

		When("container has no items key", func() {
			var items []string

			BeforeEach(func() {
				mockDeps.pages["drawer"] = &mockPageData{
					frontmatter: map[string]any{
						"title": "My Drawer",
						"inventory": map[string]any{
							"container": "parent",
						},
					},
				}

				items = job.getContainerItems("drawer")
			})

			It("should return nil", func() {
				Expect(items).To(BeNil())
			})
		})
	})

	Describe("getItemsWithContainerReference", func() {
		BeforeEach(func() {
			job = NewInventoryNormalizationJob(mockDeps, mockFmIndex, logger, nil)
		})

		When("items reference a container", func() {
			var items []string

			BeforeEach(func() {
				mockFmIndex.QueryExactMatchFunc = func(key, value string) []string {
					if key == "inventory.container" && value == "drawer" {
						return []string{"screwdriver", "hammer"}
					}
					return nil
				}

				items = job.getItemsWithContainerReference("drawer")
			})

			It("should return items referencing the container", func() {
				Expect(items).To(HaveLen(2))
				Expect(items).To(ContainElements("screwdriver", "hammer"))
			})
		})
	})

	Describe("buildNormalizationItemMarkdown", func() {
		When("called", func() {
			var markdown string

			BeforeEach(func() {
				markdown = buildNormalizationItemMarkdown()
			})

			It("should contain the title template", func() {
				Expect(markdown).To(ContainSubstring("# {{or .Title .Identifier}}"))
			})

			It("should contain the inventory template", func() {
				Expect(markdown).To(ContainSubstring("Goes in:"))
			})
		})
	})

	Describe("mapKeysToSortedSlice", func() {
		When("given an empty map", func() {
			var result []string

			BeforeEach(func() {
				result = mapKeysToSortedSlice(map[string]bool{})
			})

			It("should return empty slice", func() {
				Expect(result).To(BeEmpty())
			})
		})

		When("given a map with keys", func() {
			var result []string

			BeforeEach(func() {
				result = mapKeysToSortedSlice(map[string]bool{
					"zebra":    true,
					"apple":    true,
					"mango":    true,
				})
			})

			It("should return sorted keys", func() {
				Expect(result).To(Equal([]string{"apple", "mango", "zebra"}))
			})
		})
	})

	Describe("GetName", func() {
		BeforeEach(func() {
			job = NewInventoryNormalizationJob(mockDeps, mockFmIndex, logger, nil)
		})

		When("called", func() {
			var name string

			BeforeEach(func() {
				name = job.GetName()
			})

			It("should return the job name", func() {
				Expect(name).To(Equal(InventoryNormalizationJobName))
			})
		})
	})

	Describe("generateAuditReport with error anomalies", func() {
		BeforeEach(func() {
			job = NewInventoryNormalizationJob(mockDeps, mockFmIndex, logger, nil)
		})

		When("there is an error severity anomaly", func() {
			var err error

			BeforeEach(func() {
				anomalies := []InventoryAnomaly{
					{
						Type:        "circular_reference",
						ItemID:      "looping_item",
						Description: "Item 'looping_item' has circular reference",
						Severity:    "error",
					},
				}

				err = job.generateAuditReport(anomalies, []string{})
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should include error icon in report", func() {
				report := mockDeps.writtenPages[AuditReportPage].markdown
				Expect(report).To(ContainSubstring("❌"))
			})
		})
	})

	Describe("findCycle edge cases", func() {
		BeforeEach(func() {
			job = NewInventoryNormalizationJob(mockDeps, mockFmIndex, logger, nil)
		})

		When("there is no parent container", func() {
			var cycle []string

			BeforeEach(func() {
				mockFmIndex.data["root_container"] = map[string]string{"inventory.container": ""}

				visited := make(map[string]bool)
				path := []string{}
				cycle = job.findCycle("root_container", visited, path)
			})

			It("should return nil", func() {
				Expect(cycle).To(BeNil())
			})
		})

		When("item is visited but not in path", func() {
			var cycle []string

			BeforeEach(func() {
				// Set up data
				mockFmIndex.data["container_a"] = map[string]string{"inventory.container": "container_b"}
				mockFmIndex.data["container_b"] = map[string]string{"inventory.container": ""}

				// Pre-mark container_a as visited but don't include in path
				visited := map[string]bool{"container_a": true}
				path := []string{"container_b"}
				cycle = job.findCycle("container_a", visited, path)
			})

			It("should return nil for non-cycle revisit", func() {
				Expect(cycle).To(BeNil())
			})
		})
	})
})
