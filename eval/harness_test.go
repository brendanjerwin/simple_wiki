//revive:disable:dot-imports
package eval_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/eval"
)

var _ = Describe("FindModelPreset", func() {
	When("the model name exists", func() {
		It("should return the matching preset", func() {
			m := eval.FindModelPreset("gemini-2.5-flash")
			Expect(m).NotTo(BeNil())
			Expect(m.Name).To(Equal("gemini-2.5-flash"))
		})
	})

	When("the model name does not exist", func() {
		It("should return nil", func() {
			m := eval.FindModelPreset("nonexistent-model")
			Expect(m).To(BeNil())
		})
	})
})

var _ = Describe("computeCost", func() {
	When("computing cost for a known model", func() {
		It("should compute prompt cost correctly", func() {
			model := eval.ModelConfig{PromptCostPer1M: 1.0, CompletionCostPer1M: 0.0}
			cost := eval.ComputeCost(model, 1_000_000, 0)
			Expect(cost).To(BeNumerically("~", 1.0, 0.0001))
		})

		It("should compute completion cost correctly", func() {
			model := eval.ModelConfig{PromptCostPer1M: 0.0, CompletionCostPer1M: 2.0}
			cost := eval.ComputeCost(model, 0, 1_000_000)
			Expect(cost).To(BeNumerically("~", 2.0, 0.0001))
		})

		It("should sum both prompt and completion costs", func() {
			model := eval.ModelConfig{PromptCostPer1M: 3.0, CompletionCostPer1M: 15.0}
			cost := eval.ComputeCost(model, 1000, 500)
			Expect(cost).To(BeNumerically("~", 0.003+0.0075, 0.000001))
		})

		It("should return zero when tokens are zero", func() {
			model := eval.ModelConfig{PromptCostPer1M: 3.0, CompletionCostPer1M: 15.0}
			cost := eval.ComputeCost(model, 0, 0)
			Expect(cost).To(BeNumerically("~", 0.0))
		})
	})
})

var _ = Describe("extractJSON", func() {
	When("the raw string is a plain JSON object", func() {
		It("should return the JSON unchanged", func() {
			raw := `{"tool": "foo", "args": {}}`
			result := eval.ExtractJSON(raw)
			Expect(result).To(Equal(`{"tool": "foo", "args": {}}`))
		})
	})

	When("the JSON is wrapped in markdown code fences", func() {
		It("should extract just the JSON object", func() {
			raw := "```json\n{\"tool\": \"foo\"}\n```"
			result := eval.ExtractJSON(raw)
			Expect(result).To(Equal(`{"tool": "foo"}`))
		})
	})

	When("there is preamble text before the JSON", func() {
		It("should find and return the JSON block", func() {
			raw := `Here is the result: {"tool": "bar", "args": {"page": "test"}}`
			result := eval.ExtractJSON(raw)
			Expect(result).To(Equal(`{"tool": "bar", "args": {"page": "test"}}`))
		})
	})

	When("there is no JSON object", func() {
		It("should return the original string", func() {
			raw := "no JSON here"
			result := eval.ExtractJSON(raw)
			Expect(result).To(Equal("no JSON here"))
		})
	})

	When("the JSON contains nested objects", func() {
		It("should extract the complete nested structure", func() {
			raw := `{"tool": "create", "args": {"nested": {"key": "val"}}}`
			result := eval.ExtractJSON(raw)
			Expect(result).To(Equal(`{"tool": "create", "args": {"nested": {"key": "val"}}}`))
		})
	})
})

var _ = Describe("parseToolSelection", func() {
	When("the response is valid JSON", func() {
		It("should parse the tool name", func() {
			sel, err := eval.ParseToolSelection(`{"tool": "api_v1_PageManagementService_CreatePage", "args": {"page": "test"}}`)
			Expect(err).NotTo(HaveOccurred())
			Expect(sel.Tool).To(Equal("api_v1_PageManagementService_CreatePage"))
		})

		It("should parse the args", func() {
			sel, err := eval.ParseToolSelection(`{"tool": "create", "args": {"page": "mypage"}}`)
			Expect(err).NotTo(HaveOccurred())
			Expect(sel.Args).To(HaveKey("page"))
		})
	})

	When("the response has a null tool", func() {
		It("should parse without error", func() {
			sel, err := eval.ParseToolSelection(`{"tool": null}`)
			Expect(err).NotTo(HaveOccurred())
			Expect(sel.Tool).To(Equal(""))
		})
	})

	When("the response is not valid JSON", func() {
		It("should return an error", func() {
			_, err := eval.ParseToolSelection("not json at all")
			Expect(err).To(HaveOccurred())
		})
	})
})

var _ = Describe("argsMatchScore", func() {
	When("expected args is empty", func() {
		It("should return 1.0 (no args to check)", func() {
			score := eval.ArgsMatchScore(nil, map[string]any{"extra": "value"})
			Expect(score).To(BeNumerically("~", 1.0))
		})
	})

	When("all expected args match", func() {
		It("should return 1.0", func() {
			expected := map[string]any{"page": "test", "title": "Test"}
			actual := map[string]any{"page": "test", "title": "Test"}
			score := eval.ArgsMatchScore(expected, actual)
			Expect(score).To(BeNumerically("~", 1.0))
		})
	})

	When("no expected args match", func() {
		It("should return 0.0", func() {
			expected := map[string]any{"page": "test"}
			actual := map[string]any{"page": "other"}
			score := eval.ArgsMatchScore(expected, actual)
			Expect(score).To(BeNumerically("~", 0.0))
		})
	})

	When("half of expected args match", func() {
		It("should return 0.5", func() {
			expected := map[string]any{"page": "test", "title": "Hello"}
			actual := map[string]any{"page": "test", "title": "Wrong"}
			score := eval.ArgsMatchScore(expected, actual)
			Expect(score).To(BeNumerically("~", 0.5))
		})
	})

	When("a key is missing from actual", func() {
		It("should not count the missing key", func() {
			expected := map[string]any{"page": "test", "title": "Hello"}
			actual := map[string]any{"page": "test"}
			score := eval.ArgsMatchScore(expected, actual)
			Expect(score).To(BeNumerically("~", 0.5))
		})
	})
})

var _ = Describe("buildUserMessage", func() {
	var surface eval.ToolSurface

	BeforeEach(func() {
		surface = eval.ToolSurface{
			Label: "post",
			Tools: []eval.ToolDef{
				{Name: "tool_A", Description: "Does something"},
				{Name: "tool_B", Description: "Does another thing"},
			},
		}
	})

	When("no page context is provided", func() {
		var msg string

		BeforeEach(func() {
			msg = eval.BuildUserMessage(surface, "create a page", "")
		})

		It("should include the tool catalog", func() {
			Expect(msg).To(ContainSubstring("tool_A"))
			Expect(msg).To(ContainSubstring("tool_B"))
		})

		It("should include the user request", func() {
			Expect(msg).To(ContainSubstring("create a page"))
		})

		It("should not include the page context section", func() {
			Expect(msg).NotTo(ContainSubstring("wiki page"))
		})
	})

	When("page context is provided", func() {
		var msg string

		BeforeEach(func() {
			msg = eval.BuildUserMessage(surface, "update the title", "Page: mypage\nTitle: My Page")
		})

		It("should include the page context", func() {
			Expect(msg).To(ContainSubstring("wiki page"))
			Expect(msg).To(ContainSubstring("Page: mypage"))
		})

		It("should include the user request", func() {
			Expect(msg).To(ContainSubstring("update the title"))
		})
	})
})
