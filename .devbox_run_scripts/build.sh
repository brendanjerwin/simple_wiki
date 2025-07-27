#!/bin/bash

# Cross-compilation build script for simple_wiki
# This script should be run through devbox to ensure proper environment setup.
# Usage: devbox run build [GOOS] [GOARCH] [SKIP_FRONTEND]
# 
# Arguments:
#   GOOS: Target operating system (default: current OS)
#   GOARCH: Target architecture (default: current arch)
#   SKIP_FRONTEND: Set to "true" to skip frontend generation (default: false)
#
# Examples:
#   devbox run build                          # Build for current platform
#   devbox run build linux amd64             # Build for linux/amd64
#   devbox run build windows amd64 true      # Build for windows/amd64, skip frontend

set -e

# Parse arguments
TARGET_OS=${1:-$(go env GOOS)}
TARGET_ARCH=${2:-$(go env GOARCH)}
SKIP_FRONTEND=${3:-false}

# Build variables
COMMIT=$(git rev-parse HEAD)
BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# Binary name with platform suffix
BINARY_NAME="simple_wiki-${TARGET_OS}-${TARGET_ARCH}"
if [ "$TARGET_OS" = "windows" ]; then
    BINARY_NAME="${BINARY_NAME}.exe"
fi

echo "Building simple_wiki for ${TARGET_OS}/${TARGET_ARCH}"
echo "Output: ${BINARY_NAME}"

# Generate frontend and protos (unless explicitly skipped)
if [ "$SKIP_FRONTEND" != "true" ]; then
    echo "Generating protos and frontend"
    go generate ./...
fi

# Build the binary
echo "Building binary"
GOOS=$TARGET_OS GOARCH=$TARGET_ARCH CGO_ENABLED=0 go build \
    -ldflags "-X main.commit=$COMMIT -X main.buildTime=$BUILD_TIME" \
    -o "$BINARY_NAME" .

echo "Build complete: $BINARY_NAME"