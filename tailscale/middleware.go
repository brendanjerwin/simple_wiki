package tailscale

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/jcelliott/lumber"
)

// IdentityMiddleware creates Gin middleware that extracts Tailscale identity.
// Identity is extracted from Tailscale headers (set by Tailscale Serve) or via WhoIs.
// If identity cannot be resolved, the request continues with Anonymous identity (graceful fallback).
func IdentityMiddleware(resolver IdentityResolver, logger *lumber.ConsoleLogger) (gin.HandlerFunc, error) {
	if logger == nil {
		return nil, errors.New("logger is required")
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
		}

		// Method 2: Try WhoIs lookup (works for direct tailnet connections)
		if identity.IsAnonymous() && resolver != nil {
			var err error
			identity, err = resolver.WhoIs(ctx, c.Request.RemoteAddr)
			if err != nil {
				logger.Debug("WhoIs lookup failed: %v", err)
			}
		}

		// Always store identity in context (Anonymous is valid)
		ctx = ContextWithIdentity(ctx, identity)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}, nil
}
