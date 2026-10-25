# Plan — Issue #1299: Page-shrink circuit breaker for whole-page writes

## Incident recap

`agentos_cluster_agent_infrastructure` went from 1,136,087 bytes to 1,134 in one agent
write via a whole-page save composed from a partial read. Recovery (RestorePageVersion)
worked; this change adds prevention. Issue: https://github.com/brendanjerwin/simple_wiki/issues/1299

## Design decisions (settled with user)

One shared **shrink breaker** applied **inside the atomic write funnel** on both
full-content write paths, unified at the existing wipe-guard threshold, with an explicit
override flag and size attestation in responses.

### API design, aligned to published conventions (AIP)

- **`allow_shrink: bool` (optional, proto field 4) on `UpdatePageContentRequest` and
  `UpdateWholePageRequest`.** Follows AIP-134's `allow_missing` convention: a boolean
  request field named `allow_*` that relaxes a safety check on an Update method.
  (AIP-135's `force` is delete-specific semantics — "force" reads as steamrolling;
  `allow_shrink` states exactly what is permitted.) When false/unset: breaker armed.
  When true: breaker bypassed, write proceeds.
- **Rejection: `FAILED_PRECONDITION`** per gRPC/AIP-193 error conventions (caller must
  take explicit action before retrying). Matches the existing #1126 wipe-guard code.
  Error message states: current size, new size, the percent, that `allow_shrink=true`
  overrides, and the safer alternatives (UpdatePageContent with old_content_markdown,
  MergeFrontmatter, ClearPageContent).
- **`page_byte_size` in both write responses** (additive field) so callers attest the
  outcome of their own writes without a re-read. Additive optional fields = wire
  compatible.
- **Threshold (unified):** reject when `new_size × 10 < current_size` (i.e. new content
  < 10% of current) AND `current_size ≥ 1024` bytes. Net shrink for both modes:
  - `UpdateWholePage`: new whole markdown vs current markdown.
  - `UpdatePageContent`: resulting page (full replacement = `new_content_markdown`;
    snippet mode = `strings.Replace` result) vs current markdown.
- **Atomic check placement:** the breaker runs inside the `ModifyFrontMatterAndMarkdown`
  funnel modifier (write lock held), comparing candidate vs current markdown. This
  closes the TOCTOU window in the #1126 pre-check (which reads current content before
  the write) and inherits into any future funnel-based write path. The #1126 pre-request
  `checkWholePageWipeGuard` is **removed** — clean cutover to the atomic breaker.
- **Explicitly out of scope:** `UpdatePage` (RPC exists in proto but has no server
  implementation — unimplemented stub; guard N/A until it exists),
  `CreatePage` (no prior content to shrink), `ClearPageContent` and
  page-app prose guard (explicit-destructive by contract),
  `RestorePageVersion` (the recovery tool — must not trap).
- **Whole-page OCC hash (issue item 3): already satisfied** by #1198/#1204 — every read
  shape returns a full-page hash (frontmatter + markdown) and both write paths verify it
  atomically. No attestation mechanism needed; a size attestation would force full-page
  fetches and defeat `ReadPageSection`. Documented in the help update, not new code.
- **Item 5 (version history attribution): already live** (author, is_agent, byte_size,
  sha256 in `server/pagestore/history.go`). No change.
- **Telemetry: rejection only** (user decision) — the error is the signal; no metric/log.
- **Yield to page-app prose guard:** the breaker skips page-app-shaped writes (fence in
  current or candidate content) so the prose guard's more specific error surfaces;
  the breaker must not shadow an existing guard.
- **E2E impact:** full-replacement resets in e2e harnesses (`setupTestPage` and the
  shared page-app helper `setupPage`, plus alerts/external-links/xss/auto-refresh/
  view-toggle/frontmatter-a11y specs) are deliberate fixture resets and carry
  `allowShrink: true`. `update-page-content.spec.ts` gained breaker cases:
  a 400-status rejection asserting both sizes + override, and an override success
  asserting `pageByteSize` and the written content.
- **Frontend (`wiki-editor.ts`):** on a shrink-breaker `FAILED_PRECONDITION`, show a
  confirmation dialog naming the dropped percentage; a confirmed retry resends with
  `allow_shrink: true`. Humans legitimately trim large pages; the breaker must have a
  first-class, confirm-guarded escape at the UI edge.

## Ordered steps (Skeleton → Red → Green → Refactor per capability)

