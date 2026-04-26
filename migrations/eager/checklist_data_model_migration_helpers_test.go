//revive:disable:dot-imports
package eager

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/jcelliott/lumber"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// scanFakeReaderMutator is a minimal PageReaderMutator for scan job tests.
type scanFakeReaderMutator struct{}

func (f *scanFakeReaderMutator) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	return id, nil, nil
}

func (f *scanFakeReaderMutator) WriteFrontMatter(_ wikipage.PageIdentifier, _ wikipage.FrontMatter) error {
	return nil
}

func (f *scanFakeReaderMutator) ReadMarkdown(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	return id, "", nil
}

func (f *scanFakeReaderMutator) WriteMarkdown(_ wikipage.PageIdentifier, _ wikipage.Markdown) error {
	return nil
}

func (f *scanFakeReaderMutator) DeletePage(_ wikipage.PageIdentifier) error { return nil }

func (f *scanFakeReaderMutator) ModifyMarkdown(_ wikipage.PageIdentifier, _ func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	return nil
}

// pageWithLegacyChecklist builds a minimal TOML frontmatter string with a
// legacy checklists subtree.
func pageWithLegacyChecklist(id string) []byte {
	return []byte("+++\nidentifier = '" + id + "'\n\n[checklists.groceries]\n\n[[checklists.groceries.items]]\ntext = 'milk'\nchecked = false\n+++\n\n# Body\n")
}

// pageWithNoChecklist builds a minimal TOML frontmatter string with no
// checklist data.
func pageWithNoChecklist(id string) []byte {
	return []byte("+++\nidentifier = '" + id + "'\ntitle = 'Simple'\n+++\n\n# Body\n")
}

