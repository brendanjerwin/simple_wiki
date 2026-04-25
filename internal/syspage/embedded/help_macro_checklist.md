+++
identifier = "help_macro_checklist"
system = true
+++

#help #macros

# {{.Title}}

The Checklist macro renders an interactive checklist with add, remove, reorder, and tagging capabilities. Item-level mutations go through the typed `ChecklistService` API, which records server-stamped attribution and a per-list sync token used by CalDAV and other sync clients.

## Syntax

```
{{ Checklist "list-name" }}
```

### Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `list-name` | string | Name of the checklist, scoped to the current page |

### Example

```
{{ Checklist "grocery-list" }}
{{ Checklist "todo" }}
```

A single page can have multiple checklists with different names.

## UI Features

- **Add items**: Type in the input field and press Enter or click the add button
- **Check/uncheck**: Click the checkbox next to any item
- **Remove items**: Click the delete button on an item
- **Reorder**: Drag and drop items using the drag handle (desktop) or long-press and drag (mobile)
- **Tagging**: Add tags to items using `#tag` syntax in the item text (e.g., `Buy milk #urgent #groceries`)
- **Filter by tag**: Click tag pills to filter the list to items with that tag
- **Literal `#`**: Escape with a backslash (`\#5`) when you want the `#` to appear as plain text instead of being treated as a tag

## Two-Tier Data Model

Checklist data lives in **two** frontmatter subtrees. The split exists so generic frontmatter editors can edit user data while the wiki itself maintains attribution and sync metadata invariantly. See [[ADR-0009]] and [[ADR-0010]] for the architecture.

### `checklists.<list-name>` — user-mutable

```toml
+++
title = "My Page"

[[checklists.grocery-list.items]]
uid = "01HXXXXXXXXXXXXXXXXXXXXXXX"
text = "Buy milk"
checked = false
tags = ["urgent", "groceries"]
sort_order = 1000
description = "the brand Kirsten likes"      # optional
+++
```

User-data fields:

| Field | Type | Notes |
|---|---|---|
| `uid` | string | Stable ULID assigned by the wiki on first save. Immutable. |
| `text` | string | Free-form item text. |
| `checked` | bool | Done flag. |
| `tags` | string[] | Normalized tags extracted from text plus explicit additions. |
| `sort_order` | int | Sparse ordering value (multiples of 1000 are conventional). |
| `description` | string | Optional sub-line content under the item. |
| `due` | RFC3339 timestamp | Optional due time. |
| `alarm_payload` | string | Optional VALARM payload for CalDAV sync clients. |

### `wiki.checklists.<list-name>` — wiki-managed (reserved namespace)

```toml
[wiki.checklists.grocery-list]
sync_token = 47
updated_at = "2026-04-25T17:14:00Z"
migrated_data_model = true

[wiki.checklists.grocery-list.items.01HXXXXXXXXXXXXXXXXXXXXXXX]
created_at  = "2026-04-25T13:00:00Z"
updated_at  = "2026-04-25T17:14:00Z"
completed_at = "2026-04-25T17:14:00Z"
completed_by = "alice@example.com"
automated   = false
```

The `wiki.*` namespace is **reserved**. Generic frontmatter tools (`MergeFrontmatter`, `ReplaceFrontmatter`, `RemoveKeyAtPath`) reject writes targeting any `wiki.*` path. The only legitimate way to mutate this subtree is `ChecklistService` (or another dedicated service for future namespaces under `wiki.*`).

## For Agents

Use `ChecklistService` for item-level mutations. It is exposed both as gRPC and as MCP tools (auto-generated from the proto).

### MCP Tools

| Tool | Purpose |
|---|---|
| `api_v1_ChecklistService_AddItem` | Append a new item; the wiki generates `uid` and stamps `created_at`/`updated_at`. |
| `api_v1_ChecklistService_UpdateItem` | Mutate user-mutable fields. Wiki-managed fields on the request are silently stripped. |
| `api_v1_ChecklistService_ToggleItem` | Flip `checked`. Sets/clears `completed_at` + `completed_by`. |
| `api_v1_ChecklistService_DeleteItem` | Remove an item; tombstone is written for sync clients. |
| `api_v1_ChecklistService_ReorderItem` | Update `sort_order`. The wiki re-densifies adjacent values only on collision. |
| `api_v1_ChecklistService_ListItems` | Read items + wiki-managed metadata. |
| `api_v1_ChecklistService_GetChecklists` | Enumerate all checklists on a page. |

### Optimistic Concurrency

Mutating tools accept an optional `expected_updated_at`. Pass the value from a prior `ListItems` response to detect concurrent edits — mismatch returns `FailedPrecondition`, and you should refetch + retry.

### Attribution

The wiki derives `automated` from your authentication context — Tailscale tags, the `x-wiki-is-agent` request header, or `wiki-cli`'s default. **Do not pass `automated`, `completed_by`, or any `created_at`/`updated_at` field on input** — they're silently stripped, and only the wiki's authoritative values survive.

### Raw Frontmatter Writes Still Work (But Lose Attribution)

`checklists.*` itself is **not** reserved. Direct `MergeFrontmatter`/`ReplaceFrontmatter` calls to `checklists.*` continue to succeed — but they bypass the funnel, so the corresponding `wiki.checklists.<list>.updated_at` and `sync_token` will not advance, and CalDAV/sync clients won't see the change. Use `ChecklistService` whenever attribution or sync correctness matters.

## Tag Grammar

Checklist tags share their grammar and normalization with [[help-hashtags]]:

- `#tag` is recognized at the start of an item or after whitespace.
- Tag chars: Unicode letters, digits, hyphen, underscore. (`#home-lab` and `#home_lab` are distinct.)
- `\#tag` is an escape — renders as literal text.
- Tags are case-folded and NFKC-normalized; the canonical form goes into the `tags` array.

## Migration Note

Pages created before #984 land may have items without `uid` and without per-item metadata. The eager `ChecklistDataModelMigrationScanJob` runs once at startup, assigns ULIDs, backfills `sort_order`, and stamps each list's `wiki.checklists.<name>.migrated_data_model = true` flag. The migration is idempotent — re-running on a stamped page is a no-op.
