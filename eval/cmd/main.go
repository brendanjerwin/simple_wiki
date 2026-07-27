// Command eval runs the MCP tool-discovery evaluation harness.
//
// Usage:
//
//	eval --surface=post --model=gemini-2.5-flash --prompt=minimal
//	eval --compare=pre,post --model=gemini-2.5-flash --prompt=production
//	eval --surface=post --prompt=production --sweep=model:gemini-2.5-flash,claude-3.5-sonnet
//	eval --surface=post --model=gemini-2.5-flash --sweep=prompt:minimal,production,catalog-hint
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/brendanjerwin/simple_wiki/eval"
)

func main() {
	wikiURL := flag.String("wiki-url", "http://localhost:8050", "wiki base URL for live tool surface")
	surface := flag.String("surface", "", "single surface to eval: 'post' (live) or 'pre' (reconstructed from post)")
	compare := flag.String("compare", "", "comma-separated surfaces to compare, e.g. 'pre,post'")
	model := flag.String("model", "gemini-2.5-flash", "OpenRouter model short name")
	prompt := flag.String("prompt", "minimal", "system prompt preset name")
	sweepModel := flag.String("sweep-model", "", "comma-separated model names to sweep")
	sweepPrompt := flag.String("sweep-prompt", "", "comma-separated prompt names to sweep")
	casesTag := flag.String("cases", "", "filter cases by tag (empty = all)")
	dryRun := flag.Bool("dry-run", false, "print the configuration and estimated cost without calling the LLM")
	outFile := flag.String("out", "", "write JSON results to this file")
	flag.Parse()

	if os.Getenv("OPENROUTER_API_KEY") == "" && !*dryRun {
		log.Fatal("OPENROUTER_API_KEY not set — required for LLM calls (use --dry-run to preview without calls)")
	}

	// Fetch the live tool surface once
	log.Printf("Fetching tool surface from %s/mcp ...", *wikiURL)
	liveSurface, err := eval.FetchSurface(*wikiURL)
	if err != nil {
		log.Fatalf("fetch surface: %v", err)
	}
	log.Printf("Got %d tools from live wiki", liveSurface.ToolCount())

	// Determine which surfaces to run
	var surfaces []eval.ToolSurface
	if *compare != "" {
		parts := strings.Split(*compare, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			switch p {
			case "post":
				surfaces = append(surfaces, liveSurface)
			case "pre":
				surfaces = append(surfaces, eval.ToPrePR(liveSurface))
			default:
				log.Fatalf("unknown surface label %q (use 'pre' or 'post')", p)
			}
		}
	} else if *surface != "" {
		switch *surface {
		case "post":
			surfaces = append(surfaces, liveSurface)
		case "pre":
			surfaces = append(surfaces, eval.ToPrePR(liveSurface))
		default:
			log.Fatalf("unknown surface %q (use 'pre' or 'post')", *surface)
		}
	} else {
		// default: post only
		surfaces = append(surfaces, liveSurface)
	}

	// Determine which models to run
	var models []eval.ModelConfig
	if *sweepModel != "" {
		for _, name := range strings.Split(*sweepModel, ",") {
			name = strings.TrimSpace(name)
			m := eval.FindModelPreset(name)
			if m == nil {
				log.Fatalf("unknown model %q", name)
			}
			models = append(models, *m)
		}
	} else {
		m := eval.FindModelPreset(*model)
		if m == nil {
			log.Fatalf("unknown model %q", *model)
		}
		models = append(models, *m)
	}

	// Determine which prompts to run
	var prompts []eval.PromptPreset
	if *sweepPrompt != "" {
		for _, name := range strings.Split(*sweepPrompt, ",") {
			name = strings.TrimSpace(name)
			p := eval.FindPromptPreset(name)
			if p == nil {
				log.Fatalf("unknown prompt %q", name)
			}
			prompts = append(prompts, *p)
		}
	} else {
		p := eval.FindPromptPreset(*prompt)
		if p == nil {
			log.Fatalf("unknown prompt %q", *prompt)
		}
		prompts = append(prompts, *p)
	}

	// Filter cases by tag
	cases := eval.Cases
	if *casesTag != "" {
		cases = filterByTag(cases, *casesTag)
	}

	// Print configuration
	totalRuns := len(surfaces) * len(models) * len(prompts) * len(cases)
	fmt.Printf("\n=== MCP Tool Discovery Eval ===\n")
	fmt.Printf("  Surfaces:  %d (%s)\n", len(surfaces), labels(surfaces))
	fmt.Printf("  Models:    %d (%s)\n", len(models), modelNames(models))
	fmt.Printf("  Prompts:   %d (%s)\n", len(prompts), promptNames(prompts))
	fmt.Printf("  Cases:     %d", len(cases))
	if *casesTag != "" {
		fmt.Printf(" (tag: %s)", *casesTag)
	}
	fmt.Printf("\n  Total LLM calls: %d\n", totalRuns)

	// Estimate cost
	estCost := estimateCost(cases, surfaces, models, prompts)
	fmt.Printf("  Estimated cost: $%.4f\n\n", estCost)

	if *dryRun {
		fmt.Println("--dry-run: skipping LLM calls")
		return
	}

	// Run the grid
	ctx := context.Background()
	var allSummaries []eval.ScoreSummary
	var allResults []eval.CaseResult

	for _, surf := range surfaces {
		for _, m := range models {
			for _, p := range prompts {
				cfg := eval.Config{
					Surface: surf,
					Model:   m,
					Prompt:  p,
				}
				label := fmt.Sprintf("%s | %s | %s", surf.Label, m.Name, p.Name)
				log.Printf("Running %d cases for %s ...", len(cases), label)

				var results []eval.CaseResult
				var runErr error
				results, runErr = eval.RunConfig(ctx, cases, cfg, func(rs []eval.CaseResult, completed int) {
					if (completed+1)%10 == 0 || completed+1 == len(cases) {
						hits := 0
						for _, r := range rs {
							if r.ToolMatch || (r.ExcludedTool != "" && r.ExclusionOK) {
								hits++
							}
						}
						log.Printf("  %s: %d/%d done (%d hits, %d errs)",
							label, completed+1, len(cases), hits, countErrors(rs))
						if *outFile != "" {
							writePartialResults(*outFile, rs, cfg, cases)
						}
					}
				})
				if runErr != nil {
					log.Printf("  ERROR: %v", runErr)
				}
				summary := eval.Score(results, cfg, cases)
				allSummaries = append(allSummaries, summary)
				allResults = append(allResults, results...)

				fmt.Printf("  %s: P@1=%.1f%% (%d/%d) cost=$%.4f tokens=%d+%d\n",
					label,
					summary.PrecisionAt1*100,
					summary.ToolMatchCount, summary.CaseCount,
					summary.TotalCostUSD,
					summary.TotalPromptTokens, summary.TotalCompletionTokens)
			}
		}
	}

	// Print comparison table if multiple configs
	fmt.Println()
	if len(allSummaries) >= 2 {
		// Compare first two
		fmt.Println(eval.CompareSummaries(allSummaries[0], allSummaries[1]))
	} else if len(allSummaries) == 1 {
		s := allSummaries[0]
		fmt.Printf("=== Results: %s ===\n", s.ConfigLabel)
		fmt.Printf("Precision@1:    %.1f%% (%d/%d)\n", s.PrecisionAt1*100, s.ToolMatchCount, s.CaseCount)
		fmt.Printf("Exclusion rate:  %.1f%% (%d/%d)\n", s.ExclusionRate*100, s.ExclusionOK, s.ExclusionCount)
		fmt.Printf("Avg args match:  %.1f%%\n", s.AvgArgsMatch*100)
		fmt.Printf("Cost:            $%.4f\n", s.TotalCostUSD)
		fmt.Printf("Tokens:          %d prompt + %d completion = %d total\n",
			s.TotalPromptTokens, s.TotalCompletionTokens,
			s.TotalPromptTokens+s.TotalCompletionTokens)

		// Per-tool breakdown: failures first, sorted by accuracy ascending
		if len(s.PerTool) > 0 {
			fmt.Printf("\n=== Per-tool breakdown (failures first) ===\n")
			type toolEntry struct {
				Name  string
				Score eval.ToolScore
			}
			entries := make([]toolEntry, 0, len(s.PerTool))
			for name, ts := range s.PerTool {
				entries = append(entries, toolEntry{name, ts})
			}
			sort.Slice(entries, func(i, j int) bool {
				return entries[i].Score.Accuracy < entries[j].Score.Accuracy
			})
			fmt.Printf("%-55s %8s %8s %8s %s\n", "Tool", "Acc", "Correct", "Cases", "Misselected")
			fmt.Println(strings.Repeat("-", 100))
			for _, e := range entries {
				misselect := ""
				if e.Score.SelectedTool != "" {
					misselect = "→ " + e.Score.SelectedTool
				}
				if e.Score.IsExclusion {
					misselect = "(exclusion)"
				}
				status := "✓"
				if e.Score.Accuracy < 1.0 {
					status = "✗"
				}
				fmt.Printf("%s %-53s %7.1f%% %5d/%d  %s\n",
					status, e.Name, e.Score.Accuracy*100, e.Score.Correct, e.Score.Cases, misselect)
			}
		}
	}

	// Write results file
	if *outFile != "" {
		writeResults(*outFile, allSummaries, allResults)
		log.Printf("Results written to %s", *outFile)
	}

	_ = time.Now // keep import alive if no timer usage
}

