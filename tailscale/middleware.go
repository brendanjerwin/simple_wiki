package tailscale

import (
	"errors"

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
// All parameters are required and validated.
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
