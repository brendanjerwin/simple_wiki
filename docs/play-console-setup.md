# Google Play Console Setup for Voice Assistant

This guide walks you through setting up Simple Wiki for internal testing on Google Play Console, enabling "Hey Google" voice commands on your personal devices.

## Overview

**Goal**: Enable voice commands like "Hey Google, search my wiki for batteries" on your Android device.

**Approach**: Internal Testing Track (Private, no public release)

**Cost**: $25 one-time Google Play Console developer fee

**Timeline**:
- Setup: 1-2 hours
- Google validation: 24-48 hours
- Total: 2-3 days until voice commands work

## Prerequisites

- [ ] Google Account
- [ ] $25 for Play Console developer account (one-time)
- [ ] Release APK built: `android/app/build/outputs/apk/release/app-release.apk`
- [ ] Android device with Google Assistant
- [ ] Tailscale installed on device

## Step 1: Create Play Console Account

1. Go to [Google Play Console](https://play.google.com/console)
2. Sign in with your Google Account
3. Accept the Developer Distribution Agreement
4. Pay the $25 one-time registration fee
5. Complete your account details:
   - Developer name: (Your name or "Simple Wiki")
   - Email address: (Your email)
   - Phone number: (Optional but recommended)

**Time**: 10-15 minutes

## Step 2: Create Your App

1. In Play Console, click **"Create app"**
2. Fill in app details:
   - **App name**: Simple Wiki
   - **Default language**: English (United States)
   - **App or game**: App
   - **Free or paid**: Free
3. Accept declarations:
   - [x] "This app complies with Google Play's policies"
   - [x] "This app is not primarily for children"
4. Click **"Create app"**

**Time**: 2-3 minutes

## Step 3: Complete App Content Declarations

Play Console requires several mandatory declarations before you can upload an APK.

### 3.1: Privacy Policy

Since the app accesses personal wiki data over Tailscale:

1. Go to **App content** → **Privacy policy**
2. Enter privacy policy URL or create a simple one:

**Option A: Simple Privacy Policy**
```markdown
# Simple Wiki Privacy Policy

Simple Wiki is a personal note-taking application.

**Data Collection**: None. All data is stored on your personal wiki server.

**Data Sharing**: None. The app only communicates with your self-hosted wiki via Tailscale.

**Third Parties**: None. No analytics, no tracking, no ads.

**Contact**: [Your email]

Last updated: [Today's date]
```

Host this on GitHub Pages or paste it into a Google Doc (set to public).

3. Enter the URL
4. Click **Save**

### 3.2: App Access

1. Go to **App content** → **App access**
2. Select: "All functionality is available without special access"
3. Click **Save**

### 3.3: Ads

1. Go to **App content** → **Ads**
2. Select: "No, my app does not contain ads"
3. Click **Save**

### 3.4: Content Rating

1. Go to **App content** → **Content rating**
2. Click **Start questionnaire**
3. Select category: **Utility, productivity, communication, or other**
4. Answer questions (all "No" for violence, etc.)
5. Submit for rating (instant)
6. Click **Apply rating**

### 3.5: Target Audience

1. Go to **App content** → **Target audience**
2. Select: "18 and over"
3. Click **Next** → **Save**

### 3.6: Data Safety

1. Go to **App content** → **Data safety**
2. Answer questions:
   - **Does your app collect or share user data?**: No
   - Explanation: "App only communicates with user's self-hosted wiki server over Tailscale. No data collected or shared with third parties."
3. Click **Next** → **Submit**

**Time**: 20-30 minutes for all declarations

## Step 4: Upload Release APK

1. Go to **Testing** → **Internal testing**
2. Click **Create new release**
3. Upload APK:
   - Click **Upload**
   - Select: `android/app/build/outputs/apk/release/app-release.apk`
   - Wait for upload (10MB, ~30 seconds)
4. Fill in release details:
   - **Release name**: "1.1.0-voice - Initial Voice Assistant Release"
   - **Release notes**:
   ```
   Initial release with Google Assistant voice integration.

   Features:
   - Voice search: "Hey Google, search my wiki for [query]"
   - Two-phase retrieval with token budgeting
   - Works over Tailscale private network
   - On-device and cloud Gemini support

   Requirements:
   - Tailscale connection required
   - Wiki backend must be accessible at https://wiki.monster-orfe.ts.net
   ```
5. Click **Save**
6. Click **Review release**
7. Click **Start rollout to Internal testing**

**Time**: 5-10 minutes

## Step 5: Create Internal Testing Track

1. Still in **Testing** → **Internal testing**
2. Click **Testers** tab
3. Create an email list:
   - Click **Create email list**
   - Name: "Personal Testing"
   - Add your email address
   - Click **Save changes**
4. Copy the **opt-in URL** (you'll need this)

**Time**: 2-3 minutes

## Step 6: Join as Tester

1. Open the opt-in URL from Step 5 in your browser
2. Click **"Become a tester"**
3. You'll see: "You're a tester for Simple Wiki"
4. Click **"Download it on Google Play"**
5. Install the app on your device

**Time**: 2-3 minutes

## Step 7: Configure App Actions (CRITICAL)

This step is REQUIRED for voice commands to work.

1. Go to **Release** → **Setup** → **App content**
2. Scroll to **Advanced settings**
3. Click **App Actions** (or **Voice and Assistant**)
4. Upload `shortcuts.xml`:
   - Click **Upload shortcuts.xml**
   - Select: `android/app/src/main/res/xml/shortcuts.xml`
5. Click **Submit for review**

**Google will review your App Actions within 24-48 hours.**

**Time**: 3-5 minutes

## Step 8: Wait for Google Validation

After uploading App Actions:

1. Google validates your `shortcuts.xml`
2. Google indexes your voice intents
3. **Timeline**: 24-48 hours typically

You'll receive an email when:
- ✅ App Actions approved
- ❌ App Actions rejected (with reasons)

**No action needed**: Just wait.

## Step 9: Test Voice Commands

Once Google approves your App Actions (24-48 hours later):

### Install the App

1. On your Android device, open the Play Store
2. Go to your account → **"Manage apps & device"** → **Internal testing**
3. Find "Simple Wiki" and install

### Connect Tailscale

1. Open Tailscale app
2. Ensure connected (green checkmark)
3. Verify wiki backend is accessible

### Test Voice Commands

Try these commands:

```
"Hey Google, search my wiki for batteries"
"Hey Google, search my wiki for arduino"
"Hey Google, where are my CR2032 batteries?"
```

**Expected behavior**:
1. Google Assistant activates
2. Your app receives the query
3. Wiki search executes over Tailscale
4. Results returned to Gemini
5. Gemini speaks natural language response

## Troubleshooting

### App Actions Not Working After 48 Hours

**Check status**:
1. Play Console → **App Actions** → Check status
2. Look for rejection reasons

**Common issues**:
- `shortcuts.xml` validation errors
- Missing required fields in manifest
- Package name mismatch

**Fix**: Update `shortcuts.xml`, upload new APK, resubmit

### Voice Commands Don't Trigger App

**Possible causes**:
1. **App Actions not approved yet**: Wait 24-48 hours
2. **App not installed from Play Store**: Must use Play Store version, not sideloaded
3. **Google Assistant not updated**: Update Google app
4. **Query doesn't match intent**: Try exact phrase "search my wiki for batteries"

**Debug**:
```bash
# Check if app is registered
adb shell dumpsys package com.github.brendanjerwin.simple_wiki | grep -A 5 "VoiceActionHandler"
```

### "I couldn't reach the wiki" Error

**Cause**: Tailscale not connected or backend not running

**Fix**:
1. Open Tailscale app → Ensure connected
2. Test backend: `curl https://wiki.monster-orfe.ts.net`
3. Check wiki server is running

### No Results Found

**Cause**: Query doesn't match any wiki pages

**Fix**:
1. Test with known query: "batteries"
2. Check wiki has content
3. Verify search is working: `grpcurl -d '{"query":"batteries"}' wiki.monster-orfe.ts.net:443 api.v1.SearchService/SearchContent`

## Updating the App

When you make changes:

1. **Increment version** in `android/app/build.gradle`:
   ```gradle
   versionCode 3  // Increment by 1
   versionName "1.2.0"
   ```

2. **Rebuild release APK**:
   ```bash
   devbox run android:build:release
   ```

3. **Upload new APK** to Play Console:
   - Testing → Internal testing → Create new release
   - Upload APK
   - Add release notes
   - Review and rollout

4. **Wait for testers to update**:
   - You'll get notification in Play Store
   - Update like any other app

## Removing Test Access

To stop testing:

1. Open opt-in URL again
2. Click **"Leave the program"**
3. Uninstall app
4. Wait 1-2 days for full removal

## FAQ

**Q: Can I use this without Play Console?**
A: No, for voice commands to work with Google Assistant, App Actions must be registered via Play Console.

**Q: Can others use my app?**
A: Only if you add them to your internal testing list. It's private by default.

**Q: Does this go public?**
A: No, internal testing is completely private.

**Q: What if I want to go public later?**
A: You can promote to Open/Closed testing or Production anytime.

**Q: Do I need to renew anything?**
A: No, the $25 fee is one-time for life.

**Q: Can I test without Google validation?**
A: Yes, use direct deep links: `adb shell am start -a android.intent.action.VIEW -d "wiki://search?query=test" com.github.brendanjerwin.simple_wiki/.voice.VoiceActionHandler`

## Summary Checklist

- [ ] Created Play Console account ($25)
- [ ] Created app in console
- [ ] Completed all content declarations
- [ ] Uploaded release APK
- [ ] Created internal testing track
- [ ] Joined as tester
- [ ] Uploaded App Actions (shortcuts.xml)
- [ ] Waited 24-48 hours for approval
- [ ] Installed app from Play Store
- [ ] Connected Tailscale
- [ ] Tested voice commands successfully

**Total time**: 1-2 hours setup + 24-48 hours Google validation

## Next Steps

After voice commands are working:

1. **Test thoroughly** with various queries
2. **Document any issues** you find
3. **Iterate** on the app based on usage
4. **Enjoy** your voice-controlled personal wiki!

## Support

If you run into issues:

1. Check [voice-assistant-manual-testing.md](./voice-assistant-manual-testing.md) for test procedures
2. Review [voice-assistant-quick-test.md](./voice-assistant-quick-test.md) for quick debugging
3. Check logcat: `adb logcat -s VoiceActionHandler:*`

---

**Last Updated**: 2025-10-13
**Version**: 1.0
**Status**: Ready for internal testing
