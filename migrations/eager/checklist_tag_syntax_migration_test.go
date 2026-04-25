//revive:disable:dot-imports
package eager

import (
	"os"

	"github.com/brendanjerwin/simple_wiki/wikipage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// fakeMutator is a minimal in-memory PageReaderMutator that tracks writes for
// the per-page migration tests below.
type fakeMutator struct {
	frontmatter map[wikipage.PageIdentifier]wikipage.FrontMatter
	markdown    map[wikipage.PageIdentifier]wikipage.Markdown
	writes      int
}

func newFakeMutator() *fakeMutator {
	return &fakeMutator{
		frontmatter: map[wikipage.PageIdentifier]wikipage.FrontMatter{},
		markdown:    map[wikipage.PageIdentifier]wikipage.Markdown{},
	}
}

func (f *fakeMutator) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	fm, ok := f.frontmatter[id]
	if !ok {
		return id, nil, os.ErrNotExist
	}
	return id, fm, nil
}

func (f *fakeMutator) ReadMarkdown(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	md, ok := f.markdown[id]
	if !ok {
		return id, "", os.ErrNotExist
	}
	return id, md, nil
}

func (f *fakeMutator) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	f.frontmatter[id] = fm
	f.writes++
	return nil
}

func (f *fakeMutator) WriteMarkdown(id wikipage.PageIdentifier, md wikipage.Markdown) error {
	f.markdown[id] = md
	f.writes++
	return nil
}

func (f *fakeMutator) DeletePage(id wikipage.PageIdentifier) error {
	delete(f.frontmatter, id)
	delete(f.markdown, id)
	return nil
}

func (f *fakeMutator) ModifyMarkdown(id wikipage.PageIdentifier, modifier func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	current := f.markdown[id]
	next, err := modifier(current)
	if err != nil {
		return err
	}
	f.markdown[id] = next
	f.writes++
	return nil
}

var _ = Describe("pageNeedsChecklistMigration", func() {
	Describe("when the page has no checklists", func() {
		It("should return false", func() {
			Expect(pageNeedsChecklistMigration(map[string]any{"title": "x"})).To(BeFalse())
		})
	})

	Describe("when a checklist has a `:tag` item without the migrated flag", func() {
		It("should return true", func() {
			fm := map[string]any{
				"checklists": map[string]any{
					"groceries": map[string]any{
						"items": []any{
							map[string]any{"text": "milk :urgent", "checked": false},
						},
					},
				},
			}
			Expect(pageNeedsChecklistMigration(fm)).To(BeTrue())
		})
	})

	Describe("when the migrated flag is set", func() {
		It("should return false even if items still contain `:tag`", func() {
			fm := map[string]any{
				"checklists": map[string]any{
					"groceries": map[string]any{
						"migrated_tags_syntax": true,
						"items": []any{
							map[string]any{"text": "milk :urgent", "checked": false},
						},
					},
				},
			}
			Expect(pageNeedsChecklistMigration(fm)).To(BeFalse())
		})
	})

	Describe("when items already use `#tag`", func() {
		It("should return false", func() {
			fm := map[string]any{
				"checklists": map[string]any{
					"groceries": map[string]any{
						"items": []any{
							map[string]any{"text": "milk #urgent", "checked": false},
						},
					},
				},
			}
			Expect(pageNeedsChecklistMigration(fm)).To(BeFalse())
		})
	})

	// System pages ship with the wiki binary and are owned by syspage.Sync.
	// Even if a future embedded help page accidentally contains `:tag` text
	// inside a checklist, the migration must never touch it — those pages
	// are read-only at the wiki layer (the system-page guard rejects user
	// writes) and any rewrite would be undone on the next startup sync.
	// Skipping here is purely defensive.
	Describe("when frontmatter has system = true", func() {
		It("should return false even if items contain `:tag`", func() {
			fm := map[string]any{
				"system": true,
				"checklists": map[string]any{
					"groceries": map[string]any{
						"items": []any{
							map[string]any{"text": "milk :urgent", "checked": false},
						},
					},
				},
			}
			Expect(pageNeedsChecklistMigration(fm)).To(BeFalse())
		})
	})
})

