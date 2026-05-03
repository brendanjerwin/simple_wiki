#!/usr/bin/env bash
#
# Claude Code PostToolUse hook that runs opengrep against files an agent
# just edited via Edit/Write/NotebookEdit. Surfaces convention violations
# at edit time so the agent gets immediate feedback instead of waiting for
# CI to fail.
#
# Contract:
#   - Claude Code passes the changed file paths via the CLAUDE_FILE_PATHS
#     environment variable (space-separated). When unset (e.g. running
#     ad-hoc from the shell) we scan nothing and exit 0; we never fall back
#     to scanning the entire repo because that would be too slow for an
#     interactive hook.
#   - Exits 0 when clean, non-zero when opengrep reports findings. A
#     non-zero exit is what surfaces violations back to the agent.
#
# Bypass:
#   - For a one-off justified exception, add an inline `// nosemgrep:<rule-id>`
#     comment with a justification in the source. Do NOT disable the hook
#     globally.
#
set -e

# No file paths passed in -> nothing to do. We deliberately do NOT scan the
# whole repo (would be slow and noisy on every keystroke).
if [ -z "${CLAUDE_FILE_PATHS:-}" ]; then
    exit 0
fi

# Resolve repo root from this script's location so the hook works
# regardless of the agent's cwd at the time of the tool call.
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$REPO_ROOT"

# shellcheck disable=SC1091
. ./scripts/ensure_opengrep.sh

# Filter to files that actually exist on disk (the tool may have deleted
# some) and that opengrep knows how to scan. We let opengrep's own path
# filtering (rule `paths:` blocks) decide which rules apply per file.
FILES_TO_SCAN=()
for f in $CLAUDE_FILE_PATHS; do
    if [ -f "$f" ]; then
        FILES_TO_SCAN+=("$f")
    fi
done

if [ ${#FILES_TO_SCAN[@]} -eq 0 ]; then
    exit 0
fi

exec opengrep scan \
    --config .semgrep/rules.yml \
    --error \
    --quiet \
    --exclude='gen' \
    --exclude='vendor' \
    --exclude='node_modules' \
    --exclude='.devbox' \
    "${FILES_TO_SCAN[@]}"
