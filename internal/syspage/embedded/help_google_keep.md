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
- **Google can break the bridge any time.** When that happens, sync stops cleanly and surfaces a `protocol_drift` error to the UI. You won't lose data — both sides keep their own state.
- **Don't share screenshots of your profile page.** The master token is visible in frontmatter.

If any of this is a deal-breaker, use the [CalDAV bridge](/help_caldav/view) instead. The household member just needs an Android task app like tasks.org via DAVx5.

## Why oauth_token, not a password / App-Specific Password

Google has steadily deprecated the password-based gpsoauth `master_login` flow. App-Specific Passwords used to work — they don't anymore. Almost every account now gets `BadAuthentication` regardless of whether the ASP is correct.

The flow that *does* still work is **oauth_token capture**: sign in to Google in your browser, copy a specific HttpOnly cookie, hand it to the wiki. The wiki exchanges it for a long-lived master token via gpsoauth's `exchange_token` (a different endpoint variant that Google hasn't gated yet).

This isn't elegant. It's the only path that works.

## Setup (one-time, per user)

### 1. Sign in to the EmbeddedSetup page

In a fresh browser tab, open:

```text
https://accounts.google.com/EmbeddedSetup
```

This is Google's Android-style sign-in page. Sign in with the Google account you want to bridge.

> **Important:** When the page prompts you with "I agree" (or similar consent), click it. After that, the page may show a loading spinner forever — that's expected; ignore it and move on to the next step. The cookie is set even though the page hangs.

### 2. Copy the `oauth_token` cookie

After clicking "I agree":

1. Open DevTools (F12 or right-click → Inspect).
2. Go to the **Application** tab (Chrome) or **Storage** tab (Firefox).
3. Expand **Cookies** → `https://accounts.google.com`.
4. Find the row named **`oauth_token`** — the value will look like `oauth2_4/0Ad…` (long string).
5. Copy that value.

> The `oauth_token` cookie is HttpOnly, which means it does **not** appear in the JavaScript console. You must use the Application/Storage panel — `document.cookie` won't show it.

### 3. Paste it into your wiki profile

1. Visit **/profile** on the wiki. You'll be redirected to your own profile page.
2. Find the **Google Keep** section.
3. Enter your Google email and paste the `oauth_token` value into the password field.
4. Click **Test & Save**.

The wiki performs the gpsoauth `exchange_token` exchange to obtain a master token, verifies it works against Keep, and stores it on your profile page. The captured `oauth_token` is consumed once and never persisted — only the resulting master token is.

You should see **Connected as alice@example.com** within a couple of seconds.

### Troubleshooting the capture

- **"oauth_token isn't in the cookie list."** You missed step 1, or you completed sign-in on a different domain. The cookie only appears on `accounts.google.com` after a successful EmbeddedSetup sign-in. Try again in a private/incognito window.
- **"BadAuthentication"** after pasting. The token may be expired (they're short-lived — capture and paste within a minute or two), or the token was for a different account than the email you entered. Capture again.

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
| --- | --- | --- |
| `invalid_credentials` | Google rejected the oauth_token | Recapture (it's short-lived); make sure it's the cookie from `accounts.google.com`, not some other Google domain |
| `auth_revoked` | Master token no longer valid (you signed out, or Google revoked it) | Recapture an oauth_token and re-connect |
| `protocol_drift` | Google changed the Keep wire format | Update simple_wiki and try again |
| `rate_limited` | We're hitting Keep too hard | Wait a few minutes; sync resumes automatically |
| `bound_note_deleted` | The bound note was deleted from your Keep app | Rebind or remove the binding |

Errors branch on typed codes only — never on the human-readable banner text.

## For agents

The bridge exposes a per-user gRPC service `api.v1.KeepConnectorService` with these methods. All scope to the calling user via Tailscale identity → ProfileIdentifierFor; no method ever leaks another user's master token or bindings.

- `ExchangeAndStore(email, oauth_token) → ConnectorState` — connect.
- `Disconnect() → ConnectorState` — pause.
- `GetState() → ConnectorState` — read connector + bindings.
- `ListNotes() → KeepNoteSummary[]` — proxy to the user's Keep account.
- `ListMyBindings() → BindingState[]`
- `BindChecklist(page, list_name, keep_note_id?) → BindingState`
- `UnbindChecklist(page, list_name) → ()`
- `GetChecklistBindingState(page, list_name) → ChecklistBindingState` — small surface used by the Checklist component on render.

## Why we ship the unofficial API anyway

Google has not published a Keep API. Household members on Android who use Keep specifically (not Google Tasks) had no path into wiki checklists. The CalDAV bridge serves Apple Reminders and Android-via-DAVx5; this bridge serves the Keep audience. The reverse-engineered protocol via [kiwiz/gkeepapi](https://github.com/kiwiz/gkeepapi) is the de-facto community reference; we ported the wire format to Go and pinned a specific upstream commit. See `internal/connectors/google_keep/gateway/REFERENCE.md` in the repo for the diagnostic flow when something breaks.
