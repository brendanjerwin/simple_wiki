#!/usr/bin/env bash
#
# Stop hook: forces the assistant to verify the halt is justified
# before ending a turn.
#
# Why this exists: the assistant has demonstrated the failure pattern
# of halting at "natural checkpoints" while a TaskList still has
# pending items, the plan is approved, and auto mode is active.
# See memory entries:
#   - feedback_dont_pause_for_work_scope
#   - feedback_keep_going_and_delegate
#   - feedback_just_go_do_the_plan
#
# Implementation: emits `{"decision": "block", "reason": "..."}` on the
# first stop attempt of a turn so the model sees the reminder text as
# injected context and must briefly justify (or resume work). On the
# second stop attempt within a 120s window, allows the stop through
# (the verification has been done).
#
# Sentinel file: /tmp/claude-halt-verified-<session-id>. Touched on
# block; deleted on allow. Per-session keying lets parallel sessions
# coexist; staleness is bounded so a session that crashed mid-block
# unblocks the next one in 120s.
#
# Cf. ADR-0014 / CLAUDE.md "Triage Discipline" anti-patterns. This is
# the same shape of guardrail at the agent-runtime layer.

set -euo pipefail

# Read stdin to extract session_id (best-effort; falls back to "unknown"
# if jq can't parse). Each Stop event arrives as JSON on stdin per the
# Claude Code hook protocol.
INPUT=$(cat)
SESSION_ID=$(echo "$INPUT" | jq -r '.session_id // "unknown"' 2>/dev/null || echo "unknown")

SENTINEL="/tmp/claude-halt-verified-${SESSION_ID}"
WINDOW_SECONDS=120

# If a recent sentinel exists, the model has already been reminded
# in this turn-cycle. Allow the stop through and clear the sentinel.
if [[ -f "$SENTINEL" ]]; then
  AGE=$(( $(date +%s) - $(stat -c %Y "$SENTINEL" 2>/dev/null || echo 0) ))
  if (( AGE < WINDOW_SECONDS )); then
    rm -f "$SENTINEL"
    # Empty output means "allow the stop with no extra signal."
    exit 0
  fi
  # Stale sentinel; treat as fresh stop.
  rm -f "$SENTINEL"
fi

# First stop attempt in the verification window. Block and inject the
# reminder as the `reason` so the model must address it before its
# next stop attempt succeeds.
touch "$SENTINEL"

jq -Rs '{decision: "block", reason: .}' <<'EOF'
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

If your reason IS on the VALID list, briefly state which one applies in your next response. The hook will allow the second stop attempt through.

This reminder fires on the first Stop event of a turn-cycle because the failure pattern has been demonstrated in this repo's session history. Subsequent Stop events within 120s pass through unblocked.
EOF
