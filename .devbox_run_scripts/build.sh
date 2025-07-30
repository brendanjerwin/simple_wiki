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

# Function to compare semver tags
compare_semver() {
    local tag1="$1"
    local tag2="$2"
    
    # Remove 'v' prefix if present
    local ver1="${tag1#v}"
    local ver2="${tag2#v}"
    
    # Split on dots and dashes to get major.minor.patch and prerelease
    local v1_base=$(echo "$ver1" | cut -d'-' -f1)
    local v1_pre=$(echo "$ver1" | cut -d'-' -f2- -s)
    local v2_base=$(echo "$ver2" | cut -d'-' -f1)
    local v2_pre=$(echo "$ver2" | cut -d'-' -f2- -s)
    
    # Compare base versions (major.minor.patch) using version sort
    local base_cmp=$(printf '%s\n%s\n' "$v1_base" "$v2_base" | sort -V | head -1)
    
    if [ "$v1_base" != "$v2_base" ]; then
        # Different base versions, use version sort result
        if [ "$base_cmp" = "$v1_base" ]; then
            echo "$tag2"  # v2 is higher
        else
            echo "$tag1"  # v1 is higher
        fi
    else
        # Same base version, check prerelease
        if [ -z "$v1_pre" ] && [ -z "$v2_pre" ]; then
            echo "$tag1"  # Both are release versions, same
        elif [ -z "$v1_pre" ]; then
            echo "$tag1"  # v1 is release, v2 is prerelease - v1 is higher
        elif [ -z "$v2_pre" ]; then
            echo "$tag2"  # v2 is release, v1 is prerelease - v2 is higher
        else
            # Both are prereleases, compare prerelease identifiers
            local pre_cmp=$(printf '%s\n%s\n' "$v1_pre" "$v2_pre" | sort -V | tail -1)
            if [ "$pre_cmp" = "$v1_pre" ]; then
                echo "$tag1"
            else
                echo "$tag2"
            fi
        fi
    fi
}

# Get the highest semver tag pointing to current commit
get_highest_tag() {
    local tags=$(git tag --points-at HEAD 2>/dev/null)
    
    if [ -z "$tags" ]; then
        echo ""
        return
    fi
    
    local highest=""
    while IFS= read -r tag; do
        if [ -z "$highest" ]; then
            highest="$tag"
        else
            highest=$(compare_semver "$highest" "$tag")
        fi
    done <<< "$tags"
    
    echo "$highest"
}

# Check if current commit is tagged and format commit accordingly
TAG=$(get_highest_tag)
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