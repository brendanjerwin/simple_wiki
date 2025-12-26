// Package tailscale provides integration with Tailscale for identity and TLS.
package tailscale

import (
	"context"
	"fmt"
)

// Identity represents a Tailscale user's identity.
type Identity struct {
	LoginName   string // e.g., "user@example.com"
	DisplayName string // e.g., "John Doe"
	NodeName    string // e.g., "my-laptop"
}

// contextKey is a private type for context keys to avoid collisions.
type contextKey string

const identityContextKey contextKey = "tailscale-identity"

// anonymousLabel is a typed string for the anonymous identity representation.
// Using a distinct type prevents accidental use of raw "anonymous" strings
// and makes explicit that we're returning the canonical anonymous value.
type anonymousLabel string

const anonymousIdentity anonymousLabel = "anonymous"

// ContextWithIdentity returns a new context with the identity attached.
func ContextWithIdentity(ctx context.Context, identity *Identity) context.Context {
	return context.WithValue(ctx, identityContextKey, identity)
}

// IdentityFromContext extracts the identity from context.
// Returns nil if no identity is present.
func IdentityFromContext(ctx context.Context) *Identity {
	v := ctx.Value(identityContextKey)
	if v == nil {
		return nil
	}
	identity, ok := v.(*Identity)
	if !ok {
		return nil
	}
	return identity
}

// String returns a formatted string for logging.
func (i *Identity) String() string {
	if i == nil {
		return string(anonymousIdentity)
	}
	if i.LoginName != "" {
		return i.LoginName
	}
	if i.DisplayName != "" {
		return i.DisplayName
	}
	return string(anonymousIdentity)
}

// IsAnonymous returns true if no identity is available.
func (i *Identity) IsAnonymous() bool {
	if i == nil {
		return true
	}
	return i.LoginName == "" && i.DisplayName == ""
}

// ForLog returns a formatted string suitable for log output.
func (i *Identity) ForLog() string {
	if i == nil || i.IsAnonymous() {
		return string(anonymousIdentity)
	}
	if i.LoginName != "" && i.NodeName != "" {
		return fmt.Sprintf("%s (%s)", i.LoginName, i.NodeName)
	}
	if i.LoginName != "" {
		return i.LoginName
	}
	if i.NodeName != "" {
		return fmt.Sprintf("(%s)", i.NodeName)
	}
	return string(anonymousIdentity)
}
