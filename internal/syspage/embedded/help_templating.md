+++
identifier = "help_templating"

[wiki]
system = true
+++

#help #templating

# {{.Title}}

This wiki uses [Go `text/template`](https://pkg.go.dev/text/template) syntax for dynamic content in pages. Macros are invoked with double-curly-brace syntax — see the per-macro sections below for working examples.

## Template Context Variables

Every page template receives a context object with these fields:

| Variable | Type | Description |
|----------|------|-------------|
| `.Title` | string | Page title from frontmatter |
| `.Identifier` | string | Page identifier (URL slug) |
| `.Description` | string | Page description from frontmatter |
| `.Map` | map | Full frontmatter as key-value pairs (e.g., `.Map.blog.identifier`) |
| `.Inventory` | object | Inventory frontmatter with `.Container` and `.Items` fields |
| `.WikiAuthorization` | object | Typed view of `wiki.authorization`. Has `.ACL.Owner` (string) and `.AllowAgentAccess` (bool). Empty when the page has no `wiki.authorization` block. See [[help-system-pages]] for the rules. |

## Go Template Syntax Basics

```
{{ .Title }}                              → Output a variable
{{ if .Title }}...{{ end }}                → Conditional
{{ range .Inventory.Items }}...{{ end }}   → Loop
{{ or .Title .Identifier }}                → First non-empty value
{{ index .Map "key" }}                     → Map lookup
```

## Available Macros

### Blog

Renders a blog post list with "New Post" UI. See [[help-macro-blog]].

```
{{ Blog "my-blog" 10 }}
```

### Checklist

Renders an interactive checklist with add/remove/reorder/tagging. See [[help-macro-checklist]].

```
{{ Checklist "grocery-list" }}
```

### Survey

Renders a per-user response form. See [[help-macro-survey]].

```
{{ Survey "team-preferences" }}
```

### Map

Renders a first-class wiki map from `MapService` data. See [[help-macro-map]].

```
{{"{{ Map \"yard\" }}"}}
```

### GoogleMapsEmbed

Renders a responsive Google Maps embed. See [[help-macro-google-maps-embed]].

```
{{"{{ GoogleMapsEmbed \"https://www.google.com/maps/embed?pb=...\" }}"}}
```

### LinkTo

Renders a markdown link to another wiki page, using its title if available.

```
{{ LinkTo "page-identifier" }}
```

### ShowInventoryContentsOf

Renders a nested list of items contained in an inventory container page.

```
{{ ShowInventoryContentsOf "my-container" }}
```

### IsContainer

Returns true if a page is an inventory container (has items referencing it).

```
{{ if IsContainer .Identifier }}This is a container{{ end }}
```

### FindBy / FindByPrefix / FindByKeyExistence

Query the frontmatter index directly:

```
{{ FindBy "inventory.container" "garage" }}     → Exact match
{{ FindByPrefix "inventory.container" "gar" }}  → Prefix match
{{ FindByKeyExistence "blog.identifier" }}      → Key exists
```

Each returns a list of page identifiers.

## Template Pages

Any page with `wiki.template = true` in its frontmatter becomes a template. Templates also need `title` and `description` fields. They appear in the "New Page" dialog.

```toml
+++
identifier = "article_template"
title = "Article"
description = "Standard article layout"

[wiki]
template = true
+++
```

> [!NOTE]
> An eager startup migration moves any templates that still carry a top-level `template` flag (the pre-#997 location) into the `[wiki]` block. The helper that recognises templates only looks under `wiki.template` — so the migration is what makes legacy templates start being recognised again.

When creating a page from a template, the template's frontmatter is merged as a base, and any explicitly provided frontmatter values override the template defaults. The template's own reserved-namespace state (`wiki.template`, `wiki.system`, etc.) is **not** carried over to the new page — those flags belong to the template, not its instances.

## For Agents

Use `api_v1_PageManagementService_ListTemplates` to discover templates and `api_v1_PageManagementService_CreatePage` (with the `template` field set) to instantiate one.

### Navigating large pages efficiently

Before reading or editing a large page (>30 KB), call `api_v1_PageManagementService_ReadPageOutline` first. See [[help-handling-large-pages]] for the full workflow. It returns:

- **headings** — every heading in document order with its anchor `slug`, `byte_offset` (where the section body begins), and `byte_length` (how many bytes the section spans).
- **total_bytes** — total size of the page markdown.
- **version_hash** — the same SHA-256 hash that `ReadPage` and `UpdatePageContent` use for optimistic concurrency.

Use `byte_offset` + `byte_length` together with `UpdatePageContent`'s `old_content_markdown` / `new_content_markdown` to rewrite a specific section without touching the rest of the page.

Slugs returned by `ReadPageOutline` match the anchor IDs that `RenderPage` generates, so they are safe to use as URL fragments (`#slug`) in rendered links.
