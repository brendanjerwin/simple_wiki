package tailscale

import (
	"github.com/gin-gonic/gin"
	"github.com/jcelliott/lumber"
)

// IdentityMiddleware creates Gin middleware that extracts Tailscale identity.
// Identity is extracted from Tailscale headers (set by Tailscale Serve) or via WhoIs.
// If identity cannot be resolved, the request continues without identity (graceful fallback).
func IdentityMiddleware(resolver IResolveIdentity, logger *lumber.ConsoleLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var identity *Identity

		// Method 1: Check Tailscale headers (set by Tailscale Serve/Funnel)
		if loginName := c.Request.Header.Get("Tailscale-User-Login"); loginName != "" {
			identity = &Identity{
				LoginName:   loginName,
				DisplayName: c.Request.Header.Get("Tailscale-User-Name"),
				NodeName:    c.Request.Header.Get("Tailscale-Node-Name"),
			}
		}

		// Method 2: Try WhoIs lookup (works for direct tailnet connections)
		if identity == nil && resolver != nil {
			var err error
			identity, err = resolver.WhoIs(ctx, c.Request.RemoteAddr)
			if err != nil && logger != nil {
				logger.Debug("WhoIs lookup failed: %v", err)
			}
		}

		if identity != nil {
			// Add identity to context
			ctx = ContextWithIdentity(ctx, identity)
			c.Request = c.Request.WithContext(ctx)

			// Also store in Gin context for convenience
			c.Set("tailscale-identity", identity)
		}

		c.Next()
	}
}
