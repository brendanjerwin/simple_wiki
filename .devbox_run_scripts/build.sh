#!/bin/bash

# Cross-compilation build script for simple_wiki
# This script should be run through devbox to ensure proper environment setup.
# Usage: devbox run build [GOOS] [GOARCH] [SKIP_GENERATE]
# 
# Arguments:
#   GOOS: Target operating system (default: current OS)
#   GOARCH: Target architecture (default: current arch)
#   SKIP_GENERATE: Set to "true" to skip frontend generation (default: false)
#
# Examples:
#   devbox run build                          # Build for current platform
#   devbox run build linux amd64             # Build for linux/amd64
#   devbox run build windows amd64 true      # Build for windows/amd64, skip generation

set -e

# Parse arguments
TARGET_OS=${1:-$(go env GOOS)}
TARGET_ARCH=${2:-$(go env GOARCH)}
SKIP_GENERATE=${3:-false}

# Build variables
COMMIT_HASH=$(git rev-parse HEAD)
SHORT_COMMIT=$(git rev-parse --short HEAD)
BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# Check if current commit is tagged and format commit accordingly
TAG=$(git describe --tags --exact-match HEAD 2>/dev/null || echo "")
if [ -n "$TAG" ]; then
    COMMIT="$TAG ($SHORT_COMMIT)"
else
    COMMIT="$COMMIT_HASH"
fi

# Binary name with platform suffix
BINARY_NAME="simple_wiki-${TARGET_OS}-${TARGET_ARCH}"
if [ "$TARGET_OS" = "windows" ]; then
    BINARY_NAME="${BINARY_NAME}.exe"
fi

echo "Building simple_wiki for ${TARGET_OS}/${TARGET_ARCH}"
echo "Output: ${BINARY_NAME}"

# Generate frontend and protos (unless explicitly skipped)
if [ "$SKIP_GENERATE" != "true" ]; then
    echo "Generating protos and frontend"
    go generate ./...
fi

# Build the binary
echo "Building binary"
GOOS=$TARGET_OS GOARCH=$TARGET_ARCH CGO_ENABLED=0 go build \
    -ldflags "-X 'main.commit=$COMMIT' -X main.buildTime=$BUILD_TIME" \
    -o "$BINARY_NAME" .

echo "Build complete: $BINARY_NAME"