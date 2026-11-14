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

// nilFMModifier wraps mockPageReaderMutator but overrides ModifyFrontMatterAndMarkdown
// to pass nil FM when the page does not exist — matching real pagestore.Store behaviour.
// The base mock converts nil to empty map before calling the callback, so this wrapper
// is required to test the `if fm == nil { return nil, md, errInventorySkipWrite }` branches.
type nilFMModifier struct {
	*mockPageReaderMutator
}

func (n *nilFMModifier) ModifyFrontMatterAndMarkdown(
	id wikipage.PageIdentifier,
	fn func(wikipage.FrontMatter, wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error),
	_ wikipage.Identity,
) error {
	var currentFM wikipage.FrontMatter
	var currentMD wikipage.Markdown
	if page, ok := n.pages[string(id)]; ok {
		currentFM = page.frontmatter
		currentMD = wikipage.Markdown(page.markdown)
	}
	// Intentionally does NOT convert nil FM to empty map — simulates real pagestore behaviour.

	newFM, newMD, err := fn(currentFM, currentMD)
	if err != nil {
		return err
	}

	if n.pages[string(id)] == nil {
		n.pages[string(id)] = &mockPage{}
	}
	n.pages[string(id)].frontmatter = newFM
	n.pages[string(id)].markdown = string(newMD)
	return nil
}

// newJobWithAtomicDeps creates an InventoryNormalizationJob whose deps implement
// ModifyFrontMatterAndMarkdown, so the job uses the atomic code paths.
func newJobWithAtomicDeps(deps InventoryNormalizationDependencies, logger logging.Logger) *InventoryNormalizationJob {
	fmIndex := &mockFrontmatterIndexQueryer{data: make(map[string]map[string]string)}
	job, err := NewInventoryNormalizationJob(deps, fmIndex, logger)
	if err != nil {
		panic(err)
	}
	return job
}

