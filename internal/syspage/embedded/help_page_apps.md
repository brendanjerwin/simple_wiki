+++
identifier = "help-page-apps"
title = "Page Apps: Agent-Authored Apps (OpenUI Lang)"

[wiki]
system = true
+++

# Page Apps: Agent-Authored OpenUI Lang Apps

A **page app page** renders an app the agent wrote, composed **above** the page's markdown content. The agent writes **OpenUI Lang** (against the wiki's own component library, `wiki-core@1`); the wiki parses and renders it in-page with its own Lit components — **no arbitrary HTML/JS executes anywhere**. State persists server-side via `PageAppStateService`, so it survives reloads, devices, and other users.

Use `PageAppService` and `PageAppStateService` for authoring and state. Both are exposed as gRPC and MCP tools (auto-generated from the proto).

## The language (ground rules — verified against the parser)

- **Every statement is an assignment**: `identifier = Expression`. The **entry point is always `root = <Component>(...)`** — a program without `root =` is rejected at write time.
- **Component calls are positional**: args map onto the component's props in its declared order — `Card("Pomodoro", "body text")` sets `title` then `content`. Named syntax (`title: "x"`) is NOT valid; a mismatched arg is dropped with `excess-args`.
- **Children are an array argument**: `Stack([Text("hello"), Button("press")])`.
- **Reactive state is a `$name` declaration**: `$completed = 0`. References interpolate by **string concatenation** — `"sessions: " + $completed` (there is no `{expr}` string interpolation).
- **Unknown components and invalid statements are dropped, never fatal** — the rest renders. Errors come back structured (`{code, component, statementId, message}`) so you can self-correct.
- **Missing a component?** If an app needs a component that isn't in `wiki-core@1` (no `Slider`, `Tabs`, `Progress`, etc.), do **not** fake it with nested `Stack` hacks or drop the feature silently. Build the best program you can **without** the component, tell the user which component is missing and what app it blocked, and **open a feature-request issue on the SimpleWiki GitHub project** (`github.com/brendanjerwin/simple_wiki/issues`) naming the component and the real app it would enable — components get built based on demand from actual apps, so a concrete request with a real use case is the fastest path to getting it.

## For Agents

### Quick start (the whole authoring flow in one block)

You are on a page and the user wants an app. This is the exact sequence — nothing else needed:

1. `ReadPage` — page "help-page-apps" (this page; the language below is the whole spec).
2. Compose the program. **Mental checklist**: every statement `identifier = Expression`; entry `root = Component(positional args...)`; children are arrays `Stack([...])`; state `$name = default`; interpolation `"text " + $name` ONLY (no braces); buttons `Button("label")` or `Button("label", @ToAssistant("msg"))`.
3. Place it:
   - App about this page's content → `SetPageApp{page: "<this page>", source: <program>}` (composes above prose; prose survives).
   - App about a separate topic → `CreatePage` for a new page + `SetPageApp` there + add a wikilink here.
