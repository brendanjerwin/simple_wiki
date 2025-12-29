#!/usr/bin/env bash

# Build staticcheck from source to match the project's Go version.
# This avoids version mismatch errors when staticcheck was pre-built with an older Go.
# See: https://github.com/dominikh/go-tools/issues/1674

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
elif [ "$(cat "$STATICCHECK_VERSION_FILE")" != "$GO_VERSION" ]; then
    echo "staticcheck was built with $(cat "$STATICCHECK_VERSION_FILE") but current Go is $GO_VERSION, rebuilding..."
    NEED_REBUILD=true
fi

if [ "$NEED_REBUILD" = true ]; then
    mkdir -p "$GOBIN_DIR"
    echo "Building staticcheck with $GO_VERSION..."
    GOBIN="$GOBIN_DIR" go install honnef.co/go/tools/cmd/staticcheck@latest
    echo "$GO_VERSION" > "$STATICCHECK_VERSION_FILE"
    echo "staticcheck built and cached in $GOBIN_DIR"
fi

# Ensure GOBIN_DIR is in PATH (for use in current script context)
if [[ ":$PATH:" != *":$GOBIN_DIR:"* ]]; then
    export PATH="$GOBIN_DIR:$PATH"
fi
