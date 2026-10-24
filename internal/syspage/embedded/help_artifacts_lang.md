+++
title = "Artifacts: Agent-Authored Apps (OpenUI Lang)"
identifier = "help-artifacts-lang"
system = true
+++

# Artifact Pages: Agent-Authored OpenUI Lang Apps

An **artifact page** renders an app the agent wrote, composed **above** the page's markdown content. The agent writes **OpenUI Lang** (against the wiki's own component library, `wiki-core@1`); the wiki parses and renders it in-page with its own Lit components — **no arbitrary HTML/JS executes anywhere**. State persists server-side via `ArtifactStateService`, so it survives reloads, devices, and other users.

Use `ArtifactService` and `ArtifactStateService` for authoring and state. Both are exposed as gRPC and MCP tools (auto-generated from the proto).

## The language (ground rules — verified against the parser)

- **Every statement is an assignment**: `identifier = Expression`. The **entry point is always `root = <Component>(...)`** — a program without `root =` is rejected at write time.
- **Component calls are positional**: args map onto the component's props in its declared order — `Card("Pomodoro", "body text")` sets `title` then `content`. Named syntax (`title: "x"`) is NOT valid; a mismatched arg is dropped with `excess-args`.
- **Children are an array argument**: `Stack([Text("hello"), Button("press")])`.
- **Reactive state is a `$name` declaration**: `$completed = 0`. References interpolate by **string concatenation** — `"sessions: " + $completed` (there is no `{expr}` string interpolation).
- **Unknown components and invalid statements are dropped, never fatal** — the rest renders. Errors come back structured (`{code, component, statementId, message}`) so you can self-correct.

## For Agents

### The decision: where does the app live?

- **Default: new page + wikilink.** The app is about a recurring topic or belongs to other content ("make me a pomodoro", "a tracker for my habits"). `CreatePage` → `SetArtifact` on the new page → add a wikilink from the current page, and say in chat what you did and why.
- **Attached to this page** when the app is *for this page's content* ("put a dashboard on this page", "a tracker atop this list"). Prose stays; the app renders above it.
- **App alone on this page** only when the user wants this page to *be* the app (scratch page, "just the app"). Send `confirm_replace_content: true` — the mechanical guard rejects silently clearing substantial prose otherwise, and that's deliberate.
- **Never** convert: system pages, template pages, inventory/blog/checklist-structured pages, profile pages, or a page already hosting a different artifact.

### Writing an app: `api_v1_ArtifactService_SetArtifact`

The single write door. Program source goes in `source`; OCC via `expected_version_hash`:

```json
{
  "page": "pomodoro",
  "source": "$completed = 0\nroot = Card(\"Pomodoro\", \"sessions: \" + $completed)\n",
  "expected_version_hash": ""
}
```

