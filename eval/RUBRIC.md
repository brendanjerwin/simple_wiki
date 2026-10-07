# Eval Scenario Rubric

## Purpose

A rubric for writing realistic evaluation scenarios that test **tool discovery** — not keyword matching. Each scenario should simulate what a real user would say to a wiki chat agent, grounded in a realistic page context.

## Criteria

### 1. Query realism

**No tool/service keywords.** The query must NOT contain:
- Tool names or method names (no "ChecklistService", "AddItem", "UpsertSurvey")
- Proto field names (no "survey_name", "list_name", "connector_kind", "page_name")
- API jargon (no "RPC call", "frontmatter delimiters", "optimistic concurrency")
- Service-internal concepts the user wouldn't know (no "sync token", "dead-letter queue", "tombstone")

**User-known identifiers only.** The query may include:
- Page names/identifiers the user would realistically know (their own pages: "meeting_notes", "garden_plan")
- Names of things they created (checklist names, survey names, map names) — but using natural language ("my grocery list", "the feedback form"), not proto field names
- Content they want to write ("add a section about watering schedules")

**Conversational tone.** Queries should read like actual chat messages:
- Casual, not imperative-technical ("can you add milk to my list?" not "Append item 'Buy milk' to checklist 'grocery_list' on page 'grocery_planning'")
- Sometimes incomplete ("fix the typo in the budget section")
- Sometimes ambiguous ("where did I put the pipe wrench?")
- May include context the user assumes the agent knows ("you know that survey I made? someone just filled it out")

**Include content context.** When the scenario involves writing, the query should include the actual content, not just the intent:
- "add a note that the tomatoes need staking by July 15th" (not "add an item to a checklist")
- "change the survey question to 'How was your experience?'" (not "update the survey")

### 2. Scenario structure

**Grounded in page context.** Each case has a `Context` field describing the page state:
- What page(s) are involved and their identifiers
- What frontmatter exists (keys, not values unless relevant)
- What content is on the page (brief summary, not full markdown)
- What checklists/surveys/maps exist on the page, if relevant

The context is provided to the LLM as part of the prompt, simulating what the agent would know from reading the page. It is NOT a hint about which tool to use — it's the world the user lives in.

**Separate UserSays field.** Each case has a `UserSays` field (the raw chat message) separate from any structured query. This lets us:
- Test with varying levels of context (UserSays alone vs. UserSays + page context)
- Compare how different models handle the same natural language

**Difficulty levels.** Each case is tagged with difficulty:
- `easy`: clear intent, one obvious tool, user names the thing they want to change
- `medium`: some ambiguity — the user describes the outcome but not the mechanism, or two tools could plausibly fit
- `hard`: genuinely ambiguous — multiple tools could accomplish the task, or the user's request is vague enough that the agent must reason about what they mean

**Multi-turn scenarios** (future): some cases will have a `FollowUp` field for a second user message that disambiguates. The eval scores whether the agent asked a clarifying question (good) or guessed wrong (bad) on the first turn. Not implemented in the first pass.

### 3. What to evaluate

**Tool selection** (primary): did the agent pick the right tool?

**Surgical preference**: when multiple tools could accomplish the task, did the agent pick the most surgical one?
- Body-only edit → `UpdatePageContent` (not `UpdatePage` or `UpdateWholePage`)
- Frontmatter-only change → `Frontmatter.MergeFrontmatter` (not `UpdatePage`)
- Section read → `ReadPageSection` (not `ReadPage`)
- Single item toggle → `ChecklistService.ToggleItem` (not `UpdateItem`)

**Decline/clarify behavior**: when no tool fits or the request is too ambiguous, did the agent decline or ask a clarifying question instead of guessing wrong? Scored as:
- Correct decline: agent returns `{"tool": null}` → hit
- Correct clarification: agent asks a question instead of selecting a tool → hit
- Wrong guess: agent picks a tool that doesn't fit → miss

**Argument construction** (secondary): given the right tool, did the agent construct valid arguments? Only scored when tool selection is correct.

### 4. Anti-patterns to avoid

- **Keyword stuffing**: "Add an item to the ChecklistService_AddItem tool on the grocery_planning page with list_name 'grocery_list'" — this is an API call, not a user request
- **Telegraphing the tool**: "Search the wiki content using the search service" — the user doesn't know what a "search service" is
- **Vacuum queries**: "Add an item" — too abstract; no page, no list, no content. Real users ground their requests in their own pages
- **Over-specifying arguments**: "Set the frontmatter key 'status' to 'published' on page 'blog_post_42'" — the user says "mark that blog post as published", not "set the frontmatter key"
- **Testing the API, not the agent**: "Call the UpsertSurvey RPC with survey_name='feedback'" — we're testing whether the agent can discover the right tool from a natural request, not whether it can construct a valid RPC

### 5. Canonical page fixtures

While each case carries its own ad-hoc context, the following pages recur across scenarios for consistency. The context field should describe what's on each page when it appears:

| Page | Content | Frontmatter | Checklists/Surveys/Maps |
|---|---|---|---|
| `garden_plan` | Notes about garden layout, planting schedule, watering | `title = "Garden Plan"`, `tags = ["home", "garden"]` | Map "yard" with markers for beds, shed, compost. Checklist "tasks" with seasonal items. |
| `meeting_notes` | Weekly team meeting notes, multiple sections (Budget, Action Items, Decisions) | `title = "Meeting Notes"`, `tags = ["work"]` | None |
| `grocery_planning` | Meal planning notes | `title = "Grocery Planning"` | Checklist "this_week" with grocery items |
| `pastry_project` | Sourdough bakery project tracking | `title = "Pastry Project"`, `agent` namespace with schedules and chat context | None |
| `product_page` | Product landing page | `title = "Our Product"` | Survey "feedback" with rating and comment fields |
| `workshop` | Workshop inventory and organization | `title = "Workshop"` | Inventory containers: "toolbox_red", "shelf_workshop" |

### 6. Case structure (Go)

```go
type Case struct {
    ID              string         `json:"id"`
    UserSays        string         `json:"user_says"`     // raw chat message
    Context         string         `json:"context"`       // page state description (provided to LLM)
    ExpectedTool    string         `json:"expected_tool"`
    AcceptableTools []string       `json:"acceptable_tools,omitempty"`
    ExpectedArgs    map[string]any `json:"expected_args,omitempty"`
    ExcludedTool    string         `json:"excluded_tool,omitempty"`
    Services        []string       `json:"services"`
    Tags            []string       `json:"tags"`          // must include difficulty: "easy"/"medium"/"hard"
}
```

The LLM prompt is assembled as:
```
<system_prompt>

You are the assistant for the following wiki page:

<context>

User message: "<user_says>"

Select the single best tool from the catalog and respond as JSON:
{"tool": "<name>", "args": {...}}
If no tool is appropriate, respond: {"tool": null}
```

### 7. Scoring changes

- **Surgical preference bonus**: when `AcceptableTools` lists multiple tools, the `ExpectedTool` is the most surgical. If the model picks an `AcceptableTool` that isn't `ExpectedTool`, it counts as a hit but is flagged as "non-surgical" in the per-tool breakdown. The aggregate reports a "surgical preference rate" alongside P@1.
- **Decline scoring**: `{"tool": null}` is a valid response. For exclusion cases, returning null is the best answer. For ambiguous cases where `ExpectedTool` is empty and `ExcludedTool` is set, null counts as a hit.