var _ = Describe("ChecklistDataModelMigrationScanJob", func() {
	var (
		scanner     *MockDataDirScanner
		coordinator *jobs.JobQueueCoordinator
		readerMut   *scanFakeReaderMutator
		ulids       *ulid.SequenceGenerator
		job         *ChecklistDataModelMigrationScanJob
	)

	BeforeEach(func() {
		scanner = NewMockDataDirScanner()
		logger := lumber.NewConsoleLogger(lumber.WARN)
		coordinator = jobs.NewJobQueueCoordinator(logger)
		readerMut = &scanFakeReaderMutator{}
		ulids = ulid.NewSequenceGenerator("01HXAAAAAAAAAAAAAAAAAAAAAA")
		job = NewChecklistDataModelMigrationScanJob(scanner, coordinator, readerMut, ulids)
	})

	Describe("GetName", func() {
		It("should return the correct name", func() {
			Expect(job.GetName()).To(Equal("ChecklistDataModelMigrationScanJob"))
		})
	})

	Describe("Execute", func() {
		When("data directory does not exist", func() {
			var err error

			BeforeEach(func() {
				scanner.SetDirExists(false)
				err = job.Execute()
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not enqueue any jobs", func() {
				Expect(coordinator.GetActiveQueues()).To(BeEmpty())
			})
		})

		When("ListMDFiles returns an error", func() {
			var err error

			BeforeEach(func() {
				scanner.SetListError(errors.New("disk failure"))
				err = job.Execute()
			})

			It("should return a wrapped error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("list .md files"))
			})
		})

		When("no .md files are present", func() {
			var err error

			BeforeEach(func() {
				err = job.Execute()
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not enqueue any jobs", func() {
				Expect(coordinator.GetActiveQueues()).To(BeEmpty())
			})
		})

		When("a file cannot be read (ReadMDFile error)", func() {
			var err error

			BeforeEach(func() {
				scanner.AddFile("page.md", []byte("content"))
				scanner.SetReadError(errors.New("read failure"))
				err = job.Execute()
			})

			It("should not error (unreadable files are skipped)", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("a page has no frontmatter (no +++ prefix)", func() {
			var err error

			BeforeEach(func() {
				scanner.AddFile("page.md", []byte("# Just markdown\nno frontmatter"))
				err = job.Execute()
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not enqueue any jobs", func() {
				Expect(coordinator.GetActiveQueues()).To(BeEmpty())
			})
		})

		When("a page has no checklist data (no migration needed)", func() {
			var err error

			BeforeEach(func() {
				scanner.AddFile("clean.md", pageWithNoChecklist("clean-page"))
				err = job.Execute()
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not enqueue any jobs", func() {
				Expect(coordinator.GetActiveQueues()).To(BeEmpty())
			})
		})

		When("a page has a legacy checklist needing migration", func() {
			var err error

			BeforeEach(func() {
				scanner.AddFile("shopping.md", pageWithLegacyChecklist("shopping"))
				err = job.Execute()
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should enqueue a migration job", func() {
				Expect(coordinator.GetActiveQueues()).NotTo(BeEmpty())
			})
		})

		When("the same identifier appears in multiple files (dedup)", func() {
			var err error

			BeforeEach(func() {
				scanner.AddFile("a.md", pageWithLegacyChecklist("shopping"))
				scanner.AddFile("b.md", pageWithLegacyChecklist("shopping"))
				err = job.Execute()
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should only enqueue one migration job for the duplicated identifier", func() {
				Expect(coordinator.GetActiveQueues()).To(HaveLen(1))
			})
		})
	})
})

var _ = Describe("legacyHasAny", func() {
	When("fm has no checklists key", func() {
		It("should return false", func() {
			Expect(legacyHasAny(map[string]any{"title": "T"})).To(BeFalse())
		})
	})

	When("checklists value is not a map", func() {
		It("should return false", func() {
			fm := map[string]any{"checklists": "not a map"}
			Expect(legacyHasAny(fm)).To(BeFalse())
		})
	})

	When("checklists map has a list map entry", func() {
		It("should return true", func() {
			fm := map[string]any{
				"checklists": map[string]any{
					"groceries": map[string]any{"items": []any{}},
				},
			}
			Expect(legacyHasAny(fm)).To(BeTrue())
		})
	})

	When("checklists map contains a non-map entry only", func() {
		It("should return false", func() {
			fm := map[string]any{
				"checklists": map[string]any{
					"groceries": "not a map",
				},
			}
			Expect(legacyHasAny(fm)).To(BeFalse())
		})
	})
})

var _ = Describe("pageNeedsDataModelMigration", func() {
	When("the page is a system page", func() {
		It("should return false", func() {
			fm := map[string]any{"system_page": true}
			Expect(pageNeedsDataModelMigration(fm)).To(BeFalse())
		})
	})

	When("the page has a legacy checklists subtree", func() {
		It("should return true", func() {
			fm := map[string]any{
				"checklists": map[string]any{
					"groceries": map[string]any{"items": []any{}},
				},
			}
			Expect(pageNeedsDataModelMigration(fm)).To(BeTrue())
		})
	})

	When("wiki.checklists has a list where items is a map (old draft)", func() {
		It("should return true", func() {
			fm := map[string]any{
				"wiki": map[string]any{
					"checklists": map[string]any{
						"groceries": map[string]any{
							"items": map[string]any{"uid-1": map[string]any{"text": "milk"}},
						},
					},
				},
			}
			Expect(pageNeedsDataModelMigration(fm)).To(BeTrue())
		})
	})

	When("wiki.checklists has a list where items is a slice (new shape)", func() {
		It("should return false", func() {
			fm := map[string]any{
				"wiki": map[string]any{
					"checklists": map[string]any{
						"groceries": map[string]any{
							"items": []any{},
						},
					},
				},
			}
			Expect(pageNeedsDataModelMigration(fm)).To(BeFalse())
		})
	})

	When("the page has no checklist data at all", func() {
		It("should return false", func() {
			fm := map[string]any{"title": "T"}
			Expect(pageNeedsDataModelMigration(fm)).To(BeFalse())
		})
	})
})

var _ = Describe("readNestedMap", func() {
	When("all keys exist", func() {
		It("should return the deepest map", func() {
			fm := map[string]any{
				"a": map[string]any{
					"b": map[string]any{"c": "value"},
				},
			}
			result := readNestedMap(fm, "a", "b")
			Expect(result).To(HaveKey("c"))
		})
	})

	When("an intermediate key is missing", func() {
		It("should return nil", func() {
			fm := map[string]any{"a": map[string]any{}}
			Expect(readNestedMap(fm, "a", "missing")).To(BeNil())
		})
	})

	When("the starting map is nil", func() {
		It("should return nil", func() {
			Expect(readNestedMap(nil, "a")).To(BeNil())
		})
	})
})

var _ = Describe("ensureNestedMap", func() {
	When("the path does not exist", func() {
		It("should create intermediate maps and return the deepest", func() {
			fm := map[string]any{}
			result := ensureNestedMap(fm, "a", "b")
			Expect(result).NotTo(BeNil())
			Expect(fm).To(HaveKey("a"))
			Expect(fm["a"].(map[string]any)).To(HaveKey("b"))
		})
	})

	When("the path already exists", func() {
		It("should return the existing deepest map", func() {
			existing := map[string]any{"existing": true}
			fm := map[string]any{
				"a": map[string]any{"b": existing},
			}
			result := ensureNestedMap(fm, "a", "b")
			Expect(result).To(HaveKey("existing"))
		})
	})
})

var _ = Describe("ensureMapInParent", func() {
	When("the key is missing", func() {
		It("should create and return an empty map", func() {
			parent := map[string]any{}
			result := ensureMapInParent(parent, "child")
			Expect(result).NotTo(BeNil())
			Expect(parent).To(HaveKey("child"))
		})
	})

	When("the key holds a non-map value", func() {
		It("should replace it with an empty map", func() {
			parent := map[string]any{"child": "not a map"}
			result := ensureMapInParent(parent, "child")
			Expect(result).NotTo(BeNil())
			Expect(parent["child"]).To(BeAssignableToTypeOf(map[string]any{}))
		})
	})

	When("the key holds an existing map", func() {
		It("should return the existing map", func() {
			existing := map[string]any{"k": "v"}
			parent := map[string]any{"child": existing}
			result := ensureMapInParent(parent, "child")
			Expect(result).To(HaveKeyWithValue("k", "v"))
		})
	})
})

var _ = Describe("stringFromMap", func() {
	When("the key holds a string", func() {
		It("should return that string", func() {
			Expect(stringFromMap(map[string]any{"k": "hello"}, "k")).To(Equal("hello"))
		})
	})

	When("the key is missing", func() {
		It("should return empty string", func() {
			Expect(stringFromMap(map[string]any{}, "k")).To(BeEmpty())
		})
	})

	When("the key holds a non-string", func() {
		It("should return empty string", func() {
			Expect(stringFromMap(map[string]any{"k": 42}, "k")).To(BeEmpty())
		})
	})
})

var _ = Describe("boolFromMap", func() {
	When("the key holds true", func() {
		It("should return true", func() {
			Expect(boolFromMap(map[string]any{"k": true}, "k")).To(BeTrue())
		})
	})

	When("the key holds false", func() {
		It("should return false", func() {
			Expect(boolFromMap(map[string]any{"k": false}, "k")).To(BeFalse())
		})
	})

	When("the key is missing", func() {
		It("should return false", func() {
			Expect(boolFromMap(map[string]any{}, "k")).To(BeFalse())
		})
	})
})

var _ = Describe("int64FromMap", func() {
	When("the value is an int64", func() {
		It("should return (value, true)", func() {
			v, ok := int64FromMap(map[string]any{"k": int64(42)}, "k")
			Expect(ok).To(BeTrue())
			Expect(v).To(Equal(int64(42)))
		})
	})

	When("the value is an int", func() {
		It("should return (value, true)", func() {
			v, ok := int64FromMap(map[string]any{"k": int(7)}, "k")
			Expect(ok).To(BeTrue())
			Expect(v).To(Equal(int64(7)))
		})
	})

	When("the value is a float64", func() {
		It("should return (value, true)", func() {
			v, ok := int64FromMap(map[string]any{"k": float64(3.0)}, "k")
			Expect(ok).To(BeTrue())
			Expect(v).To(Equal(int64(3)))
		})
	})

	When("the key is missing", func() {
		It("should return (0, false)", func() {
			v, ok := int64FromMap(map[string]any{}, "k")
			Expect(ok).To(BeFalse())
			Expect(v).To(Equal(int64(0)))
		})
	})

	When("the value is a string", func() {
		It("should return (0, false)", func() {
			v, ok := int64FromMap(map[string]any{"k": "not a number"}, "k")
			Expect(ok).To(BeFalse())
			Expect(v).To(Equal(int64(0)))
		})
	})
})

var _ = Describe("int64Fallback", func() {
	When("the value is present", func() {
		It("should return the value", func() {
			Expect(int64Fallback(map[string]any{"k": int64(99)}, "k")).To(Equal(int64(99)))
		})
	})

	When("the key is missing", func() {
		It("should return 0", func() {
			Expect(int64Fallback(map[string]any{}, "k")).To(Equal(int64(0)))
		})
	})
})
