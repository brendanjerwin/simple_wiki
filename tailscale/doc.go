// Package tailscale provides integration with Tailscale for identity resolution,
// TLS certificate management, and HTTP/gRPC middleware.
//
// # Graceful Fallback Pattern
//
// This package follows a graceful fallback pattern for identity resolution.
// When Tailscale is not available or a request comes from outside the tailnet,
// functions return nil identity rather than errors. This allows the application
// to continue operating without Tailscale, falling back to other authentication
// methods or allowing anonymous access as configured.
//
// The only exception is [ErrTailscaleUnavailable], which is returned when
// the Tailscale daemon cannot be reached. This distinguishes "daemon down"
// from "request not from tailnet".
//
// # Identity Resolution Priority
//
// Identity is resolved in the following order:
//  1. Trusted headers from localhost (Tailscale Serve sets these)
//  2. WhoIs API call to the local Tailscale daemon
//
// When using Tailscale Serve, only user identity is available (LoginName, DisplayName).
// The NodeName is only available via the WhoIs API (direct tailnet access).
//
// # Components
//
//   - [LocalDetector]: Detects Tailscale availability and retrieves status
//   - [LocalIdentityResolver]: Resolves user identity from IP addresses via WhoIs
//   - [TailscaleTLSConfigurer]: Provides TLS configuration using Tailscale certificates
//   - [IdentityMiddleware]: Gin middleware for HTTP identity extraction
//   - [IdentityInterceptor]: gRPC interceptor for identity extraction
//   - [TailnetRedirector]: HTTP handler that redirects tailnet clients to HTTPS
package tailscale
