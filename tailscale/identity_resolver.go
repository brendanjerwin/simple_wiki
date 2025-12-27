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
	WhoIs(ctx context.Context, remoteAddr string) (IdentityValue, error)
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
// Returns:
//   - (identity, nil) if the address belongs to a Tailscale node with a user profile
//   - (Anonymous, nil) if the address is from a Tailscale node but has no user profile (shared/tagged node)
//   - (Anonymous, ErrTailscaleUnavailable) if the Tailscale daemon cannot be reached
func (r *LocalIdentityResolver) WhoIs(ctx context.Context, remoteAddr string) (IdentityValue, error) {
	response, err := r.client.WhoIs(ctx, remoteAddr)
	if err != nil {
		return Anonymous, ErrTailscaleUnavailable
	}

	if response.UserProfile == nil {
		return Anonymous, nil
	}

	nodeName := ""
	if response.Node != nil {
		nodeName = response.Node.ComputedName
	}

	identity := NewIdentity(
		response.UserProfile.LoginName,
		response.UserProfile.DisplayName,
		nodeName,
	)

	if identity.IsAnonymous() {
		return Anonymous, nil
	}

	return identity, nil
}

// Ensure LocalIdentityResolver implements IdentityResolver
var _ IdentityResolver = (*LocalIdentityResolver)(nil)
