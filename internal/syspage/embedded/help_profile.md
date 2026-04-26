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

## Visibility today

Profile pages are ordinary wiki pages with predictable identifiers. There
is no per-page access control yet, so any authenticated reader can view
any profile page if they know the identifier. **Do not put secrets in
your profile page.** A future change will add an owner-scoped ACL — see
issue #997 for the roadmap.

## For agents

There is no gRPC or MCP service for managing profiles in this release.
Agents that want to record per-user state should wait for the per-user
service that lands with the cloud-bridge work (#998/#999/#1000).
