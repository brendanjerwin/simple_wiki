#!/usr/bin/env bash

# This script runs all linters, tests, and builds with logging.

set -e

LOG_DIR="/tmp/simple_wiki_logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/lint_everything_$(date +%Y%m%d_%H%M%S).log"
ln -sf "$LOG_FILE" "$LOG_DIR/current_task.log"

echo "Logging to: $LOG_FILE"

{
  # Only run buf generate if there are changes in api/proto
  if git status --porcelain api/proto | grep -q '.'; then
    buf generate
  else
    echo 'No changes in api/proto, skipping buf generate.'
  fi

  go mod tidy
  go vet $(go list ./... | grep -v /gen/)
  staticcheck $(go list ./... | grep -v /gen/)
  revive -config revive.toml ./...
  go test ./...
  go run golang.org/x/tools/cmd/deadcode@latest . || true
  markdownlint --fix '**/*.md' --ignore vendor

  export CHROMIUM_BIN=$(which chromium)
  cd static/js
  bun install
  bun run build
  bun run typecheck
  timeout 300 bun run test
  bun run lint
} 2>&1 | tee "$LOG_FILE"

exit_code=${PIPESTATUS[0]}
echo ""
echo "Log saved to: $LOG_FILE"
exit $exit_code
