#!/usr/bin/env bash

# Signs the Firefox extension with AMO (addons.mozilla.org).
# This script should be run through devbox to ensure proper environment setup.
# Usage: devbox run -- bash scripts/sign-extension.sh
#
# Required environment variables:
#   AMO_API_KEY: AMO API key for signing
#   AMO_API_SECRET: AMO API secret for signing
#
# Optional environment variables:
#   GITHUB_REPOSITORY, GH_TOKEN: enable reuse of a previously signed XPI release
#     asset when AMO answers Conflict (re-run of a partially successful release)
#   SIGNED_XPI_CACHE_DIR: Directory for caching signed XPIs (avoids re-signing)
#
# Output:
#   signed-xpi-artifact/simple-wiki-companion.xpi (if signing succeeds)
#   Exit code 0 on success, 1 on failure

set -euo pipefail

if [[ -z "${AMO_API_KEY:-}" || -z "${AMO_API_SECRET:-}" ]]; then
    echo "ERROR: AMO_API_KEY and AMO_API_SECRET must be set" >&2
    exit 1
fi

# Derive extension version from git tag if available
# Firefox requires 1-4 dot-separated integers, so strip 'v' prefix and pre-release suffix
TAG=$(git tag --points-at HEAD 2>/dev/null | head -1)
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

# Build the extension XPI
go generate ./extensions/...

EXTENSION_DIR="extensions/online-order-recorder"
EXT_HASH=$(find "$EXTENSION_DIR/dist" -type f | sort | xargs sha256sum | sha256sum | cut -d' ' -f1)
CACHED_XPI="${SIGNED_XPI_CACHE_DIR:-}/simple-wiki-companion.xpi"
CACHED_HASH="${SIGNED_XPI_CACHE_DIR:-}/simple-wiki-companion.sha256"

SIGNED_XPI_ASSET_NAME="simple-wiki-companion.xpi"

mkdir -p signed-xpi-artifact

# Check cache first
if [[ -n "${SIGNED_XPI_CACHE_DIR:-}" && -f "$CACHED_XPI" && -f "$CACHED_HASH" ]] && \
   [[ "$(cat "$CACHED_HASH")" == "$EXT_HASH" ]]; then
    echo "Extension unchanged (hash $EXT_HASH), using cached signed XPI"
    cp "$CACHED_XPI" signed-xpi-artifact/simple-wiki-companion.xpi
    exit 0
fi

echo "Signing extension with AMO..."

# Strip update_url (AMO rejects http://)
SIGN_DIR="$EXTENSION_DIR/sign-staging"
cp -r "$EXTENSION_DIR/dist" "$SIGN_DIR"
node -e "
    const fs = require('fs');
    const p = '$SIGN_DIR/manifest.json';
    const m = JSON.parse(fs.readFileSync(p, 'utf8'));
    delete m.browser_specific_settings.gecko.update_url;
    fs.writeFileSync(p, JSON.stringify(m, null, 2) + '\n');
"

# Clean signed output dir to avoid ambiguity with multiple XPI files
rm -rf "$EXTENSION_DIR/signed"

SIGN_LOG=$(mktemp)
SIGN_OK=true
web-ext sign \
    --source-dir "$SIGN_DIR" \
    --artifacts-dir "$EXTENSION_DIR/signed" \
    --api-key "$AMO_API_KEY" \
    --api-secret "$AMO_API_SECRET" \
    --channel unlisted 2>&1 | tee "$SIGN_LOG" || SIGN_OK=false

rm -rf "$SIGN_DIR"

if [[ "$SIGN_OK" != "true" ]]; then
    # AMO rejects a resubmission of an already-submitted version with
    # "Conflict". That means a previous attempt of this release got as far as
    # AMO; reuse the signed XPI that attempt persisted as a release asset.
    # web-ext only exposes the failure through its message, so this is the one
    # place we must inspect it.
    if grep -q "Conflict" "$SIGN_LOG" && [[ -n "${TAG:-}" && -n "${GITHUB_REPOSITORY:-}" ]]; then
        echo "AMO reports this version was already submitted; looking for a signed XPI release asset on $TAG" >&2
        rm -rf "$EXTENSION_DIR/signed"
        mkdir -p "$EXTENSION_DIR/signed"
        if gh release download "$TAG" --repo "$GITHUB_REPOSITORY" \
            --pattern "$SIGNED_XPI_ASSET_NAME" --dir "$EXTENSION_DIR/signed" --clobber \
           && python3 - "$EXTENSION_DIR/signed/$SIGNED_XPI_ASSET_NAME" <<'PY'
import sys, zipfile
with zipfile.ZipFile(sys.argv[1]) as z:
    ok = any(n.startswith("META-INF/") and n.endswith(".rsa") for n in z.namelist())
sys.exit(0 if ok else 1)
PY
        then
            mv "$EXTENSION_DIR/signed/$SIGNED_XPI_ASSET_NAME" "$EXTENSION_DIR/signed/reused.xpi"
            echo "Reusing signed XPI from release asset $SIGNED_XPI_ASSET_NAME"
            SIGN_OK=true
        else
            echo "ERROR: AMO returned Conflict for $TAG and no valid signed XPI ($SIGNED_XPI_ASSET_NAME with META-INF/*.rsa) exists as a release asset." >&2
            echo "A pending AMO submission for this version probably exists. Approve or cancel it in the AMO console, then download the signed XPI and attach it to the release as $SIGNED_XPI_ASSET_NAME, and re-run." >&2
        fi
    fi
    if [[ "$SIGN_OK" != "true" ]]; then
        echo "AMO signing failed" >&2
        exit 1
    fi
fi

# Copy signed XPI to output
SIGNED_FILES=("$EXTENSION_DIR"/signed/*.xpi)
if [[ ! -s "${SIGNED_FILES[0]}" ]]; then
    echo "ERROR: No signed XPI file produced" >&2
    exit 1
fi
cp "${SIGNED_FILES[0]}" signed-xpi-artifact/simple-wiki-companion.xpi
echo "Extension signed successfully"

# Update cache for next build
if [[ -n "${SIGNED_XPI_CACHE_DIR:-}" ]]; then
    mkdir -p "$SIGNED_XPI_CACHE_DIR"
    cp signed-xpi-artifact/simple-wiki-companion.xpi "$CACHED_XPI"
    echo "$EXT_HASH" > "$CACHED_HASH"
    echo "Cached signed XPI (hash $EXT_HASH)"
fi
