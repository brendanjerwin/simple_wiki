package tailscale

import (
	"context"

	"github.com/jcelliott/lumber"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

// IdentityInterceptor creates a gRPC unary interceptor that extracts Tailscale identity.
// If the identity resolver is nil, or if identity cannot be resolved,
// the request continues without identity (graceful fallback).
func IdentityInterceptor(resolver IResolveIdentity, logger *lumber.ConsoleLogger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if resolver == nil {
			return handler(ctx, req)
		}

		// Extract peer address from gRPC context
		p, ok := peer.FromContext(ctx)
		if !ok || p.Addr == nil {
			return handler(ctx, req)
		}

		identity, err := resolver.WhoIs(ctx, p.Addr.String())
		if err != nil {
			if logger != nil {
				logger.Warn("Failed to resolve Tailscale identity for gRPC: %v", err)
			}
			// Continue without identity - graceful fallback
			return handler(ctx, req)
		}

		if identity != nil {
			ctx = ContextWithIdentity(ctx, identity)
		}

		return handler(ctx, req)
	}
}
