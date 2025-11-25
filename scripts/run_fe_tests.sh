#!/bin/bash

# This script runs frontend tests, handling path trimming for web-test-runner.

# Collect and process paths, removing 'static/js/' prefix if present.
processed_paths=()
for p in "$@"; do
  processed_paths+=("${p#static/js/}")
done

# Navigate to the frontend directory and run tests.
export CHROMIUM_BIN=$(which chromium)
cd static/js || exit 1
bun install || exit 1
bun run test:wtr ${CI_COVERAGE:+--coverage} "${processed_paths[@]}"
