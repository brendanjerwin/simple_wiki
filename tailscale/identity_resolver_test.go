package tailscale_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"tailscale.com/client/tailscale/apitype"
	"tailscale.com/tailcfg"

	"github.com/brendanjerwin/simple_wiki/tailscale"
)

// mockWhoIsClient implements IWhoIsClient for testing.
type mockWhoIsClient struct {
	response *apitype.WhoIsResponse
	err      error
}

func (m *mockWhoIsClient) WhoIs(_ context.Context, _ string) (*apitype.WhoIsResponse, error) {
	return m.response, m.err
}

var _ = Describe("IdentityResolver", func() {
	Describe("NewIdentityResolver", func() {
		When("creating a new identity resolver", func() {
			var resolver *tailscale.IdentityResolver

			BeforeEach(func() {
				resolver = tailscale.NewIdentityResolver()
			})

			It("should not be nil", func() {
				Expect(resolver).NotTo(BeNil())
			})
		})
	})

	Describe("NewIdentityResolverWithClient", func() {
		When("creating with a custom client", func() {
			var resolver *tailscale.IdentityResolver

			BeforeEach(func() {
				client := &mockWhoIsClient{}
				resolver = tailscale.NewIdentityResolverWithClient(client)
			})

			It("should not be nil", func() {
				Expect(resolver).NotTo(BeNil())
			})
		})
	})

	Describe("WhoIs", func() {
		When("client returns an error", func() {
			var (
				resolver *tailscale.IdentityResolver
				identity *tailscale.Identity
				err      error
			)

			BeforeEach(func() {
				client := &mockWhoIsClient{
					response: nil,
					err:      errors.New("connection refused"),
				}
				resolver = tailscale.NewIdentityResolverWithClient(client)
				identity, err = resolver.WhoIs(context.Background(), "100.64.0.1:12345")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return nil identity for graceful fallback", func() {
				Expect(identity).To(BeNil())
			})
		})

		When("client returns a valid response with user profile", func() {
			var (
				resolver *tailscale.IdentityResolver
				identity *tailscale.Identity
				err      error
			)

			BeforeEach(func() {
				client := &mockWhoIsClient{
					response: &apitype.WhoIsResponse{
						UserProfile: &tailcfg.UserProfile{
							LoginName:   "user@example.com",
							DisplayName: "Test User",
						},
						Node: &tailcfg.Node{
							ComputedName: "my-laptop",
						},
					},
					err: nil,
				}
				resolver = tailscale.NewIdentityResolverWithClient(client)
				identity, err = resolver.WhoIs(context.Background(), "100.64.0.1:12345")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the identity", func() {
				Expect(identity).NotTo(BeNil())
			})

			It("should have the correct login name", func() {
				Expect(identity.LoginName).To(Equal("user@example.com"))
			})

			It("should have the correct display name", func() {
				Expect(identity.DisplayName).To(Equal("Test User"))
			})

			It("should have the correct node name", func() {
				Expect(identity.NodeName).To(Equal("my-laptop"))
			})
		})

		When("client returns response without user profile", func() {
			var (
				resolver *tailscale.IdentityResolver
				identity *tailscale.Identity
				err      error
			)

			BeforeEach(func() {
				client := &mockWhoIsClient{
					response: &apitype.WhoIsResponse{
						UserProfile: nil,
						Node: &tailcfg.Node{
							ComputedName: "my-laptop",
						},
					},
					err: nil,
				}
				resolver = tailscale.NewIdentityResolverWithClient(client)
				identity, err = resolver.WhoIs(context.Background(), "100.64.0.1:12345")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return nil identity (anonymous)", func() {
				Expect(identity).To(BeNil())
			})
		})

		When("client returns response without node", func() {
			var (
				resolver *tailscale.IdentityResolver
				identity *tailscale.Identity
				err      error
			)

			BeforeEach(func() {
				client := &mockWhoIsClient{
					response: &apitype.WhoIsResponse{
						UserProfile: &tailcfg.UserProfile{
							LoginName:   "user@example.com",
							DisplayName: "Test User",
						},
						Node: nil,
					},
					err: nil,
				}
				resolver = tailscale.NewIdentityResolverWithClient(client)
				identity, err = resolver.WhoIs(context.Background(), "100.64.0.1:12345")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the identity", func() {
				Expect(identity).NotTo(BeNil())
			})

			It("should have empty node name", func() {
				Expect(identity.NodeName).To(BeEmpty())
			})

			It("should have the correct login name", func() {
				Expect(identity.LoginName).To(Equal("user@example.com"))
			})
		})

		When("client returns empty response", func() {
			var (
				resolver *tailscale.IdentityResolver
				identity *tailscale.Identity
				err      error
			)

			BeforeEach(func() {
				client := &mockWhoIsClient{
					response: &apitype.WhoIsResponse{},
					err:      nil,
				}
				resolver = tailscale.NewIdentityResolverWithClient(client)
				identity, err = resolver.WhoIs(context.Background(), "100.64.0.1:12345")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return nil identity", func() {
				Expect(identity).To(BeNil())
			})
		})
	})

	Describe("interface compliance", func() {
		It("should implement IResolveIdentity", func() {
			var _ tailscale.IResolveIdentity = (*tailscale.IdentityResolver)(nil)
		})
	})
})
