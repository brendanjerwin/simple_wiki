#!/usr/bin/env bash

# This script runs markdown linting with logging.

set -e

LOG_DIR="/tmp/simple_wiki_logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/lint_md_$(date +%Y%m%d_%H%M%S).log"
ln -sf "$LOG_FILE" "$LOG_DIR/current_task.log"

echo "Logging to: $LOG_FILE"

{
  markdownlint --fix '**/*.md' --ignore vendor
} 2>&1 | tee "$LOG_FILE"

exit_code=${PIPESTATUS[0]}
echo ""
echo "Log saved to: $LOG_FILE"
exit $exit_code
