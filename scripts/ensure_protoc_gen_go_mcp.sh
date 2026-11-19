#!/bin/sh

# Install protoc-gen-go-mcp from source, reinstalling when the go.mod pin changes.
# This tool is not available in nixpkgs, so we install it via go install
# (same pattern as ensure_staticcheck.sh).
# Required by buf.gen.yaml for MCP tool generation from proto services.
#
# Version-awareness: uses `go version -m` to read the module version embedded
# in the installed binary and compares it against the go.mod pin. A matching
# version is a no-op; any mismatch triggers a reinstall. This prevents a stale
# binary (built from an older pin) from silently producing output that disagrees
# with what CI or a fresh clone would produce.
#
# Never install this tool with `@latest`: an unpinned generator emits schemas
# typed as `jsontext.Value` (jsonv2), which does not compile against the
# `json.RawMessage` fields of the mcp-go version pinned in go.mod. Always let
# this script resolve the version from go.mod.

set -e

GOBIN_DIR="${DEVBOX_PROJECT_ROOT:-.}/.devbox/gobin"
BINARY="$GOBIN_DIR/protoc-gen-go-mcp"

DESIRED_VERSION="$(go list -m -f '{{.Version}}' github.com/redpanda-data/protoc-gen-go-mcp)"

NEED_INSTALL=false

if [ ! -x "$BINARY" ]; then
    echo "protoc-gen-go-mcp binary not found, installing..."
    NEED_INSTALL=true
else
    # `go version -m` prints tab-separated fields: "", "mod", module path, version.
    INSTALLED_VERSION="$(go version -m "$BINARY" 2>/dev/null \
        | awk -F'\t' '$2 == "mod" && $3 == "github.com/redpanda-data/protoc-gen-go-mcp" { print $4 }')"
    if [ "$INSTALLED_VERSION" != "$DESIRED_VERSION" ]; then
        echo "protoc-gen-go-mcp is at $INSTALLED_VERSION but go.mod pins $DESIRED_VERSION, reinstalling..."
        NEED_INSTALL=true
    fi
fi

if [ "$NEED_INSTALL" = true ]; then
    mkdir -p "$GOBIN_DIR"
    echo "Installing protoc-gen-go-mcp $DESIRED_VERSION..."
    GOBIN="$GOBIN_DIR" go install github.com/redpanda-data/protoc-gen-go-mcp/cmd/protoc-gen-go-mcp@"$DESIRED_VERSION"
    echo "protoc-gen-go-mcp $DESIRED_VERSION installed in $GOBIN_DIR"
fi
