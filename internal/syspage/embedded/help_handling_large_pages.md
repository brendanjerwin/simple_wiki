+++
identifier = "help_handling_large_pages"

[wiki]
system = true
+++

#help #large-pages #agents

# Handling Large Pages

Large wiki pages are expensive to render, search, and send to chat agents. Use this page when a page starts feeling slow, when an agent runs out of context, or when a page has become a running log instead of a usable source of truth.

## Symptoms

Treat a page as large when one or more of these are true:

- The page is above about 30 KB of markdown. Agents should call `api_v1_PageManagementService_ReadPageOutline` before reading the whole page.
- The page is near 15k tokens or more. Start trimming or splitting before this becomes a hard failure.
- Rendering, editing, or chat context feels slow.
- The useful current answer is buried under old entries, pasted raw data, or repeated sections.
- A chat agent spends most of its turn reading the page instead of doing the work.

## First Move: Read the Outline

For agent or API work, call `api_v1_PageManagementService_ReadPageOutline` before `ReadPage` on large pages. It returns headings, byte offsets, byte lengths, total bytes, and the version hash. Use the outline to pick the section you actually need instead of loading the full page.

When editing one section, prefer `UpdatePageContent` with `old_content_markdown` and `new_content_markdown`. That keeps the edit small and avoids rewriting unrelated content.

## Trim Strategies

Prefer pages that preserve decisions and current state, not every intermediate detail.

- Replace old blow-by-blow logs with short summaries that keep decisions, dates, owners, and links.
- Delete raw command output once the useful conclusion is captured.
- Keep evidence only when it will be used again. Otherwise summarize it and link to the source page, issue, PR, or release.
- Remove duplicate lists, repeated prompts, and stale "next steps" that no longer apply.
- Convert "what happened every time" into "what is true now" plus a short history when history matters.

### No Archives Principle

Do not solve a large page by moving every old section into an "archive" page and linking it back. That usually creates two bad pages: one current page that still depends on an unreadable dump, and one archive page nobody can navigate.

Use an archive only when it has a clear reader and retention purpose. Most of the time, summarize old material and delete the raw bulk.

## Splitting Strategies

Split a page when different parts have different lifecycles, owners, or readers.

Good split candidates:

- Long project logs -> one project overview plus dated decision or status pages.
- Repeated meeting notes -> one index page plus one page per meeting.
- Research dumps -> one synthesis page plus one source-notes page per source or topic.
- Operational runbooks -> one overview plus separate runbooks for each procedure.

Suggested naming:

- Keep the parent identifier stable.
- Use child identifiers with a clear prefix, such as `project-name-decisions`, `project-name-2026-05-status`, or `project-name-runbook-deploy`.
- Link both ways: parent links to children, and each child links back to the parent.
- Put a one-paragraph summary on the parent so readers can decide whether to open the child page.

## Frontmatter Offloading

Move structured data out of prose when the wiki has a typed feature for it.

- Checklists belong in `wiki.checklists.<list-name>` and should be mutated through `ChecklistService`.
- Blog collections should use `[blog]` frontmatter and the `Blog` macro instead of manually concatenating posts.
- Schedules and agent state should live in their dedicated frontmatter sections instead of copied status text.
- Inventory-like relationships should use the inventory frontmatter model instead of giant inline lists.

Frontmatter offloading helps because tools can read and mutate one structured object instead of re-parsing a wall of markdown.

## Collapsible Sections

Use collapsible headings for material that is useful but not needed on every read.

```
#^ Old Deployment Notes

This section starts collapsed in the rendered page.
```

Collapsible sections improve human scanning, but they do not reduce the markdown size or token cost when an agent reads the source. Use them for readability; use trimming or splitting for context pressure.

## Checklists

Use `{{ Checklist "list-name" }}` when a page has actionable items.

Prefer checklist items with descriptions over giant inline bullet lists:

- The item text should be the action or thing to buy.
- Put details in the item description.
- Use tags for filtering, such as `#errand`, `#blocked`, or `#today`.
- Avoid adding the same open item repeatedly; the checklist API rejects duplicate open item text.

See [[help-macro-checklist]].

## Blog Macro Pattern

Use `{{ Blog "blog-id" 10 }}` when a page is a stream of dated posts.

Each post should be its own page with `blog.identifier` frontmatter. The index page stays small and the macro aggregates the latest entries. Do not keep appending full posts to one giant page.

See [[help-macro-blog]].

## Transclusion and Includes

There is no general-purpose "include this whole page here" pattern to use as a dumping ground. Prefer purpose-built macros (`Blog`, `Checklist`, `Survey`, inventory helpers) or normal links.

If you need another page's content, link to it and summarize why it matters. Copying or including large source pages usually recreates the same context problem in a new place.

## Proactive Triggers

Act before the page becomes painful:

- Around 30 KB: agents should use `ReadPageOutline` first.
- Around 15k tokens: trim or split during the same work session.
- When a section is mostly old status: summarize it.
- When a list gets repeated updates: make it a checklist, child page, or blog stream.
- When a page has multiple unrelated audiences: split it.

## Anti-Patterns

Avoid these:

- Creating archive pages that preserve all bulk without a reader or retention reason.
- Paraphrasing source data into a page when a link and short conclusion are enough.
- Pasting entire command outputs, logs, or API responses after the useful fact is known.
- Concatenating dated posts into one page instead of using the blog macro.
- Keeping old TODO sections after the work has moved to issues, checklists, or PRs.
- Hiding bulk behind collapsible headings and assuming that solved agent token pressure.

## Agent Checklist

When asked to work on a large page:

1. Search for the relevant page and call `ReadPageOutline` first.
2. Read only the relevant section when possible.
3. If editing, update the smallest stable section.
4. If the page is too large because of stale bulk, summarize or split as part of the task.
5. Leave a short current-state summary and links to any child pages.
