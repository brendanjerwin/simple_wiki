//revive:disable:dot-imports
package main

import (
	"os"

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

var _ = Describe("writeResults", func() {
	When("writing valid results to a temp file", func() {
		var path string
		var err error

		BeforeEach(func() {
			f, tmpErr := os.CreateTemp("", "eval-results-*.json")
			Expect(tmpErr).NotTo(HaveOccurred())
			path = f.Name()
			Expect(f.Close()).To(Succeed())

			summaries := []eval.ScoreSummary{{ConfigLabel: "post|gemini|minimal", CaseCount: 2, ToolMatchCount: 1}}
			results := []eval.CaseResult{{CaseID: "c1", ToolMatch: true}, {CaseID: "c2", ToolMatch: false}}
			writeResults(path, summaries, results)
		})

		AfterEach(func() {
			_ = os.Remove(path)
		})

		It("should write a readable JSON file", func() {
			data, readErr := os.ReadFile(path)
			Expect(readErr).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring(`"summaries"`))
			Expect(string(data)).To(ContainSubstring(`"results"`))
			Expect(string(data)).To(ContainSubstring(`"timestamp"`))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

var _ = Describe("writePartialResults", func() {
	When("writing partial results to a temp file", func() {
		var path string

		BeforeEach(func() {
			f, tmpErr := os.CreateTemp("", "eval-partial-*.json")
			Expect(tmpErr).NotTo(HaveOccurred())
			path = f.Name()
			Expect(f.Close()).To(Succeed())

			surf := eval.ToolSurface{Label: "post"}
			m := eval.ModelConfig{Name: "gemini-2.5-flash"}
			p, lookupErr := resolvePrompts("", "minimal")
			Expect(lookupErr).NotTo(HaveOccurred())
			cfg := eval.Config{Surface: surf, Model: m, Prompt: p[0]}
			cases := []eval.Case{{ID: "c1"}, {ID: "c2"}}
			results := []eval.CaseResult{{CaseID: "c1", ToolMatch: true}}
			writePartialResults(path, results, cfg, cases)
		})

		AfterEach(func() {
			_ = os.Remove(path)
		})

		It("should write a partial JSON file with the partial flag", func() {
			data, readErr := os.ReadFile(path)
			Expect(readErr).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring(`"partial": true`))
			Expect(string(data)).To(ContainSubstring(`"completed": 1`))
			Expect(string(data)).To(ContainSubstring(`"total": 2`))
		})
	})
})

var _ = Describe("printRunConfig", func() {
	When("called with valid config", func() {
		It("should not panic", func() {
			surfaces := []eval.ToolSurface{{Label: "post"}}
			models := []eval.ModelConfig{{Name: "gemini-2.5-flash", PromptCostPer1M: 0.075, CompletionCostPer1M: 0.30}}
			prompts := []eval.PromptPreset{{Name: "minimal"}}
			cases := []eval.Case{{ID: "c1"}, {ID: "c2"}}

			Expect(func() {
				printRunConfig(cases, surfaces, models, prompts, "")
			}).NotTo(Panic())
		})

		It("should not panic when casesTag is set", func() {
			surfaces := []eval.ToolSurface{{Label: "post"}}
			models := []eval.ModelConfig{{Name: "gemini-2.5-flash", PromptCostPer1M: 0.075, CompletionCostPer1M: 0.30}}
			prompts := []eval.PromptPreset{{Name: "minimal"}}
			cases := []eval.Case{{ID: "c1", Tags: []string{"easy"}}}

			Expect(func() {
				printRunConfig(cases, surfaces, models, prompts, "easy")
			}).NotTo(Panic())
		})
	})
})

var _ = Describe("printSummaries", func() {
	When("called with zero summaries", func() {
		It("should not panic", func() {
			Expect(func() {
				printSummaries(nil)
			}).NotTo(Panic())
		})
	})

	When("called with one summary", func() {
		It("should not panic", func() {
			s := eval.ScoreSummary{
				ConfigLabel:    "post|gemini|minimal",
				CaseCount:      5,
				ToolMatchCount: 4,
				PrecisionAt1:   0.8,
				TotalCostUSD:   0.01,
			}
			Expect(func() {
				printSummaries([]eval.ScoreSummary{s})
			}).NotTo(Panic())
		})
	})

	When("called with two summaries", func() {
		It("should not panic", func() {
			s1 := eval.ScoreSummary{ConfigLabel: "pre|gemini|minimal", CaseCount: 5, ToolMatchCount: 3, PrecisionAt1: 0.6}
			s2 := eval.ScoreSummary{ConfigLabel: "post|gemini|minimal", CaseCount: 5, ToolMatchCount: 4, PrecisionAt1: 0.8}
			Expect(func() {
				printSummaries([]eval.ScoreSummary{s1, s2})
			}).NotTo(Panic())
		})
	})
})

var _ = Describe("printSingleSummary", func() {
	When("called with an empty PerTool map", func() {
		It("should not panic", func() {
			s := eval.ScoreSummary{
				ConfigLabel:           "post|gemini|minimal",
				CaseCount:             3,
				ToolMatchCount:        2,
				PrecisionAt1:          0.67,
				ExclusionOK:           1,
				ExclusionCount:        1,
				ExclusionRate:         1.0,
				AvgArgsMatch:          0.9,
				TotalCostUSD:          0.005,
				TotalPromptTokens:     1000,
				TotalCompletionTokens: 100,
			}
			Expect(func() { printSingleSummary(s) }).NotTo(Panic())
		})
	})

	When("called with a populated PerTool map", func() {
		It("should not panic", func() {
			s := eval.ScoreSummary{
				ConfigLabel:    "post|gemini|minimal",
				CaseCount:      3,
				ToolMatchCount: 2,
				PrecisionAt1:   0.67,
				PerTool: map[string]eval.ToolScore{
					"api_v1_PageManagementService_CreatePage": {Cases: 2, Correct: 2, Accuracy: 1.0},
					"api_v1_PageManagementService_DeletePage": {Cases: 1, Correct: 0, Accuracy: 0.0, SelectedTool: "other_tool"},
				},
			}
			Expect(func() { printSingleSummary(s) }).NotTo(Panic())
		})
	})
})

var _ = Describe("printToolBreakdown", func() {
	When("called with a mix of correct and incorrect tools", func() {
		It("should not panic", func() {
			s := eval.ScoreSummary{
				ConfigLabel: "post|gemini|minimal",
				PerTool: map[string]eval.ToolScore{
					"tool_a":           {Cases: 2, Correct: 2, Accuracy: 1.0},
					"tool_b":           {Cases: 1, Correct: 0, Accuracy: 0.0, SelectedTool: "tool_a"},
					"tool_exclusion_c": {Cases: 1, Correct: 1, Accuracy: 1.0, IsExclusion: true},
				},
			}
			Expect(func() { printToolBreakdown(s) }).NotTo(Panic())
		})
	})
})
