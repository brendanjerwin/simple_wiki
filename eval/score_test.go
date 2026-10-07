//revive:disable:dot-imports
package eval_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/eval"
)

var _ = Describe("Score", func() {
	var (
		cfg   eval.Config
		cases []eval.Case
	)

	BeforeEach(func() {
		cfg = eval.Config{
			Surface: eval.ToolSurface{Label: "post"},
			Model:   eval.ModelConfig{Name: "test-model"},
			Prompt:  eval.PromptPreset{Name: "minimal"},
		}
		cases = []eval.Case{
			{ID: "case-1", Services: []string{"ServiceA"}, ExpectedTool: "tool_A"},
			{ID: "case-2", Services: []string{"ServiceA", "ServiceB"}, ExpectedTool: "tool_B"},
			{ID: "case-3", ExcludedTool: "tool_bad", Services: []string{"ServiceB"}},
		}
	})

	When("all results are correct tool matches", func() {
		var summary eval.ScoreSummary

		BeforeEach(func() {
			results := []eval.CaseResult{
				{CaseID: "case-1", ExpectedTool: "tool_A", SelectedTool: "tool_A", ToolMatch: true},
				{CaseID: "case-2", ExpectedTool: "tool_B", SelectedTool: "tool_B", ToolMatch: true},
			}
			summary = eval.Score(results, cfg, cases)
		})

		It("should count cases correctly", func() {
			Expect(summary.CaseCount).To(Equal(2))
		})

		It("should count tool matches", func() {
			Expect(summary.ToolMatchCount).To(Equal(2))
		})

		It("should compute precision at 1 as 1.0", func() {
			Expect(summary.PrecisionAt1).To(BeNumerically("~", 1.0))
		})

		It("should build a config label", func() {
			Expect(summary.ConfigLabel).To(ContainSubstring("post"))
			Expect(summary.ConfigLabel).To(ContainSubstring("test-model"))
			Expect(summary.ConfigLabel).To(ContainSubstring("minimal"))
		})
	})

	When("some results are incorrect", func() {
		var summary eval.ScoreSummary

		BeforeEach(func() {
			results := []eval.CaseResult{
				{CaseID: "case-1", ExpectedTool: "tool_A", SelectedTool: "tool_A", ToolMatch: true},
				{CaseID: "case-2", ExpectedTool: "tool_B", SelectedTool: "tool_X", ToolMatch: false},
			}
			summary = eval.Score(results, cfg, cases)
		})

		It("should compute precision at 1 as 0.5", func() {
			Expect(summary.PrecisionAt1).To(BeNumerically("~", 0.5))
		})

		It("should count only the matching tool", func() {
			Expect(summary.ToolMatchCount).To(Equal(1))
		})
	})

	When("an exclusion result is OK", func() {
		var summary eval.ScoreSummary

		BeforeEach(func() {
			results := []eval.CaseResult{
				{CaseID: "case-3", ExcludedTool: "tool_bad", SelectedTool: "tool_other", ExclusionOK: true, ToolMatch: false},
			}
			summary = eval.Score(results, cfg, cases)
		})

		It("should count exclusion as a hit", func() {
			Expect(summary.ToolMatchCount).To(Equal(1))
		})

		It("should track exclusion stats", func() {
			Expect(summary.ExclusionCount).To(Equal(1))
			Expect(summary.ExclusionOK).To(Equal(1))
			Expect(summary.ExclusionRate).To(BeNumerically("~", 1.0))
		})
	})

	When("an exclusion result is not OK", func() {
		var summary eval.ScoreSummary

		BeforeEach(func() {
			results := []eval.CaseResult{
				{CaseID: "case-3", ExcludedTool: "tool_bad", SelectedTool: "tool_bad", ExclusionOK: false, ToolMatch: false},
			}
			summary = eval.Score(results, cfg, cases)
		})

		It("should not count the exclusion failure as a hit", func() {
			Expect(summary.ToolMatchCount).To(Equal(0))
		})

		It("should track exclusion rate as 0", func() {
			Expect(summary.ExclusionRate).To(BeNumerically("~", 0.0))
		})
	})

	When("results have args match scores", func() {
		var summary eval.ScoreSummary

		BeforeEach(func() {
			results := []eval.CaseResult{
				{CaseID: "case-1", ToolMatch: true, ArgsMatch: 0.8},
				{CaseID: "case-2", ToolMatch: true, ArgsMatch: 0.6},
			}
			summary = eval.Score(results, cfg, cases)
		})

		It("should average args match correctly", func() {
			Expect(summary.AvgArgsMatch).To(BeNumerically("~", 0.7, 0.001))
		})
	})

	When("results have cost and token data", func() {
		var summary eval.ScoreSummary

		BeforeEach(func() {
			results := []eval.CaseResult{
				{CaseID: "case-1", CostUSD: 0.001, PromptTokens: 100, CompletionTokens: 50},
				{CaseID: "case-2", CostUSD: 0.002, PromptTokens: 200, CompletionTokens: 75},
			}
			summary = eval.Score(results, cfg, cases)
		})

		It("should sum costs", func() {
			Expect(summary.TotalCostUSD).To(BeNumerically("~", 0.003, 0.0001))
		})

		It("should sum prompt tokens", func() {
			Expect(summary.TotalPromptTokens).To(Equal(300))
		})

		It("should sum completion tokens", func() {
			Expect(summary.TotalCompletionTokens).To(Equal(125))
		})
	})

	When("there are no results", func() {
		var summary eval.ScoreSummary

		BeforeEach(func() {
			summary = eval.Score(nil, cfg, cases)
		})

		It("should have zero CaseCount", func() {
			Expect(summary.CaseCount).To(Equal(0))
		})

		It("should have zero PrecisionAt1 (no division by zero)", func() {
			Expect(summary.PrecisionAt1).To(BeNumerically("~", 0.0))
		})
	})

	Describe("per-service breakdown", func() {
		When("results map to known cases with services", func() {
			var summary eval.ScoreSummary

			BeforeEach(func() {
				results := []eval.CaseResult{
					{CaseID: "case-1", ExpectedTool: "tool_A", ToolMatch: true},
					{CaseID: "case-2", ExpectedTool: "tool_B", ToolMatch: false},
				}
				summary = eval.Score(results, cfg, cases)
			})

			It("should track ServiceA accuracy", func() {
				ss := summary.PerService["ServiceA"]
				Expect(ss.Cases).To(Equal(2)) // case-1 and case-2 both involve ServiceA
				Expect(ss.Correct).To(Equal(1))
				Expect(ss.Accuracy).To(BeNumerically("~", 0.5))
			})

			It("should track ServiceB accuracy", func() {
				ss := summary.PerService["ServiceB"]
				Expect(ss.Cases).To(Equal(1)) // only case-2 involves ServiceB
				Expect(ss.Correct).To(Equal(0))
			})
		})
	})
})

