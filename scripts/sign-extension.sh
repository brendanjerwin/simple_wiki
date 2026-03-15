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
#   SIGNED_XPI_CACHE_DIR: Directory for caching signed XPIs (avoids re-signing)
#
# Output:
#   signed-xpi-artifact/simple-wiki-companion.xpi (if signing succeeds)
#   Exit code 0 on success, 1 on failure

set -euo pipefail

if [[ -z "${AMO_API_KEY:-}" || -z "${AMO_API_SECRET:-}" ]]; then
    echo "ERROR: AMO_API_KEY and AMO_API_SECRET must be set"
    exit 1
fi

# Build the extension XPI
go generate ./extensions/...

EXTENSION_DIR="extensions/online-order-recorder"
EXT_HASH=$(find "$EXTENSION_DIR/dist" -type f | sort | xargs sha256sum | sha256sum | cut -d' ' -f1)
CACHED_XPI="${SIGNED_XPI_CACHE_DIR:-}/simple-wiki-companion.xpi"
CACHED_HASH="${SIGNED_XPI_CACHE_DIR:-}/simple-wiki-companion.sha256"

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

if ! web-ext sign \
    --source-dir "$SIGN_DIR" \
    --artifacts-dir "$EXTENSION_DIR/signed" \
    --api-key "$AMO_API_KEY" \
    --api-secret "$AMO_API_SECRET" \
    --channel unlisted; then
    echo "AMO signing failed"
    rm -rf "$SIGN_DIR"
    exit 1
fi

rm -rf "$SIGN_DIR"

# Copy signed XPI to output
SIGNED_FILES=("$EXTENSION_DIR"/signed/*.xpi)
if [[ ! -s "${SIGNED_FILES[0]}" ]]; then
    echo "ERROR: No signed XPI file produced"
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
