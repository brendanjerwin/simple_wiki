#!/usr/bin/env bash
# Fix Android USB permissions for ADB

echo "Setting up Android USB permissions..."

# Create udev rules file
cat > /etc/udev/rules.d/51-android.rules << 'EOF'
# Android USB Rules - Allow all USB devices for ADB
SUBSYSTEM=="usb", ATTR{idVendor}=="*", MODE="0666", GROUP="plugdev"
EOF

# Set permissions and reload
chmod a+r /etc/udev/rules.d/51-android.rules
udevadm control --reload-rules
udevadm trigger

echo "Done! Unplug and replug your Android device, then run: adb devices"
