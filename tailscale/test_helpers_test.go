package tailscale_test

import (
	"context"

	"github.com/brendanjerwin/simple_wiki/tailscale"
)

// Compile-time interface compliance check.
var _ tailscale.IdentityResolver = (*mockIdentityResolver)(nil)

// mockIdentityResolver implements tailscale.IdentityResolver for testing.
type mockIdentityResolver struct {
	identity tailscale.IdentityValue
	err      error
}

func (m *mockIdentityResolver) WhoIs(_ context.Context, _ string) (tailscale.IdentityValue, error) {
	return m.identity, m.err
}
