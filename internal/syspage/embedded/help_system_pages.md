+++
identifier = "help_system_pages"
title = "help-system-pages"

[wiki]
system = true
+++

#help #system

# System Pages

Some pages on this wiki are **system pages** — their canonical source lives in the wiki's binary, not in the page store. They're written to the store on startup and re-synced whenever the binary is upgraded.

> [!NOTE]
> System pages exist so help and reference content ship with the wiki itself. New installs come with help out of the box, and existing installs pick up improvements on every upgrade without you having to re-author anything.

## Identifying a System Page

A page is a system page when its frontmatter includes the `system` flag under the reserved `wiki.*` namespace:

```toml
+++
identifier = "..."

[wiki]
system = true
+++
```

> [!NOTE]
> An eager startup migration moves any pages that still carry a top-level `system` flag (the pre-#997 location) into the `[wiki]` block. The helper that checks this flag only looks under `wiki.system` — so the migration is what makes legacy pages start being recognised again, not a fallback in the helper itself.

When you visit a system page you'll see a banner at the top of the content area noting that the page ships with the binary, and the **Edit** button is hidden.

## Editing System Pages

Don't. They aren't editable through the wiki UI or the public mutation tools — the API rejects writes with `FailedPrecondition` and a message pointing you here.

To propose a change:

1. Open an issue or pull request against the wiki repository.
2. Edit the corresponding `.md` file under `internal/syspage/embedded/`.
3. After the next deploy, the upgraded binary will sync the new content into your installation automatically.

## Authorization (`wiki.authorization`)

System pages are not the only pages with restricted access — any wiki page can carry a `wiki.authorization` block to gate reads and writes. The block is a sibling of `wiki.system` under the reserved `wiki.*` namespace:

```toml
[wiki.authorization]
allow_agent_access = false   # opt-in for agent (non-human) callers, default false

[wiki.authorization.acl]
owner = "alice@example.com"  # restricts to this human; omit to allow any human
```

**Rules** (enforced on every API surface — HTTP, gRPC, CalDAV, MCP):

1. Pages with no `wiki.authorization` subtree are public — everyone allowed.
2. With the subtree present, the `acl.owner` field gates humans. If set, only that owner may read/write. If the subtree exists but `acl.owner` is missing, any authenticated human may read/write.
3. `allow_agent_access` is orthogonal. Agents are denied unless this flag is `true`. The flag does not unlock human access; an agent reading a page never falls through to the owner check.
4. Anonymous callers (no Tailscale identity) cannot pass any non-empty authorization block — they have no login to match against.

Internal callers (the system-page sync, the eager migrations, the indexer) bypass these rules by going through `*Site` directly rather than through any of the API surfaces. There is no other escape hatch.

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
2. Include TOML frontmatter with `identifier = "<identifier>"` and a `[wiki]` block containing `system = true`.
3. Link it from the [[help]] index page if it should be user-discoverable.
4. Open a PR.

If the new page is documentation for a feature you're adding in the same change, the help update is part of the feature — not a follow-up. See `CLAUDE.md`'s "Help Documentation" section for the rule.
