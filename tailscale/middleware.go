package tailscale

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"os"
	"sync"

	"github.com/brendanjerwin/simple_wiki/internal/observability"
	"github.com/gin-gonic/gin"
	"github.com/jcelliott/lumber"
)

// MetricsRecorder records Tailscale identity metrics.
// Uses observability.IdentityLookupResult for consistency across the codebase.
type MetricsRecorder interface {
	RecordTailscaleLookup(result observability.IdentityLookupResult)
	RecordHeaderExtraction()
}

// Trusted-proxy identity headers. A co-located reverse proxy knowing the
// operator-configured secret may assert the end-user identity by setting
// these headers. The proxy is expected to strip any inbound headers with
// these names from untrusted clients before forwarding.
const (
	TrustedProxySecretHeader = "X-Wiki-Trusted-Proxy-Secret"
	TrustedProxyLoginHeader  = "X-Wiki-Trusted-User-Login"
	TrustedProxyNameHeader   = "X-Wiki-Trusted-User-Name"
)

// envTrustedProxySecret names the environment variable supplying the shared
// secret the trusted proxy must present. Empty means Method 0 is disabled
// (fail-closed: no identity is asserted via the trusted proxy).
const envTrustedProxySecret = "SIMPLE_WIKI_TRUSTED_PROXY_SECRET"

// trustedProxyValue caches the operator-configured trusted-proxy secret for
// the process lifetime. It is read once via the sync.OnceValue(os.Getenv)
// pattern so per-test environment changes do not poison already-cached state.
var trustedProxyValue = sync.OnceValue(func() string {
	return os.Getenv(envTrustedProxySecret)
})

// trustedProxySecretOverride, when non-empty, takes precedence over the
// cached env value. It exists to let tests drive Method 0 without mutating
// the process environment (the sync.OnceValue cache makes env-only toggling
// unreliable). It is never set in production code.
var trustedProxySecretOverride string

// trustedProxySecret returns the operator-configured trusted-proxy secret,
// or "" when Method 0 is disabled.
func trustedProxySecret() string {
	if s := trustedProxySecretOverride; s != "" {
		return s
	}
	return trustedProxyValue()
}

// assertTrustedProxyIdentity implements Method 0: trusted-proxy identity.
// If the secret is configured and the request carries a matching
// TrustedProxySecretHeader (constant-time compare) with a non-empty
// TrustedProxyLoginHeader, it returns the asserted identity and reports
// whether an identity was established. Otherwise it returns an anonymous
// identity and false so callers fall through to the other methods.
func assertTrustedProxyIdentity(r *http.Request, metrics MetricsRecorder) (IdentityValue, bool) {
	secret := trustedProxySecret()
	if secret == "" {
		return Anonymous, false
	}
	provided := r.Header.Get(TrustedProxySecretHeader)
	if subtle.ConstantTimeCompare([]byte(provided), []byte(secret)) != 1 {
		return Anonymous, false
	}
	login := r.Header.Get(TrustedProxyLoginHeader)
	if login == "" {
		return Anonymous, false
	}
	identity := NewIdentity(login, r.Header.Get(TrustedProxyNameHeader), "")
	metrics.RecordHeaderExtraction()
	return identity, true
}

