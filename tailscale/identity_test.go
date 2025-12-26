package tailscale_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/tailscale"
)

var _ = Describe("Identity", func() {
	Describe("ContextWithIdentity and IdentityFromContext", func() {
		When("identity is added to context", func() {
			var (
				ctx         context.Context
				identity    *tailscale.Identity
				resultCtx   context.Context
				resultIdent *tailscale.Identity
			)

			BeforeEach(func() {
				ctx = context.Background()
				identity = &tailscale.Identity{
					LoginName:   "user@example.com",
					DisplayName: "Test User",
					NodeName:    "test-node",
				}
				resultCtx = tailscale.ContextWithIdentity(ctx, identity)
				resultIdent = tailscale.IdentityFromContext(resultCtx)
			})

			It("should return the same identity", func() {
				Expect(resultIdent).To(Equal(identity))
			})

			It("should preserve the login name", func() {
				Expect(resultIdent.LoginName).To(Equal("user@example.com"))
			})

			It("should preserve the display name", func() {
				Expect(resultIdent.DisplayName).To(Equal("Test User"))
			})

			It("should preserve the node name", func() {
				Expect(resultIdent.NodeName).To(Equal("test-node"))
			})
		})

		When("identity is not in context", func() {
			var (
				ctx    context.Context
				result *tailscale.Identity
			)

			BeforeEach(func() {
				ctx = context.Background()
				result = tailscale.IdentityFromContext(ctx)
			})

			It("should return nil", func() {
				Expect(result).To(BeNil())
			})
		})

		When("nil identity is added to context", func() {
			var (
				ctx    context.Context
				result *tailscale.Identity
			)

			BeforeEach(func() {
				ctx = context.Background()
				ctx = tailscale.ContextWithIdentity(ctx, nil)
				result = tailscale.IdentityFromContext(ctx)
			})

			It("should return nil", func() {
				Expect(result).To(BeNil())
			})
		})
	})

	Describe("String", func() {
		When("identity has login name", func() {
			var (
				identity *tailscale.Identity
				result   string
			)

			BeforeEach(func() {
				identity = &tailscale.Identity{
					LoginName:   "user@example.com",
					DisplayName: "Test User",
				}
				result = identity.String()
			})

			It("should return the login name", func() {
				Expect(result).To(Equal("user@example.com"))
			})
		})

		When("identity has only display name", func() {
			var (
				identity *tailscale.Identity
				result   string
			)

			BeforeEach(func() {
				identity = &tailscale.Identity{
					DisplayName: "Test User",
				}
				result = identity.String()
			})

			It("should return the display name", func() {
				Expect(result).To(Equal("Test User"))
			})
		})

		When("identity is empty", func() {
			var (
				identity *tailscale.Identity
				result   string
			)

			BeforeEach(func() {
				identity = &tailscale.Identity{}
				result = identity.String()
			})

			It("should return anonymous", func() {
				Expect(result).To(Equal("anonymous"))
			})
		})

		When("identity is nil", func() {
			var (
				identity *tailscale.Identity
				result   string
			)

			BeforeEach(func() {
				identity = nil
				result = identity.String()
			})

			It("should return anonymous", func() {
				Expect(result).To(Equal("anonymous"))
			})
		})
	})

	Describe("IsAnonymous", func() {
		When("identity is nil", func() {
			var (
				identity *tailscale.Identity
				result   bool
			)

			BeforeEach(func() {
				identity = nil
				result = identity.IsAnonymous()
			})

			It("should return true", func() {
				Expect(result).To(BeTrue())
			})
		})

		When("identity has login name", func() {
			var (
				identity *tailscale.Identity
				result   bool
			)

			BeforeEach(func() {
				identity = &tailscale.Identity{LoginName: "user@example.com"}
				result = identity.IsAnonymous()
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("identity has only display name", func() {
			var (
				identity *tailscale.Identity
				result   bool
			)

			BeforeEach(func() {
				identity = &tailscale.Identity{DisplayName: "Test User"}
				result = identity.IsAnonymous()
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("identity is empty", func() {
			var (
				identity *tailscale.Identity
				result   bool
			)

			BeforeEach(func() {
				identity = &tailscale.Identity{}
				result = identity.IsAnonymous()
			})

			It("should return true", func() {
				Expect(result).To(BeTrue())
			})
		})
	})

	Describe("ForLog", func() {
		When("identity has login and node name", func() {
			var (
				identity *tailscale.Identity
				result   string
			)

			BeforeEach(func() {
				identity = &tailscale.Identity{
					LoginName: "user@example.com",
					NodeName:  "my-laptop",
				}
				result = identity.ForLog()
			})

			It("should return login with node in parentheses", func() {
				Expect(result).To(Equal("user@example.com (my-laptop)"))
			})
		})

		When("identity has only node name", func() {
			var (
				identity *tailscale.Identity
				result   string
			)

			BeforeEach(func() {
				identity = &tailscale.Identity{
					NodeName: "my-laptop",
				}
				result = identity.ForLog()
			})

			It("should return anonymous since IsAnonymous is true", func() {
				Expect(result).To(Equal("anonymous"))
			})
		})

		When("identity has login but no node name", func() {
			var (
				identity *tailscale.Identity
				result   string
			)

			BeforeEach(func() {
				identity = &tailscale.Identity{
					LoginName: "user@example.com",
				}
				result = identity.ForLog()
			})

			It("should return just login name", func() {
				Expect(result).To(Equal("user@example.com"))
			})
		})

		When("identity is nil", func() {
			var (
				identity *tailscale.Identity
				result   string
			)

			BeforeEach(func() {
				identity = nil
				result = identity.ForLog()
			})

			It("should return anonymous", func() {
				Expect(result).To(Equal("anonymous"))
			})
		})
	})
})
