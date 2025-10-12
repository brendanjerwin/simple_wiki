# Progressive Web App (PWA) Implementation Plan

## Overview

Transform simple_wiki into a Progressive Web App to enable installability on mobile/desktop devices while maintaining the existing web-first architecture.

**Context**: This is a Tailnet-only deployment using Tailscale Serve for HTTPS (`https://wiki.monster-orfe.ts.net`). TLS is handled transparently by Tailscale with automatic cert management - no application code changes needed.

## Goals

1. **Installability**: Users can install simple_wiki on mobile/desktop home screens
2. **Zero Breaking Changes**: Existing web experience remains unchanged
3. **Zero Caching**: No service worker, no cache invalidation complexity

## Non-Goals (Deferred Until User Request)

**Offline Detection** - Existing error handling already communicates network failures. `navigator.onLine` is unreliable (doesn't detect VPN issues, captive portals, etc.). If users request better offline UX, implement then based on actual use cases.

**Auto-Update Detection** - Polling every 60 seconds wastes resources (CPU, network, battery) for a problem that occurs once per day at most. Version is already checked on page load. If users request update notifications, consider WebSocket push or show notification only after natural page navigation.

## Design Principles

**Simplicity First**: Modern browsers (Chrome 93+, Safari 16.4+) support PWA installation with just a manifest and HTTPS. No service worker required.

**Offline Position**: This PWA provides ZERO offline functionality. The app is network-dependent by design. When offline, users see a clear notification and are blocked from interaction.

**Rationale**: This is a wiki backed by a server. Offline editing creates complex sync conflicts and data loss scenarios. We embrace network dependency and communicate it clearly rather than building complex conflict resolution.

## Out of Scope

- Service workers and caching strategies
- Offline functionality
- Install prompt banners (deprecated API)
- Native app store distribution
- Native API integration (filesystem, camera, etc.)
- TLS/HTTPS configuration (already handled by Tailscale Serve)

## Build Integration

**No build changes required.** The PWA implementation adds:

- One static JSON file (`manifest.json`)
- Two icon images (192px and 512px)
- One `<link>` tag in HTML template

The existing build process (`go generate ./...`) remains unchanged.

## Technical Implementation

### 1. Web App Manifest (`/static/manifest.json`)

Defines how the app appears when installed. This is a **static file** served directly.

```json
{
  "name": "Simple Wiki",
  "short_name": "Wiki",
  "description": "A simple, fast wiki for your notes",
  "start_url": "/",
  "scope": "/",
  "display": "standalone",
  "background_color": "#ffffff",
  "theme_color": "#ffffff",
  "icons": [
    {
      "src": "/static/img/pwa/icon-192.png",
      "sizes": "192x192",
      "type": "image/png"
    },
    {
      "src": "/static/img/pwa/icon-512.png",
      "sizes": "512x512",
      "type": "image/png"
    }
  ]
}
```

**Notes:**

- Minimal icon set (just 192px and 512px)
- Can initially use upscaled favicon, design better icons later
- No shortcuts, no maskable variants - keep it simple

### 2. HTML Template Update

Add manifest link to base template (likely `/static/templates/index.tmpl` or similar):

```html
<link rel="manifest" href="/static/manifest.json">
```

### 3. PWA Icons

Create two icon files:

- `/static/img/pwa/icon-192.png` (192x192px)
- `/static/img/pwa/icon-512.png` (512x512px)

**Initial Implementation**: Can upscale existing favicon or logo. Design proper icons later as polish.

## No Service Worker Required

Modern browsers support PWA installation without a service worker when:

- Site is served over HTTPS ✓ (Tailscale Serve)
- Manifest is present ✓
- Manifest has valid name, icons, start_url ✓
- Browser is modern ✓ (Chrome 93+, Safari 16.4+)

**We deliberately avoid service workers** to eliminate:

- Cache invalidation complexity
- Service worker lifecycle bugs
- Update deployment delays
- Debugging headaches

Users install via browser menu (⋮ → "Install app") instead of a custom prompt banner.

## Implementation Plan

### Phase 1: Manifest and Icons (Ship This)

**Goal:** Enable basic PWA installability

**Tasks:**

1. Create `/static/manifest.json` with minimal config
2. Create placeholder icons:
   - `/static/img/pwa/icon-192.png`
   - `/static/img/pwa/icon-512.png`
   - Can upscale existing favicon initially
3. Add manifest link to HTML template
4. Test installation on:
   - Chrome desktop (⋮ → Install app)
   - Chrome Android (⋮ → Add to Home Screen)
   - Safari iOS (Share → Add to Home Screen)

**Acceptance Criteria:**

- App installs successfully on all platforms
- Installed app opens in standalone mode (no browser chrome)
- App icon and name display correctly
- No console errors

**Estimated Effort:** 30 minutes to 1 hour

### Phase 2: Measure Adoption (Then Decide)

**Goal:** Determine if PWA installation provides value to users

**Tasks:**

1. Ship Phase 1 to production
2. Wait 2-4 weeks for organic adoption
3. Ask users:
   - Did you install the wiki as a PWA?
   - If yes: what made you want to?
   - If no: what would make you want to?
   - Do you want notifications when updates are available?
   - Do you use the wiki when offline?

**Decision Point:**

- **If multiple users install**: Proceed with polish (better icons, README docs)
- **If users request update notifications**: Implement based on actual use case (WebSocket push vs page-load check)
- **If users request offline support**: Reconsider architecture (may need service worker + sync strategy)
- **If nobody installs or cares**: Success! You saved 7 hours of work on features nobody wanted.

**Estimated Effort:** 0 hours (observation only)

### Phase 3: Polish (Only If Users Install)

**Goal:** Production-ready PWA

**Tasks:**

1. Design proper PWA icons:
   - 192x192 icon with proper padding
   - 512x512 icon with proper padding
   - Consider hiring designer or using AI generation
2. Run Lighthouse PWA audit:
   - Open DevTools → Lighthouse
   - Run PWA audit
   - Aim for score ≥ 90
3. Fix any audit failures
4. Update README with installation instructions:
   - How to install on iOS
   - How to install on Android
   - How to install on desktop
5. Test installation flow on real devices (not just emulators)

**Acceptance Criteria:**

- Icons render without distortion on all platforms
- Lighthouse PWA score ≥ 90
- Installation instructions in README
- Successful install tested on at least:
  - One iOS device
  - One Android device
  - One desktop browser
- No console errors or warnings

**Estimated Effort:** 2-3 hours

## Total Estimated Effort

- **Phase 1: 30 minutes to 1 hour** - Ship this now
- **Phase 2: Observe for 2-4 weeks** - Talk to users
- **Phase 3: 2-3 hours** - Only if users actually install and request polish

### Total Committed Effort

1 hour maximum

The beauty of this approach: you'll know if anyone cares about PWA installability before investing time in features that may not matter.

## Testing Strategy

### Phase 1 Testing

- Install on iOS, Android, desktop
- Verify standalone mode works
- Check icon rendering
- Lighthouse PWA audit (basic)

### Phase 2 Testing (If Needed)

- User interviews and feedback collection
- Track installation metrics if possible

### Phase 3 Testing (If Needed)

- Proper icon design validation
- Cross-platform testing
- Full Lighthouse PWA audit

## Future Enhancements (Implement Only On User Request)

These features are explicitly **not** part of Phase 1. Only implement if users explicitly request them after trying the basic PWA:

### Offline Features

1. **Offline notification** - If users report confusion when network fails, consider adding clear offline status indicator. But validate that existing error messages aren't sufficient first.
2. **Offline content caching** - Only if users demonstrate actual need to access wiki without network. Requires service worker + complex sync strategy.

### Update Notification Features  

1. **Auto-update detection** - If users request proactive update notifications, consider:
   - **Option A**: Check version on page navigation (zero polling cost)
   - **Option B**: WebSocket push notification (instant, no polling waste)
   - **Avoid**: 60-second polling (wastes CPU/network/battery for minimal benefit)

### Advanced PWA Features

1. **Service workers** (for caching or background sync)
2. **App shortcuts** in manifest (quick actions from home screen)
3. **Native app store distribution** (requires PWABuilder or similar)
4. **Share target API** (receive shares from other apps)
5. **Install prompt banner** (deprecated API, avoid)
6. **Push notifications** (requires service worker + permissions)

**Principle**: Measure user behavior and requests before building features. Avoid implementing "nice to have" features that solve imaginary problems.
