package tailscale

import (
	"context"
	"errors"

	"tailscale.com/client/local"
	"tailscale.com/client/tailscale/apitype"
)

// ErrTailscaleUnavailable indicates the Tailscale daemon could not be reached.
var ErrTailscaleUnavailable = errors.New("tailscale daemon unavailable")

// IdentityResolver abstracts identity resolution for testing.
type IdentityResolver interface {
	WhoIs(ctx context.Context, remoteAddr string) (*Identity, error)
}

// WhoIsQuerier abstracts the Tailscale WhoIs API for testing.
type WhoIsQuerier interface {
	WhoIs(ctx context.Context, remoteAddr string) (*apitype.WhoIsResponse, error)
}

// LocalIdentityResolver resolves Tailscale identities from remote addresses
// using the local Tailscale daemon.
type LocalIdentityResolver struct {
	client WhoIsQuerier
}

// NewIdentityResolver creates a new identity resolver.
func NewIdentityResolver() *LocalIdentityResolver {
	return &LocalIdentityResolver{
		client: &local.Client{},
	}
}

// NewIdentityResolverWithClient creates a new identity resolver with a custom client.
// This is primarily used for testing.
func NewIdentityResolverWithClient(client WhoIsQuerier) *LocalIdentityResolver {
	return &LocalIdentityResolver{
		client: client,
	}
}

// WhoIs resolves the Tailscale identity for a remote address.
// Returns ErrTailscaleUnavailable if the Tailscale daemon cannot be reached.
// Returns nil, nil if the address is not from a Tailscale node (anonymous/non-tailnet).
func (r *LocalIdentityResolver) WhoIs(ctx context.Context, remoteAddr string) (*Identity, error) {
	whois, err := r.client.WhoIs(ctx, remoteAddr)
	if err != nil {
		// Tailscale daemon not available
		return nil, ErrTailscaleUnavailable
	}

	// Extract identity from WhoIs response
	identity := &Identity{}

	if whois.UserProfile != nil {
		identity.LoginName = whois.UserProfile.LoginName
		identity.DisplayName = whois.UserProfile.DisplayName
	}

	if whois.Node != nil {
		identity.NodeName = whois.Node.ComputedName
	}

	// If we couldn't extract any identity info, return nil
	if identity.IsAnonymous() {
		return nil, nil
	}

	return identity, nil
}

// Ensure LocalIdentityResolver implements IdentityResolver
var _ IdentityResolver = (*LocalIdentityResolver)(nil)
