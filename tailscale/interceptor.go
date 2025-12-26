package tailscale

import (
	"context"

	"github.com/jcelliott/lumber"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

// IdentityInterceptor creates a gRPC unary interceptor that extracts Tailscale identity.
// Identity is extracted from gRPC metadata (headers from Tailscale Serve) or via WhoIs.
// If identity cannot be resolved, the request continues without identity (graceful fallback).
func IdentityInterceptor(resolver IResolveIdentity, logger *lumber.ConsoleLogger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		var identity *Identity

		// Method 1: Check gRPC metadata for Tailscale headers (set by Tailscale Serve/Funnel)
		// Note: Tailscale Serve only provides user identity, not node name
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if loginNames := md.Get("tailscale-user-login"); len(loginNames) > 0 {
				var displayName string
				if names := md.Get("tailscale-user-name"); len(names) > 0 {
					displayName = names[0]
				}
				identity = &Identity{
					LoginName:   loginNames[0],
					DisplayName: displayName,
				}
			}
		}

		// Method 2: Try WhoIs lookup (works for direct tailnet connections)
		if identity == nil && resolver != nil {
			p, ok := peer.FromContext(ctx)
			if ok && p.Addr != nil {
				var err error
				identity, err = resolver.WhoIs(ctx, p.Addr.String())
				if err != nil && logger != nil {
					logger.Debug("WhoIs lookup failed for gRPC: %v", err)
				}
			}
		}

		if identity != nil {
			ctx = ContextWithIdentity(ctx, identity)
		}

		return handler(ctx, req)
	}
}
