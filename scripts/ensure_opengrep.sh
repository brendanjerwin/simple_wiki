#!/bin/sh

# Download the opengrep binary if not present or if the version has changed.
# Opengrep is the OSS fork of Semgrep (LGPL-2.1) used for enforcing
# project conventions defined in CLAUDE.md.

set -e

OPENGREP_VERSION="1.17.0"
OPENGREP_DIR="${DEVBOX_PROJECT_ROOT:-.}/.devbox/opengrep"
OPENGREP_BIN="$OPENGREP_DIR/opengrep"
OPENGREP_VERSION_FILE="$OPENGREP_DIR/.opengrep_version"

NEED_DOWNLOAD=false

if [ ! -x "$OPENGREP_BIN" ]; then
    NEED_DOWNLOAD=true
elif [ ! -f "$OPENGREP_VERSION_FILE" ]; then
    NEED_DOWNLOAD=true
elif [ "$(cat "$OPENGREP_VERSION_FILE")" != "$OPENGREP_VERSION" ]; then
    echo "opengrep version changed from $(cat "$OPENGREP_VERSION_FILE") to $OPENGREP_VERSION, updating..."
    NEED_DOWNLOAD=true
fi

if [ "$NEED_DOWNLOAD" = true ]; then
    mkdir -p "$OPENGREP_DIR"

    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        linux)
            case "$ARCH" in
                x86_64)  ASSET="opengrep_manylinux_x86" ;;
                aarch64) ASSET="opengrep_manylinux_aarch64" ;;
                *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
            esac
            ;;
        darwin)
            case "$ARCH" in
                x86_64)  ASSET="opengrep_osx_x86" ;;
                arm64)   ASSET="opengrep_osx_arm64" ;;
                *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
            esac
            ;;
        *)
            echo "Unsupported OS: $OS"; exit 1 ;;
    esac

    URL="https://github.com/opengrep/opengrep/releases/download/v${OPENGREP_VERSION}/${ASSET}"
    echo "Downloading opengrep v${OPENGREP_VERSION} (${ASSET})..."
    curl -fsSL -o "$OPENGREP_BIN" "$URL"
    chmod +x "$OPENGREP_BIN"
    echo "$OPENGREP_VERSION" > "$OPENGREP_VERSION_FILE"
    echo "opengrep v${OPENGREP_VERSION} installed to $OPENGREP_DIR"
fi

# Ensure opengrep is in PATH
case ":$PATH:" in
    *":$OPENGREP_DIR:"*) ;;
    *) export PATH="$OPENGREP_DIR:$PATH" ;;
esac
