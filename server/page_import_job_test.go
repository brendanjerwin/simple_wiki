//revive:disable:dot-imports
package server

import (
	"errors"
	"os"

	"github.com/brendanjerwin/simple_wiki/pageimport"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SinglePageImportJob", func() {
	var (
		mockDeps          *mockPageReaderMutator
		logger            lumber.Logger
		resultAccumulator *PageImportResultAccumulator
	)

	BeforeEach(func() {
		mockDeps = newMockPageReaderMutator()
		logger = lumber.NewConsoleLogger(lumber.WARN)
		resultAccumulator = NewPageImportResultAccumulator()
	})

	Describe("NewSinglePageImportJob", func() {
		When("pageReaderMutator is nil", func() {
			var err error

			BeforeEach(func() {
				_, err = NewSinglePageImportJob(pageimport.ParsedRecord{}, nil, logger, resultAccumulator)
			})

			It("should return an error", func() {
				Expect(err).To(MatchError("pageReaderMutator is required"))
			})
		})

		When("logger is nil", func() {
			var err error

			BeforeEach(func() {
				_, err = NewSinglePageImportJob(pageimport.ParsedRecord{}, mockDeps, nil, resultAccumulator)
			})

			It("should return an error", func() {
				Expect(err).To(MatchError("logger is required"))
			})
		})

		When("resultAccumulator is nil", func() {
			var err error

			BeforeEach(func() {
				_, err = NewSinglePageImportJob(pageimport.ParsedRecord{}, mockDeps, logger, nil)
			})

			It("should return an error", func() {
				Expect(err).To(MatchError("resultAccumulator is required"))
			})
		})

		When("all dependencies are provided", func() {
			var (
				job *SinglePageImportJob
				err error
			)

			BeforeEach(func() {
				job, err = NewSinglePageImportJob(pageimport.ParsedRecord{}, mockDeps, logger, resultAccumulator)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a valid job", func() {
				Expect(job).NotTo(BeNil())
			})
		})
	})

	Describe("GetName", func() {
		var job *SinglePageImportJob

		BeforeEach(func() {
			job, _ = NewSinglePageImportJob(pageimport.ParsedRecord{}, mockDeps, logger, resultAccumulator)
		})

		When("called", func() {
			var name string

			BeforeEach(func() {
				name = job.GetName()
			})

			It("should return the correct job name", func() {
				Expect(name).To(Equal(PageImportJobName))
			})
		})
	})

	Describe("Execute", func() {
		When("processing a record for a new page", func() {
			var err error

			BeforeEach(func() {
				record := pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "new_page",
					Frontmatter: map[string]any{
						"title":       "New Page Title",
						"description": "A new page",
					},
				}
				job, _ := NewSinglePageImportJob(record, mockDeps, logger, resultAccumulator)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create the page", func() {
				Expect(mockDeps.hasPage("new_page")).To(BeTrue())
			})

			It("should set the identifier in frontmatter", func() {
				fm := mockDeps.getFrontmatter("new_page")
				Expect(fm["identifier"]).To(Equal("new_page"))
			})

			It("should set the title in frontmatter", func() {
				fm := mockDeps.getFrontmatter("new_page")
				Expect(fm["title"]).To(Equal("New Page Title"))
			})

			It("should set the description in frontmatter", func() {
				fm := mockDeps.getFrontmatter("new_page")
				Expect(fm["description"]).To(Equal("A new page"))
			})

			It("should track the page as created in the accumulator", func() {
				result := resultAccumulator.GetResult()
				Expect(result.CreatedPages).To(ContainElement("new_page"))
			})
		})

		When("processing a record for an existing page", func() {
			var err error

			BeforeEach(func() {
				mockDeps.setPage("existing_page", map[string]any{
					"identifier":   "existing_page",
					"title":        "Original Title",
					"existing_key": "existing_value",
				}, "# Existing Content")

				record := pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "existing_page",
					Frontmatter: map[string]any{
						"title":   "Updated Title",
						"new_key": "new_value",
					},
				}
				job, _ := NewSinglePageImportJob(record, mockDeps, logger, resultAccumulator)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should update the title", func() {
				fm := mockDeps.getFrontmatter("existing_page")
				Expect(fm["title"]).To(Equal("Updated Title"))
			})

			It("should preserve existing keys not in the import", func() {
				fm := mockDeps.getFrontmatter("existing_page")
				Expect(fm["existing_key"]).To(Equal("existing_value"))
			})

			It("should add new keys from the import", func() {
				fm := mockDeps.getFrontmatter("existing_page")
				Expect(fm["new_key"]).To(Equal("new_value"))
			})

			It("should track the page as updated in the accumulator", func() {
				result := resultAccumulator.GetResult()
				Expect(result.UpdatedPages).To(ContainElement("existing_page"))
			})
		})

		When("processing a record with inv_item template for a new page", func() {
			var err error

			BeforeEach(func() {
				record := pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "new_item",
					Template:   "inv_item",
					Frontmatter: map[string]any{
						"title": "New Inventory Item",
					},
				}
				job, _ := NewSinglePageImportJob(record, mockDeps, logger, resultAccumulator)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create the inventory structure in frontmatter", func() {
				fm := mockDeps.getFrontmatter("new_item")
				_, hasInventory := fm["inventory"]
				Expect(hasInventory).To(BeTrue())
			})

			It("should write the inventory markdown template", func() {
				md := mockDeps.getMarkdown("new_item")
				Expect(md).NotTo(BeEmpty())
			})
		})

		When("processing a record with inv_item template for an existing page", func() {
			var err error

			BeforeEach(func() {
				mockDeps.setPage("existing_item", map[string]any{
					"identifier": "existing_item",
					"title":      "Existing Item",
				}, "# Existing Content")

				record := pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "existing_item",
					Template:   "inv_item",
					Frontmatter: map[string]any{
						"title": "Updated Item",
					},
				}
				job, _ := NewSinglePageImportJob(record, mockDeps, logger, resultAccumulator)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should ensure inventory structure exists", func() {
				fm := mockDeps.getFrontmatter("existing_item")
				_, hasInventory := fm["inventory"]
				Expect(hasInventory).To(BeTrue())
			})

			It("should not overwrite markdown for existing pages", func() {
				md := mockDeps.getMarkdown("existing_item")
				Expect(md).To(Equal("# Existing Content"))
			})
		})

		When("processing a record with FieldsToDelete", func() {
			var err error

			BeforeEach(func() {
				mockDeps.setPage("page_with_fields", map[string]any{
					"identifier":      "page_with_fields",
					"title":           "Page With Fields",
					"field_to_keep":   "kept",
					"field_to_delete": "to be deleted",
					"nested": map[string]any{
						"keep_this":   "kept",
						"delete_this": "to be deleted",
					},
				}, "# Content")

				record := pageimport.ParsedRecord{
					RowNumber:      1,
					Identifier:     "page_with_fields",
					FieldsToDelete: []string{"field_to_delete", "nested.delete_this"},
				}
				job, _ := NewSinglePageImportJob(record, mockDeps, logger, resultAccumulator)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should delete the top-level field", func() {
				fm := mockDeps.getFrontmatter("page_with_fields")
				_, hasField := fm["field_to_delete"]
				Expect(hasField).To(BeFalse())
			})

			It("should preserve fields not marked for deletion", func() {
				fm := mockDeps.getFrontmatter("page_with_fields")
				Expect(fm["field_to_keep"]).To(Equal("kept"))
			})

			It("should delete the nested field", func() {
				fm := mockDeps.getFrontmatter("page_with_fields")
				nested, ok := fm["nested"].(map[string]any)
				Expect(ok).To(BeTrue())
				_, hasField := nested["delete_this"]
				Expect(hasField).To(BeFalse())
			})

			It("should preserve nested fields not marked for deletion", func() {
				fm := mockDeps.getFrontmatter("page_with_fields")
				nested, ok := fm["nested"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(nested["keep_this"]).To(Equal("kept"))
			})
		})

		When("processing a record with ArrayOps EnsureExists", func() {
			var err error

			BeforeEach(func() {
				mockDeps.setPage("page_with_array", map[string]any{
					"identifier": "page_with_array",
					"tags":       []any{"existing_tag"},
				}, "# Content")

				record := pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "page_with_array",
					ArrayOps: []pageimport.ArrayOperation{
						{
							FieldPath: "tags",
							Operation: pageimport.EnsureExists,
							Value:     "new_tag",
						},
						{
							FieldPath: "tags",
							Operation: pageimport.EnsureExists,
							Value:     "existing_tag", // Already exists
						},
					},
				}
				job, _ := NewSinglePageImportJob(record, mockDeps, logger, resultAccumulator)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should add the new value to the array", func() {
				fm := mockDeps.getFrontmatter("page_with_array")
				tags, ok := fm["tags"].([]any)
				Expect(ok).To(BeTrue())
				Expect(tags).To(ContainElement("new_tag"))
			})

			It("should not duplicate existing values", func() {
				fm := mockDeps.getFrontmatter("page_with_array")
				tags, ok := fm["tags"].([]any)
				Expect(ok).To(BeTrue())
				count := 0
				for _, t := range tags {
					if t == "existing_tag" {
						count++
					}
				}
				Expect(count).To(Equal(1))
			})
		})

		When("processing a record with ArrayOps DeleteValue", func() {
			var err error

			BeforeEach(func() {
				mockDeps.setPage("page_with_array", map[string]any{
					"identifier": "page_with_array",
					"tags":       []any{"keep_me", "delete_me", "also_keep"},
				}, "# Content")

				record := pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "page_with_array",
					ArrayOps: []pageimport.ArrayOperation{
						{
							FieldPath: "tags",
							Operation: pageimport.DeleteValue,
							Value:     "delete_me",
						},
					},
				}
				job, _ := NewSinglePageImportJob(record, mockDeps, logger, resultAccumulator)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should remove the specified value from the array", func() {
				fm := mockDeps.getFrontmatter("page_with_array")
				tags, ok := fm["tags"].([]any)
				Expect(ok).To(BeTrue())
				Expect(tags).NotTo(ContainElement("delete_me"))
			})

			It("should preserve other values in the array", func() {
				fm := mockDeps.getFrontmatter("page_with_array")
				tags, ok := fm["tags"].([]any)
				Expect(ok).To(BeTrue())
				Expect(tags).To(ContainElement("keep_me"))
				Expect(tags).To(ContainElement("also_keep"))
			})
		})

		When("processing a record with ArrayOps on []string type", func() {
			var err error

			BeforeEach(func() {
				mockDeps.setPage("page_with_string_array", map[string]any{
					"identifier": "page_with_string_array",
					"tags":       []string{"tag1", "tag2"},
				}, "# Content")

				record := pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "page_with_string_array",
					ArrayOps: []pageimport.ArrayOperation{
						{
							FieldPath: "tags",
							Operation: pageimport.EnsureExists,
							Value:     "tag3",
						},
					},
				}
				job, _ := NewSinglePageImportJob(record, mockDeps, logger, resultAccumulator)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should convert and add to the array", func() {
				fm := mockDeps.getFrontmatter("page_with_string_array")
				tags, ok := fm["tags"].([]any)
				Expect(ok).To(BeTrue())
				Expect(tags).To(ContainElement("tag3"))
			})
		})

		When("processing a record with ArrayOps on nested path", func() {
			var err error

			BeforeEach(func() {
				record := pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "new_page_with_nested_array",
					ArrayOps: []pageimport.ArrayOperation{
						{
							FieldPath: "inventory.items",
							Operation: pageimport.EnsureExists,
							Value:     "item1",
						},
					},
				}
				job, _ := NewSinglePageImportJob(record, mockDeps, logger, resultAccumulator)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create nested structure for array", func() {
				fm := mockDeps.getFrontmatter("new_page_with_nested_array")
				inventory, ok := fm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				items, ok := inventory["items"].([]any)
				Expect(ok).To(BeTrue())
				Expect(items).To(ContainElement("item1"))
			})
		})

		When("processing a record with validation errors", func() {
			var err error

			BeforeEach(func() {
				record := pageimport.ParsedRecord{
					RowNumber:        1,
					Identifier:       "valid_page",
					ValidationErrors: []string{"some validation error"},
				}
				job, _ := NewSinglePageImportJob(record, mockDeps, logger, resultAccumulator)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should skip the invalid record", func() {
				Expect(mockDeps.hasPage("valid_page")).To(BeFalse())
			})

			It("should track the failure in the accumulator", func() {
				result := resultAccumulator.GetResult()
				Expect(result.FailedRecords).To(HaveLen(1))
				Expect(result.FailedRecords[0].Identifier).To(Equal("valid_page"))
			})
		})

		When("deep merging nested frontmatter", func() {
			var err error

			BeforeEach(func() {
				mockDeps.setPage("page_with_nested", map[string]any{
					"identifier": "page_with_nested",
					"inventory": map[string]any{
						"container":    "original_container",
						"existing_key": "existing_value",
					},
				}, "# Content")

				record := pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "page_with_nested",
					Frontmatter: map[string]any{
						"inventory": map[string]any{
							"container": "new_container",
							"new_key":   "new_value",
						},
					},
				}
				job, _ := NewSinglePageImportJob(record, mockDeps, logger, resultAccumulator)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should update nested scalar values", func() {
				fm := mockDeps.getFrontmatter("page_with_nested")
				inventory, ok := fm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(inventory["container"]).To(Equal("new_container"))
			})

			It("should preserve existing nested keys not in the import", func() {
				fm := mockDeps.getFrontmatter("page_with_nested")
				inventory, ok := fm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(inventory["existing_key"]).To(Equal("existing_value"))
			})

			It("should add new nested keys from the import", func() {
				fm := mockDeps.getFrontmatter("page_with_nested")
				inventory, ok := fm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(inventory["new_key"]).To(Equal("new_value"))
			})
		})

		When("array operation targets a non-map intermediate value", func() {
			var err error

			BeforeEach(func() {
				mockDeps.setPage("page", map[string]any{
					"identifier": "page",
					"inventory":  "not a map",
				}, "")

				record := pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "page",
					ArrayOps: []pageimport.ArrayOperation{
						{
							FieldPath: "inventory.items",
							Operation: pageimport.EnsureExists,
							Value:     "item1",
						},
					},
				}
				job, _ := NewSinglePageImportJob(record, mockDeps, logger, resultAccumulator)
				err = job.Execute()
			})

			It("should not return an error from Execute", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should track the failure in the accumulator", func() {
				result := resultAccumulator.GetResult()
				Expect(result.FailedRecords).To(HaveLen(1))
				Expect(result.FailedRecords[0].Error).To(ContainSubstring("cannot navigate through non-map value"))
			})
		})

		When("array operation targets a non-array field", func() {
			var err error

			BeforeEach(func() {
				mockDeps.setPage("page", map[string]any{
					"identifier": "page",
					"tags":       "not an array",
				}, "")

				record := pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "page",
					ArrayOps: []pageimport.ArrayOperation{
						{
							FieldPath: "tags",
							Operation: pageimport.EnsureExists,
							Value:     "new_tag",
						},
					},
				}
				job, _ := NewSinglePageImportJob(record, mockDeps, logger, resultAccumulator)
				err = job.Execute()
			})

			It("should not return an error from Execute", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should track the failure in the accumulator", func() {
				result := resultAccumulator.GetResult()
				Expect(result.FailedRecords).To(HaveLen(1))
				Expect(result.FailedRecords[0].Error).To(ContainSubstring("is not an array"))
			})
		})

		When("WriteFrontMatter fails for a record", func() {
			var err error

			BeforeEach(func() {
				mockDeps.writeFrontMatterErr = errors.New("permission denied")

				record := pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "new_page",
					Frontmatter: map[string]any{
						"title": "Test",
					},
				}
				job, _ := NewSinglePageImportJob(record, mockDeps, logger, resultAccumulator)
				err = job.Execute()
			})

			It("should not return an error from Execute", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should include the error in failed records", func() {
				result := resultAccumulator.GetResult()
				Expect(result.FailedRecords).To(HaveLen(1))
				Expect(result.FailedRecords[0].Error).To(ContainSubstring("failed to write frontmatter"))
			})
		})

		When("WriteMarkdown fails for a new inv_item page", func() {
			var err error

			BeforeEach(func() {
				mockDeps.writeMarkdownErr = os.ErrPermission

				record := pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "new_item",
					Template:   "inv_item",
				}
				job, _ := NewSinglePageImportJob(record, mockDeps, logger, resultAccumulator)
				err = job.Execute()
			})

			It("should not return an error from Execute", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should include the error in failed records", func() {
				result := resultAccumulator.GetResult()
				Expect(result.FailedRecords).To(HaveLen(1))
				Expect(result.FailedRecords[0].Error).To(ContainSubstring("failed to write markdown"))
			})
		})

		When("deleting a non-existent path", func() {
			var err error

			BeforeEach(func() {
				mockDeps.setPage("page", map[string]any{
					"identifier": "page",
					"title":      "Test",
				}, "")

				record := pageimport.ParsedRecord{
					RowNumber:      1,
					Identifier:     "page",
					FieldsToDelete: []string{"nonexistent.nested.field"},
				}
				job, _ := NewSinglePageImportJob(record, mockDeps, logger, resultAccumulator)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not affect existing fields", func() {
				fm := mockDeps.getFrontmatter("page")
				Expect(fm["title"]).To(Equal("Test"))
			})
		})
	})

	Describe("GetRecord", func() {
		When("called", func() {
			var (
				job    *SinglePageImportJob
				record pageimport.ParsedRecord
			)

			BeforeEach(func() {
				record = pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "test_page",
				}
				job, _ = NewSinglePageImportJob(record, mockDeps, logger, resultAccumulator)
			})

			It("should return the record with correct fields", func() {
				returned := job.GetRecord()
				Expect(returned.RowNumber).To(Equal(1))
				Expect(returned.Identifier).To(Equal("test_page"))
			})
		})
	})
})

