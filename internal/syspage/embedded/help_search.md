+++
identifier = "help_search"
system = true
+++

#help #search

# Search

The wiki's search box (and the `api_v1_SearchService_SearchContent` MCP tool) accepts plain text **and** a small `#tag` query syntax. Tags are first-class citizens — they have their own indexed field, dedicated grammar, and contribute to ranking on plain-text queries too.

## Plain-Text Search

Type whatever you're looking for. The wiki searches page titles, rendered content, and other indexed frontmatter fields. Results come back ranked by Bleve's default scoring with title boosts.

## `#tag` Queries

Tokens that start with `#` are interpreted as **tag filters**. They never match free-text — they look up the page-level tag index directly.

| Query | Means |
|-------|-------|
| `#groceries` | Pages tagged `#groceries`. |
| `#groceries milk` | Pages tagged `#groceries` that also match the text `milk`. |
| `#groceries #urgent` | Pages tagged with **both** `#groceries` AND `#urgent`. |
| `groceries` (no `#`) | Plain full-text search. Pages tagged `#groceries` get a ranking boost (see below). |

Multiple `#tag` tokens combine with **AND** — every tag must be present. Mix freely with non-tag terms.

## Tag Boost on Plain Queries

When you search for plain text and one of your terms happens to match an indexed tag value, pages with that tag get bumped up the result list. This means a query like `home lab setup` ranks pages tagged `#home-lab` above pages that merely mention "home lab" in prose.

The boost is a `should`-clause against the tag field — not a hard filter. If no tagged page matches, you still get text-only results.

## Tag Click-Through

Anywhere a `#tag` is rendered as a pill (in page bodies, search results, checklist items), clicking the pill pops a small bubble anchored to the pill listing pages tagged that way — a quick peek, not a full search. Click the pill again, click outside, or press Escape to dismiss.

To run an explicit `#tag` search and see the full results view, type `#tag` into the search bar in the top menu. Press `Ctrl+K` (or `Cmd+K` on macOS) to focus it without leaving the keyboard. The `#tag` query is the canonical view of "everything tagged X" — there's no separate "tag index" page.

## See Also

- [[help-hashtags]] — full grammar, normalization, and authoring rules for tags.
- [[help-macro-checklist]] — checklist items use the same tag grammar inside item text.
