#!/usr/bin/env bash

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

LOG_DIR="/tmp/simple_wiki_logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/build_$(date +%Y%m%d_%H%M%S).log"
ln -sf "$LOG_FILE" "$LOG_DIR/current_task.log"

echo "Logging to: $LOG_FILE"

{
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

    if [[ "$v1_base" != "$v2_base" ]]; then
        # Different base versions, use version sort result
        if [[ "$base_cmp" = "$v1_base" ]]; then
            echo "$tag2"  # v2 is higher
        else
            echo "$tag1"  # v1 is higher
        fi
    else
        # Same base version, check prerelease
        if [[ -z "$v1_pre" && -z "$v2_pre" ]]; then
            echo "$tag1"  # Both are release versions, same
        elif [[ -z "$v1_pre" ]]; then
            echo "$tag1"  # v1 is release, v2 is prerelease - v1 is higher
        elif [[ -z "$v2_pre" ]]; then
            echo "$tag2"  # v2 is release, v1 is prerelease - v2 is higher
        else
            # Both are prereleases, compare prerelease identifiers using proper semver rules
            # For prereleases, we need to compare according to semver precedence rules
            local pre_cmp=$(printf '%s\n%s\n' "$v1_pre" "$v2_pre" | sort -V | head -1)
            if [[ "$pre_cmp" = "$v1_pre" ]]; then
                echo "$tag2"  # v1_pre comes first in sort, so v2_pre is higher
            else
                echo "$tag1"  # v2_pre comes first in sort, so v1_pre is higher
            fi
        fi
    fi
}

# Get the highest semver tag pointing to current commit
get_highest_tag() {
    local tags=$(git tag --points-at HEAD 2>/dev/null)

    if [[ -z "$tags" ]]; then
        echo ""
        return
    fi

    local highest=""
    while IFS= read -r tag; do
        if [[ -z "$highest" ]]; then
            highest="$tag"
        else
            highest=$(compare_semver "$highest" "$tag")
        fi
    done <<< "$tags"

    echo "$highest"
}

# Check if current commit is tagged and format commit accordingly
TAG=$(get_highest_tag)
if [[ -n "$TAG" ]]; then
    COMMIT="$TAG ($SHORT_COMMIT)"
else
    COMMIT="$COMMIT_HASH"
fi

# Binary name with platform suffix
BINARY_NAME="simple_wiki-${TARGET_OS}-${TARGET_ARCH}"
if [[ "$TARGET_OS" = "windows" ]]; then
    BINARY_NAME="${BINARY_NAME}.exe"
fi

echo "Building simple_wiki for ${TARGET_OS}/${TARGET_ARCH}"
echo "Output: ${BINARY_NAME}"

