#!/usr/bin/env bash
# Lint-suppression detector. Runs as a PostToolUse hook after Edit/Write/
# NotebookEdit. If the just-touched files contain a NEW lint suppression
# marker (revive:disable, nolint, staticcheck:ignore, etc.), surface it
# loudly and require justification.
#
# The user's directive (2026-05-03):
#   "whenever you write one of those, task a sub agent to ensure its
#    legit and necesary and not just lazy."
#
# This hook is the forcing function: when an agent adds a suppression,
# the non-zero exit makes it visible in the conversation, so the agent
# either justifies the suppression (with the cost the rule is hiding
# and why it's worth it) OR refactors to remove the need.
#
# CLAUDE_FILE_PATHS is provided by the harness (newline-separated paths
# of files just written). We scan only those files; the hook is fast.

set -u

# Patterns that disable a code-quality check. Keep this list focused —
# we only flag deliberate suppressions, not the words "disable" or
# "ignore" appearing in regular code.
patterns=(
  '//\s*revive:disable'              # Go: revive (project default)
  '//\s*nolint'                      # Go: golangci-lint
  '//\s*lint:ignore'                 # Go: legacy
  '//\s*staticcheck:'                # Go: staticcheck specific code
  '//\s*gocyclo:'                    # Go: cyclomatic complexity
  '//\s*nosem(grep|gp)?'             # opengrep / semgrep inline ignore
  '#\s*nosem(grep|gp)?'              # same, # comment style
  '//\s*eslint-disable'              # JS/TS
  '/\*\s*eslint-disable'             # JS/TS block style
  '#\s*pylint:\s*disable'            # Python
  '#\s*noqa'                         # Python (flake8 etc.)
  '#\s*type:\s*ignore'               # Python (mypy)
)

if [ -z "${CLAUDE_FILE_PATHS:-}" ]; then
  exit 0
fi

found_any=0
findings=""
while IFS= read -r path; do
  [ -z "$path" ] && continue
  [ -f "$path" ] || continue
  for pat in "${patterns[@]}"; do
    matches=$(grep -nE "$pat" "$path" 2>/dev/null || true)
    [ -z "$matches" ] && continue
    while IFS= read -r line; do
      [ -z "$line" ] && continue
      findings+="  $path:$line"$'\n'
      found_any=1
    done <<< "$matches"
  done
done <<< "$CLAUDE_FILE_PATHS"

if [ "$found_any" -eq 0 ]; then
  exit 0
fi

cat >&2 <<EOF
LINT-SUPPRESSION CHECK: at least one suppression marker is present in the file(s)
just written. Per user directive (2026-05-03), suppressions must NOT be lazy.

Findings:
$findings
For EACH suppression added in this edit, you must:

  1. Confirm the rule it disables and what cost the rule is hiding.
  2. Explain WHY the suppression is the right call (vs refactoring to
     comply). Reference the specific line of code or design constraint
     the rule is fighting.
  3. If unable to justify, REMOVE the suppression and refactor instead.

Default to refactoring. Suppressions are an escape hatch, not a default.

If a suppression already existed before this edit and you only moved/
extended it, that's fine — note that briefly. New suppressions need a
real justification.

Spawn a fresh sub-agent ('Explore' is enough) with the file + the
specific suppression line and ask: "Is this suppression legitimate or
lazy? Should the code be refactored instead?" Use the agent's verdict
to decide whether to keep, justify, or remove.
EOF

# Non-zero exit so the hook output surfaces in the conversation. The
# agent can still proceed; this isn't a hard block, just a forcing
# function for justification.
exit 1
