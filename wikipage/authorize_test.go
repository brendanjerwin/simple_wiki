package wikipage_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// fakeIdentity implements wikipage.Identity for tests so we don't need to
// pull in the tailscale package here.
type fakeIdentity struct {
	loginName   string
	isAgent     bool
	isAnonymous bool
}

func (f fakeIdentity) LoginName() string { return f.loginName }
func (f fakeIdentity) IsAgent() bool     { return f.isAgent }
func (f fakeIdentity) IsAnonymous() bool { return f.isAnonymous }

func human(login string) wikipage.Identity {
	return fakeIdentity{loginName: login}
}

func agent(login string) wikipage.Identity {
	return fakeIdentity{loginName: login, isAgent: true}
}

func anonymous() wikipage.Identity {
	return fakeIdentity{isAnonymous: true}
}

func ownedBy(owner string) wikipage.FrontMatter {
	return wikipage.FrontMatter{
		"wiki": map[string]any{
			"authorization": map[string]any{
				"acl": map[string]any{
					"owner": owner,
				},
			},
		},
	}
}

func ownedByWithAgentAccess(owner string, allow bool) wikipage.FrontMatter {
	return wikipage.FrontMatter{
		"wiki": map[string]any{
			"authorization": map[string]any{
				"acl": map[string]any{
					"owner": owner,
				},
				"allow_agent_access": allow,
			},
		},
	}
}

func authorizationOnly(allowAgent bool) wikipage.FrontMatter {
	return wikipage.FrontMatter{
		"wiki": map[string]any{
			"authorization": map[string]any{
				"allow_agent_access": allowAgent,
			},
		},
	}
}

var _ = Describe("Authorize", func() {
	When("the page has no wiki.authorization subtree", func() {
		fm := wikipage.FrontMatter{"title": "Public"}

		It("should allow a human", func() {
			Expect(wikipage.Authorize(human("alice@example.com"), fm)).To(Succeed())
		})

		It("should allow an agent", func() {
			Expect(wikipage.Authorize(agent("agent@wiki"), fm)).To(Succeed())
		})

		It("should allow an anonymous caller", func() {
			Expect(wikipage.Authorize(anonymous(), fm)).To(Succeed())
		})
	})

	When("the page has wiki.authorization but no acl", func() {
		fm := authorizationOnly(false)

		It("should allow any human", func() {
			Expect(wikipage.Authorize(human("alice@example.com"), fm)).To(Succeed())
			Expect(wikipage.Authorize(human("bob@example.com"), fm)).To(Succeed())
		})

		It("should deny an anonymous caller (no login to match anything)", func() {
			Expect(wikipage.Authorize(anonymous(), fm)).To(MatchError(wikipage.ErrForbidden))
		})

		It("should deny an agent when allow_agent_access is false", func() {
			Expect(wikipage.Authorize(agent("agent@wiki"), fm)).To(MatchError(wikipage.ErrForbidden))
		})
	})

	When("the page has acl.owner set", func() {
		fm := ownedBy("alice@example.com")

		It("should allow the owner", func() {
			Expect(wikipage.Authorize(human("alice@example.com"), fm)).To(Succeed())
		})

		It("should deny a different human", func() {
			Expect(wikipage.Authorize(human("bob@example.com"), fm)).To(MatchError(wikipage.ErrForbidden))
		})

		It("should deny an anonymous caller", func() {
			Expect(wikipage.Authorize(anonymous(), fm)).To(MatchError(wikipage.ErrForbidden))
		})

		It("should deny an agent (allow_agent_access not set)", func() {
			Expect(wikipage.Authorize(agent("agent@wiki"), fm)).To(MatchError(wikipage.ErrForbidden))
		})
	})

	When("the page has allow_agent_access = true", func() {
		fm := ownedByWithAgentAccess("alice@example.com", true)

		It("should allow an agent", func() {
			Expect(wikipage.Authorize(agent("agent@wiki"), fm)).To(Succeed())
		})

		It("should still allow the human owner", func() {
			Expect(wikipage.Authorize(human("alice@example.com"), fm)).To(Succeed())
		})

		It("should still deny a non-owner human", func() {
			Expect(wikipage.Authorize(human("bob@example.com"), fm)).To(MatchError(wikipage.ErrForbidden))
		})
	})

	When("the page has allow_agent_access = true and no acl", func() {
		fm := authorizationOnly(true)

		It("should allow an agent", func() {
			Expect(wikipage.Authorize(agent("agent@wiki"), fm)).To(Succeed())
		})

		It("should allow any human", func() {
			Expect(wikipage.Authorize(human("alice@example.com"), fm)).To(Succeed())
		})
	})

	When("acl.owner is empty string (misconfigured)", func() {
		fm := ownedBy("")

		It("should allow any human (empty owner is no specific owner)", func() {
			Expect(wikipage.Authorize(human("alice@example.com"), fm)).To(Succeed())
		})
	})
})