# Generate frontend and protos (unless explicitly skipped)
if [[ "$SKIP_GENERATE" != "true" ]]; then
    # Only run buf generate if proto files have changed (avoids needing protoc plugins in CI)
    # Use merge-base to detect changes across the entire branch, not just the last commit
    MERGE_BASE=$(git merge-base HEAD origin/main 2>/dev/null || echo "HEAD~1")
    if git diff --name-only "$MERGE_BASE" 2>/dev/null | grep -q '^api/proto' || \
       git status --porcelain api/proto 2>/dev/null | grep -q '.'; then
        echo "Proto changes detected, running buf generate"
        buf generate
    else
        echo "No proto changes, skipping buf generate"
    fi

    # Derive extension version from git tag if available
    # Firefox requires 1-4 dot-separated integers, so strip 'v' prefix and pre-release suffix
    if [[ -n "$TAG" ]]; then
        EXT_VER="${TAG#v}"
        EXT_VER="${EXT_VER%%-*}"
        if [[ "$EXT_VER" =~ ^[0-9]+(\.[0-9]+){0,3}$ ]]; then
            export EXTENSION_VERSION="$EXT_VER"
            echo "Extension version from git tag: $EXTENSION_VERSION"
        else
            echo "Git tag '$TAG' does not produce a valid extension version, using manifest default"
        fi
    fi

    echo "Generating extensions, frontend, and wiki-cli"
    # Extensions must be built before static/ so the XPI is present when go:embed runs
    go generate ./extensions/...

    # TODO: Move AMO signing to the running wiki server so it can inject the
    # correct update_url (based on its actual hostname) into the manifest before
    # signing. Currently signing happens at build time with no knowledge of the
    # deployment URL, so update_url is omitted from the signed XPI.

    # Use a pre-signed XPI if provided (e.g. from a prior CI job).
    # This avoids hitting AMO multiple times in a matrix build.
    if [[ -n "${SIGNED_XPI_PATH:-}" && -s "$SIGNED_XPI_PATH" ]]; then
        echo "Using pre-signed XPI from $SIGNED_XPI_PATH"
        cp "$SIGNED_XPI_PATH" static/extensions/simple-wiki-companion.xpi

    # Sign extension if AMO credentials are available (CI only).
    # To avoid AMO rate limits, skip signing when a cached signed XPI
    # matches the current extension source (set SIGNED_XPI_CACHE_DIR
    # from the workflow to enable caching).
    elif [[ -n "${AMO_API_KEY:-}" && -n "${AMO_API_SECRET:-}" ]]; then
        EXTENSION_DIR="extensions/online-order-recorder"
        EXT_HASH=$(find "$EXTENSION_DIR/dist" -type f | sort | xargs sha256sum | sha256sum | cut -d' ' -f1)
        CACHED_XPI="${SIGNED_XPI_CACHE_DIR:-}/simple-wiki-companion.xpi"
        CACHED_HASH="${SIGNED_XPI_CACHE_DIR:-}/simple-wiki-companion.sha256"

        if [[ -n "${SIGNED_XPI_CACHE_DIR:-}" && -f "$CACHED_XPI" && -f "$CACHED_HASH" ]] && \
           [[ "$(cat "$CACHED_HASH")" == "$EXT_HASH" ]]; then
            echo "Extension unchanged (hash $EXT_HASH), using cached signed XPI"
            cp "$CACHED_XPI" static/extensions/simple-wiki-companion.xpi
        else
            echo "Signing extension with AMO..."

            # AMO rejects http:// update_url, so strip it before signing.
            # The unsigned XPI (used locally) keeps it for auto-update checks.
            SIGN_DIR="$EXTENSION_DIR/sign-staging"
            cp -r "$EXTENSION_DIR/dist" "$SIGN_DIR"
            node -e "
                const fs = require('fs');
                const p = '$SIGN_DIR/manifest.json';
                const m = JSON.parse(fs.readFileSync(p, 'utf8'));
                delete m.browser_specific_settings.gecko.update_url;
                fs.writeFileSync(p, JSON.stringify(m, null, 2) + '\n');
            "

            if ! web-ext sign \
                --source-dir "$SIGN_DIR" \
                --artifacts-dir "$EXTENSION_DIR/signed" \
                --api-key "$AMO_API_KEY" \
                --api-secret "$AMO_API_SECRET" \
                --channel unlisted; then
                echo "⚠️  AMO signing failed, deploying unsigned"
            fi
            rm -rf "$SIGN_DIR"
            # Replace unsigned XPI with signed one if signing succeeded
            SIGNED_FILES=("$EXTENSION_DIR"/signed/*.xpi)
            if [[ -f "${SIGNED_FILES[0]}" ]]; then
                cp "$EXTENSION_DIR"/signed/*.xpi static/extensions/simple-wiki-companion.xpi
                echo "Extension signed successfully"

                # Update cache for next build
                if [[ -n "${SIGNED_XPI_CACHE_DIR:-}" ]]; then
                    mkdir -p "$SIGNED_XPI_CACHE_DIR"
                    cp static/extensions/simple-wiki-companion.xpi "$CACHED_XPI"
                    echo "$EXT_HASH" > "$CACHED_HASH"
                    echo "Cached signed XPI (hash $EXT_HASH)"
                fi
            else
                echo "Deploying with unsigned extension"
            fi
        fi
    else
        echo "AMO credentials not set, skipping extension signing"
    fi

    go generate ./static/...
    go generate ./cmd/wiki-cli/...
fi

# Build the binary
echo "Building binary"
GOOS=$TARGET_OS GOARCH=$TARGET_ARCH CGO_ENABLED=0 go build \
    -ldflags "-X 'main.commit=$COMMIT' -X 'main.buildTime=$BUILD_TIME'" \
    -o "$BINARY_NAME" .

echo "Build complete: $BINARY_NAME"
} 2>&1 | tee "$LOG_FILE"

exit_code=${PIPESTATUS[0]}
echo ""
echo "Log saved to: $LOG_FILE"
exit $exit_code
