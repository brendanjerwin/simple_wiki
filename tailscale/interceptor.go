package tailscale

import (
	"context"
	"errors"

	"github.com/jcelliott/lumber"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

// IdentityInterceptor creates a gRPC unary interceptor that extracts Tailscale identity.
// Identity is extracted from gRPC metadata (headers from Tailscale Serve) or via WhoIs.
// If identity cannot be resolved, the request continues with Anonymous identity (graceful fallback).
func IdentityInterceptor(resolver IdentityResolver, logger *lumber.ConsoleLogger) (grpc.UnaryServerInterceptor, error) {
	if logger == nil {
		return nil, errors.New("logger is required")
	}

	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		var identity = Anonymous

		// Method 1: Check gRPC metadata for Tailscale headers (set by Tailscale Serve/Funnel)
		//
		// Note: Tailscale Serve only provides Tailscale-User-Login and Tailscale-User-Name headers.
		// The node name is not available when using Tailscale Serve; NodeName will be empty.
		// To get the node name, the WhoIs fallback must be used (direct tailnet access).
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if loginNames := md.Get("tailscale-user-login"); len(loginNames) > 0 {
				var displayName string
				if names := md.Get("tailscale-user-name"); len(names) > 0 {
					displayName = names[0]
				}
				identity = NewIdentity(loginNames[0], displayName, "")
			}
		}

		// Method 2: Try WhoIs lookup (works for direct tailnet connections)
		if identity.IsAnonymous() && resolver != nil {
			p, ok := peer.FromContext(ctx)
			if ok && p.Addr != nil {
				var err error
				identity, err = resolver.WhoIs(ctx, p.Addr.String())
				if err != nil {
					logger.Debug("WhoIs lookup failed for gRPC: %v", err)
				}
			}
		}

		// Always store identity in context (Anonymous is valid)
		ctx = ContextWithIdentity(ctx, identity)

		return handler(ctx, req)
	}, nil
}
