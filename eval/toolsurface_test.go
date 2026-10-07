//revive:disable:dot-imports
package eval_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/eval"
)

var _ = Describe("ToolSurface", func() {
	var surface eval.ToolSurface

	BeforeEach(func() {
		surface = eval.ToolSurface{
			Label: "post",
			Tools: []eval.ToolDef{
				{Name: "tool_A", Description: "Does A"},
				{Name: "tool_B", Description: "Does B"},
				{Name: "tool_C", Description: "Does C"},
			},
		}
	})

	Describe("ToolCount", func() {
		It("should return the number of tools", func() {
			Expect(surface.ToolCount()).To(Equal(3))
		})

		When("the surface has no tools", func() {
			It("should return zero", func() {
				empty := eval.ToolSurface{}
				Expect(empty.ToolCount()).To(Equal(0))
			})
		})
	})

	Describe("Find", func() {
		When("the tool exists", func() {
			It("should return a pointer to the matching tool", func() {
				t := surface.Find("tool_B")
				Expect(t).NotTo(BeNil())
				Expect(t.Name).To(Equal("tool_B"))
				Expect(t.Description).To(Equal("Does B"))
			})
		})

		When("the tool does not exist", func() {
			It("should return nil", func() {
				t := surface.Find("nonexistent_tool")
				Expect(t).To(BeNil())
			})
		})
	})

	Describe("Names", func() {
		It("should return all tool names in order", func() {
			names := surface.Names()
			Expect(names).To(Equal([]string{"tool_A", "tool_B", "tool_C"}))
		})

		When("there are no tools", func() {
			It("should return an empty slice", func() {
				empty := eval.ToolSurface{}
				Expect(empty.Names()).To(BeEmpty())
			})
		})
	})

	Describe("Descriptions", func() {
		It("should return a map of name to description", func() {
			descs := surface.Descriptions()
			Expect(descs).To(HaveLen(3))
			Expect(descs["tool_A"]).To(Equal("Does A"))
			Expect(descs["tool_B"]).To(Equal("Does B"))
			Expect(descs["tool_C"]).To(Equal("Does C"))
		})

		When("there are no tools", func() {
			It("should return an empty map", func() {
				empty := eval.ToolSurface{}
				Expect(empty.Descriptions()).To(BeEmpty())
			})
		})
	})
})
