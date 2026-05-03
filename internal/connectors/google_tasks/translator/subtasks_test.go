//revive:disable:dot-imports
package translator_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/translator"
)

var _ = Describe("HasSubtasks", func() {
	When("the slice is empty", func() {
		It("should return false", func() {
			Expect(translator.HasSubtasks(nil)).To(BeFalse())
		})
	})

	When("no task has a parent", func() {
		var has bool

		BeforeEach(func() {
			has = translator.HasSubtasks([]translator.Task{
				{ID: "a"},
				{ID: "b"},
			})
		})

		It("should return false", func() {
			Expect(has).To(BeFalse())
		})
	})

	When("at least one task has a parent", func() {
		var has bool

		BeforeEach(func() {
			has = translator.HasSubtasks([]translator.Task{
				{ID: "a"},
				{ID: "b", Parent: "a"},
			})
		})

		It("should return true", func() {
			Expect(has).To(BeTrue())
		})
	})
})

var _ = Describe("FlattenSubtasks", func() {
	When("tasks have parents set", func() {
		var (
			input    []translator.Task
			result   []translator.Task
		)

		BeforeEach(func() {
			input = []translator.Task{
				{ID: "a"},
				{ID: "b", Parent: "a"},
				{ID: "c", Parent: "a"},
			}
			result = translator.FlattenSubtasks(input)
		})

		It("should clear all parent fields in the result", func() {
			for _, t := range result {
				Expect(t.Parent).To(Equal(""))
			}
		})

		It("should preserve the input task ids", func() {
			Expect(result[0].ID).To(Equal("a"))
			Expect(result[1].ID).To(Equal("b"))
			Expect(result[2].ID).To(Equal("c"))
		})

		It("should not mutate the input slice", func() {
			Expect(input[1].Parent).To(Equal("a"))
			Expect(input[2].Parent).To(Equal("a"))
		})
	})

	When("the slice is empty", func() {
		var result []translator.Task

		BeforeEach(func() {
			result = translator.FlattenSubtasks(nil)
		})

		It("should return a non-nil empty slice", func() {
			Expect(result).NotTo(BeNil())
			Expect(result).To(BeEmpty())
		})
	})
})
