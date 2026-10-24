#!/usr/bin/env bash
# Resolve a Chromium binary for local test runners, preferring one that
# actually launches. Order of preference:
#   1. CHROMIUM_BIN already set by the caller — never overridden.
#   2. Puppeteer's cached Chrome for Testing: on devbox hosts the Nix
#      chromium crashes with GLIBC_2.43/2.44 symbol errors (see #1282),
#      while Puppeteer's bundled build links against its own newer glibc.
#   3. `chromium` on PATH — what CI runners have; no puppeteer cache there.
#
# Prints the resolved path to stdout. With no set env var, callers use:
#   CHROMIUM_BIN="${CHROMIUM_BIN:-$(./scripts/resolve_chromium.sh)}"

if [ -n "${CHROMIUM_BIN:-}" ]; then
  echo "$CHROMIUM_BIN"
  exit 0
fi

# Newest Puppeteer-cached Chrome for Testing build that exists and is runnable.
chrome="$(find "$HOME/.cache/puppeteer/chrome" -maxdepth 3 \
  -path '*/chrome-linux64/chrome' -type f 2>/dev/null |
  sort -rV | head -1)"
if [ -n "$chrome" ] && [ -x "$chrome" ]; then
  echo "$chrome"
  exit 0
fi

# Last resort for CI runners (Ubuntu-provided chromium).
command -v chromium || command -v chromium-browser || command -v google-chrome
