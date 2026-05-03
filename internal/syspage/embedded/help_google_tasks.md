+++
identifier = "help_google_tasks"

[wiki]
system = true
+++

#help #sync #tasks

# Google Tasks bridge

> See [[help-connectors]] for the index of all checklist bridges.

Bind a wiki checklist to a Google Tasks list and items round-trip both ways. Add `Buy milk` in the wiki, it shows up on your phone in the Google Tasks app within ~30 seconds. Check it off in the app, the wiki ticks it next sync. Same the other direction.

Tasks is the Google-native task surface, so unlike the [[help-google-keep]] bridge there's a real public API and no reverse-engineered protocol. Connection is OAuth — no password capture, no master tokens.

## What it does

- Bidirectional sync between one wiki checklist and one Google Tasks list.
- Items added on either side appear on the other within ~30 seconds.
- Check/uncheck, edit text, change due date, reorder — all round-trip.
- Per-user, per-checklist. Each household member binds their own checklists to their own Tasks lists.
- **No phantom overwrites.** Each tick only patches an item if you actually changed it on the wiki side. If nothing changed locally between two ticks, the wiki sends zero PATCH calls — your phone-side edits in that window are safe.
- **Phone wins on real conflict, wiki wins on phantom conflict.** If you edit the same item on the wiki and the phone at the same time and the wiki's push collides with Google's optimistic-concurrency check (a 412 Precondition Failed), the wiki pulls the current Tasks state. If the Tasks state has actually changed since our last successful push, the wiki yields and applies the phone's edit (so your phone work isn't destroyed). If the Tasks state matches what we last pushed — i.e., the 412 was a phantom (etag desync, server-side internal bump) — the wiki refreshes the etag and re-PATCHes so YOUR wiki edit wins instead of being thrown away. Either way, we never blindly retry with an empty If-Match.

## How to set up

If your wiki operator has set up Google Tasks integration (see the operator note below if not):

1. Visit **/profile** on the wiki — you land on your own profile page.
2. Find the **Google Tasks** section and click **Connect Google Tasks**.
3. You're sent to Google's consent screen. Approve.
4. Back on the profile, you should see **Connected as you@example.com**.
5. Open any page with a `{{"{{ Checklist \"name\" }}"}}` macro.
6. Click **Bind to a cloud service** on the checklist (the picker will show Google Tasks as your only option if Tasks is your only authenticated connector).
7. Pick an existing Tasks list from the dropdown, or create a new one.
8. Done. The button is replaced with a `✓ Bound to Google Tasks list <title>` badge.

> **If the connect button isn't there:** the wiki's operator hasn't configured Google Tasks credentials yet. They'll need to follow the operator setup guide in `docs/google_tasks_setup.md` in the simple_wiki repo. This is a one-time, per-deployment setup; not something each user can do for themselves.

## What gets synced

| Wiki field | Google Tasks field | Notes |
| --- | --- | --- |
| `text` | `title` | Includes any `#tag` tokens — they round-trip as part of the title since Tasks has no native tag concept. |
| `checked` | `status` | `NEEDS-ACTION` ↔ unchecked, `COMPLETED` ↔ checked. |
| `description` | `notes` | Free-form notes; user-editable on either side. |
| `due` | `due` | Date-only on Google's side. The wiki may store a time-of-day, but it's dropped when pushed to Tasks. |
| `completed_at` | `completed` | Stamped on completion; cleared on uncheck. |
| `sort_order` | `position` | Manual ordering preserved. |

## What doesn't get synced (yet)

- **Subtasks.** The wiki has flat checklists. Tasks supports parent/child task hierarchies; the wiki does not.

## Subtasks: the asymmetry

The wiki has flat lists. This causes two distinct rough edges around subtasks:

- **The wiki refuses to bind to a Tasks list that already has subtasks.** If you try, you'll get an error. Move or delete the subtasks in the Google Tasks app first, then bind.
- **Subtasks added in Google Tasks after binding become regular items in the wiki.** They won't be deleted, but the parent-child relationship is lost on the wiki side. The next outbound sync may flatten them on the Tasks side too.

