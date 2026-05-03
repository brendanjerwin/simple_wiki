# ADR-0010: The `wiki.*` Namespace for Wiki-Managed Data

## Status

Accepted

## Context

ADR-0009 establishes the general pattern for reserved frontmatter namespaces: the wiki reserves a top-level key, declares a dedicated service as the sole mutation entry point, and routes all writes through an internal funnel.

Issue #984 needed a place to put checklist data — items, attribution, sync tokens, tombstones — such that:

1. Generic frontmatter tools (`MergeFrontmatter`, `ReplaceFrontmatter`, `RemoveKeyAtPath`) **cannot** mutate any of it. The funnel's correctness guarantees (per-item ULID, server-stamped timestamps, attribution from the auth principal, monotonic `sync_token`) are easy to bypass if even one byte of checklist state lives in a writable namespace.
2. The CalDAV bridge (#983) and other sync clients can ETag the whole list with one timestamp.
3. The split between user-mutable fields (`text`, `checked`, `tags`, `sort_order`, …) and server-derived fields (`created_at`, `updated_at`, `completed_at`, `completed_by`, `automated`) is **internal to `ChecklistService`** — it is not visible in the persistence layout.

The first attempt at this ADR put user-mutable item data at `checklists.<list>.items[]` and only the metadata at `wiki.checklists.<list>.*`. That was wrong: `checklists.*` remained writable through the generic frontmatter API, and a `MergeFrontmatter` write to it would skip the funnel's bookkeeping. The point of the dedicated service is undermined the moment any byte of its state can be reached around it.

## Decision

Introduce `wiki.*` as a reserved top-level frontmatter prefix. **Everything** the wiki manages on behalf of a feature lives under it.

For checklists specifically, the layout is:

```toml
[wiki.checklists.grocery-list]
sync_token = 47
updated_at = "2026-04-25T17:14:00Z"
migrated_data_model = true

[[wiki.checklists.grocery-list.items]]
uid          = "01HXXXXXXXXXXXXXXXXXXXXXXX"
text         = "Buy milk"
checked      = false
tags         = ["urgent"]
sort_order   = 1000
description  = "the brand Kirsten likes"
due          = "2026-04-30T17:00:00Z"
created_at   = "2026-04-25T13:00:00Z"
updated_at   = "2026-04-25T17:14:00Z"
completed_at = "2026-04-25T17:14:00Z"
completed_by = "alice@example.com"
automated    = false

[[wiki.checklists.grocery-list.tombstones]]
uid        = "01HXOLDXXXXXXXXXXXXXXXXXXX"
deleted_at = "2026-04-24T08:00:00Z"
gc_after   = "2026-05-01T08:00:00Z"
```

There is no `checklists.<list>.items[]` outside `wiki.*`. The legacy `checklists.*` top-level namespace is **migrated and removed** by the eager migration in `migrations/eager/checklist_data_model_migration.go` — that pass moves items into `wiki.checklists.<list>.items[]`, assigns ULIDs / sort orders / timestamps, and deletes the source `checklists.<list>` subtree on the same write. Once a page is stamped `wiki.checklists.<list>.migrated_data_model = true`, it has no `checklists.*` state at all.

`ChecklistService` is the only mutation entry point. The funnel internally distinguishes user-mutable fields (which it accepts from input) and server-derived fields (which it stamps), but the on-disk shape does not — both kinds live in the same item map. This is fine because the entire `wiki.*` namespace is wholesale-rejected by the generic frontmatter tools (registry-driven, per ADR-0009 and `internal/grpc/api/v1/reserved.go`).

### Reservation policy

`wiki.*` is **wholesale reject** under the registry mechanism described in ADR-0009. Any path with top-level `wiki` is rejected by `MergeFrontmatter`, `ReplaceFrontmatter`, and `RemoveKeyAtPath` with an `InvalidArgument` whose message names the responsible dedicated service. The rejection is keyed on the second path segment:

- `wiki.checklists.*` → "use `ChecklistService` instead"
- (future) `wiki.notes.*` → "use `NotesService` instead"
- Unknown second segment under `wiki.*` → generic "use the appropriate dedicated service"

`GetFrontmatter`, `MergeFrontmatter` response, and `ReplaceFrontmatter` response **strip** any `wiki.*` keys before returning, so a frontmatter editor never sees the data and never round-trips it. Reading the raw page (e.g. `PageManagementService.ReadPage`) returns the full TOML for debugging — that is by design.

### Future occupants

When a future feature needs the same dedicated-service treatment, it lands its own service and registers `wiki.<feature>.*` as a known second segment. No changes to the registry's top-level key (`wiki`) or to `MergeFrontmatter`/`ReplaceFrontmatter` are needed; only the rejection-message routing table grows.

## Why not extend `agent.*`?

`agent.*` reads as "agent-related state." Checklists are mutated by humans (web UI), agents (scheduled), and protocol bridges (CalDAV) interchangeably; classifying their data under `agent.*` mis-cues every reader. Future non-agent metadata (page indexes, table-of-contents caches, derived search payloads) faces the same naming problem. A dedicated `wiki.*` namespace solves it. `agent.*` keeps its current scope (schedules, chat_context).

## Why not nest user data outside `wiki.*` and only keep metadata inside?

The original draft of this ADR did exactly that — items at `checklists.<list>.items[]`, metadata at `wiki.checklists.<list>.*`. **Rejected**, because `checklists.*` then remains writable through the generic frontmatter API:

- A `MergeFrontmatter` write to `checklists.<list>.items` mutates user-visible state but does **not** advance `wiki.checklists.<list>.sync_token`, does **not** stamp attribution, and does **not** advance per-item `updated_at`. CalDAV ETags lie. Reflection data lies. The funnel's whole point is gone.
- Splitting the persisted shape across two namespaces also fragments grep-ability ("where does `Groceries/Household` live? `[checklists.Groceries/Household]` AND `[wiki.checklists.Groceries/Household]`?"), and asks every developer reading the file to mentally rejoin them.
- `ChecklistService` already understands the user-mutable / server-derived split internally — there is no benefit to externalizing it on disk.

Single-namespace wins on correctness, simplicity, and grep.

## Consequences

### Positive

- Generic frontmatter tools cannot mutate checklist data, period. The funnel is the only path. ADR-0009's invariants hold by construction.
- The persisted shape is one TOML subtree per checklist. Easy to grep, easy to reason about.
- CalDAV ETag is just `wiki.checklists.<list>.updated_at` — no joining required.
- Future metadata occupants under `wiki.*` (notes, page indexes) inherit the same protections automatically.

### Negative

- The eager migration is destructive: it moves items out of `checklists.*` and removes the source subtree. Users who manually edited `checklists.*` between deploy and the migration's first sweep would have their edits picked up — but generic tools cannot do that any more after the migration anyway, so the window is small.
- Pages with un-migrated `checklists.*` data still exist on disk until the eager migration sweeps them. The SSR macro and `ChecklistService` both fall back to reading `checklists.*` until the flag is set on a given list.
- Existing scheduled-agent prompts that wrote `checklists.*` via `MergeFrontmatter` no longer work. The agent-prompt audit in slice 10 already removed those (or pointed them at `ChecklistService`); any remaining ones surface as `InvalidArgument` and must be rewritten.

### Neutral

- The `agent.*` namespace is unchanged. Other top-level keys (`title`, `description`, `blog`, `inventory`) are unchanged.

## Related Decisions

- ADR-0009: Reserved frontmatter namespaces and dedicated services (the pattern this ADR applies).
- #984 — checklist data model refactor (the home for `wiki.checklists.*`).
- #983 — CalDAV bridge (downstream consumer of the data stored under `wiki.checklists.*`).
