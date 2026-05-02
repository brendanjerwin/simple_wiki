+++
identifier = "help_profile_features"
title = "Profile Features Catalog"

[wiki]
system = true
+++

#help #profile

# Profile Features Catalog

> See [[help-profile]] for how profile pages work in general (identifier scheme, first-visit creation, authorization). This page is the **catalog of bits you can paste into your profile** to light up wiki features.

## What this page is

Your wiki creates a profile page for you on first visit, seeded from `profile_template`. New features sometimes need new bits in your profile to work — a custom element, a connect-button macro, a banner. The wiki **cannot retroactively update your profile**: the syspage sync only touches pages flagged `wiki.system = true`, and your profile isn't one of them. (That's deliberate — the wiki shouldn't trample customizations you've made.)

So instead of auto-upgrading, this page lists every part of the current `profile_template` and explains why you might want it. Skim it whenever you want to refresh your profile against the latest features. Each release that ships a new profile-eligible feature adds a section here.

## How to apply changes

1. Navigate to `/profile` — the wiki redirects to your own profile page.
2. Click **Edit**.
3. Paste the snippet from the section below into the **body** of the page (anywhere below the frontmatter).
4. Save.

> [!CAUTION]
> **Never modify the frontmatter** when adding these snippets. The frontmatter (the block between `+++` markers at the top) holds critical settings like `wiki.authorization.acl` (who owns the page) and `wiki.system.*` flags. Touching it can lock you out of your own profile or break system invariants. The wiki's edit UI preserves frontmatter when you save body changes — keep it that way.
>
> Everything in this catalog goes in the **body**.

If you have shell access and prefer to use the agent surface, the `mcp__wiki__api_v1_PageManagementService_UpdatePageContent` RPC updates the body without touching the frontmatter — that's the safest CLI path.

## Macros vs. raw elements

The wiki's templating engine renders **macros** like `{{ KeepConnect }}` into the right HTML element with the right attributes wired up. You paste the macro form. Pasting raw `<keep-connect>` would land an unwired element on the page — it would render visually but wouldn't query the connector backend, scope to your identity, or surface connect/connected/error states.

The exception is `<profile-paused-banner>`, which has no macro wrapper today — paste it as-is.

When in doubt: **macro form goes inside `{{ … }}` braces; raw elements look like `<…>` HTML tags.** This catalog tells you which form each part wants.

## The parts of a profile

Each entry below is a piece of the current `profile_template`. Pick the ones you want; skip the ones you don't use.

### `<profile-paused-banner>` (raw element)

**What it does.** Silent unless one of your connector subscriptions has been paused (usually because an OAuth credential expired or was revoked). When that happens, a banner appears at the top of your profile with a click-target that scrolls to the matching connect button so you can reconnect in one click.

**Why you might want it.** If you use any of the [[help-connectors]] — Google Keep, Google Tasks, etc. — sync can stop silently when credentials expire. Without this banner, the first you'll know is when a checklist stops round-tripping. With it, paused subscriptions get a loud, actionable surface the moment you open your profile.

**How to add it.** Paste the raw element anywhere in the body. The conventional spot is at the top of a `## Connectors` section so paused-state surfaces near the connect buttons it directs you to.

```html
<profile-paused-banner></profile-paused-banner>
```

This one is the exception to the macro rule — there's no `{{ ProfilePausedBanner }}` wrapper today, so paste the element tag directly.

**When to skip it.** If you don't use any connectors at all, the banner will simply never show — there's no harm in leaving it there, but no benefit either.

### `{{ KeepConnect }}` (macro)

**What it does.** Renders the **Connect Google Keep** button (and its connected/error states). Once connected, your wiki checklists can subscribe to individual Keep notes and round-trip items.

**Why you might want it.** Add it if you want to bridge wiki checklists to the Google Keep app on your phone or laptop. Read [[help-google-keep]] before connecting — Keep uses an unofficial API that requires capturing an `oauth_token` cookie, and the trust model is worth your attention.

**How to add it.** Paste the macro into the body, conventionally under a `## Connectors` heading.

```
{{ KeepConnect }}
```

This is a macro, not a raw element — paste it exactly as shown, with the `{{ }}` braces. The templating engine expands it into a `<keep-connect>` element that queries the connector backend and scopes to the calling identity. Pasting `<keep-connect>` directly would render an unwired element and not work.

The macro takes no arguments.

**When to skip it.** Skip if you don't use Google Keep, or if you'd rather use [[help-google-tasks]] or [[help-caldav]] instead. Each checklist binding is globally exclusive — you can't subscribe one checklist to two services at once — so picking your bridge per use-case matters.

### `{{ GoogleTasksConnect "<profile-identifier>" }}` (macro)

**What it does.** Renders the **Connect Google Tasks** button (and its connected/error states). Standard OAuth — no password capture, no master tokens. Once connected, wiki checklists can subscribe to Google Tasks lists and round-trip items.

**Why you might want it.** Google Tasks is the cleanest of the Google bridges: real public API, real OAuth, no reverse-engineered protocol. If you live on Android or use Tasks via Calendar on the web, this is the connector to add. See [[help-google-tasks]] for the full setup including the per-deployment operator step.

**How to add it.** Paste the macro into the body, conventionally under a `## Connectors` heading. Replace `<profile-identifier>` with your actual profile page's identifier (a quoted string).

```
{{ GoogleTasksConnect "<profile-identifier>" }}
```

A real example, after substitution:

```
{{ GoogleTasksConnect "profile_brendanjerwin_gmail_com" }}
```

This is a macro, not a raw element — paste it exactly with the `{{ }}` braces. The templating engine expands it into a `<google-tasks-connect profile-id="…">` element with the OAuth state scoped to that profile. Pasting `<google-tasks-connect>` directly would render an unwired element and not work.

The identifier argument is **required** — it scopes the rendered element's OAuth state lookups to your specific profile page.

**Finding your profile identifier.** Two equally good ways:

1. **Look at the URL.** When you're on your profile page, the part of the URL after the host is the identifier. If you're at `https://wiki.example.com/profile_brendanjerwin_gmail_com`, then `profile_brendanjerwin_gmail_com` is your identifier — the page name *is* the identifier.
2. **Look at the frontmatter.** Open your profile in the editor and look at the `+++` frontmatter block at the top. You'll see a line like `identifier = "profile_brendanjerwin_gmail_com"`. Copy the quoted value.

**When to skip it.** Skip if you don't use Google Tasks, or if your wiki operator hasn't configured the Google Tasks OAuth client for this deployment yet (the connect button will tell you so).

## Maintenance promise

When the wiki ships a new feature that benefits from a profile-page entry — a new connector, a new system surface, a new convenience widget — this page gets a new section in the same release. The help update is part of the feature, not a follow-up.

If you find a feature in the changelog that's not yet documented here, please file an issue.
