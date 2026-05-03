+++
identifier = "help_connectors"

[wiki]
system = true
+++

#help #sync

# Checklist connectors

Connectors bridge wiki checklists to apps you already use on your phone or computer. Add an item in the wiki, it shows up in the other app. Check it off in the other app, the wiki ticks it next sync. Each connector is a real two-way bridge to a specific cloud service.

## Available bridges

- [[help-google-keep]] — **Google Keep**. Reverse-engineered API; requires capturing an `oauth_token` cookie. Read the trust-model section before connecting.
- [[help-google-tasks]] — **Google Tasks**. Standard OAuth; per-deployment setup by your wiki operator. Cleanest of the Google bridges.
- [[help-caldav]] — **CalDAV** (Apple Reminders, DAVx5 + tasks.org / Jtx Board). Built into the wiki; no per-user setup, no API tokens.

> If you're connecting Google Keep or Google Tasks for the first time on an existing profile, see [[help-profile-features]] for how to add the connect buttons to your profile body. Profile pages don't auto-upgrade when new connectors ship, so the snippet has to be pasted in once.

## Roadmap

- **iCloud Reminders** — direct OAuth bridge (vs. the current CalDAV path through Apple's calendar server). Next on deck after Tasks lands.

## How bindings work

Each binding is **one wiki checklist to one remote list**, and bindings are **globally exclusive**: you can't bind the same wiki checklist to two services at once. If `shopping_lists.this_week` is bound to your Google Tasks "Groceries" list, it can't simultaneously be bound to a Google Keep note. Pick one.

This exclusivity is per-checklist, not per-user. Different household members each bind their own checklists to their own remote lists. Two users *can* both bind the same checklist to different services from their own profiles — that's the explicit intended pattern for households where Alice prefers Tasks and Bob prefers Keep on shared lists.

## What's the same across all connectors

- Your bindings live on your profile page under `wiki.connectors.<kind>.subscriptions[]` (the storage key uses the code-level `subscriptions` term; the UI calls them bindings).
- Disconnecting (revoking auth) pauses your bindings but doesn't delete them. Reconnect to resume.
- Unbinding severs one specific binding without touching either the wiki data or the remote list.
- Sync runs every ~30 seconds via a unified scheduler.
- Your bindings are invisible to the [[help-caldav]] surface — bridges don't leak each other's metadata.

## What's different

Each bridge has its own auth model, its own field-mapping table, and its own quirks. Read the per-connector page before binding. The Keep page in particular has trust-model warnings worth your attention.
