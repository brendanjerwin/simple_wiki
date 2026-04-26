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

func (*fakeReaderMutator) ReadMarkdown(_ wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	return "", "", nil
}
func (*fakeReaderMutator) WriteMarkdown(_ wikipage.PageIdentifier, _ wikipage.Markdown) error {
	return nil
}
func (*fakeReaderMutator) DeletePage(_ wikipage.PageIdentifier) error { return nil }
func (*fakeReaderMutator) ModifyMarkdown(_ wikipage.PageIdentifier, _ func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	return nil
}

// asMap is a tiny test helper that asserts v is a non-nil map[string]any
// and returns it. Keeps the deep-walk assertions in the test bodies tidy
// and gives the linter a single point to flag if the input shape ever
// changes.
func asMap(v any) map[string]any {
	m, ok := v.(map[string]any)
	if !ok {
		Fail("expected map[string]any")
	}
	return m
}

// asAnySlice mirrors asMap for []any.
func asAnySlice(v any) []any {
	s, ok := v.([]any)
	if !ok {
		Fail("expected []any")
	}
	return s
}

var _ = Describe("ChecklistDataModelMigrationJob", func() {
	var (
		store *fakeReaderMutator
		ulids *ulid.SequenceGenerator
		job   *eager.ChecklistDataModelMigrationJob
	)

	BeforeEach(func() {
		ulids = ulid.NewSequenceGenerator(
			"01HXAAAAAAAAAAAAAAAAAAAAAA",
			"01HXBBBBBBBBBBBBBBBBBBBBBB",
		)
	})

	When("the page has a legacy-shape checklist outside the wiki namespace", func() {
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

		It("should remove the legacy checklists.* subtree entirely", func() {
			Expect(store.pages["shopping"]).NotTo(HaveKey("checklists"))
		})

		It("should move items into wiki.checklists.<list>.items[]", func() {
			items := asAnySlice(asMap(asMap(asMap(store.pages["shopping"]["wiki"])["checklists"])["groceries"])["items"])
			Expect(items).To(HaveLen(2))
		})

		It("should assign ULIDs to items lacking uid", func() {
			items := asAnySlice(asMap(asMap(asMap(store.pages["shopping"]["wiki"])["checklists"])["groceries"])["items"])
			first := asMap(items[0])
			Expect(first["uid"]).To(Equal("01HXAAAAAAAAAAAAAAAAAAAAAA"))
			second := asMap(items[1])
			Expect(second["uid"]).To(Equal("01HXBBBBBBBBBBBBBBBBBBBBBB"))
		})

		It("should backfill sort_order in 1000 increments", func() {
			items := asAnySlice(asMap(asMap(asMap(store.pages["shopping"]["wiki"])["checklists"])["groceries"])["items"])
			Expect(asMap(items[0])["sort_order"]).To(Equal(int64(1000)))
			Expect(asMap(items[1])["sort_order"]).To(Equal(int64(2000)))
		})

		It("should stamp created_at/updated_at on each item", func() {
			items := asAnySlice(asMap(asMap(asMap(store.pages["shopping"]["wiki"])["checklists"])["groceries"])["items"])
			Expect(asMap(items[0])["created_at"]).NotTo(BeNil())
			Expect(asMap(items[0])["updated_at"]).NotTo(BeNil())
		})

		It("should record automated=false for pre-existing items (no retroactive attribution)", func() {
			items := asAnySlice(asMap(asMap(asMap(store.pages["shopping"]["wiki"])["checklists"])["groceries"])["items"])
			Expect(asMap(items[0])["automated"]).To(BeFalse())
		})

		It("should stamp wiki.checklists.<list>.migrated_data_model = true", func() {
			wikiList := asMap(asMap(asMap(store.pages["shopping"]["wiki"])["checklists"])["groceries"])
			Expect(wikiList["migrated_data_model"]).To(BeTrue())
		})

		It("should initialize wiki.checklists.<list>.sync_token = 0", func() {
			wikiList := asMap(asMap(asMap(store.pages["shopping"]["wiki"])["checklists"])["groceries"])
			Expect(wikiList["sync_token"]).To(Equal(int64(0)))
		})
	})

	When("the page was migrated by the old draft (items split between checklists.* and wiki.checklists.*.items map)", func() {
		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"shopping": {
					"identifier": "shopping",
					// User-data items lived in checklists.* under the old draft.
					"checklists": map[string]any{
						"groceries": map[string]any{
							"items": []any{
								map[string]any{"text": "milk", "checked": false, "uid": "existing-uid", "sort_order": int64(1000)},
							},
						},
					},
					// Per-item metadata lived in wiki.checklists.<list>.items as a uid-keyed map.
					"wiki": map[string]any{
						"checklists": map[string]any{
							"groceries": map[string]any{
								"migrated_data_model": true,
								"sync_token":          int64(3),
								"items": map[string]any{
									"existing-uid": map[string]any{
										"created_at": "2026-04-25T13:00:00Z",
										"updated_at": "2026-04-25T14:00:00Z",
										"automated":  true,
									},
								},
							},
						},
					},
				},
			})
			job = eager.NewChecklistDataModelMigrationJob(store, ulids, "shopping")
			_ = job.Execute()
		})

		It("should move items into a slice and discard the metadata map", func() {
			items := asAnySlice(asMap(asMap(asMap(store.pages["shopping"]["wiki"])["checklists"])["groceries"])["items"])
			Expect(items).To(HaveLen(1))
			Expect(asMap(items[0])["uid"]).To(Equal("existing-uid"))
		})

		It("should preserve created_at and automated from the old metadata map", func() {
			items := asAnySlice(asMap(asMap(asMap(store.pages["shopping"]["wiki"])["checklists"])["groceries"])["items"])
			Expect(asMap(items[0])["created_at"]).To(Equal("2026-04-25T13:00:00Z"))
			Expect(asMap(items[0])["automated"]).To(BeTrue())
		})

		It("should remove the legacy checklists.* subtree", func() {
			Expect(store.pages["shopping"]).NotTo(HaveKey("checklists"))
		})

		It("should preserve sync_token (no spurious reset)", func() {
			wikiList := asMap(asMap(asMap(store.pages["shopping"]["wiki"])["checklists"])["groceries"])
			Expect(wikiList["sync_token"]).To(Equal(int64(3)))
		})
	})

	When("the page is already in the new shape (single-namespace, items as slice)", func() {
		BeforeEach(func() {
			store = newFakeReaderMutator(map[string]wikipage.FrontMatter{
				"shopping": {
					"identifier": "shopping",
					"wiki": map[string]any{
						"checklists": map[string]any{
							"groceries": map[string]any{
								"migrated_data_model": true,
								"sync_token":          int64(7),
								"updated_at":          "2026-04-25T13:00:00Z",
								"items": []any{
									map[string]any{
										"uid":        "existing-uid",
										"text":       "milk",
										"checked":    false,
										"sort_order": int64(1000),
										"created_at": "2026-04-25T13:00:00Z",
										"updated_at": "2026-04-25T13:00:00Z",
										"automated":  false,
									},
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

		It("should be idempotent (no spurious writes)", func() {
			Expect(store.writeCount).To(Equal(0))
		})
	})
})