var _ = Describe("PageImportResultAccumulator", func() {
	var accumulator *PageImportResultAccumulator

	BeforeEach(func() {
		accumulator = NewPageImportResultAccumulator()
	})

	Describe("NewPageImportResultAccumulator", func() {
		It("should create an accumulator with empty slices", func() {
			result := accumulator.GetResult()
			Expect(result.CreatedPages).To(BeEmpty())
			Expect(result.UpdatedPages).To(BeEmpty())
			Expect(result.FailedRecords).To(BeEmpty())
		})
	})

	Describe("RecordCreated", func() {
		When("recording created pages", func() {
			BeforeEach(func() {
				accumulator.RecordCreated("page1")
				accumulator.RecordCreated("page2")
			})

			It("should track created pages", func() {
				result := accumulator.GetResult()
				Expect(result.CreatedPages).To(HaveLen(2))
				Expect(result.CreatedPages).To(ContainElement("page1"))
				Expect(result.CreatedPages).To(ContainElement("page2"))
			})
		})
	})

	Describe("RecordUpdated", func() {
		When("recording updated pages", func() {
			BeforeEach(func() {
				accumulator.RecordUpdated("page1")
				accumulator.RecordUpdated("page2")
			})

			It("should track updated pages", func() {
				result := accumulator.GetResult()
				Expect(result.UpdatedPages).To(HaveLen(2))
				Expect(result.UpdatedPages).To(ContainElement("page1"))
				Expect(result.UpdatedPages).To(ContainElement("page2"))
			})
		})
	})

	Describe("RecordFailed", func() {
		When("recording failed records", func() {
			BeforeEach(func() {
				accumulator.RecordFailed(FailedPageImport{
					RowNumber:  1,
					Identifier: "failed1",
					Error:      "error1",
				})
				accumulator.RecordFailed(FailedPageImport{
					RowNumber:  2,
					Identifier: "failed2",
					Error:      "error2",
				})
			})

			It("should track failed records", func() {
				result := accumulator.GetResult()
				Expect(result.FailedRecords).To(HaveLen(2))
				Expect(result.FailedRecords[0].Identifier).To(Equal("failed1"))
				Expect(result.FailedRecords[1].Identifier).To(Equal("failed2"))
			})
		})
	})

	Describe("GetResult", func() {
		When("getting result with mixed data", func() {
			var result PageImportResult

			BeforeEach(func() {
				accumulator.RecordCreated("new_page")
				accumulator.RecordUpdated("existing_page")
				accumulator.RecordFailed(FailedPageImport{
					RowNumber:  3,
					Identifier: "failed_page",
					Error:      "error",
				})
				result = accumulator.GetResult()
			})

			It("should return a copy with all data", func() {
				Expect(result.CreatedPages).To(ContainElement("new_page"))
				Expect(result.UpdatedPages).To(ContainElement("existing_page"))
				Expect(result.FailedRecords).To(HaveLen(1))
				Expect(result.FailedRecords[0].Identifier).To(Equal("failed_page"))
			})
		})
	})
})

