#!/bin/sh

# Build staticcheck from source to match the project's Go version.
# This avoids version mismatch errors when staticcheck was pre-built with an older Go.
# See: https://github.com/dominikh/go-tools/issues/1674

# Pinned toolchain version. Do NOT use @latest here: every CI run rebuilds
# this binary from source (no cross-run gobin cache), so @latest means the
# linter version silently changes whenever a new staticcheck is released,
# flipping lint verdicts between runs on byte-identical code (the same
# failure class as simple_wiki #1243 for golangci-lint/errcheck). Bump this
# pin deliberately and re-run lint before merging.
STATICCHECK_PIN="v0.8.2"

set -e

# Use .devbox/gobin as the cache directory for Go binaries built from source
GOBIN_DIR="${DEVBOX_PROJECT_ROOT:-.}/.devbox/gobin"
STATICCHECK_BIN="$GOBIN_DIR/staticcheck"
STATICCHECK_VERSION_FILE="$GOBIN_DIR/.staticcheck_version"

# Get current Go version
GO_VERSION=$(go version | awk '{print $3}')

# Check if we need to rebuild
NEED_REBUILD=false

if [ ! -x "$STATICCHECK_BIN" ]; then
    echo "staticcheck binary not found, building from source..."
    NEED_REBUILD=true
elif [ ! -f "$STATICCHECK_VERSION_FILE" ]; then
    echo "staticcheck version file not found, rebuilding..."
    NEED_REBUILD=true
elif [ "$(cat "$STATICCHECK_VERSION_FILE")" != "$GO_VERSION $STATICCHECK_PIN" ]; then
    echo "staticcheck was built with $(cat "$STATICCHECK_VERSION_FILE") but current Go/pin is $GO_VERSION $STATICCHECK_PIN, rebuilding..."
    NEED_REBUILD=true
fi

if [ "$NEED_REBUILD" = true ]; then
    mkdir -p "$GOBIN_DIR"
    echo "Building staticcheck $STATICCHECK_PIN with $GO_VERSION..."
    GOBIN="$GOBIN_DIR" go install honnef.co/go/tools/cmd/staticcheck@"$STATICCHECK_PIN"
    echo "$GO_VERSION $STATICCHECK_PIN" > "$STATICCHECK_VERSION_FILE"
    echo "staticcheck $STATICCHECK_PIN built and cached in $GOBIN_DIR"
fi

# Ensure GOBIN_DIR is in PATH (for use in current script context)
# Use POSIX-compatible case statement instead of bash [[ ]] pattern matching
case ":$PATH:" in
    *":$GOBIN_DIR:"*) ;;
    *) export PATH="$GOBIN_DIR:$PATH" ;;
esac
