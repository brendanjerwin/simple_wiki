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
				resultIdent tailscale.IdentityValue
			)

			BeforeEach(func() {
				ctx = context.Background()
				identity = tailscale.NewIdentity(
					"user@example.com",
					"Test User",
					"test-node",
				)
				resultCtx = tailscale.ContextWithIdentity(ctx, identity)
				resultIdent = tailscale.IdentityFromContext(resultCtx)
			})

			It("should return the same identity", func() {
				Expect(resultIdent).To(Equal(identity))
			})

			It("should preserve the login name", func() {
				Expect(resultIdent.LoginName()).To(Equal("user@example.com"))
			})

			It("should preserve the display name", func() {
				Expect(resultIdent.DisplayName()).To(Equal("Test User"))
			})

			It("should preserve the node name", func() {
				Expect(resultIdent.NodeName()).To(Equal("test-node"))
			})
		})

		When("identity is not in context", func() {
			var (
				ctx    context.Context
				result tailscale.IdentityValue
			)

			BeforeEach(func() {
				ctx = context.Background()
				result = tailscale.IdentityFromContext(ctx)
			})

			It("should return Anonymous", func() {
				Expect(result).To(Equal(tailscale.Anonymous))
			})

			It("should be anonymous", func() {
				Expect(result.IsAnonymous()).To(BeTrue())
			})
		})

		When("Anonymous is added to context", func() {
			var (
				ctx    context.Context
				result tailscale.IdentityValue
			)

			BeforeEach(func() {
				ctx = context.Background()
				ctx = tailscale.ContextWithIdentity(ctx, tailscale.Anonymous)
				result = tailscale.IdentityFromContext(ctx)
			})

			It("should return Anonymous", func() {
				Expect(result).To(Equal(tailscale.Anonymous))
			})

			It("should be anonymous", func() {
				Expect(result.IsAnonymous()).To(BeTrue())
			})
		})
	})

	Describe("Anonymous singleton", func() {
		When("checking Anonymous properties", func() {
			var anon tailscale.IdentityValue

			BeforeEach(func() {
				anon = tailscale.Anonymous
			})

			It("should be anonymous", func() {
				Expect(anon.IsAnonymous()).To(BeTrue())
			})

			It("should have empty login name", func() {
				Expect(anon.LoginName()).To(BeEmpty())
			})

			It("should have empty display name", func() {
				Expect(anon.DisplayName()).To(BeEmpty())
			})

			It("should have empty node name", func() {
				Expect(anon.NodeName()).To(BeEmpty())
			})

			It("should return 'anonymous' from String()", func() {
				Expect(anon.String()).To(Equal("anonymous"))
			})

			It("should return 'anonymous' from ForLog()", func() {
				Expect(anon.ForLog()).To(Equal("anonymous"))
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
				identity = tailscale.NewIdentity(
					"user@example.com",
					"Test User",
					"",
				)
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
				identity = tailscale.NewIdentity(
					"",
					"Test User",
					"",
				)
				result = identity.String()
			})

			It("should return the display name", func() {
				Expect(result).To(Equal("Test User"))
			})
		})

		When("identity has only node name", func() {
			var (
				identity *tailscale.Identity
				result   string
			)

			BeforeEach(func() {
				identity = tailscale.NewIdentity(
					"",
					"",
					"my-laptop",
				)
				result = identity.String()
			})

			It("should return the node name", func() {
				Expect(result).To(Equal("my-laptop"))
			})
		})

		When("identity is empty", func() {
			var (
				identity *tailscale.Identity
				result   string
			)

			BeforeEach(func() {
				identity = tailscale.NewIdentity("", "", "")
				result = identity.String()
			})

			It("should return anonymous", func() {
				Expect(result).To(Equal("anonymous"))
			})
		})
	})

	Describe("IsAnonymous", func() {
		When("identity has login name", func() {
			var (
				identity *tailscale.Identity
				result   bool
			)

			BeforeEach(func() {
				identity = tailscale.NewIdentity("user@example.com", "", "")
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
				identity = tailscale.NewIdentity("", "Test User", "")
				result = identity.IsAnonymous()
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("identity has only node name", func() {
			var (
				identity *tailscale.Identity
				result   bool
			)

			BeforeEach(func() {
				identity = tailscale.NewIdentity("", "", "my-laptop")
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
				identity = tailscale.NewIdentity("", "", "")
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
				identity = tailscale.NewIdentity(
					"user@example.com",
					"",
					"my-laptop",
				)
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
				identity = tailscale.NewIdentity(
					"",
					"",
					"my-laptop",
				)
				result = identity.ForLog()
			})

			It("should return node name in parentheses", func() {
				Expect(result).To(Equal("(my-laptop)"))
			})
		})

		When("identity has login but no node name", func() {
			var (
				identity *tailscale.Identity
				result   string
			)

			BeforeEach(func() {
				identity = tailscale.NewIdentity(
					"user@example.com",
					"",
					"",
				)
				result = identity.ForLog()
			})

			It("should return just login name", func() {
				Expect(result).To(Equal("user@example.com"))
			})
		})

		When("identity is an agent", func() {
			var result string

			BeforeEach(func() {
				identity := tailscale.NewAgentIdentity(
					"user@example.com",
					"",
					"my-laptop",
				)
				result = identity.ForLog()
			})

			It("should append the [agent] suffix", func() {
				Expect(result).To(Equal("user@example.com (my-laptop) [agent]"))
			})
		})
	})

	Describe("IsAgent", func() {
		When("identity was constructed via NewIdentity", func() {
			var identity *tailscale.Identity

			BeforeEach(func() {
				identity = tailscale.NewIdentity("user@example.com", "", "")
			})

			It("should report false", func() {
				Expect(identity.IsAgent()).To(BeFalse())
			})
		})

		When("identity was constructed via NewAgentIdentity", func() {
			var identity *tailscale.Identity

			BeforeEach(func() {
				identity = tailscale.NewAgentIdentity("user@example.com", "", "")
			})

			It("should report true", func() {
				Expect(identity.IsAgent()).To(BeTrue())
			})
		})

		When("the Anonymous singleton is queried", func() {
			It("should report false", func() {
				Expect(tailscale.Anonymous.IsAgent()).To(BeFalse())
			})
		})

		When("an anonymous-but-claiming-agent identity is constructed", func() {
			var identity *tailscale.Identity

			BeforeEach(func() {
				identity = tailscale.NewAgentIdentity("", "", "")
			})

			It("should report IsAnonymous true", func() {
				Expect(identity.IsAnonymous()).To(BeTrue())
			})

			It("should report IsAgent true", func() {
				Expect(identity.IsAgent()).To(BeTrue())
			})
		})
	})

	Describe("Name", func() {
		When("identity has a login name", func() {
			var identity *tailscale.Identity

			BeforeEach(func() {
				identity = tailscale.NewIdentity("alice@example.com", "Alice", "alice-laptop")
			})

			It("should return the login name", func() {
				Expect(identity.Name()).To(Equal("alice@example.com"))
			})
		})

		When("identity has only a display name", func() {
			var identity *tailscale.Identity

			BeforeEach(func() {
				identity = tailscale.NewIdentity("", "Alice", "")
			})

			It("should return the display name", func() {
				Expect(identity.Name()).To(Equal("Alice"))
			})
		})

		When("identity has only a node name", func() {
			var identity *tailscale.Identity

			BeforeEach(func() {
				identity = tailscale.NewIdentity("", "", "alice-laptop")
			})

			It("should return the node name", func() {
				Expect(identity.Name()).To(Equal("alice-laptop"))
			})
		})

		When("identity is fully anonymous", func() {
			var identity *tailscale.Identity

			BeforeEach(func() {
				identity = tailscale.NewIdentity("", "", "")
			})

			It("should return an empty string (not 'anonymous')", func() {
				Expect(identity.Name()).To(BeEmpty())
			})
		})

		When("the Anonymous singleton is queried", func() {
			It("should return an empty string", func() {
				Expect(tailscale.Anonymous.Name()).To(BeEmpty())
			})
		})
	})
})
