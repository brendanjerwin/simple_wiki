+++
identifier = "help_chat"

[wiki]
system = true
+++

#help #agents

# Chat & Live Activity

Every page has an AI assistant chat panel. While the assistant works a turn, the
panel shows **live activity** so you can see what it is actually doing instead of
staring at a blank "thinking" state — which matters most during long-running turns
like a daily reflection or a dinner plan.

## Live tool-use progress

As the assistant invokes tools, each one appears in the message it belongs to:

- **While a tool is running** (`pending` or `in_progress`) it shows an expanded
  row: a status indicator, a glyph for the tool kind (read, edit, execute,
  search, think, fetch, …), the tool title, the elapsed time, and a detail line
  (for example the file `path:line` it is touching). The detail area is
  height-bounded and scrolls, so a chatty tool can't blow out the layout.
- **When a tool finishes** (`completed` or `failed`) the row collapses to a
  compact single-line summary (for example `✅ Read File` or `❌ Edit File`) so
  finished work gets out of the way while live work stays visible.

## Plan progress

When the assistant reports an execution plan, the panel renders it as a live
checklist that updates in place as the work proceeds: `☐` pending, `🔄` in
progress, `☑` completed. This is the at-a-glance "what is it doing right now"
indicator for multi-step turns.

## For Agents

These indicators are bound to the [Agent Client Protocol
(ACP)](https://agentclientprotocol.com) model — there is nothing to author on the
page. Any agent driven through the wiki pool surfaces them automatically:

- ACP `tool_call` / `tool_call_update` notifications carry `kind`, `status`
  (`pending` / `in_progress` / `completed` / `failed`), `rawInput`, `content`,
  `rawOutput`, and `locations`. The pool bridge forwards `kind` and a concise
  `detail` describing what the tool is doing — the affected `path:line`, else the
  tool input (for pi-acp MCP calls, the actual nested tool name + args), else the
  output. The agent's `title` is often a generic category (e.g. `mcp`), so the
  `detail` is where the specifics show up.
- ACP `plan` notifications are forwarded as a structured plan attached to the
  current assistant message — not flattened into the message text — so the UI can
  render the live checklist above.

ACP has no concept of sub-agents; an agent's own delegated work appears in the
stream as ordinary tool calls, rendered like any other.

The pool daemon self-updates: it periodically checks the wiki server's build
commit and, on a mismatch, drains in-flight work and exits so its bootstrapper
downloads the matching `wiki-cli` and restarts. A server deploy therefore
propagates to the pool within a few minutes without manual intervention. Set
`WIKI_ACP_DEBUG=1` on the pool to log the full ACP tool-call payloads for
debugging.

See also [[help-scheduled-agents]] for headless, cron-driven agent turns (which
record their activity to page history rather than the live chat panel).
