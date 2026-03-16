#!/usr/bin/env bash

# This script runs Go code generation with logging.
# Ensure .devbox/gobin is on PATH so buf can find protoc-gen-go-mcp.

set -e

GOBIN_DIR="${DEVBOX_PROJECT_ROOT:-.}/.devbox/gobin"
export PATH="$GOBIN_DIR:$PATH"

LOG_DIR="/tmp/simple_wiki_logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/go_generate_$(date +%Y%m%d_%H%M%S).log"
ln -sf "$LOG_FILE" "$LOG_DIR/current_task.log"

echo "Logging to: $LOG_FILE"

{
  go generate ./...
} 2>&1 | tee "$LOG_FILE"

exit_code=${PIPESTATUS[0]}
echo ""
echo "Log saved to: $LOG_FILE"
exit $exit_code
