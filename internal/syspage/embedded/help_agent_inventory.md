+++
identifier = "help_agent_inventory"

[wiki]
system = true
+++

#help #agents

# Agent Inventory

The **available-agents inventory** is the registry of every agent that page
chat can route to. The chat panel's agent selector (see [[help-chat]]) lists
exactly these entries.

## Where agents come from

The inventory is layered, with one authority per source:

- **Local ACP agents** — registered by the wiki's pool daemon. The pool is the
  sole authority on local agents: it registers its descriptor (name, route)
  with the server at startup and on every reconnect, and the entry goes grey
  the moment the pool disconnects.
- **Remote AG-UI agents** — loaded from the server's own static configuration
  (`--agent-config` / `WIKI_AGENT_CONFIG`, a JSON file):
  ```json
  {
    "agents": [
      {
        "id": "agui-helper",
        "display_name": "Helper",
        "description": "Platform-hosted helper with workspace access",
        "endpoint": "https://agentos.example/agent/helper"
      }
    ]
  }
  ```
  Remote entries are visible and bindable as soon as they are configured; they
  show as unavailable until the AG-UI routing backend is brought online.
- **Live platform discovery** is deliberately not implemented yet; the layering
  leaves room for it once the AgentOS platform contract stabilizes.

## The per-page binding

Which agent a page talks to is a durable property of the page, stored in the
`agent.chat_agent` frontmatter subtree. Per the reserved-namespace policy
([[ADR-0009]]), `AgentInventoryService` is its only mutation path — the generic
frontmatter tools reject `agent.*` writes. The binding records who switched and
when, and surfaces in the page's chat history as a visible event.

Any participant may switch. The change applies to everyone on the page
immediately and persists across sessions and pool restarts. Switching does not
abort an in-flight turn: the current agent finishes, and the next message
routes to the new one with the conversation transcript carried over.

## For Agents

- `AgentInventoryService.ListAgents` — inventory entries with `id`,
  `display_name`, `protocol` (`ACP_LOCAL` / `AG_UI_REMOTE`), `description`,
  `route`, and live `available`.
- `AgentInventoryService.GetPageAgent` — the page's *desired* agent.
- `AgentInventoryService.SetPageAgent` — bind an agent (optionally with
  `expected_updated_at` OCC guard; a concurrent switch returns
  `FailedPrecondition`).
- `ChatService.GetChatStatus` — includes the *effective* agent
  (`effective_agent_id`): what is actually attached right now. During a switch
  window, desired and effective differ until the next message spawns the new
  agent.

See also [[help-chat]] for the chat panel itself and
[[help-scheduled-agents]] for headless cron-driven turns.