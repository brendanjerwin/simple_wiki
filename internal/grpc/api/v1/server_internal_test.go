// Internal tests for package v1 that need access to unexported types and methods.
// These complement the external tests in server_test.go (package v1_test).
package v1

import (
	"reflect"

	"github.com/brendanjerwin/simple_wiki/pageimport"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// isSameMap reports whether a and b are the same map (same underlying pointer).
// BeIdenticalTo cannot be used for maps because Go's == operator panics on
// non-comparable types; this helper uses reflect.Pointer instead.
func isSameMap(a, b map[string]any) bool {
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

var _ = Describe("mergeFrontmatterDeep", func() {
	Describe("flat merge", func() {
		var target map[string]any

		When("source has new keys not in target", func() {
			BeforeEach(func() {
				target = map[string]any{"existing": "value"}
				mergeFrontmatterDeep(target, map[string]any{"new": "added"})
			})

			It("should add new key to target", func() {
				Expect(target["new"]).To(Equal("added"))
			})

			It("should preserve existing key", func() {
				Expect(target["existing"]).To(Equal("value"))
			})
		})

		When("source overwrites an existing scalar key", func() {
			BeforeEach(func() {
				target = map[string]any{"title": "Old Title"}
				mergeFrontmatterDeep(target, map[string]any{"title": "New Title"})
			})

			It("should overwrite the key", func() {
				Expect(target["title"]).To(Equal("New Title"))
			})
		})

		When("source contains an array value", func() {
			BeforeEach(func() {
				target = map[string]any{"tags": []any{"old-tag"}}
				mergeFrontmatterDeep(target, map[string]any{"tags": []any{"new-tag-1", "new-tag-2"}})
			})

			It("should replace the array entirely", func() {
				Expect(target["tags"]).To(Equal([]any{"new-tag-1", "new-tag-2"}))
			})
		})
	})

	Describe("deep nested merge", func() {
		var target map[string]any

		When("source partially updates a nested object, leaving sibling keys untouched", func() {
			BeforeEach(func() {
				// This is the core issue scenario: updating one nested key should not drop siblings
				target = map[string]any{
					"checklists": map[string]any{
						"todos":         map[string]any{"items": []any{"task-a"}, "group_order": []any{"work"}},
						"confirmations": map[string]any{"items": []any{"confirm-a"}, "group_order": []any{"legal"}},
					},
				}
				mergeFrontmatterDeep(target, map[string]any{
					"checklists": map[string]any{
						"todos": map[string]any{"items": []any{"task-b"}},
					},
				})
			})

			It("should update the specified nested value", func() {
				checklists, ok := target["checklists"].(map[string]any)
				Expect(ok).To(BeTrue(), "checklists should be map[string]any")
				todos, ok := checklists["todos"].(map[string]any)
				Expect(ok).To(BeTrue(), "todos should be map[string]any")
				Expect(todos["items"]).To(Equal([]any{"task-b"}))
			})

			It("should preserve sibling keys at the nested level", func() {
				checklists, ok := target["checklists"].(map[string]any)
				Expect(ok).To(BeTrue(), "checklists should be map[string]any")
				Expect(checklists).To(HaveKey("confirmations"))
			})

			It("should preserve the sibling nested object intact", func() {
				checklists, ok := target["checklists"].(map[string]any)
				Expect(ok).To(BeTrue(), "checklists should be map[string]any")
				confirmations, ok := checklists["confirmations"].(map[string]any)
				Expect(ok).To(BeTrue(), "confirmations should be map[string]any")
				Expect(confirmations["items"]).To(Equal([]any{"confirm-a"}))
				Expect(confirmations["group_order"]).To(Equal([]any{"legal"}))
			})
		})

		When("source partially updates inner keys of an existing nested object", func() {
			BeforeEach(func() {
				target = map[string]any{
					"metadata": map[string]any{
						"author":  "alice",
						"version": "1.0",
					},
				}
				mergeFrontmatterDeep(target, map[string]any{
					"metadata": map[string]any{
						"version": "2.0",
					},
				})
			})

			It("should update the specified inner key", func() {
				metadata, ok := target["metadata"].(map[string]any)
				Expect(ok).To(BeTrue(), "metadata should be map[string]any")
				Expect(metadata["version"]).To(Equal("2.0"))
			})

			It("should preserve unspecified inner keys", func() {
				metadata, ok := target["metadata"].(map[string]any)
				Expect(ok).To(BeTrue(), "metadata should be map[string]any")
				Expect(metadata["author"]).To(Equal("alice"))
			})
		})

		When("source has a nested map where target has a scalar", func() {
			BeforeEach(func() {
				target = map[string]any{"key": "scalar-value"}
				mergeFrontmatterDeep(target, map[string]any{
					"key": map[string]any{"nested": "value"},
				})
			})

			It("should replace the scalar with the nested map", func() {
				Expect(target["key"]).To(Equal(map[string]any{"nested": "value"}))
			})
		})

		When("merging three levels deep", func() {
			BeforeEach(func() {
				target = map[string]any{
					"a": map[string]any{
						"b": map[string]any{
							"keep": "preserved",
							"c":    "old-value",
						},
					},
				}
				mergeFrontmatterDeep(target, map[string]any{
					"a": map[string]any{
						"b": map[string]any{
							"c": "new-value",
						},
					},
				})
			})

			It("should update the deeply nested key", func() {
				a, ok := target["a"].(map[string]any)
				Expect(ok).To(BeTrue(), "a should be map[string]any")
				b, ok := a["b"].(map[string]any)
				Expect(ok).To(BeTrue(), "b should be map[string]any")
				Expect(b["c"]).To(Equal("new-value"))
			})

			It("should preserve the sibling key at the deepest level", func() {
				a, ok := target["a"].(map[string]any)
				Expect(ok).To(BeTrue(), "a should be map[string]any")
				b, ok := a["b"].(map[string]any)
				Expect(ok).To(BeTrue(), "b should be map[string]any")
				Expect(b["keep"]).To(Equal("preserved"))
			})
		})
	})
})

// errorJobCoordinator always returns an error from EnqueueJob and EnqueueJobWithCompletion.
type errorJobCoordinator struct{}

func (*errorJobCoordinator) GetJobProgress() jobs.JobProgress { return jobs.JobProgress{} }

func (*errorJobCoordinator) EnqueueJobWithCompletion(_ jobs.Job, _ jobs.CompletionCallback) error {
	return nil
}

func (*errorJobCoordinator) EnqueueJob(_ jobs.Job) error {
	return nil
}

var _ = Describe("makeReportJobCallback internal", func() {
	Describe("when pageReaderMutator is nil on the server", func() {
		var callback func(error)

		BeforeEach(func() {
			// Construct a Server directly to bypass NewServer validation —
			// this lets us test the defensive createErr != nil path inside the callback.
			s := &Server{
				logger:              lumber.NewConsoleLogger(lumber.WARN),
				jobQueueCoordinator: &errorJobCoordinator{},
				// pageReaderMutator intentionally left nil
			}
			acc := server.NewPageImportResultAccumulator()
			callback = s.makeReportJobCallback(acc)
		})

		It("should not panic when the callback is invoked", func() {
			Expect(func() { callback(nil) }).NotTo(Panic())
		})
	})
})

var _ = Describe("enqueueImportJobs internal", func() {
	Describe("when pageReaderMutator is nil on the server", func() {
		var err error

		BeforeEach(func() {
			s := &Server{
				logger: lumber.NewConsoleLogger(lumber.WARN),
				// pageReaderMutator intentionally left nil to trigger NewSinglePageImportJob error
			}
			acc := server.NewPageImportResultAccumulator()
			records := []pageimport.ParsedRecord{
				{Identifier: "test_page"},
			}
			err = s.enqueueImportJobs(records, acc)
		})

		It("should return an error when NewSinglePageImportJob fails", func() {
			Expect(err).To(HaveOccurred())
		})

		It("should include the record number in the error message", func() {
			Expect(err.Error()).To(ContainSubstring("1"))
		})
	})
})

var _ = Describe("asMap", func() {
	Describe("when the key is absent", func() {
		var result map[string]any
		var ok bool

		BeforeEach(func() {
			result, ok = asMap(map[string]any{"other": "v"}, "missing")
		})

		It("should return false", func() {
			Expect(ok).To(BeFalse())
		})

		It("should return nil", func() {
			Expect(result).To(BeNil())
		})
	})

	Describe("when the value is not a map[string]any", func() {
		var result map[string]any
		var ok bool

		BeforeEach(func() {
			result, ok = asMap(map[string]any{"key": "not-a-map"}, "key")
		})

		It("should return false", func() {
			Expect(ok).To(BeFalse())
		})

		It("should return nil", func() {
			Expect(result).To(BeNil())
		})
	})

	Describe("when the value is a map[string]any", func() {
		var inner map[string]any
		var result map[string]any
		var ok bool

		BeforeEach(func() {
			inner = map[string]any{"nested": 42}
			result, ok = asMap(map[string]any{"key": inner}, "key")
		})

		It("should return true", func() {
			Expect(ok).To(BeTrue())
		})

		It("should return the same map instance", func() {
			Expect(isSameMap(result, inner)).To(BeTrue())
		})
	})
})

var _ = Describe("copyMapExcept", func() {
	Describe("with no drop keys", func() {
		var inner map[string]any
		var original, result map[string]any

		BeforeEach(func() {
			inner = map[string]any{"nested": 99}
			original = map[string]any{"a": 1, "b": "two", "c": inner}
			result = copyMapExcept(original)
		})

		It("should return a new map (not the same instance)", func() {
			Expect(isSameMap(result, original)).To(BeFalse())
		})

		It("should contain all original keys", func() {
			Expect(result).To(HaveLen(3))
			Expect(result["a"]).To(Equal(1))
			Expect(result["b"]).To(Equal("two"))
		})

		It("should share the nested map value with the original (shallow copy)", func() {
			Expect(isSameMap(result["c"].(map[string]any), inner)).To(BeTrue())
		})
	})

	Describe("with one drop key", func() {
		var original, result map[string]any

		BeforeEach(func() {
			original = map[string]any{"keep": "yes", "drop": "no", "also-keep": 99}
			result = copyMapExcept(original, "drop")
		})

		It("should omit the dropped key", func() {
			Expect(result).NotTo(HaveKey("drop"))
		})

		It("should preserve the other keys", func() {
			Expect(result["keep"]).To(Equal("yes"))
			Expect(result["also-keep"]).To(Equal(99))
		})
	})

	Describe("with multiple drop keys", func() {
		var result map[string]any

		BeforeEach(func() {
			result = copyMapExcept(map[string]any{"a": 1, "b": 2, "c": 3}, "a", "c")
		})

		It("should omit all dropped keys", func() {
			Expect(result).NotTo(HaveKey("a"))
			Expect(result).NotTo(HaveKey("c"))
		})

		It("should keep the non-dropped key", func() {
			Expect(result["b"]).To(Equal(2))
		})
	})

	Describe("when dropping a key that does not exist", func() {
		var result map[string]any

		BeforeEach(func() {
			result = copyMapExcept(map[string]any{"x": 10}, "missing")
		})

		It("should still copy all existing keys", func() {
			Expect(result["x"]).To(Equal(10))
		})
	})
})

var _ = Describe("needsStrip", func() {
	Describe("when checklists is empty", func() {
		var result bool

		BeforeEach(func() {
			result = needsStrip(map[string]any{})
		})

		It("should return false", func() {
			Expect(result).To(BeFalse())
		})
	})

	Describe("when no list has an events key", func() {
		var result bool

		BeforeEach(func() {
			result = needsStrip(map[string]any{
				"todos": map[string]any{"items": []any{"item1"}, "sync_token": 1},
			})
		})

		It("should return false", func() {
			Expect(result).To(BeFalse())
		})
	})

	Describe("when a list entry is not a map[string]any", func() {
		var result bool

		BeforeEach(func() {
			result = needsStrip(map[string]any{"bad": "not-a-map"})
		})

		It("should return false", func() {
			Expect(result).To(BeFalse())
		})
	})

	Describe("when at least one list has an events key", func() {
		var result bool

		BeforeEach(func() {
			result = needsStrip(map[string]any{
				"todos":     map[string]any{"items": []any{}, "events": []any{map[string]any{"seq": 1}}},
				"groceries": map[string]any{"items": []any{"milk"}},
			})
		})

		It("should return true", func() {
			Expect(result).To(BeTrue())
		})
	})
})

var _ = Describe("stripChecklistEvents", func() {
	Describe("nil input", func() {
		var result map[string]any

		BeforeEach(func() {
			result = stripChecklistEvents(nil)
		})

		It("should return nil", func() {
			Expect(result).To(BeNil())
		})
	})

	Describe("frontmatter with no wiki key", func() {
		var input, result map[string]any

		BeforeEach(func() {
			input = map[string]any{"title": "My Page"}
			result = stripChecklistEvents(input)
		})

		It("should return the original map unchanged", func() {
			Expect(result).To(Equal(input))
		})

		It("should return the same map instance (zero allocation)", func() {
			Expect(isSameMap(result, input)).To(BeTrue())
		})
	})

	Describe("wiki key present but no checklists", func() {
		var input, result map[string]any

		BeforeEach(func() {
			input = map[string]any{
				"title": "My Page",
				"wiki":  map[string]any{"page_app": "something"},
			}
			result = stripChecklistEvents(input)
		})

		It("should return the original map unchanged", func() {
			Expect(result).To(Equal(input))
		})

		It("should return the same map instance (zero allocation)", func() {
			Expect(isSameMap(result, input)).To(BeTrue())
		})
	})

	Describe("checklists present but no events on any list (fast path)", func() {
		var input, result map[string]any

		BeforeEach(func() {
			input = map[string]any{
				"wiki": map[string]any{
					"checklists": map[string]any{
						"todos": map[string]any{
							"items":      []any{"item1"},
							"sync_token": 1,
						},
					},
				},
			}
			result = stripChecklistEvents(input)
		})

		It("should return the original map unchanged", func() {
			Expect(result).To(Equal(input))
		})

		It("should return the same map instance (zero allocation)", func() {
			Expect(isSameMap(result, input)).To(BeTrue())
		})
	})

	Describe("checklists with events", func() {
		var input map[string]any
		var result map[string]any
		var originalEvents []any
		var resultTodos map[string]any
		var inputTodos map[string]any

		BeforeEach(func() {
			originalEvents = []any{
				map[string]any{"seq": 1, "op": "add", "src": "user"},
				map[string]any{"seq": 2, "op": "toggle", "src": "user"},
			}
			input = map[string]any{
				"title": "Checklist Page",
				"wiki": map[string]any{
					"checklists": map[string]any{
						"todos": map[string]any{
							"items":      []any{map[string]any{"uid": "abc", "text": "Buy milk"}},
							"tombstones": []any{map[string]any{"uid": "old", "deleted_at": "2026-01-01"}},
							"sync_token": 42,
							"updated_at": "2026-11-12T00:00:00Z",
							"events":     originalEvents,
						},
					},
				},
			}
			result = stripChecklistEvents(input)

			wiki, ok := result["wiki"].(map[string]any)
			Expect(ok).To(BeTrue(), "result[wiki] should be map[string]any")
			checklists, ok := wiki["checklists"].(map[string]any)
			Expect(ok).To(BeTrue(), "wiki[checklists] should be map[string]any")
			resultTodos, ok = checklists["todos"].(map[string]any)
			Expect(ok).To(BeTrue(), "checklists[todos] should be map[string]any")

			inputWiki, ok := input["wiki"].(map[string]any)
			Expect(ok).To(BeTrue(), "input[wiki] should be map[string]any")
			inputChecklists, ok := inputWiki["checklists"].(map[string]any)
			Expect(ok).To(BeTrue(), "input wiki[checklists] should be map[string]any")
			inputTodos, ok = inputChecklists["todos"].(map[string]any)
			Expect(ok).To(BeTrue(), "input checklists[todos] should be map[string]any")
		})

		It("should not return the same map instance (a copy was made)", func() {
			Expect(result).NotTo(BeIdenticalTo(input))
		})

		It("should not contain events key in the stripped list", func() {
			Expect(resultTodos).NotTo(HaveKey("events"))
		})

		It("should preserve items in the stripped list", func() {
			Expect(resultTodos["items"]).To(Equal([]any{map[string]any{"uid": "abc", "text": "Buy milk"}}))
		})

		It("should preserve tombstones in the stripped list", func() {
			Expect(resultTodos["tombstones"]).To(Equal([]any{map[string]any{"uid": "old", "deleted_at": "2026-01-01"}}))
		})

		It("should preserve sync_token in the stripped list", func() {
			Expect(resultTodos["sync_token"]).To(Equal(42))
		})

		It("should preserve updated_at in the stripped list", func() {
			Expect(resultTodos["updated_at"]).To(Equal("2026-11-12T00:00:00Z"))
		})

		It("should preserve top-level non-wiki keys", func() {
			Expect(result["title"]).To(Equal("Checklist Page"))
		})

		It("should NOT mutate the original map (events still present in input)", func() {
			Expect(inputTodos["events"]).To(Equal(originalEvents))
		})
	})

	Describe("multiple lists — some with events, some without", func() {
		var input map[string]any
		var result map[string]any
		var resultTodos map[string]any
		var resultGroceries map[string]any

		BeforeEach(func() {
			input = map[string]any{
				"wiki": map[string]any{
					"checklists": map[string]any{
						"todos": map[string]any{
							"items":  []any{"item1"},
							"events": []any{map[string]any{"seq": 1}},
						},
						"groceries": map[string]any{
							"items": []any{"milk"},
							// no events key
						},
					},
				},
			}
			result = stripChecklistEvents(input)

			wiki, ok := result["wiki"].(map[string]any)
			Expect(ok).To(BeTrue(), "result[wiki] should be map[string]any")
			checklists, ok := wiki["checklists"].(map[string]any)
			Expect(ok).To(BeTrue(), "wiki[checklists] should be map[string]any")
			resultTodos, ok = checklists["todos"].(map[string]any)
			Expect(ok).To(BeTrue(), "checklists[todos] should be map[string]any")
			resultGroceries, ok = checklists["groceries"].(map[string]any)
			Expect(ok).To(BeTrue(), "checklists[groceries] should be map[string]any")
		})

		It("should remove events from the list that had them", func() {
			Expect(resultTodos).NotTo(HaveKey("events"))
		})

		It("should preserve the list that had no events", func() {
			Expect(resultGroceries["items"]).To(Equal([]any{"milk"}))
		})
	})
})

var _ = Describe("preserveReservedSubtrees", func() {
	When("existing is nil and incoming is non-nil", func() {
		var incoming map[string]any

		BeforeEach(func() {
			incoming = map[string]any{"title": "T"}
			preserveReservedSubtrees(nil, incoming)
		})

		It("should not add an agent key to incoming", func() {
			Expect(incoming).NotTo(HaveKey("agent"))
		})

		It("should leave the other keys untouched", func() {
			Expect(incoming["title"]).To(Equal("T"))
		})
	})

	When("existing is non-nil and incoming is nil", func() {
		var existing map[string]any
		var preserveCall func()

		BeforeEach(func() {
			existing = map[string]any{"agent": map[string]any{"x": 1}}
			preserveCall = func() {
				preserveReservedSubtrees(existing, nil)
			}
		})

		It("should not panic", func() {
			Expect(preserveCall).NotTo(Panic())
		})
	})

	When("both existing and incoming are nil", func() {
		var preserveCall func()

		BeforeEach(func() {
			preserveCall = func() {
				preserveReservedSubtrees(nil, nil)
			}
		})

		It("should not panic", func() {
			Expect(preserveCall).NotTo(Panic())
		})
	})

	When("existing has no agent key and incoming has none", func() {
		var existing, incoming map[string]any

		BeforeEach(func() {
			existing = map[string]any{"title": "Old"}
			incoming = map[string]any{"title": "New"}
			preserveReservedSubtrees(existing, incoming)
		})

		It("should not add an agent key to incoming", func() {
			Expect(incoming).NotTo(HaveKey("agent"))
		})
	})

	When("existing has agent and incoming has no agent", func() {
		var existing, incoming map[string]any

		BeforeEach(func() {
			existing = map[string]any{
				"title": "Old",
				"agent": map[string]any{"schedules": []any{"s1"}},
			}
			incoming = map[string]any{"title": "New"}
			preserveReservedSubtrees(existing, incoming)
		})

		It("should copy the agent subtree into incoming", func() {
			Expect(incoming).To(HaveKey("agent"))
			agent, ok := incoming["agent"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(agent["schedules"]).To(Equal([]any{"s1"}))
		})

		It("should not modify other incoming keys", func() {
			Expect(incoming["title"]).To(Equal("New"))
		})
	})
})
