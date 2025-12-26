package tailscale

import (
	"context"

	"tailscale.com/client/local"
	"tailscale.com/client/tailscale/apitype"
)

// IResolveIdentity abstracts identity resolution for testing.
type IResolveIdentity interface {
	WhoIs(ctx context.Context, remoteAddr string) (*Identity, error)
}

// IWhoIsClient abstracts the Tailscale WhoIs API for testing.
type IWhoIsClient interface {
	WhoIs(ctx context.Context, remoteAddr string) (*apitype.WhoIsResponse, error)
}

// IdentityResolver resolves Tailscale identities from remote addresses.
type IdentityResolver struct {
	client IWhoIsClient
}

// NewIdentityResolver creates a new identity resolver.
func NewIdentityResolver() *IdentityResolver {
	return &IdentityResolver{
		client: &local.Client{},
	}
}

// NewIdentityResolverWithClient creates a new identity resolver with a custom client.
// This is primarily used for testing.
func NewIdentityResolverWithClient(client IWhoIsClient) *IdentityResolver {
	return &IdentityResolver{
		client: client,
	}
}

// WhoIs resolves the identity for a remote address.
// Returns nil, nil if Tailscale is not available or the address is not from the tailnet.
// This allows graceful fallback for non-Tailscale requests.
func (r *IdentityResolver) WhoIs(ctx context.Context, remoteAddr string) (*Identity, error) {
	whois, err := r.client.WhoIs(ctx, remoteAddr)
	if err != nil {
		// Not a tailnet request or Tailscale not available
		// Return nil without error to allow graceful fallback
		return nil, nil
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

// Ensure IdentityResolver implements IResolveIdentity
var _ IResolveIdentity = (*IdentityResolver)(nil)
