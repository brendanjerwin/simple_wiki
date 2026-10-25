+++
identifier = "help-page-apps"
title = "Page Apps: Agent-Authored Apps (OpenUI Lang)"

[wiki]
system = true
+++

# Page Apps: Agent-Authored OpenUI Lang Apps

A **page app page** renders an app the agent wrote, composed **above** the page's markdown content. The agent writes **OpenUI Lang** (against the wiki's own component library, `wiki-core@3`); the wiki parses and renders it in-page with its own Lit components — **no arbitrary HTML/JS executes anywhere**. State persists server-side via `PageAppStateService`, so it survives reloads, devices, and other users.

Use `PageAppService` and `PageAppStateService` for authoring and state. Both are exposed as gRPC and MCP tools (auto-generated from the proto).

## The language (ground rules — verified against the parser)

- **Every statement is an assignment**: `identifier = Expression`. The **entry point is always `root = <Component>(...)`** — a program without `root =` is rejected at write time.
- **Component calls are positional**: args map onto the component's props in its declared order — `Card("Pomodoro", "body text")` sets `title` then `content`. Named syntax (`title: "x"`) is NOT valid; a mismatched arg is dropped with `excess-args`. Optional trailing props (variants, labels, actions) pass `undefined` to skip: `Button("cancel", undefined, "secondary")`.
- **Children are an array argument**: `Stack([Text("hello"), Button("press")])`.
- **Reactive state is a `$name` declaration**: `$completed = 0`. References interpolate by **string concatenation** — `"sessions: " + $completed` (there is no `{expr}` string interpolation).
- **Unknown components and invalid statements are dropped, never fatal** — the rest renders. Errors come back structured (`{code, component, statementId, message}`) so you can self-correct.
- **Missing a component?** If an app needs a component that isn't in `wiki-core@3` (no `Slider`, `Tabs`, `DatePicker`, etc.), do **not** fake it with nested `Stack` hacks or drop the feature silently. Build the best program you can **without** the component, tell the user which component is missing and what app it blocked, and **open a feature-request issue on the SimpleWiki GitHub project** (`github.com/brendanjerwin/simple_wiki/issues`) naming the component and the real app it would enable — components get built based on demand from actual apps, so a concrete request with a real use case is the fastest path to getting it. (`Grid`, `Card` variants, `Divider`, and `SectionHeader` exist as of `wiki-core@3`; `Slider`, `Tabs`, `Dialog`, and the `Image` component are still future.)

## Time-based apps: `Countdown`

`Countdown(seconds, label?, onComplete?, running?)` renders a live countdown that ticks down once per second and shows the remaining time as `m:ss` (`h:mm:ss` past an hour). It is **idle until `running` is true** — the gate's default and a fresh page load both leave it idle (the one exception: the gate value itself is persisted, so if a writer left `$running = true` the reload re-arms, as covered below). `running=false` pauses where the clock stands; a fresh `running=true` after a finish restarts from the full duration. When it reaches zero, `onComplete` runs (any action) and the clock stops.

**The run gate is program-owned.** Bind `running` to a `$variable`, start it with `@Set`, and **pair the stop with the tally inside `onComplete`** — `Action([@Set($holds, $holds + 1), @Set($running, false)])`. That false→true↔true→false cycle is what makes the next start a *real edge* the timer can see; a program that leaves `$running = true` forever cannot restart its timer. For chained/looping timers (Pomodoro phases), `@Set` the next timer's `seconds` (and `running` edges) from `onComplete`. Countdowns are **not persisted timing**: reloading a page starts the clock over at full duration. One nuance to know: `$running` *is* persisted like any `$variable`, so if a writer left it `true` and the page reloads mid-hold, the timer starts fresh at full duration on load — the gate resets only when `onComplete` (or a button) sets it false. That's why the paired `@Set($running, false)` in `onComplete` matters.

**Stretch-timer hold** — a timer per leg, each armed by its own start button:

```
$leftRunning = false
$rightRunning = false
$holds = 0
root = Card("Hamstring routine", Stack([
  Text("holds completed: " + $holds),
  Button("start left-leg hold", @Set($leftRunning, true)),
  Countdown(30, "left leg hold", Action([
    @Set($holds, $holds + 1),
    @Beep(),
    @Set($leftRunning, false),
  ]), $leftRunning),
  Button("start right-leg hold", @Set($rightRunning, true)),
  Countdown(30, "right leg hold", Action([
    @Set($holds, $holds + 1),
    @Beep(),
    @Set($rightRunning, false),
  ]), $rightRunning),
]))
```

