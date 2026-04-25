# ADR-0010: The `wiki.*` Namespace for Wiki-Managed Metadata

## Status

Accepted

## Context

ADR-0009 establishes the general pattern for reserved frontmatter namespaces: the wiki reserves a top-level key, declares a dedicated service as the sole mutation entry point, and routes all writes through an internal funnel that maintains server-managed metadata.

The first reservation, `agent.*`, was named when the only such metadata happened to belong to scheduled agents (`agent.schedules.*`) or the chat agent's memory (`agent.chat_context.*`). When checklist metadata (per-item timestamps, attribution, deletion tombstones, per-list `sync_token`) needed the same protection — for the CalDAV bridge in #983 — extending `agent.*` was tempting but semantically wrong. Checklists are a wiki feature, not an agent feature; an agent and a human web user mutate the same checklist via the same dedicated service, and either may end up stamped as the author of a given item.

We need a top-level namespace whose name accurately reflects "wiki-managed metadata, not user-edited and not specific to any single subsystem."

## Decision

Introduce `wiki.*` as the top-level namespace for wiki-managed metadata that is not agent-specific.

### Reservation policy

`wiki.*` is reserved **wholesale** under the registry mechanism described in ADR-0009. Any path with top-level `wiki` is rejected by `MergeFrontmatter`, `ReplaceFrontmatter`, and `RemoveKeyAtPath`. The rejection error message switches on the second path segment to name the responsible dedicated service:

- `wiki.checklists.*` → "use `ChecklistService` instead"
- (future) `wiki.notes.*` → "use `NotesService` instead"
- Unknown second segment under `wiki.*` → generic "use the appropriate dedicated service"

### Initial occupant

`wiki.checklists.<list-name>.*` stores:

- `items.<uid>.{created_at, updated_at, completed_at, completed_by, automated}` — per-item bookkeeping
- `sync_token` — monotonic counter advanced on every list mutation
- `tombstones[]` — records of deleted items (lazy-GC'd 7+ days after deletion)
- `migrated_data_model` — a per-list flag set by the eager migration job, marking the list as already promoted to the new shape

The user-facing `checklists.*` (with items having `uid`, `text`, `checked`, `tags`, `sort_order`, `description`, `due`, `alarm_payload`) is **not** reserved by this ADR. Generic frontmatter tools may continue to mutate user data; only the metadata under `wiki.checklists.*` is protected.

### Future occupants

When a future feature needs the same reserved-metadata treatment, it should land its own service and register `wiki.<feature>.*` as a known second segment so the rejection error directs callers correctly. No changes to the registry's top-level key (`wiki`) or to `MergeFrontmatter`/`ReplaceFrontmatter` are needed; only the routing table that maps second segments to service names grows.

## Why not extend `agent.*`?

- **Semantic mismatch.** `agent.*` reads as "agent-related state." Checklists are mutated by humans (web UI), agents (scheduled), and protocol bridges (CalDAV) interchangeably; classifying their metadata under `agent.*` mis-cues every reader.
- **Future occupants.** Any non-agent metadata we might reserve next (page indexes, table-of-contents caches, derived search payloads) faces the same naming problem. A dedicated `wiki.*` namespace solves it now.
- **Backward compatibility for `agent.*`.** Existing `agent.schedules.*` and `agent.chat_context.*` stay where they are. Moving them into `wiki.agent.*` would be churn for no benefit — the names are accurate where they live.

## Why not nest under each feature?

An alternative would be `checklists.<name>._meta.*` (or `checklists.<name>.wiki.*`) — keep metadata locally nested under each feature.

Rejected because:

- **Scattered reservation logic.** The registry would need per-feature path-prefix entries, multiplying guard complexity.
- **Harder to grep.** Operators looking for "what's the wiki tracking on this page?" would need to know every feature's local naming convention.
- **No single ADR home.** Each feature would re-litigate the metadata-vs-user-data split locally.

A single top-level `wiki.*` namespace centralizes the invariant ("everything under `wiki.*` is server-managed") and lets the registry stay simple.

## Consequences

### Positive

- Clean semantic separation: `agent.*` for agent-related state, `wiki.*` for wiki-managed metadata, everything else for user data.
- Wholesale reservation means future occupants need zero registry changes (only a routing table addition).
- Consistent with the `agent.*` precedent without conflating concerns.

### Negative

- Two reserved top-level namespaces instead of one. Slightly more for new readers to learn — but the names are accurate, which makes them easier to remember.
- Existing `agent.*` is now arguably mis-named (status fields and chat memory are also "wiki-managed metadata" by the new criterion). A future cleanup ADR could migrate them to `wiki.agent.schedules.*` etc., but the cost would outweigh the benefit unless other forces drive it.

### Neutral

- Other frontmatter top-level keys (`title`, `description`, `blog`, `inventory`, `checklists`, etc.) are unchanged.

## Related Decisions

- ADR-0009: Reserved frontmatter namespaces and dedicated services (the pattern this ADR applies).
- #984 — checklist data model refactor (the home for `wiki.checklists.*`).
- #983 — CalDAV bridge (downstream consumer of the metadata stored under `wiki.checklists.*`).
