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
	client    WhoIsQuerier
	agentTags map[string]struct{}
}

// NewIdentityResolver creates a new identity resolver. Pass the configured
// agent-tag set (typically from --agent-tag CLI flags) so callers connecting
// from tagged nodes are classified as agents. Pass nil or empty for "no
// nodes are agents."
func NewIdentityResolver(agentTags []string) *LocalIdentityResolver {
	return &LocalIdentityResolver{
		client:    &local.Client{},
		agentTags: tagSet(agentTags),
	}
}

// NewIdentityResolverWithClient creates a new identity resolver with a custom client.
// This is primarily used for testing.
func NewIdentityResolverWithClient(client WhoIsQuerier, agentTags []string) *LocalIdentityResolver {
	return &LocalIdentityResolver{
		client:    client,
		agentTags: tagSet(agentTags),
	}
}

// tagSet builds an O(1) lookup set from a tag slice. Returns an empty
// (non-nil) map when input is empty so the lookup path stays branch-free.
func tagSet(tags []string) map[string]struct{} {
	out := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		if t != "" {
			out[t] = struct{}{}
		}
	}
	return out
}

// WhoIs resolves the Tailscale identity for a remote address.
// Returns:
//   - (identity, nil) if the address belongs to a Tailscale node with a user profile
//   - (Anonymous, nil) if the address is from a Tailscale node but has no user profile (shared/tagged node)
//   - (Anonymous, ErrTailscaleUnavailable) if the Tailscale daemon cannot be reached
//
// When the responding node carries any tag in the configured agent-tag set,
// the returned identity reports IsAgent() == true.
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

	if r.nodeIsAgent(response) {
		identity := NewAgentIdentity(
			response.UserProfile.LoginName,
			response.UserProfile.DisplayName,
			nodeName,
		)
		if identity.IsAnonymous() {
			return Anonymous, nil
		}
		return identity, nil
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

// nodeIsAgent reports whether the WhoIs-returned node carries any of the
// configured agent tags.
func (r *LocalIdentityResolver) nodeIsAgent(response *apitype.WhoIsResponse) bool {
	if len(r.agentTags) == 0 || response.Node == nil {
		return false
	}
	for _, tag := range response.Node.Tags {
		if _, ok := r.agentTags[tag]; ok {
			return true
		}
	}
	return false
}

// Ensure LocalIdentityResolver implements IdentityResolver
var _ IdentityResolver = (*LocalIdentityResolver)(nil)
