#!/usr/bin/env bash
# Cross-compile wiki-cli for all common platforms.
# Called by go:generate from generate.go.
set -euo pipefail

OUT="../../static/cli"
mkdir -p "$OUT"

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
  if [ "$goos" = "windows" ]; then
    binary="${binary}.exe"
  fi
  echo "Building ${binary}..."
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -o "${OUT}/${binary}" .
done

echo "Done. Built ${#platforms[@]} binaries."
