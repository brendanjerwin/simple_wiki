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

# Package as .xpi (just a zip with .xpi extension)
cd dist
zip -r -FS "$SCRIPT_DIR/$OUT/online-order-recorder.xpi" . -x "*.map"
cd "$SCRIPT_DIR"

# Write version file for dynamic updates.json generation
VERSION=$(node -p "require('./manifest.json').version")
echo "$VERSION" > "$OUT/version.txt"

echo "Done. Built online-order-recorder.xpi (version ${VERSION})."
