# MCP Tool Discovery Evaluation

Measures how well an LLM tool-selector maps natural-language user requests to
the correct MCP tool, given the wiki's `tools/list` catalog. See `DESIGN.md`
for the full design document.

## Quick start

```bash
# 1. Start the wiki (provides the live tool surface)
devbox services start

# 2. Set your OpenRouter API key
export OPENROUTER_API_KEY=sk-or-...

# 3. Dry-run to preview configuration and cost
devbox run eval -- --compare=pre,post --model=gemini-2.5-flash --prompt=production --dry-run

# 4. Run the pre/post comparison
devbox run eval -- --compare=pre,post --model=gemini-2.5-flash --prompt=production --out=eval/results/baseline.json

# 5. Sweep models (same surface + prompt)
devbox run eval -- --surface=post --prompt=production --sweep-model=gemini-2.5-flash,claude-3.5-sonnet,gpt-4o-mini

# 6. Sweep prompts (same surface + model)
devbox run eval -- --surface=post --model=gemini-2.5-flash --sweep-prompt=minimal,production,catalog-hint
```

## What it measures

Given a tool catalog (from `/mcp` `tools/list`) and a natural-language user
request, does the LLM pick the correct tool?

Three independent axes:

| Axis | Flag | What it varies |
|---|---|---|
| **Surface** | `--surface` / `--compare` | pre-PR (stubs, excluded tools present) vs post-PR (curated, clean) |
| **Model** | `--model` / `--sweep-model` | OpenRouter model (gemini-flash, claude-sonnet, gpt-4o-mini, ...) |
| **Prompt** | `--prompt` / `--sweep-prompt` | system prompt preset (minimal, production, catalog-hint, ...) |

## Cost tracking

Every run prints per-configuration cost (USD) and token counts. The `--dry-run`
flag shows estimated cost before spending anything. Results files include
per-call cost breakdowns.

Models and their per-million-token costs are defined in `harness.go`
(`ModelPresets`). Update as OpenRouter pricing changes.

## Adding cases

Append to `Cases` in `cases.go`. Each case needs a stable `ID`, a natural-language
`Query`, and an `ExpectedTool` (or `ExcludedTool` for exclusion cases). Tag with
service names and categories (happy-path, disambiguation, exclusion, naming,
cross-service-confusion) for filtering.

## Architecture

```
eval/
  DESIGN.md          — full design document
  toolsurface.go     — ToolSurface type + live fetcher from /mcp
  pre_post.go        — ToPrePR transform (data-driven, not git checkout)
  cases.go           — golden case set (53 cases, all 14 services)
  prompts.go         — named system-prompt presets
  harness.go         — OpenRouter LLM call + scoring + cost tracking
  score.go           — aggregation + comparison metrics
  cmd/main.go        — CLI entrypoint
  results/           — committed baseline results (JSON, provenance-tagged)
```