4. Tell the user what you built and where.
5. Iterate on request: `GetPageApp` → modify program text → `SetPageApp` (with `expected_version_hash` from step 4's read) → hot-swap happens automatically.

If `SetPageApp` rejects: the error names the violation (missing `root =`, too large, fence chars). Fix the program, resend. Do NOT guess language rules from memory — reread the ground rules above.

### The decision: where does the app live?

- **Default: new page + wikilink.** The app is about a recurring topic or belongs to other content ("make me a pomodoro", "a tracker for my habits"). `CreatePage` → `SetPageApp` on the new page → add a wikilink from the current page, and say in chat what you did and why.
- **Attached to this page** when the app is *for this page's content* ("put a dashboard on this page", "a tracker atop this list"). Prose stays; the app renders above it.
- **App alone on this page** only when the user wants this page to *be* the app (scratch page, "just the app"). Send `confirm_replace_content: true` — the mechanical guard rejects silently clearing substantial prose otherwise, and that's deliberate.
- **Never** convert: system pages, template pages, inventory/blog/checklist-structured pages, profile pages, or a page already hosting a different page app.

### Writing an app: `api_v1_PageAppService_SetPageApp`

The single write door. Program source goes in `source`; OCC via `expected_version_hash`:

```json
{
  "page": "pomodoro",
  "source": "$completed = 0\nroot = Card(\"Pomodoro\", \"sessions: \" + $completed)\n",
  "expected_version_hash": ""
}
```

- First write on a page without a page app: omit `expected_version_hash` (empty is accepted).
- Iterating: **send the full updated program** via `SetPageApp` with the hash you read (`GetPageApp`/`WatchPageApp`). (The browser host merges streamed patches itself; your reliable agent loop is read→modify→write with OCC.)
- Conflict (`Aborted`): the page app changed since you read it. Re-run `GetPageApp`, re-apply your change, retry.
- Server validation is **conservative** (structural: non-empty, has `root =`, size cap 64 KiB, must not contain a ``` fence terminator — the wiki wraps the source itself). It cannot fully parse the language — that happens in the browser. Gross violations reject with `InvalidArgument`; partial-but-renderable programs accept.
- **Stamps are server-managed**: `lang = "openui-lang"`, `version`, `version_hash`, `library = "wiki-core@1"`, `updated_at`/`updated_by`/`automated`. Never hand-write them via frontmatter tools — the `wiki.*` namespace rejects generic frontmatter writes and names the right service.

### Reading the current app: `api_v1_PageAppService_GetPageApp`

```json
{ "page": "pomodoro" }
```

Returns `source` plus the stamps (`version_hash` is your OCC handle). `NotFound` = the page has no page app.

### Persisting state: `api_v1_PageAppStateService_MergePageAppState`

Deep-merge partial state (scalars replace, arrays union, nested structs merge). CAS'd on `state_expected_version_hash` — **independent** of the page's markdown hash:

```json
{
  "page": "pomodoro",
  "state": {"$completed": 3, "weekly": {"2026-W43": 2}},
  "state_expected_version_hash": "sha256:…"
}
```

**State keys are the program's `$`-prefixed variable names** — a `$completed` declaration persists as the key `"$completed"`. A merge key the program doesn't declare is orphaned: nothing reads it back.

- Seed/derive state for charts here. `GetPageAppState` reads it; `DeletePageAppState` resets it.
- Conflict (`Aborted`): another writer (another tab, the running app) moved first. `GetPageAppState` → re-apply → retry.
- **Never use browser `localStorage`/`sessionStorage` for page app state** — it collides across page apps and the wiki shell on one origin, and it's per-device. State via this service survives reloads, other users, other devices.

### The channel back to you: `@ToAssistant`

Buttons wire a click to operations via an **action prop** (the syntax is `Button("Ask", @ToAssistant("Tell me more"))` — one bare @step as the action argument, or `Action([@step, @step])` for several). `@ToAssistant(message, context?)` turns the press into a chat message on the page, and you (the page's agent) reply, e.g. with a new `SetPageApp`. **A button with no action automatically sends its label** as the chat message (equivalent to `@ToAssistant(label)`) — that's the simplest pattern. Render→chat→reply→hot-swap closes the loop without any bespoke shim.

### The full loop (canonical example)

User: "make me a pomodoro tracker with a reset button that remembers sessions."

1. Decide placement (see above) — likely a new page `pomodoro` + link.
2. Write the program:

```
$completed = 0
root = Card("Pomodoro", Stack([
  Text("sessions: " + $completed),
  Button("session done", @ToAssistant("increment completed sessions")),
]))
```

3. `SetPageApp(page="pomodoro", source=…)` → the page renders the app above any prose; state hydrates from `GetPageAppState`; the button's `@ToAssistant` lands a chat message on the page and you react (e.g. `MergePageAppState` with `{"$completed": 1}`, then a new `SetPageApp`).
4. User: "add a weekly chart" → you read the program (`GetPageApp`), compose the updated program text (positional args!), `SetPageApp` with the hash → `WatchPageApp` hot-swaps the renderer **without a page reload**.
5. "What's saved?" → `GetPageAppState`. "Seed 3 sessions" → `MergePageAppState`. "Turn it back into notes" → `ClearPageApp` (removes only the fence + stamps; prose and history are untouched).

### Runtime contract (the things you can't discover except by breaking them)

- **Single write door**: only `SetPageApp` writes the program. The `/edit` markdown editor also works for humans — the fence is visible in the body, and the same validation+guard funnel applies there. The frontmatter editor dialog is disabled on page app pages (its save would race the page app services).
- **Composition model**: the app renders **above** the prose; the prose is never hidden and never duplicated. Clear prose only for a deliberate app-standalone page (`confirm_replace_content: true`).
- **Styling**: comes from the wiki component library — all components carry a full
  responsive design system automatically (theme vars, dark mode, mobile breakpoints).
  You cannot add per-page-app CSS in this mode.
  **Responsive rules that come free**: `Stack` wraps on narrow screens (horizontal
  stacks become vertical below 520px); `Table` scrolls horizontally inside the card
  rather than overflowing the page; `Card` caps at 720px and centers on wide screens;
  buttons/inputs use the theme's action colors. **Design guidance**: prefer one outer
  `Card` per app; use `Stack` (vertical) for form-like flows and horizontal `Stack` for
  button rows; use `Chart` for any numeric comparison (labels+values arrays) and
  `Table` for tabular data — both are styled automatically; keep text in `Text`, never
  embed raw formatting.
- **No external resources**: programs must be self-contained; external CDN fetches are not loadable from the page app runtime. Media/images go through the wiki's existing upload surface if genuinely needed.
- **History**: every program write snapshots the page (ADR-0016). "Revert to an hour ago" = the page-history tooling; there is no separate page app history.

### Tool availability inside apps (the allowlist)

Apps can call only these tools from the rendered program — **fully-qualified ids, camelCase args** (short bare names like `GetPageApp` also resolve, but write the qualified form):

- **All `PageAppStateService` methods**: `api_v1_PageAppStateService_GetPageAppState`, `MergePageAppState`, `DeletePageAppState`
- **Read-only discovery**: `api_v1_PageAppService_GetPageApp`, `api_v1_PageHistoryService_ListPageVersions`, `api_v1_SearchService_SearchContent`, `api_v1_PageManagementService_ReadPageOutline`, `api_v1_Frontmatter_GetFrontmatter`, `api_v1_MapService_ListMapElements`, `api_v1_ChecklistService_ListItems`, `api_v1_SurveyService_GetSurvey`

Everything else — any page mutation, checklist/map/survey/connector mutation — is **excluded**: a program literally cannot mutate another page or user data; a non-allowlisted call fails with `tool-not-found` and the rest of the program still renders.

Read data via a `Query(...)` assignment, then trigger a refetch with `@Run` — actions are a positional argument on the component, and every statement must be assigned (a bare statement is unreachable and silently dropped):

```
rows = Query("api_v1_ChecklistService_ListItems", { listName: "groceries" })
refresh = Button("refresh", @Run(rows))
root = Card("Groceries", Stack([refresh, Table(["item"], rows.items)]))
```

If an app needs a destructive action, the channel is the conversation (`@ToAssistant`), where confirmation and permissions live — not the program.

### Common patterns (all parse-verified — copy and adapt)

**Counter with reset** — pure client-side state. Every `@Set` writes the live store snapshot to `PageAppStateService` automatically (CAS'd, conflict-safe), so the count survives reloads and other devices. **No agent round-trip is needed for simple state.**

```
$count = 0
root = Card("Count", Stack([
  Text("count: " + $count),
  Button("add", @Set($count, $count + 1)),
  Button("reset", @Reset($count)),
]))
```

**Text input bound to state** — `Input.onChange` flows typed text back into a `$variable`, which triggers a live re-render:

```
$name = ""
root = Card("Greeter", Stack([
  Input("your name", $name),
  Text("hi " + $name),
]))
```

**API/data lookup** — a `Query(...)` assignment calls a read-only tool at render time; `@Run(queryName)` re-fires it (e.g. after a `Button`):

```
rows = Query("api_v1_ChecklistService_ListItems", { listName: "todo" })
refresh = Button("refresh", @Run(rows))
root = Card("Todo", Stack([refresh, Table(["text"], rows.items)]))
```

**Editing wiki data (write flows through the agent or `Mutation`)** — a program *can* write its own page app state via the allowlisted `PageAppStateService_*`, but note these two write paths:

- `@Set`/`@Reset` on `$variables` → host auto-flushes the state store silently (fast, no agent involved).
- Anything touching **other pages or other data** is excluded on purpose. Use `@ToAssistant` to hand the request to the agent, which has confirmed/attributed mutation tools:

```
root = Card("Notes", Stack([
  Text("current: " + $note),
  Input("new value", $note),
  Button("save", @ToAssistant("update the note to: " + $note)),
]))
```

**Component request when nothing fits** — if the app you're writing genuinely needs a component `wiki-core@1` doesn't have (slider, tab pane, progress bar, date picker, …), don't degrade it into a confusing `Stack`-of-buttons. Tell the user what's missing, ship the best app you can **without** it, then **open a feature-request issue on this project** so the component eventually gets built: `Button("request a slider", @OpenUrl("https://github.com/brendanjerwin/simple_wiki/issues"))` renders the one-tap link for the user; you create the issue text in the same turn.

## See Also

- [[help]] — full help index
- [[help-templating]] — page macros (checklist/map/survey templates) — often the right tool *instead of* a page app
- [[help-page-history]] — restore/revert semantics the page app inherits