var _ = Describe("CompareSummaries", func() {
	When("comparing two summaries", func() {
		var output string

		BeforeEach(func() {
			pre := eval.ScoreSummary{
				PrecisionAt1:          0.70,
				ExclusionRate:         0.80,
				AvgArgsMatch:          0.90,
				TotalCostUSD:          0.0500,
				TotalPromptTokens:     1000,
				TotalCompletionTokens: 200,
				PerService: map[string]eval.ServiceScore{
					"ServiceA": {Cases: 10, Correct: 7, Accuracy: 0.70},
				},
			}
			post := eval.ScoreSummary{
				PrecisionAt1:          0.80,
				ExclusionRate:         0.85,
				AvgArgsMatch:          0.92,
				TotalCostUSD:          0.0600,
				TotalPromptTokens:     1100,
				TotalCompletionTokens: 220,
				PerService: map[string]eval.ServiceScore{
					"ServiceA": {Cases: 10, Correct: 8, Accuracy: 0.80},
				},
			}
			output = eval.CompareSummaries(pre, post)
		})

		It("should include a Precision@1 row", func() {
			Expect(output).To(ContainSubstring("Precision@1"))
		})

		It("should include cost information", func() {
			Expect(output).To(ContainSubstring("Cost"))
		})

		It("should include per-service accuracy section", func() {
			Expect(output).To(ContainSubstring("Per-service accuracy"))
			Expect(output).To(ContainSubstring("ServiceA"))
		})

		It("should format as a markdown table with pipe separators", func() {
			Expect(output).To(ContainSubstring("|"))
		})
	})
})
