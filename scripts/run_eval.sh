#!/bin/sh
# Build the eval binary once, then run it with the remaining args.
# Usage: devbox run eval -- --compare=pre,post --model=mimo-v2.5
# The leading "--" from devbox is stripped by this script.
set -e
cd "$(dirname "$0")/.."
go build -o /tmp/wiki-eval ./eval/cmd

# Strip a leading "--" if present (devbox run eval -- passes it through)
ARGS=""
for arg in "$@"; do
  if [ "$arg" = "--" ] && [ -z "$ARGS" ]; then
    continue
  fi
  ARGS="$ARGS $arg"
done

exec /tmp/wiki-eval $ARGS
