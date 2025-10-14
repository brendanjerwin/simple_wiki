#!/usr/bin/env bash
# Setup Android environment variables for devbox shell
# This script is sourced by the devbox init_hook

# Set JAVA_HOME for Gradle to find the Java compiler
# Devbox provides jdk21, so we need to point to it
if command -v java >/dev/null 2>&1; then
  JAVA_BIN=$(which java)
  export JAVA_HOME=$(dirname $(dirname $(readlink -f "$JAVA_BIN")))
fi

# Set Android SDK location (local to project for devbox reproducibility)
# Find project root by looking for devbox.json
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export ANDROID_HOME="${PROJECT_ROOT}/.android-sdk"
export ANDROID_SDK_ROOT="${ANDROID_HOME}"

# Add Android SDK tools to PATH if they exist
if [ -d "${ANDROID_HOME}/cmdline-tools/latest/bin" ]; then
  export PATH="${ANDROID_HOME}/cmdline-tools/latest/bin:${PATH}"
fi

if [ -d "${ANDROID_HOME}/platform-tools" ]; then
  export PATH="${ANDROID_HOME}/platform-tools:${PATH}"
fi

if [ -d "${ANDROID_HOME}/build-tools" ]; then
  # Add the latest build-tools version to PATH
  # Use find with -printf for robustness (handles spaces/special chars correctly)
  LATEST_BUILD_TOOLS=$(find "${ANDROID_HOME}/build-tools" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' 2>/dev/null | sort -V | tail -1)
  if [ -n "${LATEST_BUILD_TOOLS}" ]; then
    export PATH="${ANDROID_HOME}/build-tools/${LATEST_BUILD_TOOLS}:${PATH}"
  fi
fi

# Gradle optimization for Android builds
# In CI, disable daemon to avoid resource leaks and ensure clean builds
# In local dev, enable daemon for faster builds
# Parallel execution is enabled for better performance, max heap is 4GB
if [ "${CI:-false}" = "true" ]; then
  export GRADLE_OPTS="-Dorg.gradle.daemon=false -Dorg.gradle.parallel=true -Dorg.gradle.jvmargs='-Xmx4g'"
else
  export GRADLE_OPTS="-Dorg.gradle.daemon=true -Dorg.gradle.parallel=true -Dorg.gradle.jvmargs='-Xmx4g'"
fi

# Check if Android SDK is installed, show helpful message if not
if [ ! -d "${ANDROID_HOME}/cmdline-tools" ]; then
  echo ""
  echo "⚠️  Android SDK not found at ${ANDROID_HOME}"
  echo "   Run 'devbox run android:setup' to install it"
  echo ""
fi
