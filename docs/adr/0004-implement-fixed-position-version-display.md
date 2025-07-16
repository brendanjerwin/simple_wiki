# 4. Implement Fixed-Position Version Display Component

Date: 2025-01-16

## Status

Accepted

## Context

The application requires a way to display version information that is:
1. Always visible to users regardless of page content
2. Unobtrusive and doesn't interfere with main content
3. Provides transparency about the running application version
4. Demonstrates working gRPC communication from frontend to backend

## Decision

We will implement a fixed-position version display component (`<version-display>`) that:
- Positions itself in the bottom-right corner of the screen using `position: fixed`
- Displays version, commit hash, and build timestamp
- Uses semi-transparent styling (30% opacity) that becomes more opaque (90%) on hover
- Fetches data via gRPC using the Connect-ES client
- Handles loading and error states gracefully

## Consequences

### Positive
- Always visible version information for debugging and support
- Demonstrates working gRPC-Web communication
- Minimal visual impact on user experience
- Consistent positioning across all pages

### Negative
- Adds a permanent UI element that may not be needed in production
- Requires additional network request on every page load
- Fixed positioning may conflict with other fixed elements

### Neutral
- Small bundle size impact due to additional component
- Additional test coverage requirements