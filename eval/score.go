package eval

import (
	"fmt"
	"sort"
	"strings"
)

// ScoreSummary aggregates CaseResults into metrics for one Config.
type ScoreSummary struct {
	ConfigLabel           string                  `json:"config_label"`
	SurfaceLabel          string                  `json:"surface_label"`
	ModelName             string                  `json:"model_name"`
	PromptName            string                  `json:"prompt_name"`
	CaseCount             int                     `json:"case_count"`
	ToolMatchCount        int                     `json:"tool_match_count"`
	PrecisionAt1          float64                 `json:"precision_at_1"`
	ExclusionCount        int                     `json:"exclusion_count"`
	ExclusionOK           int                     `json:"exclusion_ok"`
	ExclusionRate         float64                 `json:"exclusion_rate"`
	AvgArgsMatch          float64                 `json:"avg_args_match"`
	TotalCostUSD          float64                 `json:"total_cost_usd"`
	TotalPromptTokens     int                     `json:"total_prompt_tokens"`
	TotalCompletionTokens int                     `json:"total_completion_tokens"`
	PerService            map[string]ServiceScore `json:"per_service"`
	PerTool               map[string]ToolScore    `json:"per_tool"`
}

// ToolScore is the breakdown for one expected tool.
type ToolScore struct {
	Cases        int     `json:"cases"`
	Correct      int     `json:"correct"`
	Accuracy     float64 `json:"accuracy"`
	SelectedTool string  `json:"selected_tool,omitempty"` // what the model picked (for failures)
	IsExclusion  bool    `json:"is_exclusion,omitempty"`
}

// ServiceScore is the breakdown for one service.
type ServiceScore struct {
	Cases    int     `json:"cases"`
	Correct  int     `json:"correct"`
	Accuracy float64 `json:"accuracy"`
}

// Score aggregates CaseResults into a ScoreSummary.
func Score(results []CaseResult, cfg Config, cases []Case) ScoreSummary {
	s := ScoreSummary{
		ConfigLabel:  fmt.Sprintf("%s|%s|%s", cfg.Surface.Label, cfg.Model.Name, cfg.Prompt.Name),
		SurfaceLabel: cfg.Surface.Label,
		ModelName:    cfg.Model.Name,
		PromptName:   cfg.Prompt.Name,
		CaseCount:    len(results),
		PerService:   make(map[string]ServiceScore),
		PerTool:      make(map[string]ToolScore),
	}

	caseByID := make(map[string]Case, len(cases))
	for _, c := range cases {
		caseByID[c.ID] = c
	}

	argsSum := 0.0
	argsCount := 0

	for _, r := range results {
		// For exclusion cases: avoiding the excluded tool counts as a hit.
		// The model can't "decline" in a meaningful way when there are 105
		// tools — picking a plausible non-excluded alternative is the right
		// behavior, not a failure.
		isHit := r.ToolMatch
		if r.ExcludedTool != "" && r.ExclusionOK && !r.ToolMatch {
			isHit = true
		}
		if isHit {
			s.ToolMatchCount++
		}
		if r.ExcludedTool != "" {
			s.ExclusionCount++
			if r.ExclusionOK {
				s.ExclusionOK++
			}
		}
		if r.ArgsMatch > 0 {
			argsSum += r.ArgsMatch
			argsCount++
		}
		s.TotalCostUSD += r.CostUSD
		s.TotalPromptTokens += r.PromptTokens
		s.TotalCompletionTokens += r.CompletionTokens
		// Per-service breakdown
		if c, ok := caseByID[r.CaseID]; ok {
			for _, svc := range c.Services {
				ss := s.PerService[svc]
				ss.Cases++
				if r.ToolMatch || (r.ExcludedTool != "" && r.ExclusionOK) {
					ss.Correct++
				}
				if ss.Cases > 0 {
					ss.Accuracy = float64(ss.Correct) / float64(ss.Cases)
				}
				s.PerService[svc] = ss
			}
		}

		// Per-tool breakdown (keyed by expected tool, or excluded tool for exclusion cases)
		toolKey := r.ExpectedTool
		if toolKey == "" && r.ExcludedTool != "" {
			toolKey = r.ExcludedTool
		}
		if toolKey != "" {
			ts := s.PerTool[toolKey]
			ts.Cases++
			if r.ToolMatch || (r.ExcludedTool != "" && r.ExclusionOK) {
				ts.Correct++
			}
			if !r.ToolMatch && r.SelectedTool != "" {
				ts.SelectedTool = r.SelectedTool // capture what the model picked instead
			}
			ts.IsExclusion = r.ExcludedTool != ""
			if ts.Cases > 0 {
				ts.Accuracy = float64(ts.Correct) / float64(ts.Cases)
			}
			s.PerTool[toolKey] = ts
		}
	}

	if s.CaseCount > 0 {
		s.PrecisionAt1 = float64(s.ToolMatchCount) / float64(s.CaseCount)
	}
	if s.ExclusionCount > 0 {
		s.ExclusionRate = float64(s.ExclusionOK) / float64(s.ExclusionCount)
	}
	if argsCount > 0 {
		s.AvgArgsMatch = argsSum / float64(argsCount)
	}

	return s
}