var _ = Describe("InventoryNormalizationJob atomic paths", func() {
	var logger logging.Logger

	BeforeEach(func() {
		logger = lumber.NewConsoleLogger(lumber.WARN)
	})

	// -------------------------------------------------------------------------
	// migrateContainerIfNeeded — atomic branch
	// -------------------------------------------------------------------------

	Describe("migrateContainerIfNeeded atomic path", func() {
		When("page does not exist and modifier passes nil FM", func() {
			// Covers L390-391 and L414-415 (errInventorySkipWrite return)
			var result bool

			BeforeEach(func() {
				deps := &nilFMModifier{mockPageReaderMutator: newMockPageReaderMutator()}
				job := newJobWithAtomicDeps(deps, logger)
				result = job.migrateContainerIfNeeded("nonexistent_container")
			})

			It("should return false (nothing migrated)", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("is_container has an unexpected non-bool type", func() {
			// Covers L395-396: logger.Warn for unexpected is_container type
			var result bool
			var updatedInventory map[string]any

			BeforeEach(func() {
				deps := newMockPageReaderMutator()
				deps.setPage("container", map[string]any{
					"inventory": map[string]any{
						"is_container": 42, // unexpected integer type
					},
				}, "")
				job := newJobWithAtomicDeps(deps, logger)
				result = job.migrateContainerIfNeeded("container")
				fm := deps.getFrontmatter("container")
				updatedInventory, _ = fm["inventory"].(map[string]any)
			})

			It("should return true (overwrote the bad value)", func() {
				Expect(result).To(BeTrue())
			})

			It("should overwrite is_container with boolean true", func() {
				Expect(updatedInventory["is_container"]).To(BeTrue())
			})
		})

		When("is_container is already true", func() {
			// Covers L399 and L414-415 (errInventorySkipWrite return)
			var result bool

			BeforeEach(func() {
				deps := newMockPageReaderMutator()
				deps.setPage("container", map[string]any{
					"inventory": map[string]any{
						"is_container": true,
					},
				}, "")
				job := newJobWithAtomicDeps(deps, logger)
				result = job.migrateContainerIfNeeded("container")
			})

			It("should return false (nothing to migrate)", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("page exists but has no inventory section", func() {
			// Covers L404-406: inventory section created from scratch
			var result bool
			var updatedFM map[string]any

			BeforeEach(func() {
				deps := newMockPageReaderMutator()
				deps.setPage("container", map[string]any{
					"identifier": "container",
				}, "")
				job := newJobWithAtomicDeps(deps, logger)
				result = job.migrateContainerIfNeeded("container")
				updatedFM = deps.getFrontmatter("container")
			})

			It("should return true (migrated)", func() {
				Expect(result).To(BeTrue())
			})

			It("should create an inventory section with is_container = true", func() {
				inv, ok := updatedFM["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(inv["is_container"]).To(BeTrue())
			})
		})

		When("ModifyFrontMatterAndMarkdown returns a write error", func() {
			// Covers L417-418: error logging and false return on write failure
			var result bool

			BeforeEach(func() {
				deps := newMockPageReaderMutator()
				deps.setPage("container", map[string]any{
					"inventory": map[string]any{},
				}, "")
				deps.writeFrontMatterErr = errors.New("simulated write error")
				job := newJobWithAtomicDeps(deps, logger)
				result = job.migrateContainerIfNeeded("container")
			})

			It("should return false (migration failed)", func() {
				Expect(result).To(BeFalse())
			})
		})
	})

	// -------------------------------------------------------------------------
	// processContainerForItemRemoval — atomic branch
	// -------------------------------------------------------------------------

	Describe("processContainerForItemRemoval atomic path", func() {
		When("page does not exist and modifier passes nil FM", func() {
			// Covers L713-714 and L761-762 (errInventorySkipWrite)
			var count int
			var err error

			BeforeEach(func() {
				deps := &nilFMModifier{mockPageReaderMutator: newMockPageReaderMutator()}
				job := newJobWithAtomicDeps(deps, logger)
				count, err = job.processContainerForItemRemoval("nonexistent")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return zero removed count", func() {
				Expect(count).To(Equal(0))
			})
		})

		When("page exists but has no inventory section", func() {
			// Covers L718-719 and L761-762
			var count int
			var err error

			BeforeEach(func() {
				deps := newMockPageReaderMutator()
				deps.setPage("container", map[string]any{
					"identifier": "container",
				}, "")
				job := newJobWithAtomicDeps(deps, logger)
				count, err = job.processContainerForItemRemoval("container")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return zero removed count", func() {
				Expect(count).To(Equal(0))
			})
		})

		When("page has inventory section but no items array", func() {
			// Covers L723-724 and L761-762
			var count int
			var err error

			BeforeEach(func() {
				deps := newMockPageReaderMutator()
				deps.setPage("container", map[string]any{
					"inventory": map[string]any{
						"is_container": true,
					},
				}, "")
				job := newJobWithAtomicDeps(deps, logger)
				count, err = job.processContainerForItemRemoval("container")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return zero removed count", func() {
				Expect(count).To(Equal(0))
			})
		})

		When("items exist but none reference this container", func() {
			// Covers L729-730 and L761-762
			var count int
			var err error

			BeforeEach(func() {
				deps := newMockPageReaderMutator()
				deps.setPage("container", map[string]any{
					"inventory": map[string]any{
						"items": []any{"item1"},
					},
				}, "")
				deps.setPage("item1", map[string]any{
					"inventory": map[string]any{
						"container": "other_container",
					},
				}, "")
				job := newJobWithAtomicDeps(deps, logger)
				count, err = job.processContainerForItemRemoval("container")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return zero removed count", func() {
				Expect(count).To(Equal(0))
			})
		})

		When("some items reference this container and should be removed", func() {
			// Covers L736-753 (item loop branches: non-string item, to-remove, to-keep)
			var count int
			var err error
			var updatedFM map[string]any

			BeforeEach(func() {
				deps := newMockPageReaderMutator()
				deps.setPage("container", map[string]any{
					"inventory": map[string]any{
						// Non-string element (42) exercises the !ok branch.
						// item_to_remove references this container → removed.
						// item_to_keep does not reference this container → kept.
						"items": []any{42, "item_to_remove", "item_to_keep"},
					},
				}, "")
				deps.setPage("item_to_remove", map[string]any{
					"inventory": map[string]any{"container": "container"},
				}, "")
				deps.setPage("item_to_keep", map[string]any{
					"identifier": "item_to_keep",
				}, "")
				job := newJobWithAtomicDeps(deps, logger)
				count, err = job.processContainerForItemRemoval("container")
				updatedFM = deps.getFrontmatter("container")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return one removed item", func() {
				Expect(count).To(Equal(1))
			})

			It("should retain the non-string element and the kept item", func() {
				inv, ok := updatedFM["inventory"].(map[string]any)
				Expect(ok).To(BeTrue(), "inventory should be map[string]any")
				items, ok := inv["items"].([]any)
				Expect(ok).To(BeTrue(), "items should be []any")
				Expect(items).To(HaveLen(2))
			})
		})

		When("ModifyFrontMatterAndMarkdown returns a write error after removal", func() {
			// Covers L764-765: error wrapping and return
			var count int
			var err error

			BeforeEach(func() {
				deps := newMockPageReaderMutator()
				deps.setPage("container", map[string]any{
					"inventory": map[string]any{
						"items": []any{"item1"},
					},
				}, "")
				deps.setPage("item1", map[string]any{
					"inventory": map[string]any{"container": "container"},
				}, "")
				deps.writeFrontMatterErr = errors.New("simulated write error")
				job := newJobWithAtomicDeps(deps, logger)
				count, err = job.processContainerForItemRemoval("container")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to write frontmatter for container"))
			})

			It("should return zero count", func() {
				Expect(count).To(Equal(0))
			})
		})
	})
})
