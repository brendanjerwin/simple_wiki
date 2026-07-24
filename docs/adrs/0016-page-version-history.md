# ADR 0016: Page Version History Architecture

## Status

Accepted

## Context

The wiki has no edit history. Pages mutate via `WriteFrontMatter`, `WriteMarkdown`, `ModifyMarkdown`, `ModifyFrontMatterAndMarkdown`, and soft-delete. The existing `__deleted__/` trash system handles deletion restore, but not edit history. Once a page is overwritten, the prior content is gone. Users cannot view, diff, search, or restore prior versions of a page.

The wiki needs a robust backend that captures every mutation as an immutable version snapshot, plus APIs to list, read, restore, diff, and search history. Automatic GFS-style decimation prevents unbounded disk growth without manual intervention.

## Decision

### Capture Model: Outgoing-State Snapshots

History is captured inside `Store.ModifyOrCreatePage` — the single function through which all write paths flow. While holding the page lock, the **outgoing** content (the state being replaced) is written as a version snapshot **before** the new content overwrites it.

Key invariants:
- The current live file is always the latest version. No version entry duplicates live content.
- Every version entry represents a state that was *replaced*.
- No-op writes (identical content) skip capture, avoiding history spam from migration re-saves.
- First write to a new page captures nothing (no prior state exists).
- Soft delete captures the final live content before trashing.

This trades disk space for robustness and simplicity. Pages are small (KB scale); full snapshots avoid diff/patch reconstruction complexity on read.

### Storage Layout

```
data/__history__/<mungedPageId>/
  <ulid>.md            # full content snapshot
  <ulid>.meta.json     # {version_id, page_identifier, created_at, author, is_agent, source, sha256, byte_size}
```

ULIDs are monotonically sortable, so descending filename order equals chronological order without parsing metadata.

### Identity Threading

`wikipage.Identity` (an interface `tailscale.IdentityValue` satisfies structurally) is threaded through all `Writer` and `PageModifier` methods. History records `author = identity.LoginName()` and `is_agent = identity.IsAgent()`. Internal callers (migrations, cron jobs) pass `wikipage.AnonymousIdentity`.

This follows the existing pattern from `checklistmutator` and `mapmutator`, which already thread `tailscale.IdentityValue` through their mutation methods.

### gRPC API

A new `PageHistoryService` exposes:
- `ListPageVersions` — list metadata, newest-first
- `ReadPageVersion` — read full content of a version
- `RestorePageVersion` — restore historical content as live (captures current as new version first)
- `DiffPageVersions` — unified diff between two versions
- `SearchPageHistory` — full-text search within a page's history (on-demand scan)
- `SearchHistory` — full-text search across all history (persistent Bleve index)

All read RPCs are marked `read_only`. No manual purge RPCs — decimation is automatic.

### Search Architecture

Two tiers:
1. **Per-page** (`SearchPageHistory`): on-demand scan of the page's `__history__/<id>/` directory. No persistent index needed.
2. **Global** (`SearchHistory`): persistent Bleve index over all version metadata + bodies, maintained via the existing `IndexCoordinator` job-queue pattern. The index is secondary — history is fully functional without it; it rebuilds from `__history__/` directories.

### Automatic Decimation

A `HistoryDecimationJob` (cron daily) walks `__history__/` and thins old versions per a GFS retention schedule:
- Keep **all** versions from the last 7 days
- Keep **1/week** (newest in each ISO week) for the last 26 weeks
- Keep **1/month** (newest in each calendar month) for the last 5 years
- **Purge** everything older than 5 years

No manual purge APIs. Decimation is best-effort: a failure to delete one version is logged and does not abort the run.

## Consequences

### Positive

- Full audit trail of every page mutation with author attribution and provenance
- Restore any prior version without data loss (current state is captured before restore)
- Search across all history for "who changed X and when"
- Automatic retention prevents unbounded disk growth
- No manual maintenance burden — decimation runs unattended
- Reuses existing patterns (ULID, job queue, IndexCoordinator, IdentityValue threading)

### Negative

- Breaking change to `wikipage.PageWriter` and `wikipage.PageModifier` interfaces — all callers updated
- Disk usage grows with edit frequency (full snapshots, not diffs)
- The global Bleve index adds startup indexing time proportional to history size
- Identity must be threaded through every write path, including internal jobs that have no user

### Neutral

- Decimation schedule (7d/26w/5y) is a policy choice that can be tuned without schema changes
- The `source` field in metadata enables future audit dashboards (e.g. "show all agent edits")

## References

- ADR-0006: Parallel Multi-Index Background Architecture (IndexCoordinator pattern)
- ADR-0015: Per-Checklist Operation Log (lazy GC walker precedent)
- `server/pagestore/history.go` — capture, list, read, restore, diff
- `server/pagestore/decimation.go` — GFS retention job
- `internal/grpc/api/v1/history.go` — gRPC handler
- `api/proto/api/v1/history.proto` — service definition