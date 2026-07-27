# Improve MCP Surface Area

Findings from a full audit of the Wiki MCP surface. All seven findings
addressed in one PR, except Finding 6 (naming) which is filed as a tracking
issue because it is a breaking proto-field rename requiring its own migration.

## F1 — In-process /mcp serves stub descriptions (HIGH)

`internal/mcp/server.go` registers tools via `apiv1mcp.Register*Handler` but
never calls `mcpdocs.Decorate`, so ~60 tools surface `see (api.v1.description)`
stubs or empty descriptions to agents. The wiki-cli stdio path already calls
`Decorate`; the in-process HTTP path does not.

- **Skeleton:** `internal/mcp/server.go` — add `_ = mcpdocs.Decorate(s)` after
  the registration block.
- **Red:** `internal/mcp/server_test.go` — assert no registered tool's
  description contains `see (api.v1.description)` and none is empty.
- **Green:** wire `Decorate` and watch the red test pass.
- **Discovered during F1:** SurveyService RPCs had no `(api.v1.description)`
  extensions and no proto comments, so the codegen emitted empty descriptions
  on both MCP paths. Added a `service_description` and per-RPC `description`
  extensions to `survey.proto` for all 9 RPCs so the empty-description
  regression test passes. This is additive description curation, not a design
  change — same pattern as the already-ported services.
- **Refactor:** none expected.

## F2 — ScheduledTurnService exposed against design intent (MED)

`scheduled_turn.proto` explicitly says "It is NOT exposed as MCP." The
in-process path registers it anyway; the wiki-cli path does not. Remove
`RegisterScheduledTurnServiceHandler` from `internal/mcp/server.go`.

- **Red:** test asserting `api_v1_ScheduledTurnService_CompleteScheduledTurn`
  is absent from `tools/list`.
- **Green:** remove the registration line.

## F3 — ChatService exposes pool-only RPCs with no applicability signal (MED)

ChatService has 12 MCP tools. Several are pool-daemon-only
(`SendChatReply`, `EditChatMessage`, `ReactToMessage`, `SendToolCallNotification`,
`SendPlanNotification`, `SendTurnStatus`) and one is frontend-only
(`RespondToPermission`). An LLM has no signal that these are not for it.

Approach: add a new proto extension `(api.v1.exclude_from_mcp) = true` in
`mcp_options.proto`, annotate the pool-only/front-end-only ChatService RPCs,
and enforce exclusion in `mcpdocs.Decorate` by calling `s.DeleteTools(...)`
for excluded tools. This keeps registration codegen unchanged and makes the
exclusion a documented, per-RPC decision visible in the proto — the source
of truth. Agent-applicable RPCs (`SendMessage`, `GetChatStatus`,
`CancelAgentPrompt`, `ClearChat`, `RequestPermissionFromUser`) stay.

- **Skeleton:** add `bool exclude_from_mcp = 50006` extension to
  `mcp_options.proto`.
- **Red:** decorator test — after `Decorate`, excluded chat tools are absent.
- **Green:** decorator collects excluded tool names and calls `DeleteTools`.
- **Annotate:** mark the 7 pool-only/frontend-only ChatService RPCs.
- **Regenerate** proto (`buf generate`) and commit the generated files.

## F4 — Streaming-RPC MCP policy undocumented (LOW–MED)

Streaming RPCs are correctly dropped by the codegen, but the policy is
undocumented. Add a comment to `mcp_options.proto` stating that
server-streaming / bidi-streaming RPCs are not surfaced as MCP tools and
should instead be consumed via `wiki-cli` or the gRPC-Web transcoder.

## F5 — No catalog of the service/tool surface (MED)

`mcpdocs.Decorate` returns `[]ServiceDescription` but nothing publishes it.
Add a discoverable catalog:

- Mount an HTTP handler at `/mcp/catalog` (or reuse an existing route) that
  renders the `ServiceDescription` list as JSON (machine-readable) so agents
  and tooling can enumerate services + their curated use-cases.
- Add an embedded help page `help_mcp.md` enumerating each service with a
  one-line use-case (human-readable), linked from `help.md`.

## F6 — Request-field naming inconsistency (LOW — tracked, not fixed here)

Tool names are uniform; request *field* names vary (`page` vs `page_name` vs
`name` vs `survey_name` vs `connector_kind`). Fixing requires proto field
renames that break every caller — a separate breaking-change effort. File a
tracking issue.

## F7 — Help-doc coverage gaps (LOW)

Six services have no "For Agents" help section: ConnectorService,
FileStorageService, InventoryManagementService, PageImportService,
SystemInfoService, Frontmatter. Add concise "For Agents" sections to the
relevant existing `help_*.md` pages (or new pages where no page exists),
per the repo rule that help updates ride along with MCP changes.

## Verify

- `devbox run lint:everything`
- Smoke test: start the wiki, hit `/mcp` with `tools/list`, confirm no
  stubs, no ScheduledTurn, no excluded chat tools, catalog present.
- Plan-vs-code review subagent.
- Plan-vs-transcript review subagent.