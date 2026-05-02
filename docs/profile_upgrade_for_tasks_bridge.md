# Upgrading existing profile pages for the Google Tasks bridge

## Why this is needed

The wiki ships `internal/syspage/embedded/profile_template.md` as a system
**template** page. The `/profile` route only consults that template the **first
time** a given user visits it — at that moment the rendered body is written to
their personal profile page (e.g. `profile_brendanjerwin_gmail_com_<hash>`).
After that, the profile page is just a normal user page; subsequent updates to
the embedded template are **not** retroactively applied to existing profile
pages. (See `server/profile_handler.go:createProfileFromTemplate` and the
syspage `Sync` loop in `internal/syspage/loader.go`, which only touches pages
flagged `wiki.system = true` — your personal profile is not one of them.)

That means anyone whose profile page was created before the Google Tasks
bridge landed will not see the new `<profile-paused-banner>` or the
`<google-tasks-connect>` element on their profile until they edit the page
themselves.

## Snippet to paste

Open your profile page in the wiki UI (or via
`mcp__wiki__api_v1_PageManagementService_UpdatePage`) and append this section
to the body. If you already have a `## Connectors` heading, just paste the
three element/macro lines under it.

```markdown
## Connectors

<profile-paused-banner></profile-paused-banner>

{{ KeepConnect }}

{{ GoogleTasksConnect "PROFILE_IDENTIFIER" }}
```

Replace `PROFILE_IDENTIFIER` with **your** profile page's identifier — the
slug in the URL when you view your profile, e.g.
`profile_brendanjerwin_gmail_com_4f3a91c2`. The argument is required so the
custom element can scope its OAuth state lookups to the right page.

> Note: the macros (`{{ KeepConnect }}`, `{{ GoogleTasksConnect ... }}`) are
> evaluated by the templating engine on every page render, so you can paste
> them as-is. The `<profile-paused-banner>` element is plain HTML (no macro
> wrapper) — it queries the connector backend itself and stays silent when
> nothing is paused.

## Step-by-step

1. Navigate to your profile (`/profile` redirects there) and click **Edit**.
2. Scroll to the bottom and paste the snippet above.
3. Substitute your real profile identifier into the `GoogleTasksConnect`
   argument.
4. Save.
5. Reload the page. You should now see a **Connect Google Tasks** button (or
   the connected state if you've already linked an account elsewhere) and the
   pause banner will appear when relevant.

## Coverage

This is per-page. Repeat for every household member with an existing profile.
New users who hit `/profile` after this version is deployed will get the
updated body automatically — they don't need to do anything.

---

## Should this be an automated migration?

### The case for

OSS deployers picking up this version will hit exactly the same gap: any
profile pages that already exist on their tailnet won't pick up the new
Connectors section, and the household members on those pages will silently be
unable to connect Google Tasks. A one-shot eager migration would inject the
new elements on the next boot after the upgrade, with no per-user manual
step.

### The case against

Profile pages are user-editable. A user may have already added their own
content to the page — including their own custom `## Connectors` heading or
adjacent content. A blind insertion could:

- Duplicate elements if the user has already pasted a snippet manually.
- Land in a surprising spot relative to user-authored content.
- Conflict with whatever section structure the user has built up.

There is no canonical "ConnectorService section" concept in the wiki page
model — the Connectors heading is just a markdown convention from the
template, not a structured region. Any auto-injection has to be a heuristic.

### Recommended path

Write a one-shot eager migration following the
`migrations/eager/system_template_namespace_migration.go` pattern:

1. Scan all `.md` files; filter to identifiers starting with `profile_` and
   excluding `profile_template`.
2. For each, read the markdown body. Check for an idempotency marker (e.g.
   `<profile-paused-banner>` already present, OR a frontmatter flag like
   `wiki.profile_connectors_injected = true`).
3. If the body already has a `## Connectors` heading, replace its content
   with the canonical snippet (preserving anything the user added below the
   macros). If not, append a fresh `## Connectors` section to the end.
4. Set `wiki.profile_connectors_injected = true` so re-runs noop.
5. Register the scan job in `server/site.go` next to the other eager
   migrations.

A safer variant: **only inject when the body still matches the
pre-Connectors original** (i.e. the user hasn't customized) and skip with a
log line otherwise. Users who customized would still need the manual snippet
above, but no one would be surprised by an auto-edit.

### Effort estimate

Approximately 80–120 lines of Go (scan job + per-page job + helper) plus
table-driven tests, plus the registration line in `server/site.go`. Roughly
**2 hours** for a sub-agent following the existing migration template. The
tricky parts are (a) deciding the conservative vs. aggressive variant and
(b) writing enough idempotency tests to be confident a re-run can't double
up the elements.

### Recommendation

**Yes, do this**, with the conservative variant (only inject when the body
matches the unmodified template output). Track it as a follow-up issue;
current users on this branch can use the manual snippet above in the
meantime.
