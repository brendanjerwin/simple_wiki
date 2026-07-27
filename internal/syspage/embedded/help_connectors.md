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

- Your bindings live on your profile page under `wiki.connectors.<kind>.bindings[]`.
- Disconnecting (revoking auth) pauses your bindings but doesn't delete them. Reconnect to resume.
- Unbinding severs one specific binding without touching either the wiki data or the remote list.
- Sync runs every ~30 seconds via a unified scheduler.
- Your bindings are invisible to the [[help-caldav]] surface — bridges don't leak each other's metadata.

## What's different

Each bridge has its own auth model, its own field-mapping table, and its own quirks. Read the per-connector page before binding. The Keep page in particular has trust-model warnings worth your attention.

## For Agents

Use `ConnectorService` for binding, unbinding, and syncing checklist connectors. It is exposed both as gRPC and as MCP tools (auto-generated from the proto).

### MCP Tools

| Tool | Purpose |
|---|---|
| `api_v1_ConnectorService_BeginAuth` | Start an OAuth flow for web-server auth-code connectors; no-op for Google Keep. |
| `api_v1_ConnectorService_CompleteAuth` | Exchange captured tokens/codes for long-lived credentials and persist them on the caller's profile. |
| `api_v1_ConnectorService_Disconnect` | Wipe credentials from the calling user's profile and pause all bindings. |
| `api_v1_ConnectorService_GetState` | Return the calling user's connector state and bindings. |
| `api_v1_ConnectorService_ListRemoteLists` | Enumerate remote lists the calling user owns on the named connector. |
| `api_v1_ConnectorService_ListMyBindings` | Return all bindings for the named connector with sync status. |
| `api_v1_ConnectorService_Bind` | Bind a wiki checklist to a remote list for the calling user. |
| `api_v1_ConnectorService_Unbind` | Remove the calling user's binding for a (page, list_name) pair. |
| `api_v1_ConnectorService_SyncNow` | Trigger an immediate sync for the calling user's binding on a checklist. |
| `api_v1_ConnectorService_GetChecklistBindingState` | Per-(page, list_name) binding state for the calling user; does **not** take `connector_kind`. |
| `api_v1_ConnectorService_ListDeadLetters` | Return the dead-lettered items for the calling user's binding. |
| `api_v1_ConnectorService_ClearDeadLetter` | Reset a dead-lettered item's failure count so the next sync retries it. |

All RPCs accept a `connector_kind` enum to disambiguate; pass the appropriate kind for the bridge you're driving. Every method scopes to the calling user via Tailscale identity → ProfileIdentifierFor; no method ever leaks another user's tokens or bindings.

- `BeginAuth(connector_kind) → BeginAuthResponse`
- `CompleteAuth(connector_kind, ...) → ConnectorState`
- `Disconnect(connector_kind) → ConnectorState`
- `GetState(connector_kind) → ConnectorState`
- `ListRemoteLists(connector_kind) → RemoteListSummary[]`
- `ListMyBindings(connector_kind) → BindingState[]`
- `Bind(connector_kind, page, list_name, remote_list_handle?) → BindingState`
- `Unbind(connector_kind, page, list_name) → ()`
- `SyncNow(connector_kind, page, list_name) → ()`
- `GetChecklistBindingState(page, list_name) → ChecklistBindingState`
- `ListDeadLetters(connector_kind, page, list_name) → DeadLetterItem[]`
- `ClearDeadLetter(connector_kind, page, list_name, item_uid) → ()`
