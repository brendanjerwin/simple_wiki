#!/usr/bin/env bash

# This script runs E2E tests in headed mode with logging.

set -e

LOG_DIR="/tmp/simple_wiki_logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/e2e_test_headed_$(date +%Y%m%d_%H%M%S).log"
ln -sf "$LOG_FILE" "$LOG_DIR/current_task.log"

echo "Logging to: $LOG_FILE"

{
  devbox run build
  # Prefer caller-provided binary; default to a working one (#1282).
  if [[ -z "${CHROMIUM_BIN:-}" ]]; then
    export CHROMIUM_BIN="$(./scripts/resolve_chromium.sh)"
  fi
  cd e2e || exit 1
  bun install || exit 1
  bun run test:headed
} 2>&1 | tee "$LOG_FILE"

exit_code=${PIPESTATUS[0]}
echo ""
echo "Log saved to: $LOG_FILE"
exit $exit_code
