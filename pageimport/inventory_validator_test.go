//revive:disable:dot-imports
package pageimport_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/pageimport"
)

// mockPageChecker implements PageExistenceChecker for testing.
type mockPageChecker struct {
	existingPages map[string]bool
}

func (m *mockPageChecker) PageExists(identifier string) bool {
	return m.existingPages[identifier]
}

// mockContainerGetter implements ContainerReferenceGetter for testing.
type mockContainerGetter struct {
	containerRefs map[string]string
}

func (m *mockContainerGetter) GetContainerReference(identifier string) string {
	return m.containerRefs[identifier]
}

var _ = Describe("InventoryValidator", func() {
	Describe("ValidateContainerIdentifier", func() {
		When("inventory.container is empty", func() {
			var (
				validator *pageimport.InventoryValidator
				record    pageimport.ParsedRecord
			)

			BeforeEach(func() {
				validator = pageimport.NewInventoryValidator(nil, nil)
				record = pageimport.ParsedRecord{
					RowNumber:   1,
					Identifier:  "test_page",
					Frontmatter: map[string]any{},
				}
				validator.ValidateContainerIdentifier(&record)
			})

			It("should not add validation errors", func() {
				Expect(record.ValidationErrors).To(BeEmpty())
			})
		})

		When("inventory.container is a valid normalized identifier", func() {
			var (
				validator *pageimport.InventoryValidator
				record    pageimport.ParsedRecord
			)

			BeforeEach(func() {
				validator = pageimport.NewInventoryValidator(nil, nil)
				record = pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "test_page",
					Frontmatter: map[string]any{
						"inventory": map[string]any{
							"container": "my_container",
						},
					},
				}
				validator.ValidateContainerIdentifier(&record)
			})

			It("should not add validation errors", func() {
				Expect(record.ValidationErrors).To(BeEmpty())
			})
		})

		When("inventory.container would be normalized differently", func() {
			var (
				validator *pageimport.InventoryValidator
				record    pageimport.ParsedRecord
			)

			BeforeEach(func() {
				validator = pageimport.NewInventoryValidator(nil, nil)
				record = pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "test_page",
					Frontmatter: map[string]any{
						"inventory": map[string]any{
							"container": "MyContainer",
						},
					},
				}
				validator.ValidateContainerIdentifier(&record)
			})

			It("should add a validation error", func() {
				Expect(record.ValidationErrors).To(HaveLen(1))
			})

			It("should mention normalization in the error", func() {
				Expect(record.ValidationErrors[0]).To(ContainSubstring("my_container"))
			})
		})

		When("inventory.container is invalid (fails MungeIdentifier)", func() {
			var (
				validator *pageimport.InventoryValidator
				record    pageimport.ParsedRecord
			)

			BeforeEach(func() {
				validator = pageimport.NewInventoryValidator(nil, nil)
				record = pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "test_page",
					Frontmatter: map[string]any{
						"inventory": map[string]any{
							"container": "///",
						},
					},
				}
				validator.ValidateContainerIdentifier(&record)
			})

			It("should add a validation error", func() {
				Expect(record.ValidationErrors).To(HaveLen(1))
			})

			It("should mention invalid in the error", func() {
				Expect(record.ValidationErrors[0]).To(ContainSubstring("invalid"))
			})
		})
	})

	Describe("ValidateInventoryItemsIdentifiers", func() {
		When("no inventory.items[] operations exist", func() {
			var (
				validator *pageimport.InventoryValidator
				record    pageimport.ParsedRecord
			)

			BeforeEach(func() {
				validator = pageimport.NewInventoryValidator(nil, nil)
				record = pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "test_page",
					ArrayOps:   []pageimport.ArrayOperation{},
				}
				validator.ValidateInventoryItemsIdentifiers(&record)
			})

			It("should not add validation errors", func() {
				Expect(record.ValidationErrors).To(BeEmpty())
			})
		})

		When("inventory.items[] value is valid", func() {
			var (
				validator *pageimport.InventoryValidator
				record    pageimport.ParsedRecord
			)

			BeforeEach(func() {
				validator = pageimport.NewInventoryValidator(nil, nil)
				record = pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "test_page",
					ArrayOps: []pageimport.ArrayOperation{
						{FieldPath: "inventory.items", Operation: pageimport.EnsureExists, Value: "valid_item"},
					},
				}
				validator.ValidateInventoryItemsIdentifiers(&record)
			})

			It("should not add validation errors", func() {
				Expect(record.ValidationErrors).To(BeEmpty())
			})
		})

		When("inventory.items[] value would be normalized", func() {
			var (
				validator *pageimport.InventoryValidator
				record    pageimport.ParsedRecord
			)

			BeforeEach(func() {
				validator = pageimport.NewInventoryValidator(nil, nil)
				record = pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "test_page",
					ArrayOps: []pageimport.ArrayOperation{
						{FieldPath: "inventory.items", Operation: pageimport.EnsureExists, Value: "MyItem"},
					},
				}
				validator.ValidateInventoryItemsIdentifiers(&record)
			})

			It("should add a validation error", func() {
				Expect(record.ValidationErrors).To(HaveLen(1))
			})

			It("should mention the normalized value", func() {
				Expect(record.ValidationErrors[0]).To(ContainSubstring("my_item"))
			})
		})

		When("inventory.items[] value is invalid", func() {
			var (
				validator *pageimport.InventoryValidator
				record    pageimport.ParsedRecord
			)

			BeforeEach(func() {
				validator = pageimport.NewInventoryValidator(nil, nil)
				record = pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "test_page",
					ArrayOps: []pageimport.ArrayOperation{
						{FieldPath: "inventory.items", Operation: pageimport.EnsureExists, Value: "///"},
					},
				}
				validator.ValidateInventoryItemsIdentifiers(&record)
			})

			It("should add a validation error", func() {
				Expect(record.ValidationErrors).To(HaveLen(1))
			})

			It("should mention invalid in the error", func() {
				Expect(record.ValidationErrors[0]).To(ContainSubstring("invalid"))
			})
		})

		When("inventory.items[] operation is DeleteValue", func() {
			var (
				validator *pageimport.InventoryValidator
				record    pageimport.ParsedRecord
			)

			BeforeEach(func() {
				validator = pageimport.NewInventoryValidator(nil, nil)
				record = pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "test_page",
					ArrayOps: []pageimport.ArrayOperation{
						{FieldPath: "inventory.items", Operation: pageimport.DeleteValue, Value: "SomeItem"},
					},
				}
				validator.ValidateInventoryItemsIdentifiers(&record)
			})

			It("should not validate deleted values", func() {
				Expect(record.ValidationErrors).To(BeEmpty())
			})
		})

		When("operation is on a different field", func() {
			var (
				validator *pageimport.InventoryValidator
				record    pageimport.ParsedRecord
			)

			BeforeEach(func() {
				validator = pageimport.NewInventoryValidator(nil, nil)
				record = pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "test_page",
					ArrayOps: []pageimport.ArrayOperation{
						{FieldPath: "tags", Operation: pageimport.EnsureExists, Value: "MyTag"},
					},
				}
				validator.ValidateInventoryItemsIdentifiers(&record)
			})

			It("should not validate non-inventory.items fields", func() {
				Expect(record.ValidationErrors).To(BeEmpty())
			})
		})

		When("multiple inventory.items[] operations with mixed validity", func() {
			var (
				validator *pageimport.InventoryValidator
				record    pageimport.ParsedRecord
			)

			BeforeEach(func() {
				validator = pageimport.NewInventoryValidator(nil, nil)
				record = pageimport.ParsedRecord{
					RowNumber:  1,
					Identifier: "test_page",
					ArrayOps: []pageimport.ArrayOperation{
						{FieldPath: "inventory.items", Operation: pageimport.EnsureExists, Value: "valid_item"},
						{FieldPath: "inventory.items", Operation: pageimport.EnsureExists, Value: "InvalidItem"},
						{FieldPath: "inventory.items", Operation: pageimport.EnsureExists, Value: "another_valid"},
						{FieldPath: "inventory.items", Operation: pageimport.DeleteValue, Value: "DeletedItem"},
					},
				}
				validator.ValidateInventoryItemsIdentifiers(&record)
			})

			It("should only add error for the invalid item", func() {
				Expect(record.ValidationErrors).To(HaveLen(1))
			})

			It("should mention the normalized value for the invalid item", func() {
				Expect(record.ValidationErrors[0]).To(ContainSubstring("invalid_item"))
			})

			It("should not validate deleted items", func() {
				for _, err := range record.ValidationErrors {
					Expect(err).NotTo(ContainSubstring("DeletedItem"))
				}
			})
		})
	})

	Describe("ValidateContainerExistence", func() {
		When("container exists in existing pages", func() {
			var (
				validator *pageimport.InventoryValidator
				records   []pageimport.ParsedRecord
			)

			BeforeEach(func() {
				checker := &mockPageChecker{
					existingPages: map[string]bool{"my_container": true},
				}
				validator = pageimport.NewInventoryValidator(checker, nil)
				records = []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "test_page",
						Frontmatter: map[string]any{
							"inventory": map[string]any{
								"container": "my_container",
							},
						},
					},
				}
				validator.ValidateContainerExistence(records)
			})

			It("should not add validation errors", func() {
				Expect(records[0].ValidationErrors).To(BeEmpty())
			})
		})

		When("container exists in same import", func() {
			var (
				validator *pageimport.InventoryValidator
				records   []pageimport.ParsedRecord
			)

			BeforeEach(func() {
				checker := &mockPageChecker{existingPages: map[string]bool{}}
				validator = pageimport.NewInventoryValidator(checker, nil)
				records = []pageimport.ParsedRecord{
					{
						RowNumber:   1,
						Identifier:  "my_container",
						Frontmatter: map[string]any{},
					},
					{
						RowNumber:  2,
						Identifier: "test_page",
						Frontmatter: map[string]any{
							"inventory": map[string]any{
								"container": "my_container",
							},
						},
					},
				}
				validator.ValidateContainerExistence(records)
			})

			It("should not add validation errors", func() {
				Expect(records[1].ValidationErrors).To(BeEmpty())
			})
		})

		When("container exists in import with different case", func() {
			var (
				validator *pageimport.InventoryValidator
				records   []pageimport.ParsedRecord
			)

			BeforeEach(func() {
				checker := &mockPageChecker{existingPages: map[string]bool{}}
				validator = pageimport.NewInventoryValidator(checker, nil)
				records = []pageimport.ParsedRecord{
					{
						RowNumber:   1,
						Identifier:  "my_container",
						Frontmatter: map[string]any{},
					},
					{
						RowNumber:  2,
						Identifier: "test_page",
						Frontmatter: map[string]any{
							"inventory": map[string]any{
								"container": "MyContainer", // Different case, munges to my_container
							},
						},
					},
				}
				validator.ValidateContainerExistence(records)
			})

			It("should match after munging both sides", func() {
				// Container identifier validation might add error, but existence should pass
				// Note: This test checks existence matching, not identifier validation
				// The actual identifier validation happens separately
				existenceErrors := 0
				for _, err := range records[1].ValidationErrors {
					if strings.Contains(err, "does not exist") || strings.Contains(err, "non-existent") {
						existenceErrors++
					}
				}
				Expect(existenceErrors).To(Equal(0))
			})
		})

		When("container does not exist anywhere", func() {
			var (
				validator *pageimport.InventoryValidator
				records   []pageimport.ParsedRecord
			)

			BeforeEach(func() {
				checker := &mockPageChecker{existingPages: map[string]bool{}}
				validator = pageimport.NewInventoryValidator(checker, nil)
				records = []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "test_page",
						Frontmatter: map[string]any{
							"inventory": map[string]any{
								"container": "nonexistent_container",
							},
						},
					},
				}
				validator.ValidateContainerExistence(records)
			})

			It("should add a validation error", func() {
				Expect(records[0].ValidationErrors).To(HaveLen(1))
			})

			It("should mention non-existent in the error", func() {
				Expect(records[0].ValidationErrors[0]).To(ContainSubstring("non-existent"))
			})
		})

		When("container reference is to a record with validation errors", func() {
			var (
				validator *pageimport.InventoryValidator
				records   []pageimport.ParsedRecord
			)

			BeforeEach(func() {
				checker := &mockPageChecker{existingPages: map[string]bool{}}
				validator = pageimport.NewInventoryValidator(checker, nil)
				records = []pageimport.ParsedRecord{
					{
						RowNumber:        1,
						Identifier:       "bad_container",
						Frontmatter:      map[string]any{},
						ValidationErrors: []string{"some previous error"},
					},
					{
						RowNumber:  2,
						Identifier: "test_page",
						Frontmatter: map[string]any{
							"inventory": map[string]any{
								"container": "bad_container",
							},
						},
					},
				}
				validator.ValidateContainerExistence(records)
			})

			It("should add validation error since errored records don't count", func() {
				Expect(records[1].ValidationErrors).To(HaveLen(1))
			})
		})
	})

	Describe("DetectCircularReferences", func() {
		When("self-reference in import", func() {
			var (
				validator *pageimport.InventoryValidator
				records   []pageimport.ParsedRecord
			)

			BeforeEach(func() {
				getter := &mockContainerGetter{containerRefs: map[string]string{}}
				validator = pageimport.NewInventoryValidator(nil, getter)
				records = []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "test_page",
						Frontmatter: map[string]any{
							"inventory": map[string]any{
								"container": "test_page",
							},
						},
					},
				}
				validator.DetectCircularReferences(records)
			})

			It("should add a validation error", func() {
				Expect(records[0].ValidationErrors).To(HaveLen(1))
			})

			It("should mention circular reference", func() {
				Expect(records[0].ValidationErrors[0]).To(ContainSubstring("circular"))
			})
		})

		When("cycle entirely within import", func() {
			var (
				validator *pageimport.InventoryValidator
				records   []pageimport.ParsedRecord
			)

			BeforeEach(func() {
				getter := &mockContainerGetter{containerRefs: map[string]string{}}
				validator = pageimport.NewInventoryValidator(nil, getter)
				records = []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "page_a",
						Frontmatter: map[string]any{
							"inventory": map[string]any{
								"container": "page_b",
							},
						},
					},
					{
						RowNumber:  2,
						Identifier: "page_b",
						Frontmatter: map[string]any{
							"inventory": map[string]any{
								"container": "page_c",
							},
						},
					},
					{
						RowNumber:  3,
						Identifier: "page_c",
						Frontmatter: map[string]any{
							"inventory": map[string]any{
								"container": "page_a",
							},
						},
					},
				}
				validator.DetectCircularReferences(records)
			})

			It("should add validation errors to records in the cycle", func() {
				cycleErrorCount := 0
				for _, r := range records {
					for _, err := range r.ValidationErrors {
						if strings.Contains(err, "circular") {
							cycleErrorCount++
						}
					}
				}
				Expect(cycleErrorCount).To(BeNumerically(">=", 1))
			})
		})

		When("cycle involves existing pages", func() {
			var (
				validator *pageimport.InventoryValidator
				records   []pageimport.ParsedRecord
			)

			BeforeEach(func() {
				getter := &mockContainerGetter{
					containerRefs: map[string]string{
						"existing_page": "page_a", // existing_page's container is page_a
					},
				}
				validator = pageimport.NewInventoryValidator(nil, getter)
				records = []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "page_a",
						Frontmatter: map[string]any{
							"inventory": map[string]any{
								"container": "existing_page", // page_a -> existing_page -> page_a (cycle)
							},
						},
					},
				}
				validator.DetectCircularReferences(records)
			})

			It("should detect the cycle through existing pages", func() {
				Expect(records[0].ValidationErrors).To(HaveLen(1))
			})

			It("should mention circular reference", func() {
				Expect(records[0].ValidationErrors[0]).To(ContainSubstring("circular"))
			})
		})

		When("chain without cycle", func() {
			var (
				validator *pageimport.InventoryValidator
				records   []pageimport.ParsedRecord
			)

			BeforeEach(func() {
				getter := &mockContainerGetter{
					containerRefs: map[string]string{
						"root_container": "", // root has no container
					},
				}
				checker := &mockPageChecker{
					existingPages: map[string]bool{"root_container": true},
				}
				validator = pageimport.NewInventoryValidator(checker, getter)
				records = []pageimport.ParsedRecord{
					{
						RowNumber:  1,
						Identifier: "page_a",
						Frontmatter: map[string]any{
							"inventory": map[string]any{
								"container": "page_b",
							},
						},
					},
					{
						RowNumber:  2,
						Identifier: "page_b",
						Frontmatter: map[string]any{
							"inventory": map[string]any{
								"container": "root_container",
							},
						},
					},
				}
				validator.DetectCircularReferences(records)
			})

			It("should not add validation errors", func() {
				for _, r := range records {
					for _, err := range r.ValidationErrors {
						Expect(err).NotTo(ContainSubstring("circular"))
					}
				}
			})
		})

		When("record has no container reference", func() {
			var (
				validator *pageimport.InventoryValidator
				records   []pageimport.ParsedRecord
			)

			BeforeEach(func() {
				getter := &mockContainerGetter{containerRefs: map[string]string{}}
				validator = pageimport.NewInventoryValidator(nil, getter)
				records = []pageimport.ParsedRecord{
					{
						RowNumber:   1,
						Identifier:  "test_page",
						Frontmatter: map[string]any{},
					},
				}
				validator.DetectCircularReferences(records)
			})

			It("should not add validation errors", func() {
				Expect(records[0].ValidationErrors).To(BeEmpty())
			})
		})
	})
})
