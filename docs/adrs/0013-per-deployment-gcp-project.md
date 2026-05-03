# ADR-0013: Per-Deployment GCP Project for Google API Connectors

## Status

Accepted

## Date

2026-05-02

## Context

The Google Tasks connector (and any future Google API connector) requires OAuth 2.0 to access user data. OAuth requires a registered client — a GCP project with an OAuth consent screen, an OAuth 2.0 client ID and client secret, and authorized redirect URIs.

The natural question for a self-hosted, open-source wiki: *can the project ship with a built-in shared OAuth client so operators don't have to set one up?*

Investigation says no:

- **Per-project quotas.** Google Tasks API has per-project quotas. A shared client baked into the binary would aggregate every deployment's traffic into one project's quota, throttling everyone whenever any one user is busy.
- **Testing-mode 100-user cap.** OAuth consent screens in "Testing" mode are capped at 100 unique users. A shared community client would burn through that cap with the first 100 deployments.
- **Production-mode verification.** Moving the consent screen to "Production" requires Google's app verification (privacy policy URL, homepage URL, security review for sensitive scopes). Verification is a per-project, per-publisher process. The wiki project has no central publisher to own it on behalf of every deployment.
- **RFC 7591 Dynamic Client Registration is not supported by Google.** DCR would let each deployment register its own client at boot — exactly what we want — but Google's OAuth server doesn't implement it.
- **Manual auth code paste (the old "oob" flow) was deprecated by Google for new clients in 2022.**
- **Device authorization flow (RFC 8628)** would skip the redirect URI entirely, but the `tasks` scope is **not on Google's device-flow allowlist**.

Every shared-credentials path is closed. Each deployment must own its GCP project.

## Decision

Each deployment of `simple_wiki` provisions and owns its own GCP project. Operator setup is a one-time, ~15-minute walkthrough documented at `docs/google_tasks_setup.md`, covering:

- Creating a GCP project.
- Enabling the Google Tasks API.
- Configuring the OAuth consent screen (Testing mode is sufficient for household-scale deployments under 100 users).
- Creating an OAuth 2.0 client ID + secret with the deployment's redirect URI (`https://<wiki-host>/oauth/google/callback`).

### Configuration via environment variables, not the wiki UI

OAuth credentials are deployment infrastructure, not user data. They live in environment variables read at startup:

- `SIMPLE_WIKI_GOOGLE_TASKS_CLIENT_ID`
- `SIMPLE_WIKI_GOOGLE_TASKS_CLIENT_SECRET`
- `SIMPLE_WIKI_GOOGLE_TASKS_REDIRECT_URI`

Future Google connectors (Calendar, Drive, etc.) follow the same pattern with their own `SIMPLE_WIKI_GOOGLE_<API>_*` prefix.

### Connectors are opt-in by env var presence

If the env vars for a connector are unset, the connector is **disabled** at startup. No errors, no warnings, no UI banners — the connector simply does not register with `ConnectorService`, the subscribe button does not list it as an option, and nobody is bothered. An operator who only wants Keep, or only wants Tasks, sets only the env vars they need.

## Consequences

### Positive

- Each deployment has its own quota, its own consent screen, its own verification status. No tragedy-of-the-commons.
- Honest about the cost: ~15 minutes of GCP Console clicking is the price of a Google connector. The walkthrough is dated, screenshotted, and covers verification edge cases.
- Future Google APIs follow the same pattern — no architectural revisit.
- Opt-in by env var means a fresh install with no Google credentials runs cleanly with no errors.

### Negative

- Operator setup is unavoidable. No "just install and it works" path for Google connectors. Operators who never use a Google connector pay no cost; operators who do, pay ~15 minutes once.
- Walkthrough screenshots will go stale as Google reskins the GCP Console. The walkthrough is dated; reviewers re-shoot when they notice drift.

### Neutral

- iCloud Reminders, when added, will need its own equivalent walkthrough (`docs/icloud_reminders_setup.md`) following the same env-var pattern with `SIMPLE_WIKI_ICLOUD_*`.

## Alternatives considered

- **RFC 7591 Dynamic Client Registration.** Rejected: Google's OAuth server does not implement DCR.
- **Shared community OAuth client baked into the binary.** Rejected: per-project quota aggregation, Testing-mode 100-user cap, no central publisher to own verification.
- **Manual authorization code paste (oob flow).** Rejected: Google deprecated this flow for new clients in 2022.
- **Device authorization flow (RFC 8628).** Rejected: the `tasks` scope is not on Google's device-flow allowlist.
- **Configure credentials via the wiki UI instead of env vars.** Rejected: OAuth credentials are deployment-level secrets, not per-user state. The UI would need an admin-only section, secret-redaction in the page renderer, and a backup/restore story — none of which add value over an env var.

## References

- Plan: `now-that-we-landed-groovy-pizza.md` (Operator setup section).
- ADR-0014: Limited security stance (companion ADR on what *isn't* in scope).
- `docs/google_tasks_setup.md` — operator walkthrough.
- RFC 6749 — OAuth 2.0 Authorization Framework.
- RFC 7591 — OAuth 2.0 Dynamic Client Registration Protocol (rejected: not supported by Google).
- RFC 8628 — OAuth 2.0 Device Authorization Grant (rejected: scope not on allowlist).
