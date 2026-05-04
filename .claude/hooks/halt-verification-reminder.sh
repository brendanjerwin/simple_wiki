#!/usr/bin/env bash
#
# Stop hook: injects a halt-verification reminder into the assistant's
# context whenever it tries to end a turn.
#
# Why this exists: the assistant has demonstrated the failure pattern
# of halting at "natural checkpoints" while a TaskList still has
# pending items, the plan is approved, and auto mode is active.
# See memory entries:
#   - feedback_dont_pause_for_work_scope
#   - feedback_keep_going_and_delegate
#   - feedback_just_go_do_the_plan
#
# Implementation: emits a JSON Stop-hook output with
# `hookSpecificOutput.additionalContext` set to the reminder text.
# The reminder is injected back into the assistant's context, prompting
# it to verify the halt is justified before actually stopping. If the
# halt is legitimate, the assistant acknowledges and stops; if not, it
# resumes work on the next pending todo.
#
# Cf. ADR-0014 / CLAUDE.md "Triage Discipline" anti-patterns. This is
# the same shape of guardrail at the agent-runtime layer.

set -euo pipefail

jq -Rs '{
  hookSpecificOutput: {
    hookEventName: "Stop",
    additionalContext: .
  }
}' <<'EOF'
HALT VERIFICATION REQUIRED before ending this turn.

Review your reason for stopping. The following framings are FORBIDDEN as halt reasons (they are the failure pattern this hook exists to catch):

- "natural checkpoint" / "good place to pause" / "clean stopping point"
- "user might want to course-correct" / "let me check in"
- "context budget concerns" / "preserving context for later"
- "pause for review before continuing"
- "different kind of work coming next"
- offering the user N options where one of them is stop/pause

VALID halt reasons:

- User explicitly told you to stop, end the turn, or asked a question requiring an answer
- Genuine blocker requiring user input (destructive-action confirmation; missing credentials; ambiguity that needs disambiguation)
- Plan is FULLY complete; the TaskList has no pending items
- Dependency unavailable AND you cannot proceed past it
- A subagent or long-running tool is actually running and you must wait for its result

If your reason is NOT on the VALID list above, do NOT stop. Resume the next pending TaskList item per `feedback_keep_going_and_delegate`: when a TaskList exists, work it without asking permission; delegate to subagents to preserve context budget.

If your reason IS on the VALID list, briefly state which one applies, then stop.

This reminder fires on every Stop event because the failure pattern has been demonstrated in this repo's session history. The cost of the reminder (one sentence to acknowledge a legitimate stop) is far lower than the cost of an unjustified halt.
EOF
