#!/bin/sh

# Install protoc-gen-go-mcp from source if not already present.
# This tool is not available in nixpkgs, so we install it via go install
# (same pattern as ensure_staticcheck.sh).
# Required by buf.gen.yaml for MCP tool generation from proto services.

set -e

GOBIN_DIR="${DEVBOX_PROJECT_ROOT:-.}/.devbox/gobin"
BINARY="$GOBIN_DIR/protoc-gen-go-mcp"

if [ ! -x "$BINARY" ]; then
    mkdir -p "$GOBIN_DIR"
    echo "Installing protoc-gen-go-mcp..."
    GOBIN="$GOBIN_DIR" go install github.com/redpanda-data/protoc-gen-go-mcp/cmd/protoc-gen-go-mcp@latest
    echo "protoc-gen-go-mcp installed in $GOBIN_DIR"
fi
