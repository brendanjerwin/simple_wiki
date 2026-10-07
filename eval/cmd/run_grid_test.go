//revive:disable:dot-imports
package main

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/eval"
)

var _ = Describe("runSingleConfig", func() {
	// Force an empty API key so callOpenRouter fails fast in-process and no
	// network call is ever attempted, regardless of the host environment.
	BeforeEach(func() {
		GinkgoT().Setenv("OPENROUTER_API_KEY", "")
	})

	When("no API key is set", func() {
		It("records a per-case error and returns a scored summary", func() {
			ctx := context.Background()
			cases := []eval.Case{{ID: "case-1", UserSays: "hello", ExpectedTool: "tool_a"}}
			surf := eval.ToolSurface{Label: "post"}
			model := eval.ModelConfig{Name: "test-model"}
			prompt := eval.PromptPreset{Name: "minimal"}

			summary, results := runSingleConfig(ctx, cases, surf, model, prompt, "")
			Expect(results).To(HaveLen(1))
			Expect(results[0].Error).To(ContainSubstring("OPENROUTER_API_KEY"))
			Expect(summary.CaseCount).To(Equal(1))
		})
	})

	When("an output file is provided", func() {
		It("writes partial results as the run progresses", func() {
			ctx := context.Background()
			cases := []eval.Case{{ID: "case-2", UserSays: "hello again", ExpectedTool: "tool_b"}}
			surf := eval.ToolSurface{Label: "post"}
			model := eval.ModelConfig{Name: "test-model"}
			prompt := eval.PromptPreset{Name: "minimal"}
			outFile := filepath.Join(GinkgoT().TempDir(), "partial.json")

			_, results := runSingleConfig(ctx, cases, surf, model, prompt, outFile)
			Expect(results).To(HaveLen(1))

			data, readErr := os.ReadFile(outFile)
			Expect(readErr).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring("case-2"))
		})
	})
})

var _ = Describe("runEvalGrid", func() {
	BeforeEach(func() {
		GinkgoT().Setenv("OPENROUTER_API_KEY", "")
	})

	When("given one surface, model, and prompt", func() {
		It("produces one summary per combination", func() {
			ctx := context.Background()
			cases := []eval.Case{{ID: "case-1", UserSays: "hello", ExpectedTool: "tool_a"}}
			surfaces := []eval.ToolSurface{{Label: "post"}}
			models := []eval.ModelConfig{{Name: "test-model"}}
			prompts := []eval.PromptPreset{{Name: "minimal"}}

			summaries, results := runEvalGrid(ctx, cases, surfaces, models, prompts, "")
			Expect(summaries).To(HaveLen(1))
			Expect(results).To(HaveLen(1))
			Expect(summaries[0].CaseCount).To(Equal(1))
		})
	})

	When("given an empty grid", func() {
		It("returns no summaries and no results", func() {
			ctx := context.Background()
			summaries, results := runEvalGrid(ctx, nil, nil, nil, nil, "")
			Expect(summaries).To(BeEmpty())
			Expect(results).To(BeEmpty())
		})
	})
})
