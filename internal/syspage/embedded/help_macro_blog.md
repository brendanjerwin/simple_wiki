+++
identifier = "help_macro_blog"
system = true
+++

#help #macros

# {{.Title}}

The Blog macro renders a list of the most recent blog articles, sorted by published date, with a "New Post" button for creating new entries.

## Syntax

```
{{ Blog "<blog-id>" 10 }}
```

### Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| First | string | Blog identifier — matches posts via `blog.identifier` frontmatter |
| Second | int | Maximum number of articles to display |

### Example

```
{{ Blog "engineering-blog" 10 }}
```

## How It Works

Blog posts are discovered via frontmatter, not by page name. Each blog post page has a `[blog]` section in its TOML frontmatter:

```toml
+++
title = "My First Post"

[blog]
identifier = "engineering-blog"
published-date = "2026-03-15"
subtitle = "Optional subtitle displayed below the title"
summary_markdown = "Optional markdown snippet shown in the blog list."
external_url = "Optional URL — title links here instead of the wiki page"
+++
```

### Blog Post Frontmatter Fields

| Field | Required | Description |
|-------|----------|-------------|
| `blog.identifier` | Yes | Must match the Blog macro's identifier argument |
| `blog.published-date` | Yes | Date in `YYYY-MM-DD` format, used for sorting |
| `blog.subtitle` | No | Displayed below the title in the blog list |
| `blog.summary_markdown` | No | Shown as the snippet; if absent, first ~200 chars of page content are used |
| `blog.external_url` | No | When set, the title links externally; a subdued wiki page link is shown alongside |

### Hosting Page Frontmatter

The page that contains the Blog macro can also have a `[blog]` section to configure display options:

```toml
+++
title = "My Blog"

[blog]
hide-new-post = true
+++

# {{.Title}}

{{ Blog "my-blog" 10 }}
```

| Field | Default | Description |
|-------|---------|-------------|
| `blog.hide-new-post` | `false` | When `true`, hides the "New Post" button and dialog |

### Page Naming Convention

Blog post pages follow the convention `<blog-id>-<date>-<title-slug>`, e.g., `engineering-blog-2026-03-15-hello-world`. This is organizational — discovery is always via the `blog.identifier` frontmatter query.

## UI Features

- **Article list**: Shows title (as link), published date, optional subtitle, and snippet
- **New Post button**: Opens a dialog with title, date, subtitle, optional summary (collapsible), and markdown body editor. Can be hidden via `blog.hide-new-post` on the hosting page.
- **Progressive enhancement**: Server renders the post list as static HTML; JavaScript enhances with dynamic loading
- **Load more**: Click "Load older posts" to fetch additional entries with a smooth fade-in transition

## For Agents

Agents can manage blog posts via two interfaces: **MCP tools** (when the wiki is configured as an MCP server) or the **wiki-cli** command-line tool.

### Creating a Blog Post

Use `api_v1_PageManagementService_CreatePage` with blog frontmatter, e.g.:

```json
{
  "page_name": "my-blog-2026-03-15-hello-world",
  "frontmatter_json": "{\"title\":\"Hello World\",\"blog\":{\"identifier\":\"my-blog\",\"published-date\":\"2026-03-15\"}}",
  "content_markdown": "This is my first blog post!"
}
```

### Listing Blog Posts

Use `api_v1_SearchService_ListPagesByFrontmatter`:

```json
{
  "match_key": "blog.identifier",
  "match_value": "my-blog",
  "sort_by_key": "blog.published-date",
  "sort_ascending": false,
  "max_results": 10,
  "frontmatter_keys_to_return": ["title", "blog.published-date", "blog.subtitle"],
  "content_excerpt_max_chars": 200
}
```

### Updating Post Metadata

Use `api_v1_Frontmatter_MergeFrontmatter`:

```json
{
  "page_name": "my-blog-2026-03-15-hello-world",
  "toml": "[blog]\nsubtitle = \"A new subtitle\"\nsummary_markdown = \"Updated summary for the blog list.\""
}
```

### Removing a Post from the Blog

Remove the entire `blog` key with `api_v1_Frontmatter_RemoveKeyAtPath`:

```json
{
  "page_name": "my-blog-2026-03-15-hello-world",
  "path": [{ "key": "blog" }]
}
```

### Discovery

```bash
wiki-cli list                                    # See all services
wiki-cli methods PageManagementService           # See methods in a service
wiki-cli describe api.v1.CreatePageRequest       # Inspect request fields
```
