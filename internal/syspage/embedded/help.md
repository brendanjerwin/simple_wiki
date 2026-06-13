+++
identifier = "help"

[wiki]
system = true
+++

#help

# Editing

Click the `Edit` button to edit. The pages are in [Markdown](https://www.markdownguide.org/). Your changes are saved automatically.

## Links

Use normal Markdown links like `[Open route](https://maps.app.goo.gl/wo54t5YMTyhz6rqH7)`. Fully qualified external links open in a new browser tab with `noopener noreferrer` protections. Internal wiki links like `[[help-search]]` and relative links stay in the current tab.

## Frontmatter

If you see something like this at the top of the document:

```
+++
identifier = "Help"
+++
```

Just leave it. That is called "frontmatter" and it is special information for the computers to read.

## Saving

Your content is saved automagically. You'll see `Edit` turn into `saved` or `error`. If it says `error` then something is wrong and your content didn't save.

> [!TIP]
> If you messed with the frontmatter, it might have caused an error. Try putting it back.

## Chat panel

When you open the chat panel, long-running agent work now shows **live progress** instead of a blank thinking state:

- Running tool calls stay expanded with a short status line and scroll if there are many of them.
- Running sub-agents show up as distinct cards with their agent type and elapsed time.
- Completed tool calls collapse back down into compact pills so the transcript stays readable.

If you are debugging a slow turn, watch the newest assistant message while it is still running — that is where tool and sub-agent progress appears.

## Files and Images

You can drag and drop files and images right into the page. You'll see a special block of text like this: `![Web+capture_13-7-2021_0429_www.pinclipart.com.jpeg](/uploads/sha256-CU7VONBJPNC6XSXU2I3QT2WDP2ASRMHSEKJZPQEV4DI2ML6IXJBA====?filename=Web+capture_13-7-2021_0429_www.pinclipart.com.jpeg)` and you can put it wherever you want in your document.

## Tags

Add `#tag` anywhere in a page body to organize content across pages. Clicking a tag opens the search popup with that tag pre-filled.

> [!TIP]
> If you want a literal `#` (e.g. mentioning `#5` or `#1234`), put a backslash in front of it: `\#5`. The wiki shows the `#` as plain text and skips the tag extraction. Same trick works inside checklist items.

See [[help-hashtags]] and [[help-search]] for the full grammar.

## Advanced Features

- [[help-alerts]] — Styled callout boxes (note, tip, warning, caution)
- [[help-caldav]] — Bidirectional checklist sync with native task apps (Apple Reminders, DAVx5)
- [[help-google-keep]] — Per-user Google Keep bridge (unofficial — read trust model)
- [[help-hashtags]] — Page-level hashtag tagging
- [[help-handling-large-pages]] — Strategies for trimming, splitting, and navigating large pages
- [[help-macro-blog]] — Blog macro for publishing articles
- [[help-macro-checklist]] — Checklist macro for interactive task lists
- [[help-macro-survey]] — Survey macro for collecting per-user responses
- [[help-profile]] — Per-user profile pages auto-resolved at `/profile`
- [[help-profile-features]] — Catalog of profile-page widgets and macros, with copy-paste snippets to upgrade existing profiles
- [[help-scheduled-agents]] — Cron-driven background AI agent tasks per page
- [[help-search]] — Search syntax including `#tag` queries
- [[help-system-pages]] — Pages that ship with the wiki binary
- [[help-templating]] — Template language reference (macros, variables, conditionals)
