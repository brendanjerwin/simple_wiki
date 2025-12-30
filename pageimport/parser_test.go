//revive:disable:dot-imports
package pageimport_test

import (
	"strings"

	"github.com/brendanjerwin/simple_wiki/pageimport"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ParseCSV", func() {
	Describe("valid CSV parsing", func() {
		When("parsing CSV with scalar fields", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "identifier,title,description\ntest_page,Test Title,A description"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have no parsing errors", func() {
				Expect(result.HasErrors()).To(BeFalse())
			})

			It("should parse one record", func() {
				Expect(result.Records).To(HaveLen(1))
			})

			It("should set the identifier correctly", func() {
				Expect(result.Records[0].Identifier).To(Equal("test_page"))
			})

			It("should set scalar frontmatter fields", func() {
				Expect(result.Records[0].Frontmatter["title"]).To(Equal("Test Title"))
				Expect(result.Records[0].Frontmatter["description"]).To(Equal("A description"))
			})

			It("should have row number 1", func() {
				Expect(result.Records[0].RowNumber).To(Equal(1))
			})
		})

		When("parsing CSV with array fields ([] suffix)", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "identifier,tags[],tags[],categories[]\ntest_page,tag1,tag2,cat1"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have no parsing errors", func() {
				Expect(result.HasErrors()).To(BeFalse())
			})

			It("should parse array operations", func() {
				Expect(result.Records[0].ArrayOps).To(HaveLen(3))
			})

			It("should have EnsureExists operations for array values", func() {
				for _, op := range result.Records[0].ArrayOps {
					Expect(op.Operation).To(Equal(pageimport.EnsureExists))
				}
			})

			It("should track the correct field paths", func() {
				fieldPaths := make(map[string]bool)
				for _, op := range result.Records[0].ArrayOps {
					fieldPaths[op.FieldPath] = true
				}
				Expect(fieldPaths).To(HaveKey("tags"))
				Expect(fieldPaths).To(HaveKey("categories"))
			})
		})

		When("parsing CSV with nested table.field notation", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "identifier,inventory.container\ntest_page,container_id"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have no parsing errors", func() {
				Expect(result.HasErrors()).To(BeFalse())
			})

			It("should create nested frontmatter structure", func() {
				inventory, ok := result.Records[0].Frontmatter["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(inventory["container"]).To(Equal("container_id"))
			})
		})

		When("parsing CSV with multiple rows", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "identifier,title\npage_1,Title 1\npage_2,Title 2\npage_3,Title 3"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should parse all rows", func() {
				Expect(result.Records).To(HaveLen(3))
			})

			It("should have correct row numbers", func() {
				Expect(result.Records[0].RowNumber).To(Equal(1))
				Expect(result.Records[1].RowNumber).To(Equal(2))
				Expect(result.Records[2].RowNumber).To(Equal(3))
			})
		})
	})

	Describe("template column handling", func() {
		When("parsing CSV with template column", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "identifier,template,title\ntest_page,item_template,Test Title"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should set the template field", func() {
				Expect(result.Records[0].Template).To(Equal("item_template"))
			})

			It("should not include template in frontmatter", func() {
				Expect(result.Records[0].Frontmatter).NotTo(HaveKey("template"))
			})
		})
	})

	Describe("error handling", func() {
		When("CSV content is empty", func() {
			var err error

			BeforeEach(func() {
				_, err = pageimport.ParseCSV("")
			})

			It("should return an error", func() {
				Expect(err).To(MatchError("CSV content is empty"))
			})
		})

		When("CSV has only headers (no data rows)", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "identifier,title,description"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return a Go error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have parsing errors", func() {
				Expect(result.HasErrors()).To(BeTrue())
			})

			It("should report no data rows error", func() {
				Expect(result.ParsingErrors).To(ContainElement("CSV has no data rows"))
			})
		})

		When("CSV is missing identifier column", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "title,description\nTest Title,A description"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return a Go error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have parsing errors", func() {
				Expect(result.HasErrors()).To(BeTrue())
			})

			It("should report missing identifier column", func() {
				Expect(result.ParsingErrors).To(ContainElement("CSV must have 'identifier' column"))
			})
		})

		When("CSV exceeds maximum row limit", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				var builder strings.Builder
				builder.WriteString("identifier,title\n")
				for i := 0; i <= pageimport.MaxRows; i++ {
					builder.WriteString("page_")
					builder.WriteString(string(rune('a' + (i % 26))))
					builder.WriteString("_")
					builder.WriteString(strings.Repeat("x", 10))
					builder.WriteString(",Title\n")
				}
				result, err = pageimport.ParseCSV(builder.String())
			})

			It("should not return a Go error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have parsing errors", func() {
				Expect(result.HasErrors()).To(BeTrue())
			})

			It("should report row limit exceeded", func() {
				Expect(result.ParsingErrors[0]).To(ContainSubstring("exceeds"))
				Expect(result.ParsingErrors[0]).To(ContainSubstring("row limit"))
			})
		})

		When("identifier format is invalid", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "identifier,title\n///,Invalid ID"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return a Go error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have record with validation errors", func() {
				Expect(result.Records[0].HasErrors()).To(BeTrue())
			})

			It("should report invalid identifier error", func() {
				Expect(result.Records[0].ValidationErrors[0]).To(ContainSubstring("invalid"))
			})
		})

		When("identifier would be normalized", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "identifier,title\nTestPage,Title"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return a Go error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have record with validation errors", func() {
				Expect(result.Records[0].HasErrors()).To(BeTrue())
			})

			It("should indicate normalization would occur", func() {
				Expect(result.Records[0].ValidationErrors[0]).To(ContainSubstring("would be normalized"))
			})
		})

		When("identifier is empty in a row", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "identifier,title\n,Empty ID"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return a Go error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have record with validation errors", func() {
				Expect(result.Records[0].HasErrors()).To(BeTrue())
			})

			It("should report empty identifier error", func() {
				Expect(result.Records[0].ValidationErrors).To(ContainElement("identifier cannot be empty"))
			})
		})

		When("duplicate identifiers exist", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "identifier,title\ntest_page,Title 1\ntest_page,Title 2"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return a Go error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have validation error on second row", func() {
				Expect(result.Records[1].HasErrors()).To(BeTrue())
			})

			It("should report duplicate identifier", func() {
				Expect(result.Records[1].ValidationErrors[0]).To(ContainSubstring("duplicate identifier"))
			})

			It("should reference the first occurrence row", func() {
				Expect(result.Records[1].ValidationErrors[0]).To(ContainSubstring("row 1"))
			})
		})

		When("nested path has too many levels (a.b.c)", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "identifier,a.b.c\ntest_page,value"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return a Go error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have parsing errors", func() {
				Expect(result.HasErrors()).To(BeTrue())
			})

			It("should report nested tables not supported", func() {
				Expect(result.ParsingErrors[0]).To(ContainSubstring("nested tables not supported"))
			})
		})
	})

	Describe("DELETE sentinel handling", func() {
		When("[[DELETE]] is used for scalar field", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "identifier,title,description\ntest_page,[[DELETE]],Keep this"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have no validation errors", func() {
				Expect(result.Records[0].HasErrors()).To(BeFalse())
			})

			It("should add field to FieldsToDelete", func() {
				Expect(result.Records[0].FieldsToDelete).To(ContainElement("title"))
			})

			It("should not include deleted field in frontmatter", func() {
				Expect(result.Records[0].Frontmatter).NotTo(HaveKey("title"))
			})

			It("should keep other fields in frontmatter", func() {
				Expect(result.Records[0].Frontmatter["description"]).To(Equal("Keep this"))
			})
		})

		When("[[DELETE(value)]] is used for array field", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "identifier,tags[]\ntest_page,[[DELETE(old_tag)]]"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have no validation errors", func() {
				Expect(result.Records[0].HasErrors()).To(BeFalse())
			})

			It("should create DeleteValue array operation", func() {
				Expect(result.Records[0].ArrayOps).To(HaveLen(1))
				Expect(result.Records[0].ArrayOps[0].Operation).To(Equal(pageimport.DeleteValue))
			})

			It("should capture the value to delete", func() {
				Expect(result.Records[0].ArrayOps[0].Value).To(Equal("old_tag"))
			})
		})

		When("[[DELETE]] is used in array column (invalid)", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "identifier,tags[]\ntest_page,[[DELETE]]"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return a Go error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have validation error", func() {
				Expect(result.Records[0].HasErrors()).To(BeTrue())
			})

			It("should explain correct usage", func() {
				Expect(result.Records[0].ValidationErrors[0]).To(ContainSubstring("[[DELETE(value)]]"))
			})
		})

		When("[[DELETE(value)]] is used in scalar column (invalid)", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "identifier,title\ntest_page,[[DELETE(some_value)]]"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return a Go error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have validation error", func() {
				Expect(result.Records[0].HasErrors()).To(BeTrue())
			})

			It("should explain correct usage", func() {
				Expect(result.Records[0].ValidationErrors[0]).To(ContainSubstring("only valid for array columns"))
			})
		})
	})

	Describe("duplicate array values", func() {
		When("same value appears twice in same row for same array field", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "identifier,tags[],tags[]\ntest_page,same_tag,same_tag"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return a Go error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have validation error", func() {
				Expect(result.Records[0].HasErrors()).To(BeTrue())
			})

			It("should report duplicate value", func() {
				Expect(result.Records[0].ValidationErrors[0]).To(ContainSubstring("duplicate value"))
				Expect(result.Records[0].ValidationErrors[0]).To(ContainSubstring("same_tag"))
			})
		})

		When("same value appears in delete and add for same array field", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "identifier,tags[],tags[]\ntest_page,my_tag,[[DELETE(my_tag)]]"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return a Go error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have validation error for duplicate", func() {
				Expect(result.Records[0].HasErrors()).To(BeTrue())
				Expect(result.Records[0].ValidationErrors[0]).To(ContainSubstring("duplicate value"))
			})
		})
	})

	Describe("empty cells are skipped", func() {
		When("CSV has empty cells", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "identifier,title,description,tags[]\ntest_page,,Some desc,"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not include empty scalar fields in frontmatter", func() {
				Expect(result.Records[0].Frontmatter).NotTo(HaveKey("title"))
			})

			It("should include non-empty fields", func() {
				Expect(result.Records[0].Frontmatter["description"]).To(Equal("Some desc"))
			})

			It("should not create array ops for empty cells", func() {
				Expect(result.Records[0].ArrayOps).To(BeEmpty())
			})
		})

		When("CSV has whitespace-only cells", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "identifier,title\ntest_page,   "
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should treat whitespace-only as empty", func() {
				Expect(result.Records[0].Frontmatter).NotTo(HaveKey("title"))
			})
		})
	})

	Describe("ParseResult helper methods", func() {
		When("result has records with errors", func() {
			var result *pageimport.ParseResult

			BeforeEach(func() {
				csv := "identifier,title\nvalid_page,Title\n///,Invalid\nanother_valid,Title2"
				var err error
				result, err = pageimport.ParseCSV(csv)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should count errors correctly", func() {
				Expect(result.ErrorCount()).To(Equal(1))
			})

			It("should return only valid records", func() {
				valid := result.ValidRecords()
				Expect(valid).To(HaveLen(2))
				Expect(valid[0].Identifier).To(Equal("valid_page"))
				Expect(valid[1].Identifier).To(Equal("another_valid"))
			})
		})
	})

	Describe("header parsing", func() {
		When("headers have leading/trailing whitespace", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := " identifier , title \ntest_page,Test Title"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should parse correctly with trimmed headers", func() {
				Expect(result.Records[0].Identifier).To(Equal("test_page"))
				Expect(result.Records[0].Frontmatter["title"]).To(Equal("Test Title"))
			})
		})

		When("header has empty segment in path (table..field)", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				// table..field has 3 parts ["table", "", "field"], so hits nested check first
				csv := "identifier,table..field\ntest_page,value"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return a Go error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have parsing errors", func() {
				Expect(result.HasErrors()).To(BeTrue())
			})

			It("should report nested tables not supported", func() {
				Expect(result.ParsingErrors[0]).To(ContainSubstring("nested tables not supported"))
			})
		})

		When("header has trailing dot (.field)", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				// .field has 2 parts ["", "field"], so hits empty segment check
				csv := "identifier,.field\ntest_page,value"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return a Go error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have parsing errors", func() {
				Expect(result.HasErrors()).To(BeTrue())
			})

			It("should report empty segment", func() {
				Expect(result.ParsingErrors[0]).To(ContainSubstring("empty segment"))
			})
		})

		When("CSV has empty headers", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "identifier,,title\ntest_page,ignored,Test Title"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should skip empty headers", func() {
				Expect(result.Records[0].Frontmatter).NotTo(HaveKey(""))
			})

			It("should still parse other columns", func() {
				Expect(result.Records[0].Identifier).To(Equal("test_page"))
				Expect(result.Records[0].Frontmatter["title"]).To(Equal("Test Title"))
			})
		})
	})

	Describe("case-insensitive special columns", func() {
		When("identifier column has different case", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "IDENTIFIER,title\ntest_page,Test Title"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should recognize identifier column", func() {
				Expect(result.Records[0].Identifier).To(Equal("test_page"))
			})
		})

		When("template column has different case", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "identifier,TEMPLATE\ntest_page,my_template"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should recognize template column", func() {
				Expect(result.Records[0].Template).To(Equal("my_template"))
			})
		})
	})

	Describe("row with fewer columns than headers", func() {
		When("a row has fewer columns", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "identifier,title,description,extra\ntest_page,Title"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should parse available columns", func() {
				Expect(result.Records[0].Identifier).To(Equal("test_page"))
				Expect(result.Records[0].Frontmatter["title"]).To(Equal("Title"))
			})

			It("should not include missing columns", func() {
				Expect(result.Records[0].Frontmatter).NotTo(HaveKey("description"))
				Expect(result.Records[0].Frontmatter).NotTo(HaveKey("extra"))
			})
		})
	})

	Describe("combined array operations in single row", func() {
		When("row has both add and delete operations for same array", func() {
			var (
				result *pageimport.ParseResult
				err    error
			)

			BeforeEach(func() {
				csv := "identifier,tags[],tags[],tags[]\ntest_page,new_tag,[[DELETE(old_tag)]],another_tag"
				result, err = pageimport.ParseCSV(csv)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have no validation errors", func() {
				Expect(result.Records[0].HasErrors()).To(BeFalse())
			})

			It("should have three array operations", func() {
				Expect(result.Records[0].ArrayOps).To(HaveLen(3))
			})

			It("should have correct operation types", func() {
				opTypes := make(map[pageimport.ArrayOpType]int)
				for _, op := range result.Records[0].ArrayOps {
					opTypes[op.Operation]++
				}
				Expect(opTypes[pageimport.EnsureExists]).To(Equal(2))
				Expect(opTypes[pageimport.DeleteValue]).To(Equal(1))
			})
		})
	})
})
