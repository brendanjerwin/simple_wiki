//revive:disable:dot-imports
package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/eval"
)

var _ = Describe("resolveSurfaces", func() {
	var liveSurface eval.ToolSurface

	BeforeEach(func() {
		liveSurface = eval.ToolSurface{Label: "post", Tools: []eval.ToolDef{
			{Name: "api_v1_PageManagementService_CreatePage", Description: "Creates a page"},
		}}
	})

	When("no surface flag and no compare flag are set", func() {
		It("should default to the live (post) surface", func() {
			surfaces, err := resolveSurfaces(liveSurface, "", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(surfaces).To(HaveLen(1))
			Expect(surfaces[0].Label).To(Equal("post"))
		})
	})

	When("surface is 'post'", func() {
		It("should return the live surface", func() {
			surfaces, err := resolveSurfaces(liveSurface, "", "post")
			Expect(err).NotTo(HaveOccurred())
			Expect(surfaces).To(HaveLen(1))
			Expect(surfaces[0].Label).To(Equal("post"))
		})
	})

	When("surface is 'pre'", func() {
		It("should return the pre-PR surface", func() {
			surfaces, err := resolveSurfaces(liveSurface, "", "pre")
			Expect(err).NotTo(HaveOccurred())
			Expect(surfaces).To(HaveLen(1))
			Expect(surfaces[0].Label).To(Equal("pre-PR"))
		})
	})

	When("surface is an unknown value", func() {
		It("should return an error", func() {
			_, err := resolveSurfaces(liveSurface, "", "bogus")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown surface"))
		})
	})

	When("compare is set to 'pre,post'", func() {
		It("should return both surfaces in order", func() {
			surfaces, err := resolveSurfaces(liveSurface, "pre,post", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(surfaces).To(HaveLen(2))
			Expect(surfaces[0].Label).To(Equal("pre-PR"))
			Expect(surfaces[1].Label).To(Equal("post"))
		})
	})

	When("compare contains an unknown label", func() {
		It("should return an error", func() {
			_, err := resolveSurfaces(liveSurface, "pre,unknown", "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown surface label"))
		})
	})
})

var _ = Describe("resolveModels", func() {
	When("a known model name is given", func() {
		It("should return that model config", func() {
			models, err := resolveModels("", "gemini-2.5-flash")
			Expect(err).NotTo(HaveOccurred())
			Expect(models).To(HaveLen(1))
			Expect(models[0].Name).To(Equal("gemini-2.5-flash"))
		})
	})

	When("an unknown model name is given", func() {
		It("should return an error", func() {
			_, err := resolveModels("", "bogus-model")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown model"))
		})
	})

	When("sweep-model has multiple known models", func() {
		It("should return all named models", func() {
			models, err := resolveModels("gemini-2.5-flash,gpt-4o-mini", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(models).To(HaveLen(2))
		})
	})

	When("sweep-model contains an unknown model", func() {
		It("should return an error", func() {
			_, err := resolveModels("gemini-2.5-flash,no-such-model", "")
			Expect(err).To(HaveOccurred())
		})
	})
})

var _ = Describe("resolvePrompts", func() {
	When("a known prompt name is given", func() {
		It("should return that prompt", func() {
			prompts, err := resolvePrompts("", "minimal")
			Expect(err).NotTo(HaveOccurred())
			Expect(prompts).To(HaveLen(1))
			Expect(prompts[0].Name).To(Equal("minimal"))
		})
	})

	When("an unknown prompt name is given", func() {
		It("should return an error", func() {
			_, err := resolvePrompts("", "no-such-prompt")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown prompt"))
		})
	})

	When("sweep-prompt has multiple prompts", func() {
		It("should return all named prompts", func() {
			prompts, err := resolvePrompts("minimal,production", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(prompts).To(HaveLen(2))
		})
	})
})

var _ = Describe("filterByTag", func() {
	var cases []eval.Case

	BeforeEach(func() {
		cases = []eval.Case{
			{ID: "easy-1", Tags: []string{"easy", "page"}},
			{ID: "hard-1", Tags: []string{"hard", "page"}},
			{ID: "easy-2", Tags: []string{"easy", "search"}},
		}
	})

	When("filtering by an existing tag", func() {
		It("should return only matching cases", func() {
			filtered := filterByTag(cases, "easy")
			Expect(filtered).To(HaveLen(2))
			ids := []string{filtered[0].ID, filtered[1].ID}
			Expect(ids).To(ContainElements("easy-1", "easy-2"))
		})
	})

	When("filtering by a tag with no matches", func() {
		It("should return an empty slice", func() {
			filtered := filterByTag(cases, "medium")
			Expect(filtered).To(BeEmpty())
		})
	})
})

var _ = Describe("countErrors", func() {
	When("some results have errors", func() {
		It("should count the results with non-empty Error field", func() {
			results := []eval.CaseResult{
				{CaseID: "1", Error: "timeout"},
				{CaseID: "2"},
				{CaseID: "3", Error: "parse error"},
			}
			Expect(countErrors(results)).To(Equal(2))
		})
	})

	When("no results have errors", func() {
		It("should return zero", func() {
			results := []eval.CaseResult{
				{CaseID: "1"},
				{CaseID: "2"},
			}
			Expect(countErrors(results)).To(Equal(0))
		})
	})
})

var _ = Describe("labels / modelNames / promptNames", func() {
	Describe("labels", func() {
		It("should join surface labels with a comma", func() {
			surfaces := []eval.ToolSurface{
				{Label: "pre-PR"},
				{Label: "post"},
			}
			Expect(labels(surfaces)).To(Equal("pre-PR, post"))
		})
	})

	Describe("modelNames", func() {
		It("should join model names with a comma", func() {
			models := []eval.ModelConfig{
				{Name: "gemini-2.5-flash"},
				{Name: "gpt-4o-mini"},
			}
			Expect(modelNames(models)).To(Equal("gemini-2.5-flash, gpt-4o-mini"))
		})
	})

	Describe("promptNames", func() {
		It("should join prompt names with a comma", func() {
			prompts := []eval.PromptPreset{
				{Name: "minimal"},
				{Name: "production"},
			}
			Expect(promptNames(prompts)).To(Equal("minimal, production"))
		})
	})
})

var _ = Describe("estimateCost", func() {
	When("computing cost for a single model and config", func() {
		It("should return a positive cost estimate for non-zero token models", func() {
			surfaces := []eval.ToolSurface{{Label: "post"}}
			models := []eval.ModelConfig{{Name: "test", PromptCostPer1M: 1.0, CompletionCostPer1M: 2.0}}
			prompts := []eval.PromptPreset{{Name: "minimal"}}
			cases := []eval.Case{{ID: "case-1"}, {ID: "case-2"}}

			cost := estimateCost(cases, surfaces, models, prompts)
			Expect(cost).To(BeNumerically(">", 0))
		})
	})

	When("there are no cases", func() {
		It("should return zero cost", func() {
			surfaces := []eval.ToolSurface{{Label: "post"}}
			models := []eval.ModelConfig{{Name: "test", PromptCostPer1M: 1.0, CompletionCostPer1M: 2.0}}
			prompts := []eval.PromptPreset{{Name: "minimal"}}

			cost := estimateCost(nil, surfaces, models, prompts)
			Expect(cost).To(BeNumerically("~", 0.0))
		})
	})
})
