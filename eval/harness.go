package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// ModelConfig specifies which OpenRouter model to use and its cost per token.
type ModelConfig struct {
	ID                  string  `json:"id"`                     // OpenRouter model ID, e.g. "anthropic/claude-3.5-sonnet"
	Name                string  `json:"name"`                   // short display name, e.g. "claude-3.5-sonnet"
	PromptCostPer1M     float64 `json:"prompt_cost_per_1m"`     // USD per 1M prompt tokens
	CompletionCostPer1M float64 `json:"completion_cost_per_1m"` // USD per 1M completion tokens
}

// ModelPresets is the catalog of models the harness can use. Costs are
// approximate per OpenRouter pricing; update as pricing changes.
var ModelPresets = []ModelConfig{
	{
		ID:                  "google/gemini-2.5-flash",
		Name:                "gemini-2.5-flash",
		PromptCostPer1M:     0.075,
		CompletionCostPer1M: 0.30,
	},
	{
		ID:                  "anthropic/claude-3.5-sonnet",
		Name:                "claude-3.5-sonnet",
		PromptCostPer1M:     3.00,
		CompletionCostPer1M: 15.00,
	},
	{
		ID:                  "anthropic/claude-3.5-haiku",
		Name:                "claude-3.5-haiku",
		PromptCostPer1M:     0.80,
		CompletionCostPer1M: 4.00,
	},
	{
		ID:                  "openai/gpt-4o-mini",
		Name:                "gpt-4o-mini",
		PromptCostPer1M:     0.150,
		CompletionCostPer1M: 0.600,
	},
	{
		ID:                  "openai/gpt-4o",
		Name:                "gpt-4o",
		PromptCostPer1M:     2.50,
		CompletionCostPer1M: 10.00,
	},
}

// FindModelPreset returns the model config with the given short name, or nil.
func FindModelPreset(name string) *ModelConfig {
	for i := range ModelPresets {
		if ModelPresets[i].Name == name {
			return &ModelPresets[i]
		}
	}
	return nil
}

// Config is a single evaluation configuration: one point in the
// surface × model × prompt space.
type Config struct {
	Surface   ToolSurface  `json:"surface"`
	Model     ModelConfig  `json:"model"`
	Prompt    PromptPreset `json:"prompt"`
	MaxTokens int          `json:"max_tokens,omitempty"`
}

// CaseResult is the outcome of running one case under one Config.
type CaseResult struct {
	CaseID           string         `json:"case_id"`
	ConfigLabel      string         `json:"config_label"`
	SelectedTool     string         `json:"selected_tool"`
	SelectedArgs     map[string]any `json:"selected_args,omitempty"`
	ExpectedTool     string         `json:"expected_tool"`
	ExcludedTool     string         `json:"excluded_tool,omitempty"`
	ToolMatch        bool           `json:"tool_match"`
	ArgsMatch        float64        `json:"args_match"`
	ExclusionOK      bool           `json:"exclusion_ok,omitempty"`
	PromptTokens     int            `json:"prompt_tokens"`
	CompletionTokens int            `json:"completion_tokens"`
	CostUSD          float64        `json:"cost_usd"`
	LatencyMs        int            `json:"latency_ms"`
	Error            string         `json:"error,omitempty"`
}

// RunConfig runs all cases against one Config and returns the results.
func RunConfig(ctx context.Context, cases []Case, cfg Config) ([]CaseResult, error) {
	results := make([]CaseResult, 0, len(cases))
	configLabel := fmt.Sprintf("%s|%s|%s", cfg.Surface.Label, cfg.Model.Name, cfg.Prompt.Name)

	for _, c := range cases {
		result, err := runOneCase(ctx, c, cfg, configLabel)
		if err != nil {
			return results, fmt.Errorf("case %s: %w", c.ID, err)
		}
		results = append(results, result)
	}
	return results, nil
}

