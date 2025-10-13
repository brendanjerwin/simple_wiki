#!/usr/bin/env bash
# Download and install Android SDK command-line tools
# Run this script once to set up the Android development environment
#
# Set ANDROID_SETUP_NON_INTERACTIVE=1 for automated/CI environments

set -e  # Exit on error

ANDROID_HOME="${HOME}/.android-sdk"
CMDLINE_TOOLS_VERSION="11076708"  # Latest as of 2025
CMDLINE_TOOLS_URL="https://dl.google.com/android/repository/commandlinetools-linux-${CMDLINE_TOOLS_VERSION}_latest.zip"

echo "🔧 Setting up Android SDK..."
echo ""

# Check if Android SDK already exists
if [ -d "${ANDROID_HOME}/cmdline-tools/latest" ]; then
  echo "✅ Android SDK already installed at ${ANDROID_HOME}"

  # In non-interactive mode (CI), skip reinstall
  if [ "${ANDROID_SETUP_NON_INTERACTIVE}" = "1" ]; then
    echo "Running in non-interactive mode, skipping reinstall."
    exit 0
  fi

  echo ""
  read -p "Do you want to reinstall? (y/N) " -n 1 -r
  echo ""
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Skipping installation."
    exit 0
  fi
  echo "Removing existing SDK..."
  rm -rf "${ANDROID_HOME}"
fi

# Create directory structure
echo "📁 Creating directory structure..."
mkdir -p "${ANDROID_HOME}/cmdline-tools"

# Download command-line tools
echo "📥 Downloading Android command-line tools..."
TEMP_ZIP="/tmp/android-cmdline-tools.zip"
curl -L "${CMDLINE_TOOLS_URL}" -o "${TEMP_ZIP}"

# Extract to temporary location
echo "📦 Extracting command-line tools..."
unzip -q "${TEMP_ZIP}" -d "${ANDROID_HOME}/cmdline-tools"

# Move to 'latest' directory (required structure)
mv "${ANDROID_HOME}/cmdline-tools/cmdline-tools" "${ANDROID_HOME}/cmdline-tools/latest"

# Clean up
rm "${TEMP_ZIP}"

# Set up environment for sdkmanager
export ANDROID_SDK_ROOT="${ANDROID_HOME}"
export PATH="${ANDROID_HOME}/cmdline-tools/latest/bin:${PATH}"

# Accept licenses automatically
# Note: sdkmanager --licenses returns 0 if licenses are already accepted,
# or if they are successfully accepted. Only fails on actual errors.
echo "📝 Accepting Android SDK licenses..."
if ! yes | sdkmanager --licenses >/dev/null 2>&1; then
  echo "⚠️  Warning: Failed to accept Android SDK licenses"
  echo "   This may cause build failures. Try running manually:"
  echo "   sdkmanager --licenses"
  # Don't exit - let the user see what packages get installed
fi

# Install required SDK components
echo "📦 Installing Android SDK packages..."
echo "   This may take a few minutes..."

# Install platform-tools (adb, fastboot)
sdkmanager "platform-tools"

# Install Android SDK Platform 34 (required by Capacitor 7)
sdkmanager "platforms;android-34"

# Install build-tools 34.0.0
sdkmanager "build-tools;34.0.0"

# Install emulator (optional, but useful)
sdkmanager "emulator"

# Install system images for emulator (optional)
# Uncomment if you want to use emulator
# sdkmanager "system-images;android-34;google_apis;x86_64"

echo ""
echo "✅ Android SDK installation complete!"
echo ""
echo "📍 Installed at: ${ANDROID_HOME}"
echo ""
echo "Installed packages:"
sdkmanager --list_installed | grep -E "platform|build-tools|platform-tools" | head -10
echo ""
echo "🚀 You can now use Android tools:"
echo "   - adb (Android Debug Bridge)"
echo "   - devbox run android:sync (sync web assets to Android)"
echo "   - devbox run android:build:debug (build APK)"
echo ""
echo "💡 Restart your devbox shell to use the new tools:"
echo "   exit"
echo "   devbox shell"
echo ""