// IdentityMiddlewareWithMetrics creates Gin middleware that extracts identity.
// Identity is resolved via: Method 0 (trusted proxy headers, when the
// operator-configured secret matches), Method 1 (Tailscale headers set by
// Tailscale Serve, trusted only from localhost), then Method 2 (WhoIs lookup
// for direct tailnet connections).
// If identity cannot be resolved, the request continues with Anonymous
// identity (graceful fallback).
//
// The resolver parameter may be nil. When nil, only header-based identity
// extraction is attempted (Methods 0 and 1). This is useful when Tailscale
// Serve handles all requests, so WhoIs lookups are unnecessary. When resolver
// is nil and no headers are present, requests continue as Anonymous.
//
// The logger and metrics parameters are required and validated.
func IdentityMiddlewareWithMetrics(resolver IdentityResolver, logger *lumber.ConsoleLogger, metrics MetricsRecorder) (gin.HandlerFunc, error) {
	if logger == nil {
		return nil, errors.New("logger is required")
	}
	if metrics == nil {
		return nil, errors.New("metrics is required")
	}

	return func(c *gin.Context) {
		ctx := c.Request.Context()
		identity := Anonymous

		// Method 0: trusted proxy. A co-located reverse proxy knowing the
		// operator-configured secret may assert the end-user identity.
		if id, ok := assertTrustedProxyIdentity(c.Request, metrics); ok {
			identity = id
		}

		// Method 1: Check Tailscale headers (set by Tailscale Serve/Funnel)
		// Only trust these headers from localhost (where tailscaled runs)
		// This prevents external attackers from spoofing user identity
		//
		// Note: Tailscale Serve only provides Tailscale-User-Login and Tailscale-User-Name headers.
		// The node name is not available when using Tailscale Serve; NodeName will be empty.
		// To get the node name, the WhoIs fallback must be used (direct tailnet access).
		if identity.IsAnonymous() {
			if loginName := c.Request.Header.Get("Tailscale-User-Login"); loginName != "" && isFromLocalhost(c.Request.RemoteAddr) {
				identity = NewIdentity(loginName, c.Request.Header.Get("Tailscale-User-Name"), "")
				metrics.RecordHeaderExtraction()
			}
		}

		// Method 2: Try WhoIs lookup (works for direct tailnet connections)
		if identity.IsAnonymous() && resolver != nil {
			var err error
			identity, err = resolver.WhoIs(ctx, c.Request.RemoteAddr)
			if err != nil {
				metrics.RecordTailscaleLookup(observability.ResultFailure)
				logger.Debug("WhoIs lookup failed: %v", err)
			} else if identity.IsAnonymous() {
				metrics.RecordTailscaleLookup(observability.ResultNotTailnet)
			} else {
				metrics.RecordTailscaleLookup(observability.ResultSuccess)
			}
		}

		// Always store identity in context (Anonymous is valid)
		ctx = ContextWithIdentity(ctx, identity)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}, nil
}

// IdentityHTTPMiddlewareWithMetrics wraps a plain net/http handler with identity extraction.
// It applies the same identity resolution logic as [IdentityMiddlewareWithMetrics] but for
// handlers that are not served through Gin (e.g., the MCP endpoint). Identity is resolved via
// Method 0 (trusted proxy headers), Method 1 (Tailscale headers from localhost), then Method 2
// (WhoIs lookup).
// Identity is injected into the request context so that downstream handlers can call
// [IdentityFromContext] to retrieve it. If identity cannot be resolved, the request
// continues with Anonymous identity (graceful fallback — no requests are rejected).
//
// The resolver parameter may be nil. When nil, only header-based identity extraction is attempted.
// The logger and metrics parameters are required and validated.
func IdentityHTTPMiddlewareWithMetrics(resolver IdentityResolver, logger *lumber.ConsoleLogger, metrics MetricsRecorder, next http.Handler) (http.Handler, error) {
	if logger == nil {
		return nil, errors.New("logger is required")
	}
	if metrics == nil {
		return nil, errors.New("metrics is required")
	}
	if next == nil {
		return nil, errors.New("next handler is required")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		identity := Anonymous

		// Method 0: trusted proxy. A co-located reverse proxy knowing the
		// operator-configured secret may assert the end-user identity.
		if id, ok := assertTrustedProxyIdentity(r, metrics); ok {
			identity = id
		}

		// Method 1: Check Tailscale headers (set by Tailscale Serve/Funnel)
		// Only trust these headers from localhost (where tailscaled runs)
		// This prevents external attackers from spoofing user identity
		if identity.IsAnonymous() {
			if loginName := r.Header.Get("Tailscale-User-Login"); loginName != "" && isFromLocalhost(r.RemoteAddr) {
				identity = NewIdentity(loginName, r.Header.Get("Tailscale-User-Name"), "")
				metrics.RecordHeaderExtraction()
			}
		}

		// Method 2: Try WhoIs lookup (works for direct tailnet connections)
		if identity.IsAnonymous() && resolver != nil {
			var err error
			identity, err = resolver.WhoIs(ctx, r.RemoteAddr)
			if err != nil {
				metrics.RecordTailscaleLookup(observability.ResultFailure)
				logger.Debug("WhoIs lookup failed: %v", err)
			} else if identity.IsAnonymous() {
				metrics.RecordTailscaleLookup(observability.ResultNotTailnet)
			} else {
				metrics.RecordTailscaleLookup(observability.ResultSuccess)
			}
		}

		// Always store identity in context (Anonymous is valid)
		ctx = ContextWithIdentity(ctx, identity)
		next.ServeHTTP(w, r.WithContext(ctx))
	}), nil
}