Each timer's fourth positional argument binds its `$running` gate. Completing a hold tallies, beeps, and sets the gate false, so the next `@Set(..., true)` is a genuine false→true edge and the timer restarts from 30 — press the next button when you're actually ready for that hold.

### Audio feedback: `@Beep`

`@Beep(kind?)` is an **action step** (usable anywhere an action runs: a `Button`'s action or a `Countdown`'s `onComplete`; compose with `Action([...])`). It plays a short synthesized tone — no external audio files, no agent round-trip. Kinds: `@Beep()` (single 880 Hz tone), `@Beep("double")` (two quick beeps), `@Beep("low")` (a lower-pitched 440 Hz tone). Use it for eyes-free timing apps — a stretch routine's `onComplete` beeps so you can hold a pose looking away from the screen.

Autoplay policy: the user's first interaction with the page (e.g. pressing the start button) unlocks audio; a `Countdown` the user themselves started with that click then beeps on completion, playing through that unlock. A timer that starts without any prior interaction (e.g. a persisted `$running = true` re-armed by a reload) stays silent — audio is feedback, never control flow.

## Variants, spacing, and progress

**Button variants** (last positional prop): `Button("label", @action, variant)`. `primary` is the saturated blue CTA (default), `secondary` is the muted bordered button for tertiary actions, `ghost` is the transparent tertiary text button, `danger` is red for destructive actions. Use one `primary` per the visual emphasis you want; destructive actions get `danger`, everything else stays `secondary`/`ghost`.

**Text size variants** (second positional prop): `Text("content", variant)` — `title` (large heading-style, use inside apps instead of faking headers with bold), `body` (default), `caption` (small secondary text for hints and asides).

**Spacing tokens**: `Stack([children], direction?, gap?)` `gap` accepts 4pt-grid tokens — `"sm"`=4px, `"md"`=8px, `"lg"`=16px, `"xl"`=24px — or a raw CSS gap value when you genuinely need one. Cards and stacks default to a 12px rhythm; prefer the tokens so multi-card layouts stay on the grid.

**Progress**(total, current?, label?) renders "N of M" with a fill bar; `current` is reactive — `Progress($total, $done, "label?")` and `@Set($done, $done + 1)` advance it live. Good for tallies that aren't a countdown.

**Steps**(current, total, onBack?, onNext?, label?) renders "step N of M" with back/next buttons when you wire actions: `Steps($i, 5, @Set($i, $i - 1), @Set($i, $i + 1), "wizard")`. A button renders only when its action is provided; clamp `$i` to the range yourself (e.g. `Action([@Set($i, min($i + 1, 5))])`).

```
$i = 1
root = Card("Wizard", Stack([
  Steps($i, 5, @Set($i, $i - 1), @Set($i, $i + 1), "setup wizard"),
  Text("on step " + $i, "caption"),
]))
```

## Layout: `Grid`, `Span`, `Divider`, `SectionHeader`

**Grid**(children, cols?, gap?) — children FIRST, then columns: `Grid([a, b, c, d], 2, "lg")` lays four children into a 2-column grid. `cols` accepts `2`, `3`, or the `"sidebar"` preset (240px main column + flexible rest). `gap` takes the same 4pt tokens as Stack. Grids collapse to one column below 520px — author as if each cell may stack vertically.

**Span**(n, children) — makes its children span `n` grid columns *inside a Grid*: `Grid([Span(2, [Text("wide")]), Text("narrow")], 2)`. `n` accepts 1, 2, or 3 — anything else is rejected at parse time. Rendered as inline `grid-column: span n`. Note the opposite argument order from Grid (count first here — `n` is a required number, so a reversed call fails loudly rather than silently mis-binding).

**Divider**() — a horizontal rule. **SectionHeader**(title, trailing?) — a titled section row with a hairline rule and optional trailing content: `SectionHeader("Workout", [Text("4 sets", "caption")])`.

```
root = Card("Routine", Stack([
  SectionHeader("Warm-up", [Text("5 min", "caption")]),
  Grid([Text("arm circles"), Text("leg swings")], 2, "md"),
  Divider(),
  SectionHeader("Workout", [Text("4 sets", "caption")]),
  Grid([Progress(3, 1, "set 1"), Progress(3, 2, "set 2")], 2),
]))
```

