+++
identifier = "help_profile"
title = "Profile Pages"

[wiki]
system = true
+++

#help

# Profile Pages

The wiki gives every authenticated user a personal **profile page**. Visit
`/profile` from any browser tab — the wiki resolves your Tailscale identity
and redirects you to your own profile page.

## Identifier scheme

The profile-page identifier is derived deterministically from your login
(email) so the URL is stable and predictable:

- `brendanjerwin@gmail.com` → `/profile_brendanjerwin_gmail_com/view`
- `alice+tag@example.co.uk` → `/profile_alice_tag_example_co_uk/view`

The transformation is: lowercase, collapse runs of non-alphanumeric
characters into single underscores, prefix with `profile_`.

## First visit

Your profile page is created automatically the first time you visit
`/profile`. It is seeded from the `profile_template` system page (see
`/profile_template/view`). If you'd like to change what every user's
default profile contains, edit the embedded template in the source repo
at `internal/syspage/embedded/profile_template.md`.

After the first visit, your profile page behaves like any other wiki
page — edit it freely from the normal editor. Subsequent visits to
`/profile` redirect to the page without re-creating it.

## Who can use `/profile`?

- **Real human users** (resolved from a Tailscale login) — yes.
- **Anonymous callers** (no Tailscale identity) — `403`.
- **The Tailscale agent identity** (tagged nodes, wiki-cli's default
  agent claim) — `403`.

## Authorization

Each profile page ships with a `wiki.authorization` block that limits
access to its owner:

```toml
[wiki.authorization]
allow_agent_access = false

[wiki.authorization.acl]
owner = "you@example.com"
```

The wiki enforces this on every API surface (HTTP page reads/writes,
gRPC PageManagement / Frontmatter / Checklist / Search, CalDAV). Other
users on the tailnet — and agents — get a 403 / `PermissionDenied`. Only
internal startup machinery (the syspage sync, the eager migrations, the
indexer) bypasses these rules; no external caller does.

If you'd like to share your profile page with another tailnet user,
edit the `acl.owner` to the new owner (you'll lose access at the next
write) — or remove the `wiki.authorization.acl` block entirely to make
the page readable by every authenticated human while still keeping
agents out.

## Connectors

Per-user external-app connector state lives on your profile page under
`wiki.connectors.*`. The Google Keep bridge stores its credentials and
bindings here — see [[help-google-keep]] for the full setup. Other
connector types (Google Tasks, etc.) will follow the same pattern.

The `KeepConnect` macro on the default profile template renders the
connect/disconnect UI; if your profile was created before Keep landed,
add `{{"{{ KeepConnect }}"}}` to your profile page's body to enable it.

## For agents

There is no gRPC or MCP service for managing profiles directly. Agents
that need per-user external-service state should call the
per-connector services (e.g. `api.v1.KeepConnectorService`) — those
scope to the calling identity automatically.
