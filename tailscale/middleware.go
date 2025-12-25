package tailscale

import (
	"github.com/gin-gonic/gin"
	"github.com/jcelliott/lumber"
)

// IdentityMiddleware creates Gin middleware that extracts Tailscale identity.
// If the identity resolver is nil, or if identity cannot be resolved,
// the request continues without identity (graceful fallback).
func IdentityMiddleware(resolver IResolveIdentity, logger *lumber.ConsoleLogger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if resolver == nil {
			c.Next()
			return
		}

		ctx := c.Request.Context()
		identity, err := resolver.WhoIs(ctx, c.Request.RemoteAddr)
		if err != nil {
			if logger != nil {
				logger.Warn("Failed to resolve Tailscale identity: %v", err)
			}
			// Continue without identity - graceful fallback
			c.Next()
			return
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
