//revive:disable:dot-imports
package server

import (
	"errors"

	"github.com/brendanjerwin/simple_wiki/pkg/logging"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("InventoryNormalizer", func() {
	var (
		deps       *mockPageReaderMutator
		normalizer *InventoryNormalizer
		logger     logging.Logger
	)

	BeforeEach(func() {
		deps = newMockPageReaderMutator()
		logger = lumber.NewConsoleLogger(lumber.WARN)
		normalizer = NewInventoryNormalizer(deps, logger)
	})

	Describe("NormalizePage", func() {
		When("the page has no inventory.items", func() {
			var (
				result *NormalizeResult
				err    error
			)

			BeforeEach(func() {
				deps.setPage("test_container", map[string]any{
					"identifier": "test_container",
					"title":      "Test Container",
				}, "")
				result, err = normalizer.NormalizePage("test_container")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not create any pages", func() {
				Expect(result.CreatedPages).To(BeEmpty())
			})
		})

		When("the page has inventory.items with existing pages", func() {
			var (
				result *NormalizeResult
				err    error
			)

			BeforeEach(func() {
				deps.setPage("test_container", map[string]any{
					"identifier": "test_container",
					"inventory": map[string]any{
						"items": []any{"item1", "item2"},
					},
				}, "")
				deps.setPage("item1", map[string]any{"identifier": "item1"}, "")
				deps.setPage("item2", map[string]any{"identifier": "item2"}, "")

				result, err = normalizer.NormalizePage("test_container")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not create any pages since they exist", func() {
				Expect(result.CreatedPages).To(BeEmpty())
			})
		})

		When("the page has inventory.items with missing pages", func() {
			var (
				result             *NormalizeResult
				err                error
				containerInventory map[string]any
			)

			BeforeEach(func() {
				deps.setPage("test_container", map[string]any{
					"identifier": "test_container",
					"inventory": map[string]any{
						"items": []any{"new_item1", "new_item2"},
					},
				}, "")

				result, err = normalizer.NormalizePage("test_container")

				fm := deps.getFrontmatter("test_container")
				var ok bool
				containerInventory, ok = fm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue(), "inventory should be a map")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create the missing pages", func() {
				Expect(result.CreatedPages).To(HaveLen(2))
				Expect(result.CreatedPages).To(ContainElements("new_item1", "new_item2"))
			})

			It("should set is_container = true on the container", func() {
				Expect(containerInventory).NotTo(BeNil())
				Expect(containerInventory["is_container"]).To(BeTrue())
			})
		})

		When("the page has some existing and some missing items", func() {
			var (
				result *NormalizeResult
				err    error
			)

			BeforeEach(func() {
				deps.setPage("test_container", map[string]any{
					"identifier": "test_container",
					"inventory": map[string]any{
						"items": []any{"existing_item", "new_item"},
					},
				}, "")
				deps.setPage("existing_item", map[string]any{"identifier": "existing_item"}, "")

				result, err = normalizer.NormalizePage("test_container")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should only create the missing page", func() {
				Expect(result.CreatedPages).To(HaveLen(1))
				Expect(result.CreatedPages).To(ContainElement("new_item"))
			})
		})

		When("CreateItemPage fails", func() {
			var (
				result *NormalizeResult
				err    error
			)

			BeforeEach(func() {
				deps.setPage("test_container", map[string]any{
					"identifier": "test_container",
					"inventory": map[string]any{
						"items": []any{"failing_item"},
					},
				}, "")
				deps.writeMarkdownErr = errors.New("write failed")

				result, err = normalizer.NormalizePage("test_container")
			})

			It("should not return an error (failures are tracked)", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not include the failed page in created pages", func() {
				Expect(result.CreatedPages).To(BeEmpty())
			})

			It("should track the failed page with error details", func() {
				Expect(result.FailedPages).To(HaveLen(1))
				Expect(result.FailedPages[0].ItemID).To(Equal("failing_item"))
				Expect(result.FailedPages[0].ContainerID).To(Equal("test_container"))
				Expect(result.FailedPages[0].Error).To(HaveOccurred())
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
				deps.setPage("test_page", map[string]any{
					"identifier": "test_page",
					"title":      "Test Page",
				}, "")
				err = normalizer.ensureIsContainerField("test_page")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not modify the frontmatter", func() {
				Expect(deps.getFrontmatter("test_page")).NotTo(HaveKey("inventory"))
			})
		})

		When("the page has empty items array", func() {
			var (
				err       error
				inventory map[string]any
			)

			BeforeEach(func() {
				deps.setPage("test_page", map[string]any{
					"identifier": "test_page",
					"inventory": map[string]any{
						"items": []any{},
					},
				}, "")
				err = normalizer.ensureIsContainerField("test_page")
				var ok bool
				inventory, ok = deps.getFrontmatter("test_page")["inventory"].(map[string]any)
				Expect(ok).To(BeTrue(), "inventory should be a map")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not set is_container", func() {
				Expect(inventory).NotTo(HaveKey("is_container"))
			})
		})

		When("is_container is already true", func() {
			var err error

			BeforeEach(func() {
				deps.setPage("test_page", map[string]any{
					"identifier": "test_page",
					"inventory": map[string]any{
						"items":        []any{"item1"},
						"is_container": true,
					},
				}, "")
				err = normalizer.ensureIsContainerField("test_page")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("is_container needs to be set", func() {
			var (
				err       error
				inventory map[string]any
			)

			BeforeEach(func() {
				deps.setPage("test_page", map[string]any{
					"identifier": "test_page",
					"inventory": map[string]any{
						"items": []any{"item1"},
					},
				}, "")
				err = normalizer.ensureIsContainerField("test_page")
				var ok bool
				inventory, ok = deps.getFrontmatter("test_page")["inventory"].(map[string]any)
				Expect(ok).To(BeTrue(), "inventory should be a map")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should set is_container to true", func() {
				Expect(inventory["is_container"]).To(BeTrue())
			})
		})

		When("WriteFrontMatter fails", func() {
			var err error

			BeforeEach(func() {
				deps.setPage("test_page", map[string]any{
					"identifier": "test_page",
					"inventory": map[string]any{
						"items": []any{"item1"},
					},
				}, "")
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
				deps.setPage("container", map[string]any{
					"inventory": map[string]any{
						"items": []string{"item1", "item2"},
					},
				}, "")
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
				deps.setPage("container", map[string]any{
					"inventory": map[string]any{
						"items": []string{"PascalCaseItem", "another-item"},
					},
				}, "")
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
				deps.setPage("container", map[string]any{
					"inventory": map[string]any{
						"items": []string{"///"},
					},
				}, "")
				_, err = normalizer.GetContainerItems("container")
			})

			It("should return an error about invalid item identifier", func() {
				Expect(err).To(MatchError(ContainSubstring("invalid item identifier")))
			})
		})

		When("the page has items as []any", func() {
			var items []string
			var err error

			BeforeEach(func() {
				deps.setPage("container", map[string]any{
					"inventory": map[string]any{
						"items": []any{"item1", "item2"},
					},
				}, "")
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
				deps.setPage("container", map[string]any{
					"identifier": "container",
				}, "")
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
				deps.setPage("container", map[string]any{
					"inventory": map[string]any{
						"is_container": true,
					},
				}, "")
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
				deps.setPage("container", map[string]any{
					"inventory": map[string]any{
						"items": []any{"///"},  // This will fail MungeIdentifier
					},
				}, "")
				_, err = normalizer.GetContainerItems("container")
			})

			It("should return an error about invalid item identifier", func() {
				Expect(err).To(MatchError(ContainSubstring("invalid item identifier")))
				Expect(err.Error()).To(ContainSubstring("///"))
			})
		})

		When("reading frontmatter fails with non-NotExist error", func() {
			var err error
			var localDeps *mockPageReaderMutator
			var localNormalizer *InventoryNormalizer

			BeforeEach(func() {
				localDeps = newMockPageReaderMutator()
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
			var (
				err       error
				fm        map[string]any
				inventory map[string]any
			)

			BeforeEach(func() {
				err = normalizer.CreateItemPage("new_item", "container_id")
				fm = deps.getFrontmatter("new_item")
				var ok bool
				inventory, ok = fm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue(), "inventory should be a map")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create frontmatter with identifier", func() {
				Expect(fm["identifier"]).To(Equal("new_item"))
			})

			It("should create frontmatter with container reference", func() {
				Expect(inventory["container"]).To(Equal("container_id"))
			})
		})

		When("creating an item without container", func() {
			var (
				err       error
				inventory map[string]any
			)

			BeforeEach(func() {
				err = normalizer.CreateItemPage("standalone_item", "")
				fm := deps.getFrontmatter("standalone_item")
				var ok bool
				inventory, ok = fm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue(), "inventory should be a map")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not have container in inventory", func() {
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

		When("item identifier is invalid", func() {
			var err error

			BeforeEach(func() {
				err = normalizer.CreateItemPage("///", "container")
			})

			It("should return an error about invalid item identifier", func() {
				Expect(err).To(MatchError(ContainSubstring("invalid item identifier")))
			})
		})

		When("container identifier is invalid", func() {
			var err error

			BeforeEach(func() {
				err = normalizer.CreateItemPage("valid_item", "///")
			})

			It("should return an error about invalid container identifier", func() {
				Expect(err).To(MatchError(ContainSubstring("invalid container identifier")))
			})
		})
	})
})

var _ = Describe("PageInventoryNormalizationJob", func() {
	var (
		deps   *mockPageReaderMutator
		logger logging.Logger
	)

	BeforeEach(func() {
		deps = newMockPageReaderMutator()
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
				deps.setPage("container_page", map[string]any{
					"identifier": "container_page",
					"inventory": map[string]any{
						"items": []any{"new_item"},
					},
				}, "")

				job = NewPageInventoryNormalizationJob("container_page", deps, logger)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create the missing item page", func() {
				Expect(deps.hasPage("new_item")).To(BeTrue())
			})
		})

		When("normalization creates no pages", func() {
			var (
				job *PageInventoryNormalizationJob
				err error
			)

			BeforeEach(func() {
				deps.setPage("regular_page", map[string]any{
					"identifier": "regular_page",
				}, "")

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

	Describe("Execute with failed pages", func() {
		When("normalization fails to create some pages", func() {
			var (
				job *PageInventoryNormalizationJob
				err error
			)

			BeforeEach(func() {
				deps.setPage("container_page", map[string]any{
					"identifier": "container_page",
					"inventory": map[string]any{
						"items": []any{"new_item"},
					},
				}, "")
				deps.writeMarkdownErr = errors.New("write failed")

				job = NewPageInventoryNormalizationJob("container_page", deps, logger)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("NormalizePage returns an error", func() {
			var (
				job *PageInventoryNormalizationJob
				err error
			)

			BeforeEach(func() {
				// Set up a page with an invalid item identifier that will fail MungeIdentifier
				deps.setPage("container_page", map[string]any{
					"identifier": "container_page",
					"inventory": map[string]any{
						"items": []any{"///"},
					},
				}, "")

				job = NewPageInventoryNormalizationJob("container_page", deps, logger)
				err = job.Execute()
			})

			It("should return the error", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("handling page read errors", func() {
		When("a page returns an error during read", func() {
			var (
				job *PageInventoryNormalizationJob
				err error
			)

			BeforeEach(func() {
				// Set up a container that references an item that will error on read
				deps.setPage("container_page", map[string]any{
					"identifier": "container_page",
					"inventory": map[string]any{
						"items": []any{"error_item"},
					},
				}, "")
				// error_item exists but returns error when read
				deps.setPageWithError("error_item", errors.New("simulated read error"))

				job = NewPageInventoryNormalizationJob("container_page", deps, logger)
				err = job.Execute()
			})

			It("should not return an error", func() {
				// Errors during item processing are logged, not returned
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("verifying markdown content", func() {
		When("a page is created with markdown", func() {
			var markdown string

			BeforeEach(func() {
				deps.setPage("container_page", map[string]any{
					"identifier": "container_page",
					"inventory": map[string]any{
						"items": []any{"new_item"},
					},
				}, "")

				job := NewPageInventoryNormalizationJob("container_page", deps, logger)
				_ = job.Execute()
				markdown = deps.getMarkdown("new_item")
			})

			It("should write markdown with title template", func() {
				Expect(markdown).To(ContainSubstring("# {{or .Title .Identifier}}"))
			})

			It("should write markdown with inventory template", func() {
				Expect(markdown).To(ContainSubstring("IsContainer"))
			})
		})
	})
})
