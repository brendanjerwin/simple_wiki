#!/usr/bin/env bash
# Cross-compile wiki-cli for all common platforms.
# Called by go:generate from generate.go.
set -euo pipefail

OUT="../../static/cli"
mkdir -p "$OUT"

# Embed the current git commit so the CLI can check version compatibility
# with the running wiki server at startup.
COMMIT=$(git rev-parse HEAD 2>/dev/null || echo "dev")
LDFLAGS="-X main.commit=${COMMIT}"

platforms=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
)

for platform in "${platforms[@]}"; do
  IFS='/' read -r goos goarch <<< "$platform"
  binary="wiki-cli-${goos}-${goarch}"
  if [[ "$goos" = "windows" ]]; then
    binary="${binary}.exe"
  fi
  echo "Building ${binary} (commit: ${COMMIT})..."
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -ldflags "${LDFLAGS}" -o "${OUT}/${binary}" .
done

echo "Done. Built ${#platforms[@]} binaries."