### 1. Proto + codegen (unblocks 2–4)
- `api/proto/api/v1/page_management.proto`:
  - `UpdatePageContentRequest`: `optional bool allow_shrink = 5;` (field 4 is already
    `old_content_markdown`; comment: breaker override, see below).
    `UpdatePageContentResponse`: `int64 page_byte_size = 4;`
  - `UpdateWholePageRequest`: `optional bool allow_shrink = 4;`
    `UpdateWholePageResponse`: `int64 page_byte_size = 4;`
  - Refresh the `api.v1.description` strings on both methods: state the breakers
    (10% / 1KB floor), `allow_shrink` override, and `page_byte_size` attestation.
- Regenerate via `devbox run go:generate` (slow: wiki-cli cross builds; use a long
  timeout, background if needed). Commit regenerated
  `gen/go/**` + `static/js/gen/**`.

### 2. Breaker core (TDD)
- Skeleton + tests in `internal/grpc/api/v1/server_test.go` (Ginkgo style, house rules:
  Arrange/Act in BeforeEach, assert-only It blocks, blank lines between blocks):
  - `UpdatePageContent` full replacement: 9KB page, 50-byte payload →
    `FailedPrecondition`, message contains both sizes, percent, `allow_shrink`,
    alternatives; write suppressed.
  - Same + `allow_shrink: true` → success, content written.
  - Snippet replacement: `old_content_markdown` = 4KB section replaced by 20 bytes on
    a ≥1KB page (net < 10%) → breaker trips; tiny page (<1KB) → passes.
  - `UpdateWholePage`: existing #1126 guard tests must keep passing with the atomic
    breaker (same codes/message substrings); replace the pre-check-only fixtures with
    atomic-path ones; concurrent-markdown TOCTOU fixture: breaker sees the *funnel's*
    current markdown, not the pre-read.
- Red: run `devbox run go:test -- ./internal/grpc/api/v1/` (scoped) → fails.
- Green in `page_management.go`:
  - New `checkShrinkBreaker(currentSizeBytes int, candidateMarkdown wikipage.Markdown,
    allowShrink bool) error` — pure function; threshold constants shared with the
    removed wipe guard (10%, 1KB).
  - Call it inside both funnels after the candidate is computed; wire `req.GetAllowShrink()`.
  - Remove `checkWholePageWipeGuard` + its call site; delete obsolete #1126 pre-check
    fixtures that assert pre-read behavior (per clean-cutover rule).
- Refactor: single shared implementation, no flag-parameter variants.

### 3. Response byte size (TDD, same test file)
- Assert `resp.PageByteSize` equals the stored/new content byte length on success for
  both RPCs (Ginkgo assertions in the success `When` blocks created in step 2).

### 4. Frontend escape hatch (`static/js/web-components/wiki-editor.ts`)
- Read the save flow first. On shrink `FAILED_PRECONDITION`: dialog quoting the error's
  percentages; confirmed → resend same request with `allowShrink: true`; cancelled →
  error view unchanged.
- Unit tests next to the component (`.test.js`, sinon timers/stubs per repo rules).

### 5. Help + docs (same change, not a follow-up)
- `internal/syspage/embedded/help_handling_large_pages.md`: rename "UpdateWholePage
  circuit breaker" section → "Shrink circuit breakers": covers both RPCs, thresholds,
  `allow_shrink`, `page_byte_size` self-check, and the section-read guidance (whole-page
  hash already protects clipped readers — cite that the OCC hash covers the full page).
- `help_mcp.md` if it describes the update tools (verify first; only touch if it does).
- Proto descriptions (step 1) carry the same contract to MCP descriptions.

### 6. Verification gates
- `devbox run go:test` (full)
- `devbox run fe:test -- <touched tests>`; `devbox run fe:lint`; `devbox run fe:typecheck`
- `devbox run lint:everything`
- [Substituted during execution] The live smoke did NOT run: `devbox services` could not bind a
  reachable server in this sandbox (repeated starts returned 000 on :8050). In its place, scoped
  e2e runs exercise the breaker over the real Connect transport. This covers `UpdatePageContent`
  fully (both modes); the `UpdateWholePage` live path was NOT exercised against a running process
  end-to-end — its breaker is covered by Go handler tests only, and the wire-level e2e
  `page_byte_size` assertion covers the UpdatePageContent path, not UpdateWholePage.
- Original step (not executed): throwaway script hitting the gRPC surface: create 2KB page,
  attempted 100-byte `UpdateWholePage` → expect FailedPrecondition; with
  `allow_shrink` → success; assert `page_byte_size` in responses.

### 7. End-of-work reviews (mandatory)
- Plan-vs-code review subagent (plan file + `git diff main...HEAD`).
- Plan-vs-transcript review subagent (plan file + session transcript).
- Resolve or explicitly waive findings. Then commit/push/PR per repo flow.