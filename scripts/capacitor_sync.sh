#!/usr/bin/env bash
set -e

# Build frontend
cd static/js
bun install
bun run build
cd ../..

# Run Capacitor sync from project root with node_modules in PATH
export PATH="$PWD/static/js/node_modules/.bin:$PATH"
export NODE_PATH="$PWD/static/js/node_modules:$NODE_PATH"

# Pass CAPACITOR_DEV environment variable if set
if [ -n "$CAPACITOR_DEV" ]; then
    CAPACITOR_DEV=true npx @capacitor/cli sync android
else
    npx @capacitor/cli sync android
fi

# Clean up node_modules from assets
rm -rf android/app/src/main/assets/public/js/node_modules
