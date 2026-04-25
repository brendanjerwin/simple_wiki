// Internal tests for package v1 that need access to unexported types and methods.
// These complement the external tests in server_test.go (package v1_test).
package v1

import (
	"github.com/brendanjerwin/simple_wiki/pageimport"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

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