// CompareSummaries produces a markdown table comparing two ScoreSummaries.
func CompareSummaries(pre, post ScoreSummary) string {
	var b strings.Builder
	b.WriteString("| Metric | Pre-PR | Post-PR | Delta |\n")
	b.WriteString("|---|---|---|---|\n")
	b.WriteString(fmt.Sprintf("| Precision@1 | %.1f%% | %.1f%% | %+.1f%% |\n",
		pre.PrecisionAt1*100, post.PrecisionAt1*100, (post.PrecisionAt1-pre.PrecisionAt1)*100))
	b.WriteString(fmt.Sprintf("| Exclusion rate | %.1f%% | %.1f%% | %+.1f%% |\n",
		pre.ExclusionRate*100, post.ExclusionRate*100, (post.ExclusionRate-pre.ExclusionRate)*100))
	b.WriteString(fmt.Sprintf("| Avg args match | %.1f%% | %.1f%% | %+.1f%% |\n",
		pre.AvgArgsMatch*100, post.AvgArgsMatch*100, (post.AvgArgsMatch-pre.AvgArgsMatch)*100))
	b.WriteString(fmt.Sprintf("| Cost (USD) | $%.4f | $%.4f | $%.4f |\n",
		pre.TotalCostUSD, post.TotalCostUSD, post.TotalCostUSD-pre.TotalCostUSD))
	b.WriteString(fmt.Sprintf("| Prompt tokens | %d | %d | %+d |\n",
		pre.TotalPromptTokens, post.TotalPromptTokens, post.TotalPromptTokens-pre.TotalPromptTokens))
	b.WriteString(fmt.Sprintf("| Completion tokens | %d | %d | %+d |\n",
		pre.TotalCompletionTokens, post.TotalCompletionTokens, post.TotalCompletionTokens-pre.TotalCompletionTokens))

	// Per-service breakdown
	b.WriteString("\n### Per-service accuracy\n\n")
	b.WriteString("| Service | Pre-PR | Post-PR | Delta |\n")
	b.WriteString("|---|---|---|---|\n")
	services := make([]string, 0, len(pre.PerService))
	for svc := range pre.PerService {
		services = append(services, svc)
	}
	sort.Strings(services)
	for _, svc := range services {
		preS := pre.PerService[svc]
		postS, ok := post.PerService[svc]
		if !ok {
			postS = ServiceScore{}
		}
		b.WriteString(fmt.Sprintf("| %s | %.1f%% (%d/%d) | %.1f%% (%d/%d) | %+.1f%% |\n",
			svc,
			preS.Accuracy*100, preS.Correct, preS.Cases,
			postS.Accuracy*100, postS.Correct, postS.Cases,
			(postS.Accuracy-preS.Accuracy)*100))
	}

	return b.String()
}
