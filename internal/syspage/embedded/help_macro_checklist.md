+++
identifier = "help_macro_checklist"
system = true
+++

#help #macros

# {{.Title}}

The Checklist macro renders an interactive checklist with add, remove, reorder, and tagging capabilities.

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

## Frontmatter Data Structure

Checklist data is stored in the page's frontmatter under `checklists.<list-name>`:

```toml
+++
title = "My Page"

[checklists.grocery-list]

[[checklists.grocery-list.items]]
text = "Buy milk"
checked = false
tags = ["urgent", "groceries"]

[[checklists.grocery-list.items]]
text = "Buy eggs"
checked = true
tags = []
+++
```

### JSON Representation

```json
{
  "checklists": {
    "grocery-list": {
      "items": [
        { "text": "Buy milk", "checked": false, "tags": ["urgent", "groceries"] },
        { "text": "Buy eggs", "checked": true, "tags": [] }
      ]
    }
  }
}
```

## Tag Grammar

Checklist tags share their grammar and normalization with [[help-hashtags]]:

- `#tag` is recognized at the start of an item or after whitespace.
- Tag chars: Unicode letters, digits, hyphen, underscore. (`#home-lab` and `#home_lab` are distinct.)
- `\#tag` is an escape — renders as literal text.
- Tags are case-folded and NFKC-normalized; the canonical form goes into the `tags` array.

## For Agents

Checklist updates require a **read-modify-write** pattern because `MergeFrontmatter` replaces the entire `checklists.<name>` subtree:

1. Read current frontmatter via `api_v1_Frontmatter_GetFrontmatter`
2. Extract the items array from `checklists.<name>.items`
3. Append/modify/reorder
4. Write back the full subtree via `api_v1_Frontmatter_MergeFrontmatter`

### Important Notes

- The `MergeFrontmatter` call replaces the entire `checklists.<name>` subtree, so you must include all existing items when updating.
- Tags are extracted from `#tag` syntax in the item text but also stored in the `tags` array — the array is the source of truth for the rendered tag pill UI.
- Empty `tags` arrays can be omitted — they default to `[]`.