// runOneCase sends one case to OpenRouter and scores the response.
func runOneCase(ctx context.Context, c Case, cfg Config, configLabel string) (CaseResult, error) {
	result := CaseResult{
		CaseID:       c.ID,
		ConfigLabel:  configLabel,
		ExpectedTool: c.ExpectedTool,
		ExcludedTool: c.ExcludedTool,
	}

	// Build the user message: tool catalog + query
	userMsg := buildUserMessage(cfg.Surface, c.Query)

	start := time.Now()
	resp, err := callOpenRouter(ctx, cfg, cfg.Prompt.Prompt, userMsg)
	result.LatencyMs = int(time.Since(start).Milliseconds())
	if err != nil {
		result.Error = err.Error()
		return result, nil
	}

	result.PromptTokens = resp.Usage.PromptTokens
	result.CompletionTokens = resp.Usage.CompletionTokens
	result.CostUSD = computeCost(cfg.Model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)

	// Parse the LLM's JSON response
	parsed, parseErr := parseToolSelection(resp.Choices[0].Message.Content)
	if parseErr != nil {
		result.Error = fmt.Sprintf("parse error: %v (raw: %q)", parseErr, resp.Choices[0].Message.Content)
		return result, nil
	}

	result.SelectedTool = parsed.Tool
	result.SelectedArgs = parsed.Args

	// Score
	result.ToolMatch = parsed.Tool == c.ExpectedTool

	if c.ExcludedTool != "" {
		result.ExclusionOK = parsed.Tool != c.ExcludedTool
	}

	if len(c.ExpectedArgs) > 0 && parsed.Args != nil {
		result.ArgsMatch = argsMatchScore(c.ExpectedArgs, parsed.Args)
	} else if len(c.ExpectedArgs) == 0 {
		result.ArgsMatch = 1.0 // no args to check
	}

	return result, nil
}

// buildUserMessage assembles the tool catalog + user query into one message.
func buildUserMessage(surface ToolSurface, query string) string {
	// Compact the tool catalog: name + description only (drop inputSchema for now
	// to keep token costs manageable; can be toggled on later).
	tools := make([]map[string]string, len(surface.Tools))
	for i, t := range surface.Tools {
		tools[i] = map[string]string{
			"name":        t.Name,
			"description": t.Description,
		}
	}
	catalog, _ := json.Marshal(tools)
	return fmt.Sprintf(`Tool catalog:
%s

User request: %q

Select the single best tool and respond as JSON: {"tool": "<name>", "args": {...}}
If no tool is appropriate, respond: {"tool": null}`, string(catalog), query)
}

// toolSelection is the parsed JSON response from the LLM.
type toolSelection struct {
	Tool string         `json:"tool"`
	Args map[string]any `json:"args"`
}

// parseToolSelection extracts the tool selection from the LLM response.
func parseToolSelection(raw string) (toolSelection, error) {
	var sel toolSelection
	// Try to find JSON in the response (LLMs sometimes wrap it in markdown)
	jsonStr := extractJSON(raw)
	if err := json.Unmarshal([]byte(jsonStr), &sel); err != nil {
		return sel, fmt.Errorf("unmarshal %q: %w", jsonStr, err)
	}
	return sel, nil
}

// extractJSON finds the first { ... } block in a string, handling markdown fences.
func extractJSON(raw string) string {
	// Strip markdown code fences if present
	start := -1
	for i := 0; i < len(raw); i++ {
		if raw[i] == '{' {
			start = i
			break
		}
	}
	if start < 0 {
		return raw
	}
	// Find matching close brace (simple depth counting)
	depth := 0
	for i := start; i < len(raw); i++ {
		switch raw[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return raw[start : i+1]
			}
		}
	}
	return raw[start:]
}

// argsMatchScore returns the fraction of expected arg keys present and correct.
func argsMatchScore(expected, actual map[string]any) float64 {
	if len(expected) == 0 {
		return 1.0
	}
	matched := 0
	for key, want := range expected {
		if got, ok := actual[key]; ok {
			if fmt.Sprintf("%v", got) == fmt.Sprintf("%v", want) {
				matched++
			}
		}
	}
	return float64(matched) / float64(len(expected))
}

// computeCost calculates the USD cost of one LLM call.
func computeCost(model ModelConfig, promptTokens, completionTokens int) float64 {
	return float64(promptTokens)*model.PromptCostPer1M/1_000_000 +
		float64(completionTokens)*model.CompletionCostPer1M/1_000_000
}

// openRouterResponse is the response shape from OpenRouter's chat completions API.
type openRouterResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int     `json:"prompt_tokens"`
		CompletionTokens int     `json:"completion_tokens"`
		TotalTokens      int     `json:"total_tokens"`
		Cost             float64 `json:"cost,omitempty"`
	} `json:"usage"`
}

// callOpenRouter sends a chat completion request to OpenRouter.
func callOpenRouter(ctx context.Context, cfg Config, systemPrompt, userMessage string) (*openRouterResponse, error) {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENROUTER_API_KEY not set")
	}

	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 1024
	}

	body := map[string]any{
		"model": cfg.Model.ID,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userMessage},
		},
		"temperature": 0,
		"max_tokens":  maxTokens,
	}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("HTTP-Referer", "https://github.com/brendanjerwin/simple_wiki")
	req.Header.Set("X-Title", "simple-wiki-mcp-eval")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openrouter %d: %s", resp.StatusCode, string(respBytes))
	}

	var orResp openRouterResponse
	if err := json.Unmarshal(respBytes, &orResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	if len(orResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &orResp, nil
}