If you need hierarchy, keep using Google Tasks directly without the wiki bridge for those lists.

## The invisible marker — please leave it alone

The wiki adds an invisible trailing line to each task's `notes` field, looking like:

```text
— wiki:uid=01HZX9...
```

This is how the wiki tracks which Tasks item corresponds to which wiki item. It's the same identity across renames, edits, and reorders.

If you accidentally delete it (e.g. you cleared a task's notes), the wiki tries to recover by matching item text — but identical text in the same list can confuse it. Best to leave the marker line alone. Edit your notes *above* the marker; don't delete it.

## What does *not* round-trip

- **Alarms (`VALARM` payload).** Wiki checklists can carry alarm metadata that flows through the [[help-caldav]] bridge to Apple Reminders. Tasks has no alarm primitive on the API; alarms stay wiki-side and don't appear in the Tasks app.
- **Tags as native concepts.** Tasks doesn't have tags, so the wiki's `#tag` tokens travel as part of the `title` text. They survive a round-trip but don't filter or group on the Tasks side.

## Disconnecting and unbinding

- **Disconnect** (profile page) — revokes the OAuth token and pauses all your bindings. Reconnecting later resumes them; bindings aren't lost.
- **Unbind** (per-checklist) — severs one specific binding. The Google Tasks list and the wiki checklist both stay exactly as they are; only the connection between them is removed.

## Troubleshooting

- **"Sync paused" badge on a checklist.** Click it. An inline reconnect modal opens; reauthorize via Google. The badge tells you when sync paused so you know what window of changes might be missing.
- **Items I added in the wiki aren't showing up in Tasks.** Check your profile page. If the connector shows paused, click the paused badge and reconnect. If it's connected and items still aren't appearing, give it the full ~30 second tick window.
- **Items I added in Google Tasks aren't showing up in the wiki.** Same drill — check pause state on the profile page first. If connected, the next ~30 second tick should pull them in.
- **"Tasks integration not configured by this wiki's operator."** The operator hasn't set the OAuth env vars. Point them at `docs/google_tasks_setup.md`.
- **Bind picker is empty.** You're connected but have no Tasks lists yet. The picker still shows a "Create new" entry as the first option — pick it and the wiki creates a fresh Tasks list named after your wiki checklist on the spot.

## "Create new" — Bind to a fresh Tasks list

If you don't already have a Tasks list set up (or you'd rather not pick an existing one), the bind picker's first entry is **Create new "<list-name>"**. Pick it and the wiki calls Tasks's `tasklists.insert` for you, names the new list after your wiki checklist's `list_name`, and binds to it on the spot. The bind ceremony also pushes every wiki item already on the checklist into the new Tasks list immediately — each task gets stamped with the wiki:uid marker so subsequent inbound syncs round-trip cleanly. You won't have to wait for the first cron tick to see your items in Google Tasks.

Mirrors the [[help-google-keep]] bridge's behavior — empty `remote_list_handle` means "make a new one."

## Errors you might see

