+++
identifier = "help_system_pages"
title = "help-system-pages"
system = true
+++

#help #system

# System Pages

Some pages on this wiki are **system pages** — their canonical source lives in the wiki's binary, not in the page store. They're written to the store on startup and re-synced whenever the binary is upgraded.

> [!NOTE]
> System pages exist so help and reference content ship with the wiki itself. New installs come with help out of the box, and existing installs pick up improvements on every upgrade without you having to re-author anything.

## Identifying a System Page

A page is a system page when its frontmatter includes:

```toml
+++
system = true
+++
```

When you visit a system page you'll see a banner at the top of the content area noting that the page ships with the binary, and the **Edit** button is hidden.

## Editing System Pages

Don't. They aren't editable through the wiki UI or the public mutation tools — the API rejects writes with `FailedPrecondition` and a message pointing you here.

To propose a change:

1. Open an issue or pull request against the wiki repository.
2. Edit the corresponding `.md` file under `internal/syspage/embedded/`.
3. After the next deploy, the upgraded binary will sync the new content into your installation automatically.

## How Sync Works

On startup (after the indexes are open and before HTTP serving begins) the wiki:

1. Walks the embedded help corpus.
2. For each embedded page, reads the existing on-disk page (if any).
3. Compares the embedded markdown + frontmatter to what's stored.
4. **Only writes when they differ.** No-op startups produce no log noise and don't churn version hashes.

Writes go through the same page-write API that user edits use, so indexes (bleve, frontmatter), hash recomputation, and any other downstream hooks all run normally. The "system page" guard lives at the gRPC layer and rejects external mutations only — internal startup writes flow through the lower-level page mutator and are not affected.

## Adding a New System Page

For contributors:

1. Create a new `internal/syspage/embedded/<identifier>.md`.
2. Include TOML frontmatter with `identifier = "<identifier>"` and `system = true`.
3. Link it from the [[help]] index page if it should be user-discoverable.
4. Open a PR.

If the new page is documentation for a feature you're adding in the same change, the help update is part of the feature — not a follow-up. See `CLAUDE.md`'s "Help Documentation" section for the rule.
