# Plan: #1320 — wiki-core@3 component expansion (remaining backlog)

## Context

Issue #1320 tracks the unshipped remainder of #1302. Already shipped (cross-check, do not rebuild):
Tabs, Accordion, Switch, Select docs, Dialog, Toast, Drawer (#1322), Tooltip (#1326+#1329) — all in v3.98.0.

Remaining backlog, from the reopened issue:

- P3 remainder (form controls): Checkbox/Radio, Slider, NumberInput, Textarea, DatePicker
- P4 (display/polish): Badge/Pill, Icon, Avatar, Skeleton, List
- P5: Image component (wiki upload surface), background-image support on Card
- P6: graphics stack tiers (Canvas2D/SVG declarative primitives, then Lottie, then regl)
- Popover: out of scope unless trivially expressible (Tooltip already shipped).

## Non-negotiable constraints (from issue comment + repo rules)

- Lit components only; design tokens from page-app-design-system.ts; follow #1298/#1306 patterns.
- Every slice: e2e coverage + help docs update (internal/syspage/embedded/help_page_apps.md).
- Devbox scripts for tests: devbox run fe:test, fe:lint, fe:typecheck, e2e:test -- <specs>.
- PRs: each component family opens its own PR with Refs #1320 (final slice Closes #1320).
- TDD: skeleton, red, green, refactor.

## Serialization model

All components live in shared files: static/js/openui-lang/library.ts, library.test.ts,
page-app-design-system.ts, internal/syspage/embedded/help_page_apps.md.
Parallel workers editing these would conflict, so components run one family per sequential wave;
validation checks (test/lint/typecheck/e2e) are independent, so they fan out in parallel.

## Waves

### Wave A — P3 form controls (Refs #1320)

- A1 Checkbox/Radio: Checkbox(label, checked, onChange); Radio group as one component.
- A2 Slider: Slider(label, min, max, step, value, onChange) bound to a $variable.
- A3 NumberInput: numeric Input; binds $value.
- A4 Textarea: multiline Input; binds $variable.
- A5 DatePicker: date-validated Input adapter (native input type=date if clean in stateless render).

### Wave B — P4 display/polish (Refs #1320)

- B1 Badge/Pill: Badge(text, variant) — inline pill, Button-like variant enum.
- B2 Icon: Icon(name) — curated set (check, close, info, warn, error, chevron); no arbitrary fetches.
- B3 Avatar: Avatar(src, initials, size) — src from FileStorageService URL; initials fallback.
- B4 Skeleton: Skeleton(kind) — shimmer loading block; pure CSS.
- B5 List: List(items, ordered) — bullet/numbered list; plain strings or Text children.

### Wave C — P5 imagery (Refs #1320)

- C1 Image: Image(src, alt, width, height, fit).
- C2 background-image on Card: extend CardSchema with a validated backgroundImage prop.

### Wave D — P6 graphics harness

- D1 Canvas2D/SVG declarative primitives: scope-gated to what the validation funnel safely renders.
- D2 Lottie/regl: deferred unless D1 reveals a clean path; document as deferred.

### Validation checks (each its own task, parallel)

After all component PRs merge, one task per check — these are independent and fan out in parallel:

- **fe:test** — `devbox run fe:test`, full suite, report pass/fail counts.
- **fe:lint** — `devbox run fe:lint`, report violations with file:line.
- **fe:typecheck** — `devbox run fe:typecheck`, clean or error output.
- **opengrep:lint** — `devbox run opengrep:lint`, convention checks with rule ids.
- **e2e:scoped** — `devbox run e2e:test -- <page-app component specs>`, per-spec pass/fail.
- **lint:everything** — `devbox run lint:everything`, last gate before push.

### Screenshot + PR body

Capture one screenshot per component (Checkbox, Radio, Slider, NumberInput, Textarea, DatePicker,
Badge, Icon, Avatar, Skeleton, List, Image, Sketch) against the running server or Storybook, then
assemble the final PR description with an embedded screenshot per component. Required by the user;
rides the consolidation PR.

**Capture path** (per component-pr-screenshots skill): Storybook iframe screenshots via
puppeteer chrome (`~/.omp/puppeteer/chrome/...`), URL `/iframe.html?id=<story-id>&viewMode=story`,
clipped to the element rect. Host PNGs IN-REPO at `docs/screenshots/<component>/N-<state>.png`
(GitHub raw URLs — wiki uploads fail GitHub camo). PR body via `gh api ... --input body.json`;
verify each raw URL 200s.

**Prerequisite: stories must exist first.** Only `countdown.stories.ts` and `layout.stories.ts`
exist today. Task `task_c92305c31524` creates stories for all 13 new components (one story per
major visual state, following layout.stories.ts patterns) BEFORE the screenshot task runs.
Wire: stories task → screenshot task → consolidation PR.

### Pre-ship gate

- All component PRs merged, then final PR Closes #1320:
  - consolidated screenshots in PR description (one per component),
  - release cut + deploy verification via the GetVersion commit probe,
  - close comment summarizing shipped vs deferred.

## Orchestrator notes

- Fan out one Orca worker per component family; serialize on shared files, parallelize validations.
- Every dispatched worker: e2e spec + help docs update in scope; devbox scripts only.
- Post-implementation validation fan-out (parallel, one per check):
  fe:test / fe:lint / fe:typecheck / opengrep:lint / scoped e2e specs.
- **API alignment (from PLAN-1302.md, the project's own agreed roadmap)**:
  - `Radio` in PLAN-1302 is called `RadioGroup` — the shipped component is `Radio` (label? first, options, selected, onSelect). Keep `Radio`; note the rename in PR body.
  - `Skeleton` in PR5 is FOR Query loading states — must accept a loading/`Query`-fed pattern, not only a standalone placebholder block.
  - `Badge` kinds use the existing `--color-*-soft` token pairs mentioned in PLAN-1302 G6 (no parallel *-soft family).
  - `Image(fileHash|src, alt?, fit?, aspectRatio?)` — `fileHash` variant over the wiki file-upload surface preferred over bare `src`; Card `image:`/`minHeight:` props stay DEFERRED per PLAN-1302 G5 (unsanitized `url()` interpolation was the original drop reason; any reintroduction MUST pass sanitation review).
  - `Sketch` (PR7 graphics tier 1) is opt-in declarative canvas/SVG primitives; shapes/gradients/particles with $state-fed animation params; no arbitrary JS; Lottie/regl deferred.