| Banner | What it means | What to do |
| --- | --- | --- |
| `auth_failed` | OAuth token rejected (revoked, expired beyond refresh, or rotated and lost) | Click the paused badge on profile or checklist; reauthorize via Google |
| `subscription_collision` | Someone else already bound this checklist to a different cloud service or list | Use a different checklist or have them unbind first |
| `subtasks_present` | Tried to bind to a Tasks list that has subtasks | Open the Tasks app, flatten the list (move subtasks to top level), then retry |
| `tasks_api_not_enabled` | The Google Tasks API is not enabled on the GCP project that issued the OAuth client | Click the activation URL in the error message (or visit the [Google Cloud Console](https://console.developers.google.com/apis/api/tasks.googleapis.com/overview) for the project that owns your OAuth client) and enable the Tasks API, then retry |
| `permission_denied` | Google rejected the request with a generic 403 (token valid, but the resource is off-limits) | Check the OAuth scope grant on your profile; if you recently changed scopes, reauthorize |
| `rate_limited` | Hitting Tasks API too hard | Wait a few minutes; sync resumes automatically |

## CalDAV invisibility

Your bindings are invisible to CalDAV. If you also bind a checklist to Apple Reminders or DAVx5 via [[help-caldav]], the Reminders/DAVx5 client never sees `wiki.connectors.*` state — the bridges don't leak each other's metadata.

## For agents

The bridge is exposed through the unified per-user gRPC service `api.v1.ConnectorService`. All RPCs accept a `connector_kind` enum to disambiguate; pass `CONNECTOR_KIND_GOOGLE_TASKS` for Tasks flows. Every method scopes to the calling user via Tailscale identity → ProfileIdentifierFor; no method ever leaks another user's tokens or subscriptions.

- `BeginAuth(connector_kind=GOOGLE_TASKS) → BeginAuthResponse` — returns the Google authorization URL with PKCE `code_challenge` (S256) and a single-use server-side state token.
- `CompleteAuth(connector_kind=GOOGLE_TASKS, code, state) → ConnectorState` — exchanges the authorization code for refresh + access tokens; persists `wiki.connectors.google_tasks.*`.
- `Disconnect(connector_kind=GOOGLE_TASKS) → ConnectorState` — pauses subscriptions; preserves `item_id_map` and cursor for clean resume.
- `GetState(connector_kind=GOOGLE_TASKS) → ConnectorState` — reads connector + subscriptions.
- `ListRemoteLists(connector_kind=GOOGLE_TASKS) → RemoteListSummary[]` — proxies to the user's Tasks account (`tasklists.list`).
- `ListMySubscriptions(connector_kind=GOOGLE_TASKS) → SubscriptionState[]`
- `Subscribe(connector_kind=GOOGLE_TASKS, page, list_name, remote_list_handle) → SubscriptionState` — `remote_list_handle` is the Google `tasklist.id`, or empty string to bind to a new Tasks list (the wiki calls `tasklists.insert` with `title=list_name` and binds to the freshly-created list). Refuses with `FailedPrecondition` if a non-empty target list already contains subtasks.
- `Unsubscribe(connector_kind=GOOGLE_TASKS, page, list_name) → ()`
- `GetChecklistSubscriptionState(page, list_name) → ChecklistSubscriptionState` — small surface used by the Checklist component on render. **Does not take connector_kind**; returns whichever connector owns the checklist.
- `ListDeadLetters(connector_kind=GOOGLE_TASKS, page, list_name) → DeadLetterItem[]`
- `ClearDeadLetter(connector_kind=GOOGLE_TASKS, page, list_name, item_uid) → ()`

Errors branch on typed connect codes, never on banner text. `auth_failed` → `Unauthenticated`; subscription collision → `AlreadyExists`; subtask refuse-to-subscribe → `FailedPrecondition`; `tasks_api_not_enabled` → `FailedPrecondition` (with the Google activation URL embedded in the message); `permission_denied` → `PermissionDenied`; `rate_limited` → `ResourceExhausted`.

## Why OAuth instead of paste-token

Google publishes a real Tasks API with an OAuth scope (`https://www.googleapis.com/auth/tasks`). No master tokens, no reverse-engineering, no plaintext credential-equivalents on profile pages. The flow is the standard authorization-code grant with PKCE (S256) and RFC 9207 issuer validation. The refresh token is stored at-rest in plaintext on your profile — see ADR-0014 for the deliberate trust-perimeter rationale (the Tailnet is the perimeter, not the filesystem).

This bridge is per-deployment opt-in: each operator provisions their own GCP project and OAuth client. The wiki itself ships no shared credentials. See `docs/google_tasks_setup.md` for the operator walkthrough.
