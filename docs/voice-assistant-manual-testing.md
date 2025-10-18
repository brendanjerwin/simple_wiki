# Voice Assistant Manual Testing Guide

## Overview

This guide provides step-by-step instructions for manually testing the voice assistant integration on a physical Android device.

## Prerequisites

### Device Requirements

- Physical Android device (not emulator)
- Android OS with Google Assistant support
- Tailscale app installed and configured
- USB debugging enabled

### Development Machine Setup

- ADB installed and configured
- Device connected via USB
- Wiki backend running and accessible via Tailscale

## Setup Instructions

### 1. Install the APK

```bash
# From project root
devbox run android:install

# Or manually:
adb install -r android/app/build/outputs/apk/debug/app-debug.apk
```

Verify installation:

```bash
adb shell pm list packages | grep simple_wiki
```

Expected output: `package:com.github.brendanjerwin.simple_wiki`

### 2. Verify Tailscale Connection

On your Android device:

1. Open Tailscale app
2. Ensure it's connected (green checkmark)
3. Verify the wiki backend hostname is accessible

### 3. Configure Google Assistant (Optional)

For testing with voice commands:

1. Open Settings → Apps → Default apps → Digital assistant app
2. Ensure Google Assistant is set as default
3. Grant necessary permissions

## Manual Test Cases

### Test 1: Direct Intent Testing (No Assistant Required)

Test the deep link functionality without Google Assistant.

**Test Command:**

```bash
adb shell am start -a android.intent.action.VIEW \
  -d "wiki://search?query=batteries" \
  com.github.brendanjerwin.simple_wiki/.voice.VoiceActionHandler
```

**Expected Behavior:**

- App launches
- VoiceActionHandler receives the query
- Search is performed for "batteries"
- Check logcat for results

**Verification:**

```bash
adb logcat | grep -i "VoiceAction"
```

Look for:

- Query parameter extracted: "batteries"
- SearchOrchestrator called
- Results returned or error message

### Test 2: App Actions Registration

Verify that the app's actions are registered with the system.

**Test Command:**

```bash
# Check app's declared capabilities
adb shell dumpsys package com.github.brendanjerwin.simple_wiki | grep -A 20 "Activity:"
```

**Expected Output:**

- VoiceActionHandler activity listed
- Intent filter for `wiki` scheme visible
- `android:exported="true"` present

### Test 3: Successful Search

**Setup:**
Ensure you have wiki pages that will match the query (e.g., a page about batteries).

**Test Steps:**

1. Use Test 1 command with a known query
2. Monitor logcat output
3. Verify search results

**Expected Results:**

- VoiceSearchResult with `success=true`
- Pages list populated
- frontmatterJson contains valid JSON
- renderedMarkdown contains template-expanded content

**Verification Commands:**

```bash
# Watch logs during test
adb logcat -c  # Clear logs first
adb shell am start -a android.intent.action.VIEW -d "wiki://search?query=<your-query>" com.github.brendanjerwin.simple_wiki/.voice.VoiceActionHandler
adb logcat | grep -E "(VoiceAction|SearchOrchestrator|WikiApiClient)"
```

### Test 4: Empty Search Results

Test handling of queries that return no results.

**Test Command:**

```bash
adb shell am start -a android.intent.action.VIEW \
  -d "wiki://search?query=xyz_nonexistent_page" \
  com.github.brendanjerwin.simple_wiki/.voice.VoiceActionHandler
```

**Expected Results:**

- VoiceSearchResult with `success=true` (empty results are valid)
- Empty pages list
- totalPages=0
- No error message

### Test 5: Network Error Handling

Test error handling when Tailscale is disconnected.

**Test Steps:**

1. Disconnect Tailscale on the device
2. Run the search intent
3. Monitor logcat for error handling

**Expected Results:**

- VoiceSearchResult with `success=false`
- error="Could not reach wiki. Check Tailscale connection."
- Empty pages list
- User-friendly error message (no technical details)

**Verification:**

```bash
adb logcat | grep -i "error"
```

### Test 6: Timeout Handling

Test behavior when API requests time out.

**Test Steps:**

