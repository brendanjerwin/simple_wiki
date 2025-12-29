#!/usr/bin/env bash

# This script runs Go linting with logging.

set -e

LOG_DIR="/tmp/simple_wiki_logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/go_lint_$(date +%Y%m%d_%H%M%S).log"
ln -sf "$LOG_FILE" "$LOG_DIR/current_task.log"

echo "Logging to: $LOG_FILE"

# Ensure staticcheck is built from source and available
source ./scripts/ensure_staticcheck.sh

{
  echo "Running staticcheck..."
  staticcheck $(go list ./... | grep -v /gen/)
  echo ""
  echo "Running revive..."
  revive -config revive.toml -set_exit_status ./...
  echo ""
  echo "Running golangci-lint (godox for TODO detection)..."
  golangci-lint run
} 2>&1 | tee "$LOG_FILE"

exit_code=${PIPESTATUS[0]}
echo ""
echo "Log saved to: $LOG_FILE"
exit $exit_code
