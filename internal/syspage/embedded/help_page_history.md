+++
identifier = "help-page-history"

[wiki]
system = true
+++

#help-page-history

# Page History

Every time you save a page, the wiki captures the **prior** content as a version snapshot. You can list, read, diff, restore, and search historical versions.

## How It Works

- **Automatic capture**: No action needed. Every save — whether from the web UI, an agent, a migration, or a connector sync — captures the outgoing state before it's overwritten.
- **No-op saves are skipped**: If the content doesn't change (e.g. a migration that re-saves the same text), no version is captured.
- **Author attribution**: Each version records who made the change (from the Tailscale identity) and whether it was an automated agent.
- **Soft deletes are captured**: When a page is moved to trash, its final live content is captured as a version first — so even after trash is purged, the page's history survives.
- **Opt-out**: Pages can disable history capture by setting `[wiki.history]` `opt_out = true` in their frontmatter. Useful for system/metrics pages that are written frequently and don't need version history.

## Automatic Cleanup (Decimation)

Old versions are automatically thinned to prevent unbounded disk growth:

| Age | Retention |
|-----|-----------|
| Last 7 days | All versions kept |
| 7 days – 26 weeks | 1 per week (newest in each ISO week) |
| 26 weeks – 5 years | 1 per month (newest in each calendar month) |
| Older than 5 years | Purged |

Decimation runs daily at 3 AM. No manual intervention needed.

## For Agents

The `PageHistoryService` gRPC API (also available as MCP tools) provides:

- `ListPageVersions` — List all versions of a page (newest-first), with metadata (author, timestamp, source, size, sha256).
- `ReadPageVersion` — Read the full content of a specific version.
- `RestorePageVersion` — Restore a historical version as the live page. The current live content is captured as a new version first, so no history is lost.
- `DiffPageVersions` — Get a unified diff between two versions.
- `SearchPageHistory` — Full-text search within a single page's version history.
- `SearchHistory` — Full-text search across all page history, with filters by page, author, and time range.

## Technical Notes

- Version IDs are ULIDs (monotonically sortable, time-prefixed).
- Snapshots are full content (not diffs) — pages are small, and full snapshots avoid reconstruction complexity.
- The `source` field records what triggered the capture: `write_frontmatter`, `write_markdown`, `modify_markdown`, `modify_fm_md`, `soft_delete`, `restore`, or `migration`.
- The global history search index is secondary — history is fully functional without it; the index rebuilds from the on-disk `__history__/` directories.