var _ = Describe("PageImportReportJob", func() {
	var (
		mockDeps          *mockPageReaderMutator
		logger            lumber.Logger
		resultAccumulator *PageImportResultAccumulator
	)

	BeforeEach(func() {
		mockDeps = newMockPageReaderMutator()
		logger = lumber.NewConsoleLogger(lumber.WARN)
		resultAccumulator = NewPageImportResultAccumulator()
	})

	Describe("NewPageImportReportJob", func() {
		When("pageReaderMutator is nil", func() {
			var err error

			BeforeEach(func() {
				_, err = NewPageImportReportJob(nil, logger, resultAccumulator)
			})

			It("should return an error", func() {
				Expect(err).To(MatchError("pageReaderMutator is required"))
			})
		})

		When("logger is nil", func() {
			var err error

			BeforeEach(func() {
				_, err = NewPageImportReportJob(mockDeps, nil, resultAccumulator)
			})

			It("should return an error", func() {
				Expect(err).To(MatchError("logger is required"))
			})
		})

		When("resultAccumulator is nil", func() {
			var err error

			BeforeEach(func() {
				_, err = NewPageImportReportJob(mockDeps, logger, nil)
			})

			It("should return an error", func() {
				Expect(err).To(MatchError("resultAccumulator is required"))
			})
		})

		When("all dependencies are provided", func() {
			var (
				job *PageImportReportJob
				err error
			)

			BeforeEach(func() {
				job, err = NewPageImportReportJob(mockDeps, logger, resultAccumulator)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a valid job", func() {
				Expect(job).NotTo(BeNil())
			})
		})
	})

	Describe("GetName", func() {
		When("called", func() {
			var name string

			BeforeEach(func() {
				job, _ := NewPageImportReportJob(mockDeps, logger, resultAccumulator)
				name = job.GetName()
			})

			It("should return PageImportJob", func() {
				Expect(name).To(Equal(PageImportJobName))
			})
		})
	})

	Describe("Execute", func() {
		When("job completes successfully with created pages", func() {
			var err error

			BeforeEach(func() {
				resultAccumulator.RecordCreated("new_page")
				job, _ := NewPageImportReportJob(mockDeps, logger, resultAccumulator)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create the report page", func() {
				Expect(mockDeps.hasPage(PageImportReportPage)).To(BeTrue())
			})

			It("should set the report page title", func() {
				fm := mockDeps.getFrontmatter(PageImportReportPage)
				Expect(fm["title"]).To(Equal("Page Import Report"))
			})

			It("should include created pages in the report", func() {
				md := mockDeps.getMarkdown(PageImportReportPage)
				Expect(md).To(ContainSubstring("[[new_page]]"))
			})

			It("should include summary counts", func() {
				md := mockDeps.getMarkdown(PageImportReportPage)
				Expect(md).To(ContainSubstring("Pages created:** 1"))
			})
		})

		When("job completes with updated pages", func() {
			var err error

			BeforeEach(func() {
				resultAccumulator.RecordUpdated("existing_page")
				job, _ := NewPageImportReportJob(mockDeps, logger, resultAccumulator)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should include updated pages in the report", func() {
				md := mockDeps.getMarkdown(PageImportReportPage)
				Expect(md).To(ContainSubstring("[[existing_page]]"))
				Expect(md).To(ContainSubstring("Pages Updated"))
			})
		})

		When("job completes with failed records", func() {
			var err error

			BeforeEach(func() {
				resultAccumulator.RecordFailed(FailedPageImport{
					RowNumber:  1,
					Identifier: "invalid_page",
					Error:      "validation error 1; validation error 2",
				})
				job, _ := NewPageImportReportJob(mockDeps, logger, resultAccumulator)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should include failed records in the report", func() {
				md := mockDeps.getMarkdown(PageImportReportPage)
				Expect(md).To(ContainSubstring("Failed Records"))
				Expect(md).To(ContainSubstring("invalid_page"))
			})

			It("should include the validation errors", func() {
				md := mockDeps.getMarkdown(PageImportReportPage)
				Expect(md).To(ContainSubstring("validation error 1"))
			})
		})

		When("job has no failures", func() {
			var err error

			BeforeEach(func() {
				resultAccumulator.RecordCreated("success_page")
				job, _ := NewPageImportReportJob(mockDeps, logger, resultAccumulator)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should indicate no failures in report", func() {
				md := mockDeps.getMarkdown(PageImportReportPage)
				Expect(md).To(ContainSubstring("No failures"))
			})
		})

		When("failed record has no identifier", func() {
			var err error

			BeforeEach(func() {
				resultAccumulator.RecordFailed(FailedPageImport{
					RowNumber:  1,
					Identifier: "",
					Error:      "identifier is required",
				})
				job, _ := NewPageImportReportJob(mockDeps, logger, resultAccumulator)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should show placeholder for missing identifier", func() {
				md := mockDeps.getMarkdown(PageImportReportPage)
				Expect(md).To(ContainSubstring("(no identifier)"))
			})
		})

		When("report generation fails due to WriteFrontMatter error", func() {
			var err error

			BeforeEach(func() {
				mockDeps.writeFrontMatterErr = errors.New("write error")
				job, _ := NewPageImportReportJob(mockDeps, logger, resultAccumulator)
				err = job.Execute()
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to write import report frontmatter"))
			})
		})

		When("no records are provided", func() {
			var err error

			BeforeEach(func() {
				// Empty accumulator
				job, _ := NewPageImportReportJob(mockDeps, logger, resultAccumulator)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create a report with zero counts", func() {
				md := mockDeps.getMarkdown(PageImportReportPage)
				Expect(md).To(ContainSubstring("Pages created:** 0"))
				Expect(md).To(ContainSubstring("Pages updated:** 0"))
				Expect(md).To(ContainSubstring("Failed records:** 0"))
			})
		})
	})
})