- First write on a page without an artifact: omit `expected_version_hash` (empty is accepted).
- Iterating: **send the full updated program** via `SetArtifact` with the hash you read (`GetArtifact`/`WatchArtifact`). (The browser host merges streamed patches itself; your reliable agent loop is read→modify→write with OCC.)
- Conflict (`Aborted`): the artifact changed since you read it. Re-run `GetArtifact`, re-apply your change, retry.
- Server validation is **conservative** (structural: non-empty, has `root =`, size cap 64 KiB, must not contain a ``` fence terminator — the wiki wraps the source itself). It cannot fully parse the language — that happens in the browser. Gross violations reject with `InvalidArgument`; partial-but-renderable programs accept.
- **Stamps are server-managed**: `lang = "openui-lang"`, `version`, `version_hash`, `library = "wiki-core@1"`, `updated_at`/`updated_by`/`automated`. Never hand-write them via frontmatter tools — the `wiki.*` namespace rejects generic frontmatter writes and names the right service.

### Reading the current app: `api_v1_ArtifactService_GetArtifact`

```json
{ "page": "pomodoro" }
```

Returns `source` plus the stamps (`version_hash` is your OCC handle). `NotFound` = the page has no artifact.

### Persisting state: `api_v1_ArtifactStateService_MergeArtifactState`

Deep-merge partial state (scalars replace, arrays union, nested structs merge). CAS'd on `state_expected_version_hash` — **independent** of the page's markdown hash:

```json
{
  "page": "pomodoro",
  "state": {"completed_sessions": 3, "weekly": {"2026-W43": 2}},
  "state_expected_version_hash": "sha256:…"
}
```

- Seed/derive state for charts here. `GetArtifactState` reads it; `DeleteArtifactState` resets it.
- Conflict (`Aborted`): another writer (another tab, the running app) moved first. `GetArtifactState` → re-apply → retry.
- **Never use browser `localStorage`/`sessionStorage` for artifact state** — it collides across artifacts and the wiki shell on one origin, and it's per-device. State via this service survives reloads, other users, other devices.

### The channel back to you: `@ToAssistant`

Buttons wire a click to operations via an **action prop** (the syntax is `Button("Ask", @ToAssistant("Tell me more"))` — one bare @step as the action argument, or `Action([@step, @step])` for several). `@ToAssistant(message, context?)` turns the press into a chat message on the page, and you (the page's agent) reply, e.g. with a new `SetArtifact`. **A button with no action automatically sends its label** as the chat message (equivalent to `@ToAssistant(label)`) — that's the simplest pattern. Render→chat→reply→hot-swap closes the loop without any bespoke shim.

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

3. `SetArtifact(page="pomodoro", source=…)` → the page renders the app above any prose; state hydrates from `GetArtifactState`; the button's `@ToAssistant` lands a chat message on the page and you react (e.g. `MergeArtifactState` bumping `completed_sessions`, then a new `SetArtifact`).
4. User: "add a weekly chart" → you read the program (`GetArtifact`), compose the updated program text (positional args!), `SetArtifact` with the hash → `WatchArtifact` hot-swaps the renderer **without a page reload**.
5. "What's saved?" → `GetArtifactState`. "Seed 3 sessions" → `MergeArtifactState`. "Turn it back into notes" → `ClearArtifact` (removes only the fence + stamps; prose and history are untouched).

### Runtime contract (the things you can't discover except by breaking them)

- **Single write door**: only `SetArtifact` writes the program. The `/edit` markdown editor also works for humans — the fence is visible in the body, and the same validation+guard funnel applies there. The frontmatter editor dialog is disabled on artifact pages (its save would race the artifact services).
- **Composition model**: the app renders **above** the prose; the prose is never hidden and never duplicated. Clear prose only for a deliberate app-standalone page (`confirm_replace_content: true`).
- **Styling**: comes from the wiki component library — you cannot add per-artifact CSS in this mode. Colors/fonts follow the wiki theme.
- **No external resources**: programs must be self-contained; external CDN fetches are not loadable from the artifact runtime. Media/images go through the wiki's existing upload surface if genuinely needed.
- **History**: every program write snapshots the page (ADR-0016). "Revert to an hour ago" = the page-history tooling; there is no separate artifact history.

### Tool availability inside apps (the allowlist)

Apps can call only these tools from the rendered program: **all `ArtifactStateService` methods**, plus read-only discovery (`GetArtifact`, `ListPageVersions`, `SearchContent`, `GetPageOutline`, `GetFrontmatter`, `ListMapElements`, `ListItems`, `GetSurvey`). Everything else — any page mutation, checklist/map/survey/connector mutation — is **excluded**: a program literally cannot mutate another page or user data. If an app needs a destructive action, the channel is the conversation (`@ToAssistant`), where confirmation and permissions live — not the program.

## See Also

- [[help]] — full help index
- [[help-templating]] — page macros (checklist/map/survey templates) — often the right tool *instead of* an artifact
- [[help-page-history]] — restore/revert semantics the artifact inherits