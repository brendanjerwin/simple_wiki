+++
identifier = "help"
system = true
+++

#help

# Editing

Click the `Edit` button to edit. The pages are in [Markdown](https://www.markdownguide.org/). Your changes are saved automatically.

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

## Files and Images

You can drag and drop files and images right into the page. You'll see a special block of text like this: `![Web+capture_13-7-2021_0429_www.pinclipart.com.jpeg](/uploads/sha256-CU7VONBJPNC6XSXU2I3QT2WDP2ASRMHSEKJZPQEV4DI2ML6IXJBA====?filename=Web+capture_13-7-2021_0429_www.pinclipart.com.jpeg)` and you can put it wherever you want in your document.

## Tags

Add `#tag` anywhere in a page body to organize content across pages. Clicking a tag opens the search popup with that tag pre-filled.

> [!TIP]
> If you want a literal `#` (e.g. mentioning `#5` or `#1234`), put a backslash in front of it: `\#5`. The wiki shows the `#` as plain text and skips the tag extraction. Same trick works inside checklist items.

See [[help-hashtags]] and [[help-search]] for the full grammar.

## Advanced Features

- [[help-alerts]] — Styled callout boxes (note, tip, warning, caution)
- [[help-templating]] — Template language reference (macros, variables, conditionals)
- [[help-macro-blog]] — Blog macro for publishing articles
- [[help-macro-checklist]] — Checklist macro for interactive task lists
- [[help-macro-survey]] — Survey macro for collecting per-user responses
- [[help-scheduled-agents]] — Cron-driven background AI agent tasks per page
- [[help-search]] — Search syntax including `#tag` queries
- [[help-hashtags]] — Page-level hashtag tagging
- [[help-system-pages]] — Pages that ship with the wiki binary
