#!/usr/bin/env bash
# Lint-suppression detector. Runs as a PostToolUse hook after Edit/Write/
# NotebookEdit. If the just-touched files contain a NEW lint suppression
# marker (revive:disable, nolint, staticcheck:ignore, etc.), surface it
# loudly and require justification.
#
# Per `feedback_no_lazy_lint_suppressions.md` — default to refactoring
# when a rule trips; suppression is the last resort with documented
# justification.

set -u

patterns=(
  '//\s*revive:disable'
  '//\s*nolint'
  '//\s*lint:ignore'
  '//\s*staticcheck:'
  '//\s*gocyclo:'
  '//\s*nosem(grep|gp)?'
  '#\s*nosem(grep|gp)?'
  '//\s*eslint-disable'
  '/\*\s*eslint-disable'
  '#\s*pylint:\s*disable'
  '#\s*noqa'
  '#\s*type:\s*ignore'
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

exit 1
