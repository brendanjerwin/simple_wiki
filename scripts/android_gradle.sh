#!/usr/bin/env bash
# Wrapper script to run Gradle with proper JAVA_HOME set
set -e

# Set JAVA_HOME to devbox JDK
# When running via devbox run, JAVA_HOME should already be set by devbox
if [ -z "$JAVA_HOME" ] && command -v java >/dev/null 2>&1; then
  JAVA_BIN=$(which java)
  # For devbox, java is typically a symlink in /nix/store, resolve it properly
  if [[ "$JAVA_BIN" == *"/nix/store/"* ]]; then
    # Extract the OpenJDK path from the nix store path
    JAVA_HOME=$(dirname $(dirname "$JAVA_BIN"))
    if [ -d "$JAVA_HOME/lib/openjdk" ]; then
      export JAVA_HOME="$JAVA_HOME/lib/openjdk"
    fi
  else
    export JAVA_HOME=$(dirname $(dirname $(readlink -f "$JAVA_BIN")))
  fi
fi

echo "Using JAVA_HOME: $JAVA_HOME"

# Source the Android environment setup
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/setup_android_env.sh"

# Run gradle with passed arguments
cd android && ./gradlew "$@"
