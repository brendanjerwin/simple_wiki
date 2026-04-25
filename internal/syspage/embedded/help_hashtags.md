+++
identifier = "help_hashtags"
system = true
+++

#help #search

# Hashtags

Drop `#tag` anywhere in a page body and the wiki extracts it, indexes it, and renders it as a clickable pill. Tags are **inline in the body** — there's no separate frontmatter list to keep in sync.

## Quick Examples

```
This page is about #home-lab and #self-hosting.

We're tracking #project_alpha through Q2.

Today's highlights: #recipe #weekend #2026
```

Each `#…` becomes a pill linking to a search filtered by that tag.

## Grammar

A `#` starts a tag only when:

- It's at the **start of the line** or follows **whitespace** or **punctuation other than `[` and `(`** (so `[link](#anchor)` does **not** become a tag).
- It's followed by at least one tag character.

Tag characters are:

- Unicode **letters** (any language)
- **Digits** (`#2026` is fine)
- **Hyphen** (`-`) and **underscore** (`_`)

Hyphens and underscores are **not** collapsed — `#home-lab` and `#home_lab` are distinct tags. Pick one and stick with it.

## Escape

Type a backslash before the `#` to render a literal hash:

```
The price is \#5 per pound.
```

The wiki shows `#5` as plain text and does not extract a tag.

## Code Skipping

Tags inside fenced code blocks (` ``` `) and inline code spans (`` ` ``) are **not** extracted. Use this when you're documenting tag syntax without accidentally tagging your help page.

## Length Cap

Tags are capped at 64 characters after normalization. Anything longer is truncated. (If you're producing 65-character tags, you probably want a different approach.)

## Normalization

Tags are case-folded and Unicode NFKC-normalized:

- `#Groceries` and `#groceries` are the same tag.
- Stylized Unicode like `#ＡＢＣ` (fullwidth) folds to `#abc`.
- Non-letter/digit/`-`/`_` characters inside the tag are dropped.

The displayed pill preserves the original casing/spelling from each occurrence; the index key is the normalized form.

## Click Behavior

Clicking a tag pill runs `#tag` as a search query (see [[help-search]]). There's no dedicated tag index page — the search results page is the canonical "all pages tagged X" view.

## See Also

- [[help-search]] — `#tag` query syntax, AND semantics, and tag-aware boosts on plain text.
- [[help-macro-checklist]] — checklist items use the same tag grammar.
