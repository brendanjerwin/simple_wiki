+++
identifier = "help_caldav"
system = true
+++

#help #sync

# CalDAV Sync (Apple Reminders, DAVx5)

Every page with a `{{"{{ Checklist }}"}}` macro is also a CalDAV calendar collection. Each named checklist on the page becomes a calendar; each checklist item becomes a `VTODO`. Subscribe from a phone and the checklists show up in the OS task app — Apple Reminders on iOS, tasks.org or Jtx Board on Android via DAVx5. Edits round-trip both ways.

The macro is a view over the data; CalDAV is a view over the same data. Both read and write the `wiki.checklists.*` frontmatter through the same funnel, so attribution, sync tokens, and tombstones stay consistent across both surfaces. See [[help-macro-checklist]] for the underlying data model.

## Subscribe from iOS

1. iPhone: **Settings → Calendar → Accounts → Add Account → Other → Add CalDAV Account**.
2. **Server**: the wiki page URL with a trailing slash, e.g. `https://wiki.example.com/shopping/`. The trailing slash matters — Apple Reminders is finicky about it.
3. **User Name** and **Password**: type anything. The wiki ignores the `Authorization` header entirely; Tailscale is the only authority.
4. **Description**: free text — this is the account label that shows up in Settings.
5. Tap **Next**, then **Save**. Open Apple Reminders. Each checklist on the page appears as a List under the new account.

One CalDAV account per page. If you want `shopping` and `home-projects` on the same phone, add two accounts.

## Subscribe from Android (DAVx5)

1. Install [DAVx5](https://www.davx5.com/) from F-Droid or Google Play.
2. Install a task UI — [tasks.org](https://tasks.org/) or [Jtx Board](https://jtx.techbee.at/). DAVx5 syncs; the task UI renders.
3. In DAVx5: **+ → Login with URL and user name**.
4. **Base URL**: the wiki page URL with a trailing slash, e.g. `https://wiki.example.com/shopping/`.
5. **User name**: anything (again, ignored by the wiki).
6. **Password**: leave blank or type anything. DAVx5 doesn't actually require it for unauthenticated servers but the field is mandatory in the form.
7. After login, DAVx5 discovers the page's checklists. Tick the task lists you want to sync.
8. Open tasks.org / Jtx Board. The checklists show up as task lists.

## What round-trips

| Wiki field | iCalendar property | Notes |
|---|---|---|
| `text` | `SUMMARY` | First line of the item. |
| `checked` | `STATUS` | `NEEDS-ACTION` ↔ unchecked, `COMPLETED` ↔ checked. `PERCENT-COMPLETE` mirrors for older clients. |
| `completed_at` | `COMPLETED` | Stamped server-side when `checked` flips false→true. Cleared on true→false. |
| `tags` | `CATEGORIES` | Comma-joined on output. On input, `CATEGORIES` and `#tags` parsed from `DESCRIPTION` are unioned and normalized via the same grammar as [[help-hashtags]]. |
| `sort_order` | `X-APPLE-SORT-ORDER` | Preserves manual ordering. `PRIORITY` is also written as a 1..9 bucket fallback for clients that ignore the X- property. |
| `due` | `DUE` | RFC3339 ↔ iCalendar date/time. |
| `description` | `DESCRIPTION` | RFC 5545 escaped on output. Capped at 64 KB on input — larger payloads return `413 Payload Too Large`. |
| `alarm_payload` | `VALARM` | `ACTION:DISPLAY` with `TRIGGER` (relative `-PT15M` or absolute timestamp) and a `DESCRIPTION`. JSON shape stored in frontmatter; round-trips intact. |
| (page link) | `URL` | Read-only back-link to the wiki page. |
| `uid` | `UID` | Stable ULID assigned on first save. Immutable. |
| `created_at`, `updated_at` | `CREATED`, `LAST-MODIFIED`, `DTSTAMP` | Server-stamped. |

`#tags` typed inline in `DESCRIPTION` from the task UI are extracted and merged with `CATEGORIES`, so writing `Buy milk #urgent` from Apple Reminders results in `tags = ["urgent"]` on the wiki side.

## What is silently dropped

The wiki accepts these properties on inbound `PUT` and discards them rather than rejecting the request. This keeps task apps happy when they send rich payloads the wiki doesn't model.

- `RRULE` — no recurrence support.
- `RELATED-TO` — no parent/child / subtask support.
- `GEO` — no geofencing.
- `LOCATION` — no location fields.
- `ORGANIZER`, `ATTENDEE` — no multi-user task assignment.
- `CLASS` — no per-item visibility model; all items inherit page visibility.

These are out of scope for v1 of the bridge. Round-trip them if you need them: edit the page in the wiki UI instead.

## Attribution

Writes from CalDAV use the **Tailscale principal** of the device making the request as `completed_by`. The `automated` flag is derived from `principal.IsAgent()` — phones are humans, scheduled agents are not. The wiki UI renders agent edits with an "automated" badge; mobile clients see them as normal tasks because `VTODO` has no native concept of agent attribution.

The `Authorization` header that iOS and DAVx5 send is **never read** by the wiki. The username and password you typed during account setup are decorative — they exist to satisfy the OS form.

## Multiple lists, multiple accounts

- **One page = one CalDAV account.** Each named checklist on the page is a separate calendar collection under that account.
- **Two pages = two CalDAV accounts.** Add each page URL as its own account on the phone. There's no top-level "all checklists on the wiki" collection.
- This is intentional: it scopes access in the OS UI to the page you actually subscribed to, and it keeps the URL routing simple.

## Troubleshooting

**Off-tailnet → connection failure or 403.**
The wiki only serves CalDAV from the tailnet. If your phone leaves Tailscale (VPN dropped, on a foreign Wi-Fi without the Tailscale app), requests fail. Reconnect Tailscale and the next sync recovers. This is by design — there's no public-internet auth path.

**"All my edits got rolled back."**
A `412 Precondition Failed` means another writer (the wiki UI, another phone, an agent) changed the same item between your read and your write. The task app should resync automatically and present the server's current state. Make your edit again.

**Items don't appear after subscribing.**
Two common causes:

1. The page has no `{{"{{ Checklist }}"}}` macro yet, or its checklists are empty. Add at least one item via the wiki UI.
2. The server URL is wrong — double-check the trailing slash and that the page identifier in the URL matches an existing wiki page.

**Apple Reminders shows the account but no lists.**
Apple Reminders re-discovers calendars on a delay. Pull-to-refresh in Reminders, or toggle the account off and on under Settings → Calendar → Accounts.

**DAVx5 says "0 collections found".**
Confirm the URL ends in `/`. DAVx5's discovery walks `PROPFIND` from the base URL — without the slash it asks the wrong path.

## See Also

- [[help-macro-checklist]] — the data model these lists sync against.
- [[help-hashtags]] — tag grammar shared between checklist items and `CATEGORIES`.
- [[help-system-pages]] — why this page can't be edited from the wiki UI.
- [[help-search]] — `#tag` queries across pages and checklist items.
