#!/usr/bin/env bash

set -euo pipefail

BASELINE_FILE="${MUTATION_BASELINE_FILE:-.mutation-baseline.json}"
BASE_REF="${MUTATION_BASE_REF:-origin/main}"
WORKERS="${MUTATION_WORKERS:-2}"
LOG_DIR="/tmp/simple_wiki_logs"
mkdir -p "$LOG_DIR"
OUTPUT_FILE="${MUTATION_OUTPUT_FILE:-$LOG_DIR/gremlins_$(date +%Y%m%d_%H%M%S).json}"

if [ ! -f "$BASELINE_FILE" ]; then
  echo "Mutation baseline not found: $BASELINE_FILE" >&2
  exit 1
fi

THRESHOLD="${MUTATION_THRESHOLD_EFFICACY:-$(go run ./cmd/read-mutation-baseline "$BASELINE_FILE")}"

echo "Running gremlins mutation testing against $BASE_REF"
echo "Minimum patch efficacy: $THRESHOLD%"
echo "Gremlins output: $OUTPUT_FILE"

go run github.com/go-gremlins/gremlins/cmd/gremlins@latest --silent unleash . \
  --diff "$BASE_REF" \
  --threshold-efficacy "$THRESHOLD" \
  --workers "$WORKERS" \
  --exclude-files '^vendor/' \
  --exclude-files '^gen/' \
  --exclude-files '^\.semgrep/' \
  --output "$OUTPUT_FILE" \
  "$@"
