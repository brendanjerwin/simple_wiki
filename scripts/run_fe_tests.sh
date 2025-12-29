#!/usr/bin/env bash

# This script runs frontend tests, handling path trimming for web-test-runner.

LOG_DIR="/tmp/simple_wiki_logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/fe_test_$(date +%Y%m%d_%H%M%S).log"
ln -sf "$LOG_FILE" "$LOG_DIR/current_task.log"

echo "Logging to: $LOG_FILE"
echo "Starting frontend tests at $(date)" | tee "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Collect and process paths, removing 'static/js/' prefix if present.
processed_paths=()
for p in "$@"; do
  processed_paths+=("${p#static/js/}")
done

# Navigate to the frontend directory
export CHROMIUM_BIN=$(which chromium)
cd static/js || exit 1

echo "Installing dependencies..." | tee -a "$LOG_FILE"
bun install 2>&1 | tee -a "$LOG_FILE"
install_exit=$?
if [ $install_exit -ne 0 ]; then
  echo "bun install failed with exit code $install_exit" | tee -a "$LOG_FILE"
  echo "Log saved to: $LOG_FILE"
  exit $install_exit
fi

echo "" | tee -a "$LOG_FILE"
echo "Running tests with 5 minute timeout..." | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"

# Run tests with timeout, capturing both stdout and stderr
timeout 300 bun run test:wtr ${CI_COVERAGE:+--coverage} "${processed_paths[@]}" 2>&1 | tee -a "$LOG_FILE"
test_exit=${PIPESTATUS[0]}

echo "" | tee -a "$LOG_FILE"
if [ $test_exit -eq 124 ]; then
  echo "TIMEOUT: Tests exceeded 5 minute limit" | tee -a "$LOG_FILE"
elif [ $test_exit -ne 0 ]; then
  echo "Tests failed with exit code: $test_exit" | tee -a "$LOG_FILE"
else
  echo "Tests completed successfully" | tee -a "$LOG_FILE"
fi

echo "Finished at $(date)" | tee -a "$LOG_FILE"
echo "" | tee -a "$LOG_FILE"
echo "Log saved to: $LOG_FILE"
exit $test_exit
