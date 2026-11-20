#!/usr/bin/env bash

# This script runs Go linting with logging.

set -e

LOG_DIR="/tmp/simple_wiki_logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/go_lint_$(date +%Y%m%d_%H%M%S).log"
ln -sf "$LOG_FILE" "$LOG_DIR/current_task.log"

echo "Logging to: $LOG_FILE"

# Ensure staticcheck and golangci-lint are built from source and available
. ./scripts/ensure_staticcheck.sh
. ./scripts/ensure_golangci_lint.sh

{
  echo "Running gofmt on first-party Go files..."
  # vendor/ is third-party code and gen/ is generated output: neither is ours to
  # reformat, and regenerating gen/ would undo a local sweep.
  # Only files present on disk: a staged deletion would otherwise make gofmt
  # fail on a missing path instead of reporting formatting.
  first_party_go=$(git ls-files '*.go' | grep -vE '^(vendor|gen)/' | while read -r go_file; do
    if [ -f "$go_file" ]; then printf '%s\n' "$go_file"; fi
  done)
  if [ -n "$first_party_go" ]; then
    unformatted=$(printf '%s\n' "$first_party_go" | xargs gofmt -l)
    if [ -n "$unformatted" ]; then
      echo "The following first-party Go files are not gofmt-clean:"
      printf '%s\n' "$unformatted"
      echo "Fix with: printf '%s\\n' \"\$unformatted\" | xargs gofmt -w"
      exit 1
    fi
  fi
  echo ""
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
