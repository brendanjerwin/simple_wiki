# Voice Assistant Quick Test Commands

Quick reference for manual functional testing.

## Install & Launch

```bash
# Install APK
devbox run android:install

# Test basic search (replace "batteries" with your query)
adb shell am start -a android.intent.action.VIEW \
  -d "wiki://search?query=batteries" \
  com.github.brendanjerwin.simple_wiki/.voice.VoiceActionHandler
```

## Monitor Logs

```bash
# Clear and watch voice action logs
adb logcat -c && adb logcat | grep -E "(VoiceAction|SearchOrchestrator)"
```

## Test Scenarios

### 1. Successful Search

```bash
adb shell am start -a android.intent.action.VIEW \
  -d "wiki://search?query=<your-known-page>" \
  com.github.brendanjerwin.simple_wiki/.voice.VoiceActionHandler
```

Expected: Results returned, no errors

### 2. Empty Results

```bash
adb shell am start -a android.intent.action.VIEW \
  -d "wiki://search?query=xyz_nonexistent" \
  com.github.brendanjerwin.simple_wiki/.voice.VoiceActionHandler
```

Expected: Empty results, success=true, no error

### 3. Network Error (disconnect Tailscale first)

```bash
adb shell am start -a android.intent.action.VIEW \
  -d "wiki://search?query=test" \
  com.github.brendanjerwin.simple_wiki/.voice.VoiceActionHandler
```

Expected: error="Could not reach wiki. Check Tailscale connection."

## Verify Installation

```bash
# Check if installed
adb shell pm list packages | grep simple_wiki

# Check activity registration
adb shell dumpsys package com.github.brendanjerwin.simple_wiki | grep -A 5 "VoiceActionHandler"
```

## Troubleshooting

```bash
# View all errors
adb logcat *:E

# Increase log buffer
adb logcat -G 16M

# Check Tailscale connectivity
adb shell ping <wiki-backend-hostname>
```

See [voice-assistant-manual-testing.md](./voice-assistant-manual-testing.md) for complete testing guide.
