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

	caseByID := indexCasesByID(cases)
	argsSum, argsCount := 0.0, 0

	for _, r := range results {
		accumulateResultCounts(&s, r, &argsSum, &argsCount)
		if c, ok := caseByID[r.CaseID]; ok {
			accumulateServiceBreakdown(&s, r, c.Services)
		}
		accumulateToolBreakdown(&s, r)
	}

	finalizeRates(&s, argsSum, argsCount)
	return s
}

// indexCasesByID builds a lookup map from case ID to Case.
func indexCasesByID(cases []Case) map[string]Case {
	m := make(map[string]Case, len(cases))
	for _, c := range cases {
		m[c.ID] = c
	}
	return m
}

// accumulateResultCounts updates top-level counters from one result.
func accumulateResultCounts(s *ScoreSummary, r CaseResult, argsSum *float64, argsCount *int) {
	// For exclusion cases: avoiding the excluded tool counts as a hit.
	isHit := r.ToolMatch || (r.ExcludedTool != "" && r.ExclusionOK && !r.ToolMatch)
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
		*argsSum += r.ArgsMatch
		*argsCount++
	}
	s.TotalCostUSD += r.CostUSD
	s.TotalPromptTokens += r.PromptTokens
	s.TotalCompletionTokens += r.CompletionTokens
}

// accumulateServiceBreakdown updates per-service scores for a result.
func accumulateServiceBreakdown(s *ScoreSummary, r CaseResult, services []string) {
	isCorrect := r.ToolMatch || (r.ExcludedTool != "" && r.ExclusionOK)
	for _, svc := range services {
		ss := s.PerService[svc]
		ss.Cases++
		if isCorrect {
			ss.Correct++
		}
		if ss.Cases > 0 {
			ss.Accuracy = float64(ss.Correct) / float64(ss.Cases)
		}
		s.PerService[svc] = ss
	}
}

// accumulateToolBreakdown updates per-tool scores for a result.
func accumulateToolBreakdown(s *ScoreSummary, r CaseResult) {
	toolKey := r.ExpectedTool
	if toolKey == "" && r.ExcludedTool != "" {
		toolKey = r.ExcludedTool
	}
	if toolKey == "" {
		return
	}
	ts := s.PerTool[toolKey]
	ts.Cases++
	if r.ToolMatch || (r.ExcludedTool != "" && r.ExclusionOK) {
		ts.Correct++
	}
	if !r.ToolMatch && r.SelectedTool != "" {
		ts.SelectedTool = r.SelectedTool
	}
	ts.IsExclusion = r.ExcludedTool != ""
	if ts.Cases > 0 {
		ts.Accuracy = float64(ts.Correct) / float64(ts.Cases)
	}
	s.PerTool[toolKey] = ts
}

// finalizeRates computes derived rate fields after all results are accumulated.
func finalizeRates(s *ScoreSummary, argsSum float64, argsCount int) {
	if s.CaseCount > 0 {
		s.PrecisionAt1 = float64(s.ToolMatchCount) / float64(s.CaseCount)
	}
	if s.ExclusionCount > 0 {
		s.ExclusionRate = float64(s.ExclusionOK) / float64(s.ExclusionCount)
	}
	if argsCount > 0 {
		s.AvgArgsMatch = argsSum / float64(argsCount)
	}
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
