#!/bin/sh

# Build golangci-lint from source to match the project's Go version.
# This avoids version mismatch errors when golangci-lint was pre-built with an older Go.
# Same pattern as ensure_staticcheck.sh.

set -e

# Use .devbox/gobin as the cache directory for Go binaries built from source
GOBIN_DIR="${DEVBOX_PROJECT_ROOT:-.}/.devbox/gobin"
GOLANGCI_LINT_BIN="$GOBIN_DIR/golangci-lint"
GOLANGCI_LINT_VERSION_FILE="$GOBIN_DIR/.golangci_lint_version"

# Get current Go version
GO_VERSION=$(go version | awk '{print $3}')

# Check if we need to rebuild
NEED_REBUILD=false

if [ ! -x "$GOLANGCI_LINT_BIN" ]; then
    echo "golangci-lint binary not found, building from source..."
    NEED_REBUILD=true
elif [ ! -f "$GOLANGCI_LINT_VERSION_FILE" ]; then
    echo "golangci-lint version file not found, rebuilding..."
    NEED_REBUILD=true
elif [ "$(cat "$GOLANGCI_LINT_VERSION_FILE")" != "$GO_VERSION" ]; then
    echo "golangci-lint was built with $(cat "$GOLANGCI_LINT_VERSION_FILE") but current Go is $GO_VERSION, rebuilding..."
    NEED_REBUILD=true
fi

if [ "$NEED_REBUILD" = true ]; then
    mkdir -p "$GOBIN_DIR"
    echo "Building golangci-lint with $GO_VERSION..."
    GOBIN="$GOBIN_DIR" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
    echo "$GO_VERSION" > "$GOLANGCI_LINT_VERSION_FILE"
    echo "golangci-lint built and cached in $GOBIN_DIR"
fi

# Ensure GOBIN_DIR is in PATH (for use in current script context)
# Use POSIX-compatible case statement instead of bash [[ ]] pattern matching
case ":$PATH:" in
    *":$GOBIN_DIR:"*) ;;
    *) export PATH="$GOBIN_DIR:$PATH" ;;
esac