func filterByTag(cases []eval.Case, tag string) []eval.Case {
	var out []eval.Case
	for _, c := range cases {
		for _, t := range c.Tags {
			if t == tag {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

func countErrors(results []eval.CaseResult) int {
	count := 0
	for _, r := range results {
		if r.Error != "" {
			count++
		}
	}
	return count
}

func writePartialResults(path string, results []eval.CaseResult, cfg eval.Config, cases []eval.Case) {
	summary := eval.Score(results, cfg, cases)
	data := map[string]any{
		"summaries": []eval.ScoreSummary{summary},
		"results":   results,
		"timestamp": time.Now().Format(time.RFC3339),
		"partial":   true,
		"completed": len(results),
		"total":     len(cases),
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o644)
}

func labels(surfaces []eval.ToolSurface) string {
	parts := make([]string, len(surfaces))
	for i, s := range surfaces {
		parts[i] = s.Label
	}
	return strings.Join(parts, ", ")
}

func modelNames(models []eval.ModelConfig) string {
	parts := make([]string, len(models))
	for i, m := range models {
		parts[i] = m.Name
	}
	return strings.Join(parts, ", ")
}

func promptNames(prompts []eval.PromptPreset) string {
	parts := make([]string, len(prompts))
	for i, p := range prompts {
		parts[i] = p.Name
	}
	return strings.Join(parts, ", ")
}

// estimateCost computes a rough cost estimate for the full run.
// Assumes ~2k prompt tokens per call (tool catalog + query) and ~50 completion tokens.
func estimateCost(cases []eval.Case, surfaces []eval.ToolSurface, models []eval.ModelConfig, prompts []eval.PromptPreset) float64 {
	// Rough token estimate: tool catalog is ~20k chars → ~5k tokens; query ~50 tokens; system prompt ~200 tokens
	estPromptTokens := 5250
	estCompletionTokens := 50
	totalCalls := len(cases) * len(surfaces) * len(models) * len(prompts)

	var cost float64
	for _, m := range models {
		callsPerModel := len(cases) * len(surfaces) * len(prompts)
		cost += float64(callsPerModel) * (float64(estPromptTokens)*m.PromptCostPer1M/1_000_000 +
			float64(estCompletionTokens)*m.CompletionCostPer1M/1_000_000)
	}
	_ = totalCalls
	return cost
}

func writeResults(path string, summaries []eval.ScoreSummary, results []eval.CaseResult) {
	data := map[string]any{
		"summaries": summaries,
		"results":   results,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Printf("marshal results: %v", err)
		return
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		log.Printf("write results: %v", err)
	}
}
