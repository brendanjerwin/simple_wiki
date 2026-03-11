#!/usr/bin/env bash
# Build the online-order-recorder Firefox extension.
# Called by go:generate from generate.go.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

OUT="../../static/extensions"
mkdir -p "$OUT"

# Install dependencies
bun install

# Typecheck
bun run typecheck

# Build the three entry points
bun run build

# Copy static assets into dist/
cp manifest.json dist/
cp popup.html dist/
cp popup.css dist/
cp -r icons dist/

# Override version from environment if set (derived from git tag by build.sh)
if [[ -n "${EXTENSION_VERSION:-}" ]]; then
    echo "Setting extension version to $EXTENSION_VERSION (from git tag)"
    node -e "
        const fs = require('fs');
        const p = 'dist/manifest.json';
        const m = JSON.parse(fs.readFileSync(p, 'utf8'));
        m.version = process.argv[1];
        fs.writeFileSync(p, JSON.stringify(m, null, 2) + '\n');
    " "$EXTENSION_VERSION"
fi

# Package as .xpi (just a zip with .xpi extension)
cd dist
zip -r -FS "$SCRIPT_DIR/$OUT/simple-wiki-companion.xpi" . -x "*.map"
cd "$SCRIPT_DIR"

# Write version file for dynamic updates.json generation
VERSION=$(node -p "require('./dist/manifest.json').version")
echo "$VERSION" > "$OUT/version.txt"

echo "Done. Built simple-wiki-companion.xpi (version ${VERSION})."
