# MCP Tool Discovery Evaluation Framework

## Goal

Measure how well an LLM tool-selector can map natural-language user requests
to the correct MCP tool, given the wiki's `tools/list` surface. Use this to:

1. **Quantify the improve-mcp-surface PR's value** (pre-PR vs post-PR comparison).
2. **Establish a baseline** so future renames (F6/issue #1151) can measure their
   impact before merging.
3. **Compare models and system prompts** to find the configuration that maximizes
   tool-discovery accuracy.

## What we're measuring

**Tool selection accuracy**: given a user request in natural language, does
the selector pick the correct MCP tool from the `tools/list` catalog?

Secondary metrics:
- **Argument construction**: given the correct tool, does the selector emit
  valid arguments (right field names, right types)?
- **Exclusion precision**: does the selector correctly avoid excluded/pool-only
  tools?

## Three independent axes

The eval is a function of three variables. Each can be swept independently:

```
accuracy = f(surface, model, system_prompt)
```

| Axis | What it varies | Why |
|---|---|---|
| **Surface** | pre-PR (stubs, empty desc, excluded tools present) vs post-PR (curated, clean) | Quantifies the PR; future branches (renames) plug in here |
| **Model** | `smol`, `default`, `slow`, or any named model the `completion` tool supports | Find which models handle 105-tool catalogs well; compare cost/accuracy tradeoffs |
| **System prompt** | the preamble the selector sees before the tool catalog | Test the production chatPreamble vs a minimal prompt vs a tuned variant |

A single eval run fixes all three. A **sweep** varies one axis while holding the
others constant. Results are a cube: surface × model × prompt → score.

### System prompts

The harness supports named system-prompt presets so they can be compared:

| Preset | Content |
|---|---|
| `minimal` | "Select the best MCP tool for the user's request." |
| `production` | The real `chatPreamble` from `cmd/wiki-cli/pool.go` (the "Discovering what you can do" section + tool-use rules) |
| `production-no-discovery` | The production preamble with the "Discovering what you can do" section stripped — tests whether that section helps |
| `catalog-hint` | Minimal + "A machine-readable service catalog is available at /mcp/catalog" — tests whether pointing agents at the catalog helps |

Custom prompts can be passed inline via `--prompt-file=path/to/prompt.md`.

### Models

The harness uses the `completion` tool (available in the eval kernel) with
configurable `model`:

| Model tag | Meaning |
|---|---|
| `smol` | Fast/cheap — good for iteration |
| `default` | The session's default model |
| `slow` | Most capable — for final baselines |
| `glm-4.6` | Explicit model name (any the completion tool supports) |

Temperature is always 0 for reproducibility. The harness records the exact
model + prompt + surface in every result file for provenance.

## Architecture

```
eval/
  toolsurface.go      — ToolSurface type (list of ToolDef), live fetcher, pre-PR transformer
  toolsurface_test.go
  pre_post.go         — ToPrePR transform (data-driven, not git checkout)
  cases.go            — the golden case set (query → expected tool + args)
  cases_test.go
  prompts.go          — named system-prompt presets + loader
  prompts_test.go
  harness.go          — runs cases: surface × model × prompt → CaseResult
  harness_test.go
  score.go            — aggregates CaseResults into metrics (P@1, MRR, per-service, delta)
  score_test.go
  main.go             — CLI: `devbox run eval` with --surface, --model, --prompt, --sweep flags
  main_test.go        — integration test (pre_post comparison with smol model)
  README.md           — how to run, how to add cases, how to sweep
  results/            — committed baseline JSON results (provenance-tagged)
```

### ToolSurface

A `ToolSurface` is a snapshot of the MCP `tools/list` response — an ordered
list of `{name, description, inputSchema}` triples. Two surfaces are needed:

- **post-PR**: live from the running wiki's `/mcp` endpoint (the current branch).
- **pre-PR**: reconstructed by taking the post-PR surface and applying the inverse
  of the PR's changes:
  - Restore stub descriptions (`"X — see (api.v1.description).\n"`) for the 11
    ported services.
  - Restore empty descriptions for SurveyService (all 9 RPCs).
  - Re-add the 7 excluded ChatService tools (SendChatReply, EditChatMessage,
    ReactToMessage, SendToolCallNotification, SendPlanNotification,
    SendTurnStatus, RespondToPermission).
  - Re-add `api_v1_ScheduledTurnService_CompleteScheduledTurn`.

This reconstruction is data-driven (a transform applied to the live surface),
not a git checkout — so it works against any future branch too.

### Cases

Each case is:

```go
type Case struct {
    ID           string         // stable identifier
    Query        string         // natural-language user request
    ExpectedTool string         // the canonical correct tool name
    ExpectedArgs map[string]any // optional: expected argument keys/values
    ExcludedTool string         // optional: for exclusion cases, a tool that must NOT be selected
    Services     []string       // which services this case exercises
    Tags         []string       // e.g. "checklist", "survey", "naming", "exclusion"
}
```

Cases are organized by service and cover:

1. **Happy path**: "Add milk to the grocery list" → `ChecklistService_AddItem`
2. **Disambiguation**: "Change the survey question" → `SurveyService_UpsertSurvey`
   (not `SurveyService_UpdateField` — tests description clarity)
3. **Exclusion**: "Reply to the user's chat message" → should NOT select
   `ChatService_SendChatReply` (excluded); ExcludedTool field set
4. **Naming sensitivity**: "Submit a survey response" →
   `SurveyService_SubmitResponse` with `survey_name` field (pre-PR naming);
   a parallel case set for post-rename would use `name`
5. **Cross-service confusion**: "Move the workshop drill to the garage shelf"
   → `InventoryManagementService_MoveInventoryItem` (not `MapService_MoveMarker`)
6. **Read vs write**: "Show me the version history of this page" →
   `PageHistoryService_ListPageVersions` (read-only, not RestorePageVersion)

Target: ~60-80 cases covering all 14 exposed services.

### Harness

The harness presents the ToolSurface to an LLM and asks it to select a tool.
The prompt is assembled as:

```
<system_prompt>

Tool catalog:
<tools/list JSON, compact>

User request: "<query>"

Respond as JSON: {"tool": "<name>", "args": {...}}
```

The LLM call goes through the eval kernel's `completion` tool, which supports
`model: "smol" | "default" | "slow"` or explicit model names. Temperature is 0.

### Scoring

For each case:
- **Tool match** (binary): selected tool == expected tool (1/0)
- **Args match** (partial): fraction of expected arg keys present and correct
- **Exclusion precision** (binary): for exclusion cases, did the selector
  avoid the excluded tool?

Aggregate metrics per (surface, model, prompt) configuration:
- **Precision@1** — fraction of cases where the top pick is correct
- **Per-service breakdown** — accuracy by service to surface weak spots
- **Delta** — the headline number when comparing two configurations

### Running

```bash
# Single configuration
devbox run eval -- --surface=post --model=smol --prompt=minimal

# Pre/post comparison (the headline PR evaluation)
devbox run eval -- --compare=pre,post --model=smol --prompt=production

# Model sweep: same surface + prompt, vary the model
devbox run eval -- --surface=post --prompt=production --sweep=model:smol,default,slow

# Prompt sweep: same surface + model, vary the prompt
devbox run eval -- --surface=post --model=smol --sweep=prompt:minimal,production,catalog-hint

# Full grid (expensive): all surfaces × models × prompts
devbox run eval -- --sweep=surface:pre,post --sweep=model:smol,default --sweep=prompt:minimal,production
```

Output: a markdown comparison table printed to stdout + a JSON results file
in `eval/results/` tagged with surface+model+prompt+timestamp.

### LLM cost

~60-80 cases × 2 surfaces = ~120-160 LLM calls per comparison run. At ~2k tokens
per call (tool catalog + query + response), that's ~320k tokens for a single
comparison. A full grid sweep (2 surfaces × 3 models × 4 prompts × 80 cases) is
~1,920 calls / ~4M tokens — expensive but runnable.

## What this tells us about F6 (naming)

If the eval shows low accuracy on `survey_name` cases (e.g. the selector
guesses `name` instead of `survey_name`), that quantifies the naming tax. A
rename branch can then run the same eval with a post-rename surface variant
and measure the delta. The decision to rename becomes data-driven, not vibes.

## Implementation plan

1. `eval/toolsurface.go` + `eval/pre_post.go` — ToolSurface type, live fetcher, pre-PR transformer
2. `eval/cases.go` — golden case set (start with ~20, grow to ~60-80)
3. `eval/prompts.go` — named system-prompt presets
4. `eval/harness.go` — LLM judge harness with scoring
5. `eval/score.go` — aggregation + comparison metrics
6. `eval/main.go` — CLI with sweep flags
7. `eval/main_test.go` — integration test (pre/post with smol model)
8. Wire `devbox run eval` script in `devbox.json`
9. Initial baseline run + results committed

## Open questions

- **Determinism**: temperature 0 for reproducibility. LLM output is still
  non-deterministic across model versions, so results are point-in-time
  snapshots, not eternal truths. The harness records model + prompt + surface
  provenance in every result file.
- **Catalog size at the prompt**: 105 tools × ~200 chars each = ~21k chars in
  the prompt. Some models may struggle with this context length. The harness
  can optionally truncate descriptions to a max length per tool to test whether
  verbosity helps or hurts.