**Card variants** (third positional prop): `Card("title", children, variant)` — `elevated` (stronger shadow, no border) for hero content, `outlined` (flat, thin border) for quiet panels, `filled` (sunken background) for grouped secondary content, `accent` (accent-tinted wash) for highlight cards. The default card stays as before. (Background-image hero cards are coming later; request via issue if blocked on them today.)

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
- **Stamps are server-managed**: `lang = "openui-lang"`, `version`, `version_hash`, `library = "wiki-core@3"`, `updated_at`/`updated_by`/`automated`. Never hand-write them via frontmatter tools — the `wiki.*` namespace rejects generic frontmatter writes and names the right service.

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
  embed raw formatting. Control emphasis with button `variant`s (`primary`/`secondary`/
  `ghost`/`danger`) and text size with `Text` `variant`s (`title`/`body`/`caption`);
  keep spacing on the 4pt grid with `Stack` gap tokens `sm`/`md`/`lg`/`xl`.
- **No external resources**: programs must be self-contained; external CDN fetches are not loadable from the page app runtime. Media/images go through the wiki's existing upload surface if genuinely needed.
- **History**: every program write snapshots the page (ADR-0016). "Revert to an hour ago" = the page-history tooling; there is no separate page app history.

### Tool availability inside apps (the allowlist)

Apps can call only these tools from the rendered program — **fully-qualified ids, camelCase args** (short bare names like `GetPageApp` also resolve, but write the qualified form):

- **All `PageAppStateService` methods**: `api_v1_PageAppStateService_GetPageAppState`, `MergePageAppState`, `DeletePageAppState` — these are **pinned to the app's own page**: any `page` in the call args is ignored, and a state call always acts on the page hosting the app.
- **Read-only discovery**: `api_v1_PageAppService_GetPageApp`, `api_v1_PageHistoryService_ListPageVersions`, `api_v1_SearchService_SearchContent`, `api_v1_PageManagementService_ReadPageOutline`, `api_v1_Frontmatter_GetFrontmatter`, `api_v1_MapService_ListMapElements`, `api_v1_ChecklistService_ListItems`, `api_v1_SurveyService_GetSurvey`

Everything else — any page mutation, checklist/map/survey/connector mutation — is **excluded**: a program literally cannot mutate another page or user data (page app state is likewise pinned to the app's own page); a non-allowlisted call fails with `tool-not-found` and the rest of the program still renders.

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

**Text input bound to state** — an `Input` bound to a `$variable` hydrates from state and re-renders the app live. Inside an `onChange` step the typed value is referenced as **`$value`** (the control fires `@change` — commit/blur, not every keystroke):

```
$name = ""
root = Card("Greeter", Stack([
  Input("your name", $name, @Set($name, $value)),
  Text("hi " + $name),
]))
```

**API/data lookup** — a `Query(...)` assignment calls a read-only tool at render time; `@Run(queryName)` re-fires it (e.g. after a `Button`):

```
rows = Query("api_v1_ChecklistService_ListItems", { listName: "todo" })
refresh = Button("refresh", @Run(rows))
root = Card("Todo", Stack([refresh, Table(["text"], rows.items)]))
```

**Editing wiki data (write flows through the agent or `Mutation`)** — a program *can* write its own page app state via the allowlisted `PageAppStateService_*` — these calls always address the page the app is rendered on; a `page` argument in the tool call is ignored — but note these two write paths:

- `@Set`/`@Reset` on `$variables` → host auto-flushes the state store silently (fast, no agent involved).
- Anything touching **other pages or other data** is excluded on purpose. Use `@ToAssistant` to hand the request to the agent, which has confirmed/attributed mutation tools:

```
$note = ""
root = Card("Notes", Stack([
  Text("current: " + $note),
  Input("new value", $note, @Set($note, $value)),
  Button("save elsewhere", @ToAssistant("update the note to: " + $note)),
]))
```

**Component request when nothing fits** — if the app you're writing genuinely needs a component `wiki-core@3` doesn't have (slider, tab pane, date picker, …), don't degrade it into a confusing `Stack`-of-buttons. Tell the user what's missing, ship the best app you can **without** it, then **open a feature-request issue on this project** so the component eventually gets built: `Button("request a slider", @OpenUrl("https://github.com/brendanjerwin/simple_wiki/issues"))` renders the one-tap link for the user; you create the issue text in the same turn. (`wiki-core@3` added the layout batch — `Grid`/`Span`, `Divider`, `SectionHeader`, Card `variant`s.)

## See Also

- [[help]] — full help index
- [[help-templating]] — page macros (checklist/map/survey templates) — often the right tool *instead of* a page app
- [[help-page-history]] — restore/revert semantics the page app inherits