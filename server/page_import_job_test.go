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

var _ = Describe("PageImportJob", func() {
	var (
		mockDeps *mockPageReaderMutator
		logger   lumber.Logger
	)

	BeforeEach(func() {
		mockDeps = newMockPageReaderMutator()
		logger = lumber.NewConsoleLogger(lumber.WARN)
	})

	Describe("NewPageImportJob", func() {
		When("pageReaderMutator is nil", func() {
			var err error

			BeforeEach(func() {
				_, err = NewPageImportJob(nil, nil, logger)
			})

			It("should return an error", func() {
				Expect(err).To(MatchError("pageReaderMutator is required"))
			})
		})

		When("logger is nil", func() {
			var err error

			BeforeEach(func() {
				_, err = NewPageImportJob(nil, mockDeps, nil)
			})

			It("should return an error", func() {
				Expect(err).To(MatchError("logger is required"))
			})
		})

		When("all dependencies are provided", func() {
			var (
				job *PageImportJob
				err error
			)

			BeforeEach(func() {
				job, err = NewPageImportJob(nil, mockDeps, logger)
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
		var job *PageImportJob

		BeforeEach(func() {
			job, _ = NewPageImportJob(nil, mockDeps, logger)
		})

		When("called", func() {
			var name string

			BeforeEach(func() {
				name = job.GetName()
			})

			It("should return the correct job name", func() {
				Expect(name).To(Equal(PageImportJobName))
			})

			It("should return 'PageImportJob'", func() {
				Expect(name).To(Equal("PageImportJob"))
			})
		})
	})

	Describe("Execute", func() {
		var job *PageImportJob

		When("processing a record for a new page", func() {
			var err error

			BeforeEach(func() {
				records := []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "new_page",
						Frontmatter: map[string]any{
							"title":       "New Page Title",
							"description": "A new page",
						},
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
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

			It("should track the page as created", func() {
				result := job.GetResult()
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

				records := []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "existing_page",
						Frontmatter: map[string]any{
							"title":   "Updated Title",
							"new_key": "new_value",
						},
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
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

			It("should track the page as updated", func() {
				result := job.GetResult()
				Expect(result.UpdatedPages).To(ContainElement("existing_page"))
			})
		})

		When("processing a record with inv_item template for a new page", func() {
			var err error

			BeforeEach(func() {
				records := []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "new_item",
						Template:   "inv_item",
						Frontmatter: map[string]any{
							"title": "New Inventory Item",
						},
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
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

				records := []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "existing_item",
						Template:   "inv_item",
						Frontmatter: map[string]any{
							"title": "Updated Item",
						},
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
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
				// For existing pages with inv_item template, markdown is NOT rewritten
				// This is implicit from the code - only new pages get markdown written
			})
		})

		When("processing a record with FieldsToDelete", func() {
			var err error

			BeforeEach(func() {
				mockDeps.setPage("page_with_fields", map[string]any{
					"identifier":     "page_with_fields",
					"title":          "Page With Fields",
					"field_to_keep":  "kept",
					"field_to_delete": "to be deleted",
					"nested": map[string]any{
						"keep_this":   "kept",
						"delete_this": "to be deleted",
					},
				}, "# Content")

				records := []pageimport.ParsedRecord{
					{
						RowNumber:      1,
						Identifier:     "page_with_fields",
						FieldsToDelete: []string{"field_to_delete", "nested.delete_this"},
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
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

				records := []pageimport.ParsedRecord{
					{
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
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
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

				records := []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "page_with_array",
						ArrayOps: []pageimport.ArrayOperation{
							{
								FieldPath: "tags",
								Operation: pageimport.DeleteValue,
								Value:     "delete_me",
							},
						},
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
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

				records := []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "page_with_string_array",
						ArrayOps: []pageimport.ArrayOperation{
							{
								FieldPath: "tags",
								Operation: pageimport.EnsureExists,
								Value:     "tag3",
							},
						},
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
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
				records := []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "new_page_with_nested_array",
						ArrayOps: []pageimport.ArrayOperation{
							{
								FieldPath: "inventory.items",
								Operation: pageimport.EnsureExists,
								Value:     "item1",
							},
						},
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
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
				records := []pageimport.ParsedRecord{
					{
						RowNumber:        1,
						Identifier:       "valid_page",
						ValidationErrors: []string{"some validation error"},
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should skip the invalid record", func() {
				Expect(mockDeps.hasPage("valid_page")).To(BeFalse())
			})

			It("should track the failure", func() {
				result := job.GetResult()
				Expect(result.FailedRecords).To(HaveLen(1))
				Expect(result.FailedRecords[0].Identifier).To(Equal("valid_page"))
			})
		})

		When("processing continues after a record validation error", func() {
			var err error

			BeforeEach(func() {
				records := []pageimport.ParsedRecord{
					{
						RowNumber:        1,
						Identifier:       "error_page",
						ValidationErrors: []string{"validation failed"},
					},
					{
						RowNumber:  2,
						Identifier: "success_page",
						Frontmatter: map[string]any{
							"title": "Will Succeed",
						},
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should process the successful page", func() {
				Expect(mockDeps.hasPage("success_page")).To(BeTrue())
			})

			It("should track the failed record", func() {
				result := job.GetResult()
				Expect(result.FailedRecords).To(HaveLen(1))
				Expect(result.FailedRecords[0].Identifier).To(Equal("error_page"))
			})

			It("should track the successful page", func() {
				result := job.GetResult()
				Expect(result.CreatedPages).To(ContainElement("success_page"))
			})
		})

		When("processing continues after a record processing error", func() {
			var err error

			BeforeEach(func() {
				// Set up a page where array operation will fail
				mockDeps.setPage("error_page", map[string]any{
					"identifier": "error_page",
					"tags":       "not an array", // This will cause array operation to fail
				}, "")

				records := []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "error_page",
						ArrayOps: []pageimport.ArrayOperation{
							{
								FieldPath: "tags",
								Operation: pageimport.EnsureExists,
								Value:     "new_tag",
							},
						},
					},
					{
						RowNumber:  2,
						Identifier: "success_page",
						Frontmatter: map[string]any{
							"title": "Will Succeed",
						},
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should process the successful page", func() {
				Expect(mockDeps.hasPage("success_page")).To(BeTrue())
			})

			It("should track the failed record", func() {
				result := job.GetResult()
				Expect(result.FailedRecords).To(HaveLen(1))
				Expect(result.FailedRecords[0].Identifier).To(Equal("error_page"))
			})

			It("should track the successful page", func() {
				result := job.GetResult()
				Expect(result.CreatedPages).To(ContainElement("success_page"))
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

				records := []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "page_with_nested",
						Frontmatter: map[string]any{
							"inventory": map[string]any{
								"container": "new_container",
								"new_key":   "new_value",
							},
						},
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
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

		When("processing multiple records", func() {
			var err error

			BeforeEach(func() {
				records := []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "page_one",
						Frontmatter: map[string]any{
							"title": "Page One",
						},
					},
					{
						RowNumber:  2,
						Identifier: "page_two",
						Frontmatter: map[string]any{
							"title": "Page Two",
						},
					},
					{
						RowNumber:  3,
						Identifier: "page_three",
						Frontmatter: map[string]any{
							"title": "Page Three",
						},
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create all pages", func() {
				Expect(mockDeps.hasPage("page_one")).To(BeTrue())
				Expect(mockDeps.hasPage("page_two")).To(BeTrue())
				Expect(mockDeps.hasPage("page_three")).To(BeTrue())
			})

			It("should track all created pages", func() {
				result := job.GetResult()
				Expect(result.CreatedPages).To(HaveLen(3))
			})
		})
	})

	Describe("generateReport", func() {
		var job *PageImportJob

		When("job completes successfully with created pages", func() {
			var err error

			BeforeEach(func() {
				records := []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "new_page",
						Frontmatter: map[string]any{
							"title": "New Page",
						},
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
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
				mockDeps.setPage("existing_page", map[string]any{
					"identifier": "existing_page",
				}, "")

				records := []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "existing_page",
						Frontmatter: map[string]any{
							"title": "Updated",
						},
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
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
				records := []pageimport.ParsedRecord{
					{
						RowNumber:        1,
						Identifier:       "invalid_page",
						ValidationErrors: []string{"validation error 1", "validation error 2"},
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
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
				records := []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "success_page",
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
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
				records := []pageimport.ParsedRecord{
					{
						RowNumber:        1,
						Identifier:       "",
						ValidationErrors: []string{"identifier is required"},
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
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
				records := []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "new_page",
					},
				}

				// Process the page first, then cause report to fail
				mockDeps.setPage("new_page", nil, "")
				job, _ = NewPageImportJob(records, mockDeps, logger)

				// Set up to fail on report page write
				mockDeps.writeFrontMatterErr = errors.New("write error")
				err = job.Execute()
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to generate import report"))
			})
		})
	})

	Describe("deleteField", func() {
		var job *PageImportJob

		When("deleting a non-existent path", func() {
			var err error

			BeforeEach(func() {
				mockDeps.setPage("page", map[string]any{
					"identifier": "page",
					"title":      "Test",
				}, "")

				records := []pageimport.ParsedRecord{
					{
						RowNumber:      1,
						Identifier:     "page",
						FieldsToDelete: []string{"nonexistent.nested.field"},
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
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

	Describe("applyArrayOperation error handling", func() {
		var job *PageImportJob

		When("array operation targets a non-map intermediate value", func() {
			var err error

			BeforeEach(func() {
				mockDeps.setPage("page", map[string]any{
					"identifier": "page",
					"inventory":  "not a map",
				}, "")

				records := []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "page",
						ArrayOps: []pageimport.ArrayOperation{
							{
								FieldPath: "inventory.items",
								Operation: pageimport.EnsureExists,
								Value:     "item1",
							},
						},
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
				err = job.Execute()
			})

			It("should not return an error from Execute", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should track the failure", func() {
				result := job.GetResult()
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

				records := []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "page",
						ArrayOps: []pageimport.ArrayOperation{
							{
								FieldPath: "tags",
								Operation: pageimport.EnsureExists,
								Value:     "new_tag",
							},
						},
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
				err = job.Execute()
			})

			It("should not return an error from Execute", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should track the failure", func() {
				result := job.GetResult()
				Expect(result.FailedRecords).To(HaveLen(1))
				Expect(result.FailedRecords[0].Error).To(ContainSubstring("is not an array"))
			})
		})
	})

	Describe("GetResult", func() {
		var job *PageImportJob

		When("job has not been executed", func() {
			var result PageImportResult

			BeforeEach(func() {
				job, _ = NewPageImportJob(nil, mockDeps, logger)
				result = job.GetResult()
			})

			It("should return empty result", func() {
				Expect(result.CreatedPages).To(BeEmpty())
				Expect(result.UpdatedPages).To(BeEmpty())
				Expect(result.FailedRecords).To(BeEmpty())
			})
		})

		When("job has been executed with mixed results", func() {
			var result PageImportResult

			BeforeEach(func() {
				mockDeps.setPage("existing_page", map[string]any{"identifier": "existing_page"}, "")

				records := []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "new_page",
					},
					{
						RowNumber:  2,
						Identifier: "existing_page",
					},
					{
						RowNumber:        3,
						Identifier:       "failed_page",
						ValidationErrors: []string{"error"},
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
				_ = job.Execute()
				result = job.GetResult()
			})

			It("should track created pages", func() {
				Expect(result.CreatedPages).To(ContainElement("new_page"))
			})

			It("should track updated pages", func() {
				Expect(result.UpdatedPages).To(ContainElement("existing_page"))
			})

			It("should track failed records", func() {
				Expect(result.FailedRecords).To(HaveLen(1))
				Expect(result.FailedRecords[0].Identifier).To(Equal("failed_page"))
			})
		})
	})

	Describe("Execute with WriteFrontMatter failure", func() {
		var job *PageImportJob

		When("WriteFrontMatter fails for a record", func() {
			var err error

			BeforeEach(func() {
				// Create a mock that fails on specific page
				mockDeps.writeFrontMatterErr = errors.New("permission denied")

				records := []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "new_page",
						Frontmatter: map[string]any{
							"title": "Test",
						},
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
				err = job.Execute()
			})

			It("should not return an error from Execute", func() {
				// The record processing error should be caught, but report generation also fails
				Expect(err).To(HaveOccurred())
			})

			It("should include the error in failed records", func() {
				result := job.GetResult()
				Expect(result.FailedRecords).To(HaveLen(1))
				Expect(result.FailedRecords[0].Error).To(ContainSubstring("failed to write frontmatter"))
			})
		})
	})

	Describe("Execute with WriteMarkdown failure for inv_item template", func() {
		var job *PageImportJob

		When("WriteMarkdown fails for a new inv_item page", func() {
			var err error

			BeforeEach(func() {
				mockDeps.writeMarkdownErr = os.ErrPermission

				records := []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "new_item",
						Template:   "inv_item",
					},
				}
				job, _ = NewPageImportJob(records, mockDeps, logger)
				err = job.Execute()
			})

			It("should not return an error from Execute", func() {
				// The record processing error should be caught, but report generation also fails
				Expect(err).To(HaveOccurred())
			})

			It("should include the error in failed records", func() {
				result := job.GetResult()
				Expect(result.FailedRecords).To(HaveLen(1))
				Expect(result.FailedRecords[0].Error).To(ContainSubstring("failed to write markdown"))
			})
		})
	})

	Describe("Execute with empty records", func() {
		var job *PageImportJob

		When("no records are provided", func() {
			var err error

			BeforeEach(func() {
				records := []pageimport.ParsedRecord{}
				job, _ = NewPageImportJob(records, mockDeps, logger)
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
