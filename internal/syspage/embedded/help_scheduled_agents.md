+++
identifier = "help_scheduled_agents"

[wiki]
system = true
+++

#help #agents

# Scheduled Agents

Pages can declare cron-driven background AI agent tasks in their frontmatter. The wiki server fires them on schedule, runs them in headless one-shot agent instances on the pool, and records what they did so future interactive chats on the same page see the history.

> [!NOTE]
> The `agent.*` top-level frontmatter namespace is **reserved**. Generic frontmatter tools (`MergeFrontmatter`, `ReplaceFrontmatter`, `RemoveKeyAtPath`) reject writes touching it. All schedule and chat-context mutations go through `api_v1_AgentMetadataService_*`.
>
> Other reserved namespaces follow the same pattern documented in [[ADR-0009]]: `wiki.*` (wiki-managed metadata for checklists and future siblings — see [[ADR-0010]] and [[help-macro-checklist]]).

## Creating a schedule

Use the `api_v1_AgentMetadataService_UpsertSchedule` MCP tool:

```json
{
  "page": "my_project",
  "schedule": {
    "id": "friday_draft",
    "cron": "0 0 18 * * 5",
    "prompt": "Draft the weekend status update based on this week's notes.",
    "max_turns": 30,
    "enabled": true,
    "timezone": "America/New_York"
  }
}
```

The wiki validates both the cron expression and (when set) the IANA timezone at write time and rejects bad input with `InvalidArgument`.

### Fields

| Field | Notes |
| ----- | ----- |
| `id` | Stable identifier, unique within the page. Required. |
| `cron` | 6-field expression `sec min hr dom mon dow` (5-field also accepted). |
| `prompt` | The user prompt the headless agent receives. |
| `max_turns` | Cancel the turn after this many agent message chunks. Default 20. |
| `enabled` | When `false`, the schedule is persisted but not registered with cron. |
| `timezone` | Optional IANA timezone name. Empty/unset means UTC. |

The wiki-managed status fields (`last_run`, `last_status`, `last_error_message`, `last_duration_seconds`) are silently stripped on write — only the wiki itself can mutate them.

### Cron examples

| Expression | Fires |
| ---------- | ----- |
| `*/10 * * * * *` | Every 10 seconds |
| `0 */15 * * * *` | Every 15 minutes |
| `0 0 9 * * 1` | 9:00 AM every Monday |
| `0 0 18 * * 5` | 6:00 PM every Friday |

### Timezones

Cron expressions are interpreted in the IANA timezone named by the schedule's `timezone` field. **If `timezone` is empty or unset, the cron expression is interpreted in `UTC`** — the default was chosen to remove timezone ambiguity from the contract; users who want local time set `timezone` explicitly.

Invalid IANA names are rejected at write time with `InvalidArgument`. The wiki prepends `CRON_TZ=<timezone>` to the cron expression when registering with `robfig/cron`, so a `timezone` change takes effect on the next page save (or pool restart).

## Status lifecycle

Each fire walks the schedule through these states:

```
UNSPECIFIED ─┐
             ├─→ RUNNING ─┬─→ OK ──────┐
OK ──────────┤            ├─→ ERROR ───┼─→ RUNNING (next fire)
ERROR ───────┤            └─→ TIMEOUT ─┘
TIMEOUT ─────┘
```

`RUNNING → RUNNING` is illegal — if a previous fire is still in flight, the next cron fire is **skipped** rather than dispatched. A stuck `RUNNING` (e.g. the pool died mid-turn) clears via the per-turn hard timeout (default 10 minutes), after which the next fire proceeds normally.

## Background activity log

Every terminal transition (`OK`, `ERROR`, `TIMEOUT`) appends an entry to `agent.chat_context.background_activity` — a 50-entry rolling log on the page. Interactive chat preambles include this log so users see what background agents have been doing.

Scheduled agents are encouraged to call `api_v1_AgentMetadataService_AppendBackgroundActivitySummary` before they finish:

```json
{
  "page": "my_project",
  "schedule_id": "friday_draft",
  "summary": "Drafted weekend update covering 3 milestones and 2 blockers."
}
```

The summary attaches to the most recent matching entry in the log. Without it, interactive chat users will know a turn ran but not what it did.

## Listing & deleting schedules

```
api_v1_AgentMetadataService_ListSchedules     {"page":"my_project"}
api_v1_AgentMetadataService_DeleteSchedule    {"page":"my_project","schedule_id":"friday_draft"}
```

Delete is idempotent — removing an unknown id returns success. Deleting the entire page also unregisters all of its scheduled agents from cron.

## Operational notes

- Concurrency is shared across all schedules: by default 2 turns can run simultaneously across the whole wiki (`--agent-schedule-concurrency`). Backlog capacity is 256 (`--agent-schedule-queue-capacity`).
- The pool spawns a new short-lived ACP agent per fire, in a unique systemd unit. Per-turn journal logs are available there.
- If the pool is not running when a cron fires, the schedule transitions straight to `ERROR` with a "dispatch failed" message. Restart the pool and the next fire will proceed.
