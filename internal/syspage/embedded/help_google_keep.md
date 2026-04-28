+++
identifier = "help_google_keep"

[wiki]
system = true
+++

#help #sync #keep

# Google Keep bridge

Connect your Google account so wiki checklists can be bound to Google Keep notes — items show up in the Keep app on your phone, and (eventually) edits round-trip.

The macro is a view over the data; Keep is another view over the same data. The wiki uses an unofficial reverse-engineered Keep API — there is no public Google API to use instead.

## ⚠️ Trust model and risk

This bridge uses an unofficial Keep API that Google can change without notice. Read this section carefully before connecting.

- **You will store a Google master token on your wiki profile page in plaintext.** A master token is credential-equivalent to your Google password — anyone who can read your profile page can act as you against Google Keep (and potentially other Google services). The wiki sits behind Tailscale and profile pages are read-restricted to your principal, so the realistic exposure is "someone with admin access to the wiki host." That's the same trust posture as the existing CalDAV bridge.
- **App-Specific Passwords are required.** This means you have 2-Step Verification enabled on the Google account.
- **Google can break the bridge any time.** When that happens, sync stops cleanly and surfaces a `protocol_drift` error to the UI. You won't lose data — both sides keep their own state.
- **Don't share screenshots of your profile page.** The master token is visible in frontmatter.

If any of these is a deal-breaker, use the [CalDAV bridge](/help_caldav/view) instead. The household member just needs an Android task app like tasks.org via DAVx5.

## Setup (one-time, per user)

### 1. Enable 2-Step Verification on your Google account

If you don't already have it on, go to **Google Account → Security → 2-Step Verification** and turn it on. App-Specific Passwords require it.

### 2. Create an App-Specific Password

1. Open **Google Account → Security → 2-Step Verification → App passwords**.
2. Click **Create new**, name it `simple_wiki keep` (anything works — this is just a label).
3. Google generates a 16-character password like `abcd efgh ijkl mnop`. You'll need it once and you can paste it with or without spaces.

### 3. Connect on your wiki profile

1. Visit **/profile** on the wiki. You'll be redirected to your own profile page.
2. Find the **Google Keep** section.
3. Enter your Google email and the App-Specific Password from step 2.
4. Click **Test & Save**. The wiki performs the gpsoauth exchange to obtain a master token, verifies it works against Keep, and stores it on your profile page. The App-Specific Password is consumed once and discarded — only the master token persists.

You should see **Connected as alice@example.com** within a couple of seconds. If you see an error, double-check the email + ASP and the 2-Step Verification status on your account.

## Bind a wiki checklist to a Keep note

1. Open any page with a `{{"{{ Checklist \"name\" }}"}}` macro.
2. The checklist now shows a **🔗 Bind to Keep List** button (only visible if *you* have KeepConnect configured — other household members see their own button or no button, depending on their own profile).
3. Click it. A picker appears:
   - **Use existing Keep note** — pick from a dropdown of the list-typed notes in your account.
   - **Create new Keep note named "<list_name>"** — make a fresh Keep note in your account.
4. Click **Bind**. The button is replaced with a **Synced with Keep** pill and an Unbind affordance.

### Multi-user (household) bindings

Each user's bindings live on their own profile page. The same wiki checklist can be bound to *different* Keep notes by different users — Alice binds `groceries` to her Keep note, Bob (signing in via his own Tailscale identity) binds the same `groceries` checklist to his own Keep note. The wiki is the hub; each person's Keep note is a peer.

Per-user collision rules:
- You can't bind the same `(page, list)` twice (rebind to a different note instead).
- You can't bind two different checklists to the same Keep note in your account.
- Two different users binding the same `(page, list)` to their own Keep notes is the explicit intended pattern.

## What sync does today

This bridge ships with **verification-only sync** in v1: the wiki periodically confirms your master token still works and that the bound Keep note still exists. Bidirectional data sync (item adds, edits, checks, deletes) is the next iteration.

What works today:
- Connect / disconnect.
- Bind / unbind.
- Listing your Keep notes (for the picker).
- Verifying that a bound note still exists.

What's not in v1:
- Items added on the wiki do **not** appear in the Keep note yet.
- Items checked in Keep do **not** flip on the wiki yet.

This will land in a follow-up. Track it on the GitHub issue for the bridge.

## Disconnecting

From your profile page, click **Disconnect Google Keep**. This wipes the master token but **preserves your bindings** as paused. Reconnecting later resumes them with no rebind needed.

To remove a single binding (without disconnecting the whole connector), click the **✕** next to its row in the bindings list, or click **Unbind** on the Checklist component itself. Wiki data and Keep notes are both left exactly as they are — unbind is a connection severance, not a delete.

## Errors you might see

| Banner | What it means | What to do |
|---|---|---|
| `invalid_credentials` | Google rejected the email + ASP | Verify the App-Specific Password and your email |
| `auth_revoked` | Master token no longer valid (you changed your Google password, or revoked the ASP) | Generate a fresh ASP and re-connect |
| `protocol_drift` | Google changed the Keep wire format | Update simple_wiki and try again |
| `rate_limited` | We're hitting Keep too hard | Wait a few minutes; sync resumes automatically |
| `bound_note_deleted` | The bound note was deleted from your Keep app | Rebind or remove the binding |

Errors branch on typed codes only — never on the human-readable banner text.

## For agents

The bridge exposes a per-user gRPC service `api.v1.KeepConnectorService` with these methods. All scope to the calling user via Tailscale identity → ProfileIdentifierFor; no method ever leaks another user's master token or bindings.

- `ExchangeAndStore(email, app_specific_password) → ConnectorState` — connect.
- `Disconnect() → ConnectorState` — pause.
- `GetState() → ConnectorState` — read connector + bindings.
- `ListNotes() → KeepNoteSummary[]` — proxy to the user's Keep account.
- `ListMyBindings() → BindingState[]`
- `BindChecklist(page, list_name, keep_note_id?) → BindingState`
- `UnbindChecklist(page, list_name) → ()`
- `GetChecklistBindingState(page, list_name) → ChecklistBindingState` — small surface used by the Checklist component on render.

## Why we ship the unofficial API anyway

Google has not published a Keep API. Household members on Android who use Keep specifically (not Google Tasks) had no path into wiki checklists. The CalDAV bridge serves Apple Reminders and Android-via-DAVx5; this bridge serves the Keep audience. The reverse-engineered protocol via [kiwiz/gkeepapi](https://github.com/kiwiz/gkeepapi) is the de-facto community reference; we ported the wire format to Go and pinned a specific upstream commit. See `internal/keep/protocol/REFERENCE.md` in the repo for the diagnostic flow when something breaks.
