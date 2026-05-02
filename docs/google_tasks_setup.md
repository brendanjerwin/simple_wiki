# Google Tasks bridge — operator setup

This doc walks the wiki operator through the one-time GCP project setup required to enable the Google Tasks bridge for end users.

If you don't do this, the wiki's Tasks integration stays disabled with a clear "not configured by this wiki's operator" message in the user-facing UI. No errors, no half-working state — it's an opt-in feature gated on these env vars being present.

## Why this is required

Google does not allow shared OAuth credentials for self-hosted apps. Each deployment of simple_wiki must provision its own Google Cloud Platform project and create its own OAuth client. Dynamic Client Registration (DCR) isn't an option — Google's IdP doesn't support it for the Tasks API. So this is a per-deployment, one-time chore.

## Step-by-step

> **Note on screenshots:** the GCP Console UI changes regularly. This doc describes the flow without inline screenshots because they go stale fast. If you're adding screenshots, include a `_Last verified: YYYY-MM-DD_` marker so the next operator can tell at a glance whether they're current.

### 1. Create a GCP project

1. Go to <https://console.cloud.google.com>.
2. Click the project picker (top of the page) → **New Project**.
3. Name it something memorable, e.g. `MyHousehold-SimpleWiki`. The name is internal; users won't see it unless your OAuth consent screen is misconfigured.
4. Leave organization / location at defaults unless you have a reason to change them.
5. Click **Create**.

### 2. Enable the Google Tasks API

1. In your new project, go to **APIs & Services → Library**.
2. Search for **Google Tasks API**.
3. Click it, then click **Enable**.

### 3. Configure the OAuth consent screen

1. Go to **APIs & Services → OAuth consent screen**.
2. Choose **External** user type, click **Create**.
3. Fill out the **App information**:
   - **App name**: e.g. `MyHousehold Wiki`
   - **User support email**: your email
   - **App logo**: optional; users see this on the consent screen
   - **Developer contact information**: your email
