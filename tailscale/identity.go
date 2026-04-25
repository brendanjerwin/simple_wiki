// Package tailscale provides integration with Tailscale for identity and TLS.
package tailscale

import (
	"context"
	"fmt"
)

// IdentityValue represents either a real identity or anonymous.
// This eliminates nil checks throughout the codebase.
type IdentityValue interface {
	IsAnonymous() bool
	LoginName() string
	DisplayName() string
	NodeName() string
	ForLog() string
	String() string
	// IsAgent reports whether this identity belongs to an automated caller
	// (a tagged Tailscale device, an anonymous caller asserting agent-ness
	// via metadata, or wiki-cli running with its default agent assumption).
	// Used by services that record attribution (e.g. ChecklistService).
	IsAgent() bool
	// Name returns a short, stable identifier for the principal — typically
	// the LoginName, falling back to DisplayName then NodeName. Empty for
	// anonymous identities. Used by services that record attribution
	// (e.g. ChecklistService.completed_by).
	Name() string
}

const anonymousLabel = "anonymous"
const agentSuffix = " [agent]"

// Anonymous is the singleton representing no identity.
var Anonymous IdentityValue = anonymousIdentity{}

type anonymousIdentity struct{}

func (anonymousIdentity) IsAnonymous() bool   { return true }
func (anonymousIdentity) LoginName() string   { return "" }
func (anonymousIdentity) DisplayName() string { return "" }
func (anonymousIdentity) NodeName() string    { return "" }
func (anonymousIdentity) ForLog() string      { return anonymousLabel }
func (anonymousIdentity) String() string      { return anonymousLabel }
func (anonymousIdentity) IsAgent() bool       { return false }
func (anonymousIdentity) Name() string        { return "" }

var _ IdentityValue = anonymousIdentity{}

// Identity represents a Tailscale user's identity.
type Identity struct {
	loginName   string // private - use LoginName() method
	displayName string
	nodeName    string
	isAgent     bool
}

// NewIdentity creates a new identity with the given values. The resulting
// identity reports IsAgent() == false. Use NewAgentIdentity for callers
// the resolver has classified as automated.
func NewIdentity(loginName, displayName, nodeName string) *Identity {
	return &Identity{
		loginName:   loginName,
		displayName: displayName,
		nodeName:    nodeName,
	}
}

// NewAgentIdentity creates an identity that reports IsAgent() == true.
// Callers: the Tailscale resolver when a node tag matches the agent set,
// gRPC interceptors for anonymous self-claim via metadata header, and
// wiki-cli when WIKI_CLI_HUMAN is unset.
func NewAgentIdentity(loginName, displayName, nodeName string) *Identity {
	return &Identity{
		loginName:   loginName,
		displayName: displayName,
		nodeName:    nodeName,
		isAgent:     true,
	}
}

// Ensure Identity implements IdentityValue
var _ IdentityValue = (*Identity)(nil)

func (i *Identity) IsAnonymous() bool {
	return i.loginName == "" && i.displayName == "" && i.nodeName == ""
}

func (i *Identity) LoginName() string   { return i.loginName }
func (i *Identity) DisplayName() string { return i.displayName }
func (i *Identity) NodeName() string    { return i.nodeName }
func (i *Identity) IsAgent() bool       { return i.isAgent }

func (i *Identity) Name() string {
	if i.loginName != "" {
		return i.loginName
	}
	if i.displayName != "" {
		return i.displayName
	}
	return i.nodeName
}

func (i *Identity) String() string {
	if i.IsAnonymous() {
		return anonymousLabel
	}
	if i.loginName != "" {
		return i.loginName
	}
	if i.displayName != "" {
		return i.displayName
	}
	return i.nodeName
}

func (i *Identity) ForLog() string {
	base := i.forLogBase()
	if i.isAgent {
		return base + agentSuffix
	}
	return base
}

func (i *Identity) forLogBase() string {
	if i.IsAnonymous() {
		return anonymousLabel
	}
	if i.loginName != "" && i.nodeName != "" {
		return fmt.Sprintf("%s (%s)", i.loginName, i.nodeName)
	}
	if i.loginName != "" {
		return i.loginName
	}
	if i.nodeName != "" {
		return fmt.Sprintf("(%s)", i.nodeName)
	}
	return anonymousLabel
}

// Context key for identity
type contextKey string

const identityContextKey contextKey = "tailscale-identity"

// ContextWithIdentity returns a new context with the identity attached.
func ContextWithIdentity(ctx context.Context, identity IdentityValue) context.Context {
	return context.WithValue(ctx, identityContextKey, identity)
}

// IdentityFromContext extracts the identity from context.
// Returns Anonymous if no identity is present.
func IdentityFromContext(ctx context.Context) IdentityValue {
	v := ctx.Value(identityContextKey)
	if v == nil {
		return Anonymous
	}
	identity, ok := v.(IdentityValue)
	if !ok {
		return Anonymous
	}
	return identity
}
