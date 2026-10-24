# ADR 0017: Artifact Pages with OpenUI Lang — Architecture and Composition Model

## Status

Accepted (supersedes the v1 arbitrary-HTML design discussed for issue #1272; Lang is the only mode in this scope)

## Context

The wiki hosts interactive views only via purpose-built macros (checklist, map, survey — each a reserved namespace plus a dedicated service). Arbitrary small apps have no authoring path. Issue #1272 asked for "artifact pages": pages whose default view renders an agent-authored app *composed above* the page's markdown content, with persistent state.

Two candidate authoring languages were evaluated:

1. **Agent-authored arbitrary HTML/JS** rendered in a sandboxed iframe. Expressive, but requires the full containment surface (iframe sandbox posture change per ADR-0014, per-route CSP/nosniff/manifest MIME machinery, asset directories, template choke-point bypass for `{{ }}`, lifecycle plumbing for asset dirs).
2. **OpenUI Lang** (`@openuidev/lang-core`, MIT, from thesysdev/openui): a compact, streaming-first UI language designed to be *spoken by LLMs*, rendered by a parser that treats agent output as **data, not code** (unknown components drop; invalid args drop; the rest renders), with `mergeStatements` for cheap conversational iteration and a `ToolSpec` surface shaped like ours already is.

The decisive factors for Lang-primary: (a) **no arbitrary HTML executes anywhere** — rendering is data-only, which preserves ADR-0014's "no page-authored code executes" posture without new sandbox machinery; (b) agents are *teachable* — `generateSystemPrompt` over our own library spec ships as the agent's skill page, so an agent that reads it speaks the language on the first try; (c) iteration is cheap — a patch is 2–3 Lang lines merged host-side via `mergeStatements`, not a full re-emit; (d) the blast radius is structurally closed (Addendum 24): reads power dashboards, the artifact's own CAS'd state is the only write primitive, and destructive intent routes through the chat loop where confirmation and attribution already live.

## Decision

### The artifact model

A page whose markdown body carries a ` ```wiki-artifact ` fenced block holding the Lang program; `wiki.artifact` frontmatter stamps it:

```toml
[wiki.artifact]
lang = "openui-lang"        # only mode in this scope
version = 5                 # server-stamped per accepted write
version_hash = "sha256:…"   # hash of the program; OCC handle
library = "wiki-core@1"     # which wiki component library version the program targets
updated_at / updated_by / automated   # identity-derived stamps, never client-writable
```

### View composition (never XOR)

`renderPageContent` no longer branches artifact-vs-markdown: it **always renders the prose** (with the fence removed by `SplitArtifactFences`) and prepends `<wiki-artifact-host>` when a fence exists. App on top, prose below. A fence-only page renders the app alone (the prose section is simply absent — the agent cleared the body via the same `SetArtifact`). Markdown content is never hidden behind the app: searched prose is visible on the page by construction (criterion 11).

### Service surface

- **`ArtifactService`** — `GetArtifact` / `SetArtifact` (the only program write; OCC via `expected_version_hash`) / `ClearArtifact` / `WatchArtifact` (server-streaming hot-swap driver).
- **`ArtifactStateService`** — dedicated, CAS'd (its own namespace-scoped hash, *not* page-markdown OCC), watchable: `GetState` / `MergeState` / `DeleteState` / `WatchState` under the reserved `wiki.artifact_state` subtree. The two-OCCs-are-two-streams rule from ADR-0009/0010 applies verbatim: source OCC and state OCC are independent.

### Server as validator+stamper

The Go server has no JS runtime, so **`mergeStatements` never runs server-side**. The agent emits a patch; the **host** merges (vendored lang-core) and submits the merged program via `SetArtifact` with the base version hash; the server validates, stamps, and lets `WatchArtifact` fire. There is no `MergeArtifactPatch` RPC. There are no `CommitArtifact`/`GetArtifactLog` RPCs — ADR-0016 page snapshots (the fence lives in the body) are the *only* history; restore rides `PageHistoryService`.

### The wiki component library (the teaching artifact)

`createLibrary`/`defineComponent`/`defineAction` over the wiki's own primitives, rendered as Lit components in the host: `Card`, `Stack`, `Text`, `Table`, `Chart`, `Input`, `Select`, `Button`, plus wiki-native renders (`Checklist`, `Map`, `Survey` as Lang components binding the existing custom elements — `<wiki-checklist>`, `<wiki-map>`, `<wiki-survey>`). Versioned `wiki-core@1` and pinned in `wiki.artifact.library`; the library spec + `generateSystemPrompt` output ship as `help_artifacts_lang` — the agent skill page.

**Renderer path (spike result, recorded per criterion 14): Path A chosen** — the wiki binds its own Lit components via lang-core's framework-free parser + runtime (`createParser(library.toJSONSchema())`, `createStore`/`initialize` for `$variables`, `evaluateRoot`/`evaluateElementProps` for prop evaluation, `createQueryManager(toolProvider)` for Query/Mutation, `mergeStatements` for patches). No Angular bridge, no React, no iframe: the host element (`<wiki-artifact-host>`), the library (`static/js/openui-lang/library.ts`), and the renderer (`static/js/openui-lang/renderer.ts`) are all Lit/TS in the standard frontend bundle. Verified behaviors pinned in unit tests: positional args map by zod key order (`Card("t", [children])`), `$var` declarations hydrate from `GetArtifactState` (persisted wins; pristine keys restore declared defaults), unknown components drop with structured `meta.errors` (never fatal), `@ToAssistant`/`@Set`/`@Reset`/`@OpenUrl` surface as action events the host executes.

### toolProvider is an allowlist

The host's `toolProvider` bridge is a hard-coded, test-pinned allowlist (not derived from tool discovery): all of `ArtifactStateService` (the artifact's own state, CAS'd), read-only discovery (`GetArtifact`, `ListPageVersions`, `SearchContent`, `GetPageOutline`, `GetFrontmatter`, `ListMapElements`, `ListItems`, `GetSurvey`). **Every** PageManagement/Checklist/Map/Survey/Connector mutation is excluded. A program calling a non-allowlisted tool gets `tool-not-found`; the parser renders the rest. A stored program can never touch another page's content or another user's data. Destructive intent routes through the chat door (`@ToAssistant`), where a human and full permissions already sit.

### Validation funnel (conservative floor)

The server has no Lang parser, so the Go validator (in `internal/grpc/api/v1/artifact.go`) is **structural only** — gross violations reject as `InvalidArgument`; partial-but-renderable programs accept, because the authoritative parse is browser-side lang-core and Lang's own runtime drops invalid statements gracefully. **Every write path flows through this funnel** — `SetArtifact` natively, and `/edit` → `UpdatePageContent`/`UpdateWholePage`/`CreatePage` via changed-fence detection at the write choke point — so no route can land an unvalidated (or un-stamped, or un-guarded) fence.

### Templating choke-point bypass

Lang programs contain `@`/`$` but **not `{{ }}`** — still, `SplitArtifactFences` lives inside the templating choke points (`templating.ExecuteTemplate`, `templating.ValidateTemplate`) so program content never reaches Go template parsing, and any current-or-future caller of those two functions inherits the bypass by construction. Its two outputs are load-bearing: (1) fence content → validation funnel / `GetArtifact`, never rendered as prose, never indexed; (2) prose remainder with the fence **removed** → goldmark, Bleve, executors. Without removal the prose section would render the whole program as a literal code block under the app.

### Not built (deliberately)

No iframes, no `/artifact/` asset route, no per-artifact directories, no CSP/nosniff/manifest MIME surface, no `window.wiki` shim, no multi-file `PutArtifactFile(s)` RPCs, no `MergeArtifactPatch`/`CommitArtifact`/`GetArtifactLog`. If Lang's library ever cannot express a real app need, an html/multi-file mode is *additive* to this model (same services, same frontmatter, same history — new `lang` value, one more host branch) and is a future issue, per the bridge-deletion plan discussed on #1272 Addendum 20.

## Consequences

### Positive

- Zero new containment machinery while enabling agent-authored interactive views: no iframe, no asset route, no CSP surface, no per-artifact dirs, no sandbox posture change to ADR-0014.
- The authoring language is *tailored to the author* (LLM-first, streaming, patch-cheap) with a self-correction loop (structured `onError`).
- State is a first-class service primitive (cross-device, CAS'd, watchable) — browser `localStorage` is not a state path (per-origin collisions, per-device only).
- Rendered view and search agree by construction (prose indexed == prose rendered, fence removed from both).

### Negative

- Expressiveness is capped at the wiki component library this scope ships (v1 primitives + wiki-native renders). Apps outside that vocabulary wait for library growth or a future html mode.
- One vendor dependency (`@openuidev/lang-core`, MIT) bundled into the frontend; upstream vocabulary changes must be tracked against the `wiki-core@1` spec pin.
- A conservative Go validator exists only to *never accept what the renderer chokes on*; it cannot be authoritative — the browser is.

### Neutral

- History/lifecycle/ACL come free from the page machinery (ADR-0016 snapshots; page ACL enforced on view; rename/trash/purge semantics unchanged).
- The frontmatter editor dialog is disabled on artifact pages (its load path strips reserved subtrees; its save path races `SetArtifact`); humans edit the full body via the normal `/edit` markdown editor, which routes fence changes through the same funnel.

## References

- ADR-0009 / ADR-0010 — reserved namespaces, dedicated CAS'd services, `wiki.*` policy
- ADR-0014 — limited security stance (unchanged by this design)
- ADR-0016 — page version history (the only history; snapshot machinery)
- Issue #1272 (body + addenda) — full design walkthroughs and workflow acceptance criteria (W1–W7)
- thesysdev/openui `@openuidev/lang-core` — parser, runtime, `mergeStatements`, `generateSystemPrompt`