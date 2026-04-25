//revive:disable:dot-imports
package eager_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/migrations/eager"
	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// fakeReaderMutator is the smallest possible PageReaderMutator — only
// ReadFrontMatter and WriteFrontMatter are exercised by the migration.
type fakeReaderMutator struct {
	pages map[string]wikipage.FrontMatter
	writeCount int
}

func newFakeReaderMutator(initial map[string]wikipage.FrontMatter) *fakeReaderMutator {
	return &fakeReaderMutator{pages: initial}
}

func (f *fakeReaderMutator) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	fm, ok := f.pages[string(id)]
	if !ok {
		return id, nil, errors.New("not found")
	}
	return id, fm, nil
}

func (f *fakeReaderMutator) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	f.writeCount++
	f.pages[string(id)] = fm
	return nil
}

func (f *fakeReaderMutator) ReadMarkdown(_ wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	return "", "", nil
}
func (f *fakeReaderMutator) WriteMarkdown(_ wikipage.PageIdentifier, _ wikipage.Markdown) error {
	return nil
}
func (f *fakeReaderMutator) DeletePage(_ wikipage.PageIdentifier) error { return nil }
func (f *fakeReaderMutator) ModifyMarkdown(_ wikipage.PageIdentifier, _ func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	return nil
}

var _ = Describe("ChecklistDataModelMigrationJob", func() {
	var (
		store   *fakeReaderMutator
		ulids   *ulid.SequenceGenerator
		job     *eager.ChecklistDataModelMigrationJob
	)

	BeforeEach(func() {
		ulids = ulid.NewSequenceGenerator(
			"01HXAAAAAAAAAAAAAAAAAAAAAA",
			"01HXBBBBBBBBBBBBBBBBBBBBBB",
		)
	})

	When("the page has a legacy-shape checklist", func() {
		var migrationErr error

		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"shopping": {
					"identifier": "shopping",
					"checklists": map[string]any{
						"groceries": map[string]any{
							"items": []any{
								map[string]any{"text": "milk", "checked": false},
								map[string]any{"text": "bread", "checked": true},
							},
						},
					},
				},
			})
			job = eager.NewChecklistDataModelMigrationJob(store, ulids, "shopping")
			migrationErr = job.Execute()
		})

		It("should not error", func() {
			Expect(migrationErr).NotTo(HaveOccurred())
		})

		It("should assign ULIDs to items lacking uid", func() {
			items := store.pages["shopping"]["checklists"].(map[string]any)["groceries"].(map[string]any)["items"].([]any)
			first := items[0].(map[string]any)
			Expect(first["uid"]).To(Equal("01HXAAAAAAAAAAAAAAAAAAAAAA"))
			second := items[1].(map[string]any)
			Expect(second["uid"]).To(Equal("01HXBBBBBBBBBBBBBBBBBBBBBB"))
		})

		It("should backfill sort_order in 1000 increments", func() {
			items := store.pages["shopping"]["checklists"].(map[string]any)["groceries"].(map[string]any)["items"].([]any)
			Expect(items[0].(map[string]any)["sort_order"]).To(Equal(int64(1000)))
			Expect(items[1].(map[string]any)["sort_order"]).To(Equal(int64(2000)))
		})

		It("should populate wiki.checklists.<list>.items.<uid>.created_at", func() {
			meta := store.pages["shopping"]["wiki"].(map[string]any)["checklists"].(map[string]any)["groceries"].(map[string]any)["items"].(map[string]any)
			itemMeta := meta["01HXAAAAAAAAAAAAAAAAAAAAAA"].(map[string]any)
			Expect(itemMeta["created_at"]).NotTo(BeNil())
			Expect(itemMeta["updated_at"]).NotTo(BeNil())
		})

		It("should stamp migrated_data_model = true", func() {
			wikiList := store.pages["shopping"]["wiki"].(map[string]any)["checklists"].(map[string]any)["groceries"].(map[string]any)
			Expect(wikiList["migrated_data_model"]).To(BeTrue())
		})
	})

	When("the page has already been migrated", func() {
		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"shopping": {
					"identifier": "shopping",
					"checklists": map[string]any{
						"groceries": map[string]any{
							"items": []any{
								map[string]any{"text": "milk", "checked": false, "uid": "existing", "sort_order": int64(1000)},
							},
						},
					},
					"wiki": map[string]any{
						"checklists": map[string]any{
							"groceries": map[string]any{
								"migrated_data_model": true,
								"items": map[string]any{
									"existing": map[string]any{"created_at": "2026-04-25T13:00:00Z"},
								},
							},
						},
					},
				},
			})
			job = eager.NewChecklistDataModelMigrationJob(store, ulids, "shopping")
			_ = job.Execute()
			_ = job.Execute() // run twice
		})

		It("should be idempotent (no spurious writes for already-migrated lists)", func() {
			Expect(store.writeCount).To(Equal(0))
		})
	})
})