4. Click **Save and Continue**.
5. **Scopes** step: click **Add or Remove Scopes**, search for `tasks`, and check `https://www.googleapis.com/auth/tasks` (read/write — the readonly variant doesn't satisfy `tasks.insert`/`patch`/`delete`).
6. Click **Save and Continue**.
7. **Test users**: add each household member's Google email. Cap is 100 — sufficient for households. Each test user must explicitly accept this app's consent screen the first time they connect.
8. Click **Save and Continue**, then **Back to Dashboard**.

> **Stay in Testing mode.** Production-mode verification requires a privacy policy URL, app verification by Google, and supports >100 users. For household use, Testing mode is indefinite and sufficient.

### 4. Create the OAuth client credentials

1. Go to **APIs & Services → Credentials**.
2. Click **Create Credentials → OAuth client ID**.
3. **Application type**: **Web application**.
4. **Name**: e.g. `simple_wiki web client`. Internal label only.
5. **Authorized redirect URIs**: add exactly:
   ```text
   https://<your-tailscale-hostname>/oauth/google/callback
   ```
   Replace `<your-tailscale-hostname>` with your wiki's actual hostname (e.g. `wiki.tailnet-name.ts.net`).

   > **⚠️ Byte-exact match required.** RFC 9700 §4.1.1 requires the redirect URI in the OAuth request to byte-match what's registered here. Trailing slash matters. Scheme matters. Path matters. If `https://wiki.example.com/oauth/google/callback` is registered, then `https://wiki.example.com/oauth/google/callback/` (with trailing slash) will be rejected by Google with `redirect_uri_mismatch`.
6. Click **Create**. A modal shows your **Client ID** and **Client Secret**. Copy both now — you'll plug them into the wiki's environment in the next step.

### 5. Configure the wiki

Add these three env vars to your wiki service environment (`devbox.json` env section, systemd `EnvironmentFile`, Docker `--env-file`, however you run it):

```bash
export SIMPLE_WIKI_GOOGLE_TASKS_CLIENT_ID="<your-client-id>"
export SIMPLE_WIKI_GOOGLE_TASKS_CLIENT_SECRET="<your-client-secret>"
export SIMPLE_WIKI_GOOGLE_TASKS_REDIRECT_URI="https://<your-tailscale-hostname>/oauth/google/callback"
```

Restart the wiki service so it picks up the new environment.

## How to know it worked

After restart, walk through the verification flow. Each step has a concrete success signal — if a step doesn't behave as described, jump to the troubleshooting section.

1. **Open `/profile` on the wiki.** You should see a **Google Tasks** section with a **Connect Google Tasks** button. *If the section says "not configured by this wiki's operator", the env vars didn't make it into the running service environment.*
2. **Click Connect Google Tasks.** You should be redirected to Google's consent screen showing your project's app name (e.g. `MyHousehold Wiki`) and the requested scope (`See, edit, create, and delete all your tasks`). *If you see `redirect_uri_mismatch` here, the redirect URI registered in GCP doesn't byte-match the env var. Trailing slash is the most common cause.*
3. **Approve.** You should be redirected back to your wiki profile, which should now read **Connected as <your-google-email>**.
4. **Open any page with a `{{"{{ Checklist \"name\" }}"}}` macro.** The checklist should now show a **Subscribe to Google Tasks list** button.
5. **Click it.** A list picker should open populated with your actual Google Tasks lists. *If the picker is empty but you have Tasks lists in the Google Tasks app, check that the OAuth consent screen scope includes `tasks` (not `tasks.readonly`) and that the test user email matches the Google account you authenticated with.*
6. **Pick a list and confirm.** The button should be replaced with `✓ Synced with Google Tasks list <title>`.
7. **Add an item to the wiki checklist.** Within ~30 seconds, it should appear in your Google Tasks app. If you inspect via the Tasks API, the task's `notes` field will contain a trailing line like `— wiki:uid=01HZX9...` — that's the wiki's identity marker.
8. **Add an item in the Google Tasks app.** Within ~30 seconds, it should appear in the wiki checklist.

## Troubleshooting

- **"Tasks integration not configured by this wiki's operator."** Env vars aren't set, or aren't exported into the service environment. `printenv | grep SIMPLE_WIKI_GOOGLE_TASKS` from a shell *inside* the service environment to verify. Common pitfall: setting them in your interactive shell but not in the systemd unit's `EnvironmentFile`.
- **Redirect URI mismatch error from Google** (`Error 400: redirect_uri_mismatch`). The `SIMPLE_WIKI_GOOGLE_TASKS_REDIRECT_URI` env var doesn't byte-match the URI registered in GCP. Check trailing slash, scheme (`http` vs `https`), and exact hostname/path. Both must be character-for-character identical.
- **Scope downgrade error** (`scope ... not granted`). The OAuth consent screen's scope list doesn't include `https://www.googleapis.com/auth/tasks` (read/write). Re-edit the consent screen and add it. Note this is *not* `tasks.readonly`.
- **`access_denied` in the callback URL.** User declined consent on Google's screen. Click Connect again to retry.
- **`invalid_grant` after a long delay.** The user's refresh token may have been revoked at <https://myaccount.google.com/permissions>, or rotated and lost in a race. The wiki retries once automatically; if that fails, the subscription is marked paused and the user gets a click-target badge to reconnect.
- **`org_internal` consent screen instead of External.** You picked Internal user type and don't have a Workspace organization. Edit the consent screen and switch to External.
- **Test user not in test list.** A non-test-user trying to authenticate gets a "this app is in testing" wall. Add them to the test users list (cap 100) under OAuth consent screen → Test users.

## Maintenance

- The OAuth consent screen in Testing mode has no expiration; you don't need to re-verify periodically.
- The OAuth client credentials don't expire. The Client Secret can be rotated (regenerated) from the Credentials page; rotating requires updating the wiki's env var and restarting.
- If Google deprecates an API or scope, the wiki will surface a typed error to the user via the per-checklist paused badge. Consult upstream Google Tasks API release notes.

## Threat model note

The Client Secret is server-to-server credential material; it should be treated as confidential. The wiki stores user-side OAuth refresh tokens at-rest in plaintext on user profile pages — this is deliberate, see ADR-0014. The Tailnet is the perimeter; if your wiki is accessible from outside your Tailnet, this stance no longer holds and you need to revisit the security model before deploying.
