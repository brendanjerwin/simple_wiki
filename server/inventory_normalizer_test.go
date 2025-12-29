//revive:disable:dot-imports
package server

import (
	"errors"
	"os"

	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// mockNormalizerDeps implements InventoryNormalizationDependencies for testing.
type mockNormalizerDeps struct {
	frontmatters        map[string]map[string]any
	markdown            map[string]string
	readFrontMatterErr  error
	writeFrontMatterErr error
	writeMarkdownErr    error
}

func newMockNormalizerDeps() *mockNormalizerDeps {
	return &mockNormalizerDeps{
		frontmatters: make(map[string]map[string]any),
		markdown:     make(map[string]string),
	}
}

func (m *mockNormalizerDeps) ReadFrontMatter(page wikipage.PageIdentifier) (wikipage.PageIdentifier, map[string]any, error) {
	if m.readFrontMatterErr != nil {
		return "", nil, m.readFrontMatterErr
	}
	fm, ok := m.frontmatters[string(page)]
	if !ok {
		return "", nil, os.ErrNotExist
	}
	return page, fm, nil
}

func (m *mockNormalizerDeps) ReadMarkdown(page wikipage.PageIdentifier) (wikipage.PageIdentifier, string, error) {
	md, ok := m.markdown[string(page)]
	if !ok {
		return "", "", os.ErrNotExist
	}
	return page, md, nil
}

func (m *mockNormalizerDeps) WriteFrontMatter(page wikipage.PageIdentifier, fm map[string]any) error {
	if m.writeFrontMatterErr != nil {
		return m.writeFrontMatterErr
	}
	m.frontmatters[string(page)] = fm
	return nil
}

func (m *mockNormalizerDeps) WriteMarkdown(page wikipage.PageIdentifier, md string) error {
	if m.writeMarkdownErr != nil {
		return m.writeMarkdownErr
	}
	m.markdown[string(page)] = md
	return nil
}

func (*mockNormalizerDeps) DeletePage(_ wikipage.PageIdentifier) error {
	return nil
}

var _ = Describe("InventoryNormalizer", func() {
	var (
		deps       *mockNormalizerDeps
		normalizer *InventoryNormalizer
		logger     lumber.Logger
	)

	BeforeEach(func() {
		deps = newMockNormalizerDeps()
		logger = lumber.NewConsoleLogger(lumber.WARN)
		normalizer = NewInventoryNormalizer(deps, logger)
	})

	Describe("NormalizePage", func() {
		When("the page has no inventory.items", func() {
			var (
				createdPages []string
				err          error
			)

			BeforeEach(func() {
				deps.frontmatters["test_container"] = map[string]any{
					"identifier": "test_container",
					"title":      "Test Container",
				}
				createdPages, err = normalizer.NormalizePage("test_container")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not create any pages", func() {
				Expect(createdPages).To(BeEmpty())
			})
		})

		When("the page has inventory.items with existing pages", func() {
			var (
				createdPages []string
				err          error
			)

			BeforeEach(func() {
				deps.frontmatters["test_container"] = map[string]any{
					"identifier": "test_container",
					"inventory": map[string]any{
						"items": []any{"item1", "item2"},
					},
				}
				deps.frontmatters["item1"] = map[string]any{"identifier": "item1"}
				deps.frontmatters["item2"] = map[string]any{"identifier": "item2"}

				createdPages, err = normalizer.NormalizePage("test_container")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not create any pages since they exist", func() {
				Expect(createdPages).To(BeEmpty())
			})
		})

		When("the page has inventory.items with missing pages", func() {
			var (
				createdPages []string
				err          error
			)

			BeforeEach(func() {
				deps.frontmatters["test_container"] = map[string]any{
					"identifier": "test_container",
					"inventory": map[string]any{
						"items": []any{"new_item1", "new_item2"},
					},
				}

				createdPages, err = normalizer.NormalizePage("test_container")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create the missing pages", func() {
				Expect(createdPages).To(HaveLen(2))
				Expect(createdPages).To(ContainElements("new_item1", "new_item2"))
			})

			It("should set is_container = true on the container", func() {
				fm := deps.frontmatters["test_container"]
				inventory, ok := fm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(inventory["is_container"]).To(BeTrue())
			})
		})

		When("the page has some existing and some missing items", func() {
			var (
				createdPages []string
				err          error
			)

			BeforeEach(func() {
				deps.frontmatters["test_container"] = map[string]any{
					"identifier": "test_container",
					"inventory": map[string]any{
						"items": []any{"existing_item", "new_item"},
					},
				}
				deps.frontmatters["existing_item"] = map[string]any{"identifier": "existing_item"}

				createdPages, err = normalizer.NormalizePage("test_container")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should only create the missing page", func() {
				Expect(createdPages).To(HaveLen(1))
				Expect(createdPages).To(ContainElement("new_item"))
			})
		})

		When("CreateItemPage fails", func() {
			var (
				createdPages []string
				err          error
			)

			BeforeEach(func() {
				deps.frontmatters["test_container"] = map[string]any{
					"identifier": "test_container",
					"inventory": map[string]any{
						"items": []any{"failing_item"},
					},
				}
				deps.writeMarkdownErr = errors.New("write failed")

				createdPages, err = normalizer.NormalizePage("test_container")
			})

			It("should not return an error (failures are logged)", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not include the failed page in created pages", func() {
				Expect(createdPages).To(BeEmpty())
			})
		})
	})

	Describe("ensureIsContainerField", func() {
		When("the page does not exist", func() {
			var err error

			BeforeEach(func() {
				err = normalizer.ensureIsContainerField("nonexistent")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the page has no inventory section", func() {
			var err error

			BeforeEach(func() {
				deps.frontmatters["test_page"] = map[string]any{
					"identifier": "test_page",
					"title":      "Test Page",
				}
				err = normalizer.ensureIsContainerField("test_page")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not modify the frontmatter", func() {
				Expect(deps.frontmatters["test_page"]).NotTo(HaveKey("inventory"))
			})
		})

		When("the page has empty items array", func() {
			var err error

			BeforeEach(func() {
				deps.frontmatters["test_page"] = map[string]any{
					"identifier": "test_page",
					"inventory": map[string]any{
						"items": []any{},
					},
				}
				err = normalizer.ensureIsContainerField("test_page")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not set is_container", func() {
				inventory, ok := deps.frontmatters["test_page"]["inventory"].(map[string]any)
				Expect(ok).To(BeTrue(), "inventory should be a map")
				Expect(inventory).NotTo(HaveKey("is_container"))
			})
		})

		When("is_container is already true", func() {
			var err error

			BeforeEach(func() {
				deps.frontmatters["test_page"] = map[string]any{
					"identifier": "test_page",
					"inventory": map[string]any{
						"items":        []any{"item1"},
						"is_container": true,
					},
				}
				err = normalizer.ensureIsContainerField("test_page")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("is_container needs to be set", func() {
			var err error

			BeforeEach(func() {
				deps.frontmatters["test_page"] = map[string]any{
					"identifier": "test_page",
					"inventory": map[string]any{
						"items": []any{"item1"},
					},
				}
				err = normalizer.ensureIsContainerField("test_page")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should set is_container to true", func() {
				inventory, ok := deps.frontmatters["test_page"]["inventory"].(map[string]any)
				Expect(ok).To(BeTrue(), "inventory should be a map")
				Expect(inventory["is_container"]).To(BeTrue())
			})
		})

		When("WriteFrontMatter fails", func() {
			var err error

			BeforeEach(func() {
				deps.frontmatters["test_page"] = map[string]any{
					"identifier": "test_page",
					"inventory": map[string]any{
						"items": []any{"item1"},
					},
				}
				deps.writeFrontMatterErr = errors.New("write failed")

				err = normalizer.ensureIsContainerField("test_page")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to write frontmatter"))
			})
		})
	})

	Describe("GetContainerItems", func() {
		When("the page has items as []string", func() {
			var items []string
			var err error

			BeforeEach(func() {
				deps.frontmatters["container"] = map[string]any{
					"inventory": map[string]any{
						"items": []string{"item1", "item2"},
					},
				}
				items, err = normalizer.GetContainerItems("container")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the items", func() {
				Expect(items).To(HaveLen(2))
				Expect(items).To(ContainElements("item1", "item2"))
			})
		})

		When("the page has []string items that need munging", func() {
			var items []string
			var err error

			BeforeEach(func() {
				deps.frontmatters["container"] = map[string]any{
					"inventory": map[string]any{
						"items": []string{"PascalCaseItem", "another-item"},
					},
				}
				items, err = normalizer.GetContainerItems("container")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should munge the identifiers to snake_case", func() {
				Expect(items).To(HaveLen(2))
				Expect(items).To(ContainElement("pascal_case_item"))
				Expect(items).To(ContainElement("another_item"))
			})
		})

		When("the page has []string items with invalid identifier", func() {
			var err error

			BeforeEach(func() {
				deps.frontmatters["container"] = map[string]any{
					"inventory": map[string]any{
						"items": []string{"///"},
					},
				}
				_, err = normalizer.GetContainerItems("container")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should include invalid identifier context", func() {
				Expect(err.Error()).To(ContainSubstring("invalid item identifier"))
			})
		})

		When("the page has items as []any", func() {
			var items []string
			var err error

			BeforeEach(func() {
				deps.frontmatters["container"] = map[string]any{
					"inventory": map[string]any{
						"items": []any{"item1", "item2"},
					},
				}
				items, err = normalizer.GetContainerItems("container")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the items", func() {
				Expect(items).To(HaveLen(2))
				Expect(items).To(ContainElements("item1", "item2"))
			})
		})

		When("the page does not exist", func() {
			var items []string
			var err error

			BeforeEach(func() {
				items, err = normalizer.GetContainerItems("nonexistent")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return nil", func() {
				Expect(items).To(BeNil())
			})
		})

		When("the page has no inventory section", func() {
			var items []string
			var err error

			BeforeEach(func() {
				deps.frontmatters["container"] = map[string]any{
					"identifier": "container",
				}
				items, err = normalizer.GetContainerItems("container")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return nil", func() {
				Expect(items).To(BeNil())
			})
		})

		When("the page has no items key", func() {
			var items []string
			var err error

			BeforeEach(func() {
				deps.frontmatters["container"] = map[string]any{
					"inventory": map[string]any{
						"is_container": true,
					},
				}
				items, err = normalizer.GetContainerItems("container")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return nil", func() {
				Expect(items).To(BeNil())
			})
		})

		When("an item has invalid identifier", func() {
			var err error

			BeforeEach(func() {
				deps.frontmatters["container"] = map[string]any{
					"inventory": map[string]any{
						"items": []any{"///"},  // This will fail MungeIdentifier
					},
				}
				_, err = normalizer.GetContainerItems("container")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should include invalid identifier context", func() {
				Expect(err.Error()).To(ContainSubstring("invalid item identifier"))
			})

			It("should include the identifier value", func() {
				Expect(err.Error()).To(ContainSubstring("///"))
			})
		})

		When("reading frontmatter fails with non-NotExist error", func() {
			var err error
			var localDeps *mockNormalizerDeps
			var localNormalizer *InventoryNormalizer

			BeforeEach(func() {
				localDeps = newMockNormalizerDeps()
				localDeps.readFrontMatterErr = errors.New("permission denied")
				localNormalizer = NewInventoryNormalizer(localDeps, lumber.NewConsoleLogger(lumber.WARN))
				_, err = localNormalizer.GetContainerItems("container")
			})

			It("should return an error with frontmatter read context", func() {
				Expect(err).To(MatchError(ContainSubstring("failed to read frontmatter")))
			})
		})
	})

	Describe("CreateItemPage", func() {
		When("creating a new item page", func() {
			var err error

			BeforeEach(func() {
				err = normalizer.CreateItemPage("new_item", "container_id")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create frontmatter with identifier", func() {
				fm := deps.frontmatters["new_item"]
				Expect(fm["identifier"]).To(Equal("new_item"))
			})

			It("should create frontmatter with container reference", func() {
				fm := deps.frontmatters["new_item"]
				inventory, ok := fm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue(), "inventory should be a map")
				Expect(inventory["container"]).To(Equal("container_id"))
			})
		})

		When("creating an item without container", func() {
			var err error

			BeforeEach(func() {
				err = normalizer.CreateItemPage("standalone_item", "")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not have container in inventory", func() {
				fm := deps.frontmatters["standalone_item"]
				inventory, ok := fm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue(), "inventory should be a map")
				Expect(inventory).NotTo(HaveKey("container"))
			})
		})

		When("WriteFrontMatter fails", func() {
			var err error

			BeforeEach(func() {
				deps.writeFrontMatterErr = errors.New("write failed")
				err = normalizer.CreateItemPage("failing_item", "container")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to write frontmatter"))
			})
		})

		When("WriteMarkdown fails", func() {
			var err error

			BeforeEach(func() {
				deps.writeMarkdownErr = errors.New("markdown write failed")
				err = normalizer.CreateItemPage("failing_item", "container")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to write markdown"))
			})
		})
	})
})

var _ = Describe("PageInventoryNormalizationJob", func() {
	var (
		deps   *mockNormalizerDeps
		logger lumber.Logger
	)

	BeforeEach(func() {
		deps = newMockNormalizerDeps()
		logger = lumber.NewConsoleLogger(lumber.WARN)
	})

	Describe("NewPageInventoryNormalizationJob", func() {
		When("creating a new job", func() {
			var job *PageInventoryNormalizationJob

			BeforeEach(func() {
				job = NewPageInventoryNormalizationJob("test_page", deps, logger)
			})

			It("should create a job with the correct page ID", func() {
				Expect(job.pageID).To(Equal(wikipage.PageIdentifier("test_page")))
			})

			It("should have a normalizer", func() {
				Expect(job.normalizer).NotTo(BeNil())
			})
		})
	})

	Describe("Execute", func() {
		When("normalization creates pages", func() {
			var (
				job *PageInventoryNormalizationJob
				err error
			)

			BeforeEach(func() {
				deps.frontmatters["container_page"] = map[string]any{
					"identifier": "container_page",
					"inventory": map[string]any{
						"items": []any{"new_item"},
					},
				}

				job = NewPageInventoryNormalizationJob("container_page", deps, logger)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create the missing item page", func() {
				Expect(deps.frontmatters).To(HaveKey("new_item"))
			})
		})

		When("normalization creates no pages", func() {
			var (
				job *PageInventoryNormalizationJob
				err error
			)

			BeforeEach(func() {
				deps.frontmatters["regular_page"] = map[string]any{
					"identifier": "regular_page",
				}

				job = NewPageInventoryNormalizationJob("regular_page", deps, logger)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("GetName", func() {
		When("getting the job name", func() {
			var (
				job  *PageInventoryNormalizationJob
				name string
			)

			BeforeEach(func() {
				job = NewPageInventoryNormalizationJob("test_page", deps, logger)
				name = job.GetName()
			})

			It("should return the correct name", func() {
				Expect(name).To(Equal(PageInventoryNormalizationJobName))
			})
		})
	})
})
