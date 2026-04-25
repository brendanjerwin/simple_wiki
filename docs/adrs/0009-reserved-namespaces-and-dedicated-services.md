# ADR-0009: Reserved Frontmatter Namespaces and Dedicated Services

## Status

Accepted

## Context

Wiki pages store structured data in TOML frontmatter at the top of each `.md` file. Most fields are "loose" — `title`, `description`, ad-hoc keys, simple structured config like `[blog]` or `[inventory]` — and are happily edited by users, agents, and tools using the generic frontmatter API (`MergeFrontmatter`, `ReplaceFrontmatter`, `RemoveKeyAtPath`).

Some structured data, however, has **non-trivial invariants** the wiki itself enforces:

- `agent.schedules.*` — cron expressions must validate; status fields (`last_run`, `last_status`, `last_error_message`, `last_duration_seconds`) are populated by the wiki on each fire and must not be writable from outside.
- `agent.chat_context.*` — `last_updated` is server-stamped; `background_activity` is a rolling log appended by the wiki when scheduled fires terminate.
- `wiki.checklists.*` (introduced alongside this ADR) — items have stable identity (`uid`); per-item `created_at`/`updated_at`/`completed_at`/`completed_by`/`automated` and per-collection `sync_token` must advance correctly on every mutation for CalDAV (#983) and reflection use cases to work.

When such data was reachable through generic frontmatter tools, two problems arose:

1. **"Remember to bump" hazards.** Every mutation path had to remember to advance the wiki-managed metadata. Adding a new mutation path silently broke downstream consumers.
2. **Spoofing risk.** An external write could set `completed_by = "sean"` to fake human attribution, or stamp a stale `updated_at` to confuse sync clients.

`agent.schedules.*` and `agent.chat_context.*` were addressed ad-hoc by reserving the `agent.*` namespace and routing all mutation through `AgentMetadataService`. This ADR formalizes that pattern and applies it to `wiki.checklists.*` (see ADR-0010 for the namespace decision).

## Decision

Establish a uniform policy for any frontmatter subtree with non-trivial invariants:

### 1. Reserved namespace, registry-driven

Generic frontmatter tools (`MergeFrontmatter`, `ReplaceFrontmatter`, `RemoveKeyAtPath`) consult a small in-process registry of reserved top-level keys. A request that touches a reserved key is rejected with `InvalidArgument`, and the error message names the dedicated service the caller should use instead.

The registry is wholesale: registering top-level key `wiki` reserves `wiki.*` in its entirety. Future occupants (`wiki.notes.*`, `wiki.toc.*`) inherit the protection without guard-code changes.

### 2. Dedicated service

A typed gRPC/MCP service is the sole legitimate mutation entry point for the namespace. It accepts user-mutable fields, derives wiki-managed fields from authoritative sources (auth principal, server clock, diff state), and writes both the user-mutable subtree and the reserved metadata subtree atomically.

### 3. Internal mutator funnel

Inside the wiki process, every caller of the dedicated service — gRPC handlers, web UI form handlers, future protocol bridges (e.g. CalDAV in #983) — converges on a single internal mutator function. The funnel performs the diff, attribution derivation, and atomic write. The funnel is the only code path that writes the reserved subtree.

### 4. Silent strip on dedicated-service input

When the dedicated service accepts a payload that includes wiki-managed fields, it strips them silently — input never overrides server-derived values. (This matches how `AgentMetadataService.UpsertSchedule` strips `last_run` etc.)

### 5. Identity-derived attribution

Wiki-managed fields that record "who did this" (`completed_by`) and "was this automated" (`automated`) are derived from the wiki's identity subsystem, not from input or protocol-layer details. The funnel calls `ctx.IdentityValue().Name()` and `ctx.IdentityValue().IsAgent()`. Each entry-point's auth handler populates the identity; the funnel stays protocol-agnostic.

### 6. Optimistic concurrency (OCC)

Mutating RPCs accept an optional `expected_updated_at` timestamp. The funnel compares it to the persisted value before applying the mutation; mismatch returns `codes.FailedPrecondition`. Callers that observe the precondition failure should refetch the current state and retry.

### 7. User-data namespaces remain user-mutable by default

The reservation applies to the **metadata** namespace (the wiki-managed bookkeeping), not necessarily to the user-data namespace it shadows. For checklists, `wiki.checklists.*` is reserved but `checklists.*` (the user-facing items) remains writable through the generic frontmatter tools. Raw frontmatter writes to `checklists.*` continue to work but bypass the funnel — they will not advance `sync_token` or stamp attribution. The dedicated service is the *correct* path; the generic tools are tolerated for tooling and human edits.

This is a deliberate trade-off documented here so future reservations can be evaluated consistently. Reserve metadata aggressively; reserve user data only when bypass would corrupt invariants the wiki cannot rebuild from the data alone.

## Currently Reserved Namespaces

| Top-level key | Service | Funnel | Why reserved |
| --- | --- | --- | --- |
| `agent.*` | `AgentMetadataService` | `AgentScheduleStore`, `AgentChatContextStore` | Cron status, chat memory, background activity log |
| `wiki.*` | `ChecklistService` (and future siblings) | `checklistMutator` | Per-item timestamps, attribution, sync tokens |

## Consequences

### Positive

- Correctness by construction. Adding a new mutation path inherits invariant maintenance for free; no convention to remember.
- Spoofing impossible for reserved metadata. The wiki derives all attribution from auth context.
- Surfaces in tooling. `wiki-cli describe api.v1.<service>` documents the contract; `wiki-cli list` shows users the canonical entry points.
- Test discipline. The funnel has one place to assert metadata invariants, instead of N call sites.

### Negative

- More API surface. Each reserved namespace needs a service, breaking the convenience of "just use `MergeFrontmatter`."
- Migration cost when extending. Reserving a previously-loose namespace requires auditing every caller.
- Coupled to the wiki's auth provider. The funnel derives attribution from identity context, so this works only inside the wiki process — external tools cannot mutate reserved namespaces directly. (This is by design.)

### Neutral

- Generic frontmatter tools remain the right answer for loose data. Don't reserve a namespace unless there's a real invariant to defend.

## When to Apply This Pattern

Use the reserved-namespace + dedicated-service + funnel pattern when **any** of the following are true:

- The wiki must maintain server-stamped fields (timestamps, counters, sync tokens).
- The wiki must derive fields from auth context (attribution).
- The data has identity (UIDs, sort-stable ordering) that external writes must not corrupt.
- Downstream consumers (sync protocols, scheduled fires, indexes) depend on metadata advancing correctly on every mutation.

Do **not** apply this pattern for:

- Loose, user-edited config (`title`, `description`, blog/inventory metadata).
- Data the wiki only reads but never enforces invariants on.

## Related Decisions

- ADR-0001: gRPC and gRPC-Web APIs (the transport this pattern uses).
- ADR-0010: The `wiki.*` namespace for wiki-managed metadata (the first concrete application of this ADR's pattern).
- `help_scheduled_agents` — original reserved-namespace policy for `agent.*`, now folded into this ADR.
- #984 — checklist data model refactor (the issue that prompted formalization).
- #983 — CalDAV bridge (downstream consumer that depends on the funnel for sync correctness).
