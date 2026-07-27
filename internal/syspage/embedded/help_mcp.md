+++
identifier = "help_mcp"

[wiki]
system = true
+++

#help #agents

# {{.Title}}

The wiki exposes its API as **MCP tools** so AI agents (Claude Code, `wiki-cli mcp`, any MCP client) can read and mutate pages, checklists, maps, surveys, history, and more. This page is the human-readable catalog of what's available. For a machine-readable catalog, fetch `https://<your-wiki>/mcp/catalog`.

## Two MCP transports

| Transport | Endpoint | Use when |
|---|---|---|
| **Streamable HTTP** | `https://<your-wiki>/mcp` | Claude Code or any MCP-over-HTTP client. In-process, no subprocess. |
| **stdio** | `wiki-cli mcp --url https://<your-wiki>` | Local tooling that speaks MCP over stdin/stdout. |

Both surfaces advertise the same tools with the same curated descriptions. Tool names follow the pattern `api_v1_<ServiceName>_<MethodName>`.

## Service catalog

Each service owns a reserved frontmatter namespace or a distinct concern. The descriptions below are the `(api.v1.service_description)` proto extensions — the same text agents see in their tool list.

| Service | Use-case |
|---|---|
| `PageManagementService` | CRUD for wiki pages: markdown content paired with structured frontmatter. Includes server-side rendering, identifier generation, templates, and large-page section reads. |
| `Frontmatter` | Reads and writes structured frontmatter as a JSON-like object. Writes touching the top-level `agent` namespace are rejected — use `AgentMetadataService` for those. |
| `AgentMetadataService` | Agent-managed page state: cron schedules, conversation memory, and the rolling background-activity log. The only legitimate mutation path for the `agent.*` frontmatter namespace. |
| `SearchService` | Discovery over wiki pages: Bleve full-text search plus exact-match frontmatter-key lookups with sort. |
| `PageHistoryService` | Version history for every page: list, read, restore, diff, and full-text search across all past versions. |
| `ChecklistService` | Item-level checklist mutation with server-derived attribution, optimistic concurrency, and sync-token bookkeeping. Owns `wiki.checklists.*`. |
| `MapService` | First-class wiki maps: markers, polygons, circles, tracks. The only mutation entry point for `maps.<name>` and `agent.maps.<name>`. |
| `SurveyService` | Survey configuration (question + fields) and user response writes. Owns `wiki.surveys.<name>`. |
| `ConnectorService` | Bind wiki checklists to remote lists (Google Keep, Google Tasks). Per-user OAuth, sync, dead-letter management. |
| `FileStorageService` | Upload, query, and delete attached files. Returns content hashes and URLs. |
| `InventoryManagementService` | Track physical inventory items and containers for a workshop/lab wiki. Create item pages, move items, look up locations. |
| `PageImportService` | Bulk-import wiki pages from CSV. Dry-run preview then background job. |
| `SystemInfoService` | Runtime info: commit hash, build time, Tailscale identity, background-job queue status. |
| `ChatService` | Brokers chat between browsers and an AI agent. Only a subset of RPCs are exposed as MCP tools (the pool-daemon-only and frontend-only RPCs are excluded). |

## What is NOT exposed as MCP

- **`ScheduledTurnService`** — pool↔server bridge for headless agent turns. Never exposed as MCP; it is internal to the scheduler.
- **Streaming RPCs** (`WatchPage`, `WatchList`, `StreamJobStatus`, `SubscribeChat`, etc.) — MCP's request/response tool model has no representation for an open stream. Use the gRPC-Web transcoder or `wiki-cli` for streaming updates.
- **Pool-only / frontend-only ChatService RPCs** — `SendChatReply`, `EditChatMessage`, `ReactToMessage`, `SendToolCallNotification`, `SendPlanNotification`, `SendTurnStatus`, and `RespondToPermission` are annotated `(api.v1.exclude_from_mcp)` and removed from the tool surface. Agent-applicable chat tools (`SendMessage`, `GetChatStatus`, `CancelAgentPrompt`) remain.

## Discovering tools at runtime

Agents should call `tools/list` on the MCP endpoint to see the live tool set with descriptions and input schemas. The `/mcp/catalog` URL returns just the service-level descriptions (no schemas) as JSON for lightweight enumeration.

## See Also

- [[help-macro-checklist]] — Checklist macro and `ChecklistService` tool reference
- [[help-macro-map]] — Map macro and `MapService` tool reference
- [[help-macro-survey]] — Survey macro and `SurveyService` usage
- [[help-page-history]] — `PageHistoryService` tool reference
- [[help-search]] — `SearchService` and `#tag` query syntax
- [[help-scheduled-agents]] — `AgentMetadataService` schedules
- [[help-chat]] — Chat panel and `ChatService`
- [[help-templating]] — Template macros and `PageManagementService.ListTemplates`