var _ = Describe("rewriteChecklistTags", func() {
	Describe("when an item has a single `:tag` at the end", func() {
		var (
			fm     map[string]any
			result bool
		)

		BeforeEach(func() {
			fm = map[string]any{
				"checklists": map[string]any{
					"groceries": map[string]any{
						"items": []any{
							map[string]any{"text": "milk :urgent", "checked": false},
						},
					},
				},
			}
			result = rewriteChecklistTags(fm)
		})

		It("should report a change", func() {
			Expect(result).To(BeTrue())
		})

		It("should rewrite the item text", func() {
			groceries := fm["checklists"].(map[string]any)["groceries"].(map[string]any)
			items := groceries["items"].([]any)
			Expect(items[0].(map[string]any)["text"]).To(Equal("milk #urgent"))
		})

		It("should set the migrated flag on the checklist subtree", func() {
			groceries := fm["checklists"].(map[string]any)["groceries"].(map[string]any)
			Expect(groceries["migrated_tags_syntax"]).To(Equal(true))
		})
	})

	Describe("when an item has multiple `:tag` segments", func() {
		var fm map[string]any

		BeforeEach(func() {
			fm = map[string]any{
				"checklists": map[string]any{
					"todo": map[string]any{
						"items": []any{
							map[string]any{"text": ":urgent buy milk :groceries", "checked": false},
						},
					},
				},
			}
			rewriteChecklistTags(fm)
		})

		It("should rewrite all `:tag` to `#tag`", func() {
			todo := fm["checklists"].(map[string]any)["todo"].(map[string]any)
			items := todo["items"].([]any)
			Expect(items[0].(map[string]any)["text"]).To(Equal("#urgent buy milk #groceries"))
		})
	})

	Describe("when the checklist is already flagged migrated", func() {
		var (
			fm     map[string]any
			result bool
		)

		BeforeEach(func() {
			fm = map[string]any{
				"checklists": map[string]any{
					"old": map[string]any{
						"migrated_tags_syntax": true,
						"items": []any{
							map[string]any{"text": "milk :legacy", "checked": false},
						},
					},
				},
			}
			result = rewriteChecklistTags(fm)
		})

		It("should report no change", func() {
			Expect(result).To(BeFalse())
		})

		It("should leave items untouched", func() {
			old := fm["checklists"].(map[string]any)["old"].(map[string]any)
			items := old["items"].([]any)
			Expect(items[0].(map[string]any)["text"]).To(Equal("milk :legacy"))
		})
	})

	Describe("when an item contains `:` mid-word (not a tag)", func() {
		var fm map[string]any

		BeforeEach(func() {
			fm = map[string]any{
				"checklists": map[string]any{
					"meet": map[string]any{
						"items": []any{
							map[string]any{"text": "Meet 2:30pm with Alice :urgent", "checked": false},
						},
					},
				},
			}
			rewriteChecklistTags(fm)
		})

		It("should rewrite only the boundary-anchored `:urgent`", func() {
			meet := fm["checklists"].(map[string]any)["meet"].(map[string]any)
			items := meet["items"].([]any)
			Expect(items[0].(map[string]any)["text"]).To(Equal("Meet 2:30pm with Alice #urgent"))
		})
	})
})

var _ = Describe("ChecklistTagSyntaxMigrationJob", func() {
	Describe("when the page has legacy `:tag` items", func() {
		var (
			mut       *fakeMutator
			pageID    wikipage.PageIdentifier
			executeErr error
		)

		BeforeEach(func() {
			mut = newFakeMutator()
			pageID = wikipage.PageIdentifier("test-page")
			mut.frontmatter[pageID] = map[string]any{
				"checklists": map[string]any{
					"groceries": map[string]any{
						"items": []any{
							map[string]any{"text": "milk :urgent", "checked": false},
						},
					},
				},
			}

			job := NewChecklistTagSyntaxMigrationJob(mut, string(pageID))
			executeErr = job.Execute()
		})

		It("should not return an error", func() {
			Expect(executeErr).NotTo(HaveOccurred())
		})

		It("should write the migrated frontmatter exactly once", func() {
			Expect(mut.writes).To(Equal(1))
		})

		It("should rewrite the item text and set the migrated flag", func() {
			fm := mut.frontmatter[pageID]
			groceries := fm["checklists"].(map[string]any)["groceries"].(map[string]any)
			items := groceries["items"].([]any)
			Expect(items[0].(map[string]any)["text"]).To(Equal("milk #urgent"))
			Expect(groceries["migrated_tags_syntax"]).To(Equal(true))
		})
	})

	Describe("when the page is already flagged migrated", func() {
		var (
			mut       *fakeMutator
			executeErr error
		)

		BeforeEach(func() {
			mut = newFakeMutator()
			pageID := wikipage.PageIdentifier("done-page")
			mut.frontmatter[pageID] = map[string]any{
				"checklists": map[string]any{
					"old": map[string]any{
						"migrated_tags_syntax": true,
						"items":                []any{},
					},
				},
			}

			job := NewChecklistTagSyntaxMigrationJob(mut, string(pageID))
			executeErr = job.Execute()
		})

		It("should not return an error", func() {
			Expect(executeErr).NotTo(HaveOccurred())
		})

		It("should perform zero writes", func() {
			Expect(mut.writes).To(Equal(0))
		})
	})
})
