package tailscale

import (
	"errors"
	"net/http"

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

// IdentityMiddlewareWithMetrics creates Gin middleware that extracts Tailscale identity.
// Identity is extracted from Tailscale headers (set by Tailscale Serve) or via WhoIs.
// If identity cannot be resolved, the request continues with Anonymous identity (graceful fallback).
//
// The resolver parameter may be nil. When nil, only header-based identity extraction is attempted.
// This is useful when Tailscale Serve handles all requests, so WhoIs lookups are unnecessary.
// When resolver is nil and no headers are present, requests continue as Anonymous.
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
		var identity = Anonymous

		// Method 1: Check Tailscale headers (set by Tailscale Serve/Funnel)
		// Only trust these headers from localhost (where tailscaled runs)
		// This prevents external attackers from spoofing user identity
		//
		// Note: Tailscale Serve only provides Tailscale-User-Login and Tailscale-User-Name headers.
		// The node name is not available when using Tailscale Serve; NodeName will be empty.
		// To get the node name, the WhoIs fallback must be used (direct tailnet access).
		if loginName := c.Request.Header.Get("Tailscale-User-Login"); loginName != "" && isFromLocalhost(c.Request.RemoteAddr) {
			identity = NewIdentity(loginName, c.Request.Header.Get("Tailscale-User-Name"), "")
			metrics.RecordHeaderExtraction()
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

// IdentityHTTPMiddlewareWithMetrics wraps a plain net/http handler with Tailscale identity extraction.
// It applies the same identity resolution logic as [IdentityMiddlewareWithMetrics] but for
// handlers that are not served through Gin (e.g., the MCP endpoint).
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
		var identity = Anonymous

		// Method 1: Check Tailscale headers (set by Tailscale Serve/Funnel)
		// Only trust these headers from localhost (where tailscaled runs)
		// This prevents external attackers from spoofing user identity
		if loginName := r.Header.Get("Tailscale-User-Login"); loginName != "" && isFromLocalhost(r.RemoteAddr) {
			identity = NewIdentity(loginName, r.Header.Get("Tailscale-User-Name"), "")
			metrics.RecordHeaderExtraction()
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
