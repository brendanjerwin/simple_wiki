# Google Tasks bridge — Brendan's personal recipe

Reproducibility notes for re-running the GCP setup from scratch on Brendan's deployment. Not for publishing; the generic guide is in `google_tasks_setup.md`. This file just captures the values specific to this deployment so a redo doesn't require rediscovery.

> **Last redo: `<placeholder>`** — update when you re-run.

## Values to plug in

| Field | Value |
| --- | --- |
| Tailnet hostname | `<placeholder — e.g. wiki.tail-XXXXX.ts.net>` |
| Wiki redirect URI | `https://<tailnet-hostname>/oauth/google/callback` |
| Google account (project owner) | `brendanjerwin@gmail.com` |
| GCP project name | `<placeholder — e.g. brendan-simplewiki>` |
| GCP project ID | `<placeholder — assigned by Google on creation>` |
| OAuth consent app name | `<placeholder — e.g. Brendan's Wiki>` |
| Support email | `brendanjerwin@gmail.com` |
| Developer contact | `brendanjerwin@gmail.com` |

## Test users

The household. Update if anyone joins/leaves:

- `brendanjerwin@gmail.com`
- `<placeholder — household member 1>`
- `<placeholder — household member 2>`

## Where the env vars live

Service env file: `<placeholder — e.g. /etc/simple-wiki/env or devbox.json>`

```bash
export SIMPLE_WIKI_GOOGLE_TASKS_CLIENT_ID="<placeholder>"
export SIMPLE_WIKI_GOOGLE_TASKS_CLIENT_SECRET="<placeholder>"
export SIMPLE_WIKI_GOOGLE_TASKS_REDIRECT_URI="https://<tailnet-hostname>/oauth/google/callback"
```

## Deployment-specific gotchas

- `<placeholder — anything specific to Brendan's host setup, e.g. service name, restart command, log location>`
- `<placeholder — anything about the Tailscale ACL or HTTPS cert config that the generic guide skips>`

## Smoke-test recipe

Personal abbreviation of the verification section in the generic guide:

1. Restart wiki: `<placeholder — e.g. systemctl --user restart simple-wiki>`
2. Open `<https://<tailnet-hostname>/profile>`
3. Connect Google Tasks → consent → confirm "Connected as <brendanjerwin@gmail.com>"
4. On a known checklist page (e.g. `<placeholder — e.g. /shopping/this_week>`), subscribe to a Tasks list
5. Add an item, verify in Google Tasks app on the phone within 30s
6. Add an item in the phone app, verify in the wiki within 30s

If any step fails, fall back to the generic guide's troubleshooting section.