1. This requires temporarily blocking network or slowing it down
2. Alternative: Check logs for timeout exception handling code paths

**Expected Results:**

- VoiceSearchResult with `success=false`
- error="Wiki search timed out. Try again."
- Includes retry suggestion

### Test 7: Multiple Pages Response

Test formatting of multiple search results.

**Test Command:**

```bash
# Use a query that returns multiple pages
adb shell am start -a android.intent.action.VIEW \
  -d "wiki://search?query=<query-with-multiple-results>" \
  com.github.brendanjerwin.simple_wiki/.voice.VoiceActionHandler
```

**Expected Results:**

- Multiple pages in pages list
- Each page has: title, identifier, frontmatterJson, renderedMarkdown
- Pages maintain search result order
- frontmatterJson is valid JSON for each page

### Test 8: Complex Frontmatter Parsing

Test that complex TOML frontmatter is correctly converted to JSON.

**Setup:**
Create a test page with complex frontmatter:

```toml
title = "Test Page"
tags = ["tag1", "tag2", "tag3"]
created = "2025-01-10T14:30:00Z"
location = "Lab"
```

**Test Steps:**

1. Search for the test page
2. Verify frontmatterJson in logs

**Expected Results:**

- JSON contains all fields
- tags array is properly formatted: `["tag1","tag2","tag3"]`
- Timestamps preserved as strings
- Valid JSON structure

## Logcat Filtering

Useful logcat filters for debugging:

```bash
# Voice Action logs only
adb logcat VoiceActionHandler:* *:S

# Full voice integration stack
adb logcat | grep -E "(VoiceAction|SearchOrchestrator|WikiApiClient|TokenBudget)"

# Errors only
adb logcat *:E

# Continuous monitoring with timestamp
adb logcat -v time | grep -i wiki
```

## Common Issues and Solutions

### Issue: App not launching

**Solution:** Check if package is installed:

```bash
adb shell pm list packages | grep simple_wiki
```

If not present, reinstall APK.

### Issue: Intent not triggering

**Solution:** Verify intent filter registration:

```bash
adb shell dumpsys package com.github.brendanjerwin.simple_wiki | grep -A 10 "android.intent.action.VIEW"
```

### Issue: Network errors

**Solution:** Verify Tailscale connection:

```bash
adb shell ping <wiki-backend-hostname>
```

### Issue: No logs visible

**Solution:** Increase log buffer size:

```bash
adb logcat -G 16M
```

## Test Results Template

Use this template to document test results:

```markdown
## Test Results - [Date]

### Environment
- Device: [Model]
- Android Version: [Version]
- Tailscale Status: Connected/Disconnected
- Wiki Backend: [Hostname/IP]

### Test Case 1: Direct Intent Testing
- Status: ✅ Pass / ❌ Fail
- Notes: [Any observations]

### Test Case 2: App Actions Registration
- Status: ✅ Pass / ❌ Fail
- Notes: [Any observations]

[Continue for all test cases...]

### Issues Found
1. [Issue description]
   - Severity: Critical/High/Medium/Low
   - Steps to reproduce: [Steps]

### Overall Assessment
- [ ] All critical tests pass
- [ ] Ready for Google Assistant integration testing
- [ ] Ready for demo video recording
```

## Next Steps

After completing manual testing:

1. **Document Results:** Fill out the test results template
2. **Fix Issues:** Address any critical or high-severity issues found
3. **Google Assistant Testing:** If manual tests pass, proceed to voice testing with Google Assistant
4. **Performance Testing:** Measure latency (voice → response)
5. **Demo Video:** Record demonstration of working features

## Performance Benchmarks

When testing, measure these metrics:

- **Intent to Search Start:** Time from intent received to SearchOrchestrator.search() called
- **Search API Call:** Time for searchContent() to return
- **Page Fetch Time:** Time for parallel readPage() calls
- **Total Response Time:** End-to-end time from intent to result

Target: p90 < 2 seconds for full flow

## Notes

- Manual testing requires a physical device; emulators do not support Google Assistant integration
- Some tests can be automated in the future using UI testing frameworks
- Keep detailed logs for any issues encountered
- Performance may vary based on network conditions and device capabilities
