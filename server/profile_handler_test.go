//revive:disable:dot-imports
package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/gin-gonic/gin"
)

// fakeProfileMutator is the minimum PageReaderMutator for the profile-handler
// tests. Only ReadFrontMatter, ReadMarkdown, WriteFrontMatter, and
// WriteMarkdown are exercised; the rest are no-ops.
type fakeProfileMutator struct {
	mu             sync.Mutex
	frontmatter    map[wikipage.PageIdentifier]wikipage.FrontMatter
	markdown       map[wikipage.PageIdentifier]wikipage.Markdown
	notFoundForFM  map[wikipage.PageIdentifier]bool
	writtenFM      map[wikipage.PageIdentifier]wikipage.FrontMatter
	writtenMD      map[wikipage.PageIdentifier]wikipage.Markdown
	readMDIDs      map[wikipage.PageIdentifier]int
	readFMIDs      map[wikipage.PageIdentifier]int
}

func newFakeProfileMutator() *fakeProfileMutator {
	return &fakeProfileMutator{
		frontmatter:   map[wikipage.PageIdentifier]wikipage.FrontMatter{},
		markdown:      map[wikipage.PageIdentifier]wikipage.Markdown{},
		notFoundForFM: map[wikipage.PageIdentifier]bool{},
		writtenFM:     map[wikipage.PageIdentifier]wikipage.FrontMatter{},
		writtenMD:     map[wikipage.PageIdentifier]wikipage.Markdown{},
		readMDIDs:     map[wikipage.PageIdentifier]int{},
		readFMIDs:     map[wikipage.PageIdentifier]int{},
	}
}

func (f *fakeProfileMutator) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.readFMIDs[id]++
	if f.notFoundForFM[id] {
		return id, nil, os.ErrNotExist
	}
	if fm, ok := f.frontmatter[id]; ok {
		return id, fm, nil
	}
	return id, nil, os.ErrNotExist
}

func (f *fakeProfileMutator) ReadMarkdown(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.readMDIDs[id]++
	if md, ok := f.markdown[id]; ok {
		return id, md, nil
	}
	return id, "", os.ErrNotExist
}

func (f *fakeProfileMutator) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writtenFM[id] = fm
	f.frontmatter[id] = fm
	delete(f.notFoundForFM, id)
	return nil
}

func (f *fakeProfileMutator) WriteMarkdown(id wikipage.PageIdentifier, md wikipage.Markdown) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writtenMD[id] = md
	f.markdown[id] = md
	return nil
}

func (*fakeProfileMutator) DeletePage(_ wikipage.PageIdentifier) error { return nil }

func (*fakeProfileMutator) ModifyMarkdown(_ wikipage.PageIdentifier, _ func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	return nil
}

// fakeFrontmatterIndex is a stub IQueryFrontmatterIndex that returns no
// matches and no values. The profile handler does not exercise the index
// directly; templating.ExecuteTemplate may call into it for macro
// expansion, and an empty index is fine for the simple template body the
// profile_template ships with.
type fakeFrontmatterIndex struct{}

func (fakeFrontmatterIndex) QueryExactMatch(_, _ string) []wikipage.PageIdentifier { return nil }
func (fakeFrontmatterIndex) QueryExactMatchSortedBy(_, _, _ string, _ bool, _ int) []wikipage.PageIdentifier {
	return nil
}
func (fakeFrontmatterIndex) QueryPrefixMatch(_, _ string) []wikipage.PageIdentifier  { return nil }
func (fakeFrontmatterIndex) QueryKeyExistence(_ string) []wikipage.PageIdentifier    { return nil }
func (fakeFrontmatterIndex) GetValue(_ wikipage.PageIdentifier, _ wikipage.DottedKeyPath) wikipage.Value {
	return ""
}

// makeContext builds a Gin context with the given identity injected and a
// fresh ResponseRecorder so tests can assert status, body, and Location.
func makeContext(identity tailscale.IdentityValue) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req, _ := http.NewRequest(http.MethodGet, "/profile", nil)
	req = req.WithContext(tailscale.ContextWithIdentity(req.Context(), identity))
	c.Request = req
	return c, rec
}

var _ = Describe("resolveAndRedirectToProfile", func() {
	var (
		mutator *fakeProfileMutator
		index   fakeFrontmatterIndex
	)

	BeforeEach(func() {
		mutator = newFakeProfileMutator()
		index = fakeFrontmatterIndex{}
	})

	When("the caller is anonymous", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			c, r := makeContext(tailscale.Anonymous)
			rec = r
			resolveAndRedirectToProfile(c, tailscale.Anonymous, mutator, index)
		})

		It("should respond 403", func() {
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})

		It("should explain why", func() {
			Expect(rec.Body.String()).To(ContainSubstring("authenticated user"))
		})

		It("should not touch the page store", func() {
			Expect(mutator.writtenFM).To(BeEmpty())
			Expect(mutator.writtenMD).To(BeEmpty())
			Expect(mutator.readFMIDs).To(BeEmpty())
		})
	})

	When("the caller is the agent identity", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			agent := tailscale.NewAgentIdentity("agent@wiki", "Agent", "node")
			c, r := makeContext(agent)
			rec = r
			resolveAndRedirectToProfile(c, agent, mutator, index)
		})

		It("should respond 403", func() {
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})

		It("should not touch the page store", func() {
			Expect(mutator.writtenFM).To(BeEmpty())
			Expect(mutator.readFMIDs).To(BeEmpty())
		})
	})

	When("the caller is a real human and the profile page already exists", func() {
		var (
			rec        *httptest.ResponseRecorder
			expectedID wikipage.PageIdentifier
		)

		BeforeEach(func() {
			user := tailscale.NewIdentity("alice@example.com", "Alice", "alice-laptop")
			expectedID, _ = wikipage.ProfileIdentifierFor("alice@example.com")
			mutator.frontmatter[expectedID] = wikipage.FrontMatter{
				"identifier": string(expectedID),
				"title":      "Profile: alice@example.com",
			}
			c, r := makeContext(user)
			rec = r
			resolveAndRedirectToProfile(c, user, mutator, index)
		})

		It("should redirect 302 to the canonical view URL", func() {
			Expect(rec.Code).To(Equal(http.StatusFound))
			Expect(rec.Header().Get("Location")).To(Equal("/" + string(expectedID) + "/view"))
		})

		It("should not write to the page store", func() {
			Expect(mutator.writtenFM).To(BeEmpty())
			Expect(mutator.writtenMD).To(BeEmpty())
		})

		It("should not read the template page", func() {
			Expect(mutator.readMDIDs[profileTemplateIdentifier]).To(Equal(0))
		})
	})

	When("the profile page is missing and the template exists", func() {
		var (
			rec        *httptest.ResponseRecorder
			expectedID wikipage.PageIdentifier
		)

		BeforeEach(func() {
			user := tailscale.NewIdentity("bob.smith@example.com", "Bob", "bob-laptop")
			expectedID, _ = wikipage.ProfileIdentifierFor("bob.smith@example.com")
			// Profile page does not exist (default behavior of fake).
			// Template page is present and properly flagged.
			mutator.frontmatter[profileTemplateIdentifier] = wikipage.FrontMatter{
				"identifier": "profile_template",
				"title":      "Profile Template",
				"wiki": map[string]any{
					"template": true,
					"system":   true,
				},
			}
			mutator.markdown[profileTemplateIdentifier] = wikipage.Markdown("# {{.Title}}\n\nHello.\n")
			c, r := makeContext(user)
			rec = r
			resolveAndRedirectToProfile(c, user, mutator, index)
		})

		It("should redirect 302 to the new profile page", func() {
			Expect(rec.Code).To(Equal(http.StatusFound))
			Expect(rec.Header().Get("Location")).To(Equal("/" + string(expectedID) + "/view"))
		})

		It("should write the new profile frontmatter with the user's identifier and title", func() {
			fm, ok := mutator.writtenFM[expectedID]
			Expect(ok).To(BeTrue())
			Expect(fm["identifier"]).To(Equal(string(expectedID)))
			Expect(fm["title"]).To(Equal("Profile: bob.smith@example.com"))
		})

		It("should not carry the template's wiki.template / wiki.system flags into the new page", func() {
			fm, ok := mutator.writtenFM[expectedID]
			Expect(ok).To(BeTrue())
			wikiSubtree, ok := fm["wiki"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(wikiSubtree).NotTo(HaveKey("template"))
			Expect(wikiSubtree).NotTo(HaveKey("system"))
		})

		It("should write the rendered markdown body", func() {
			md, ok := mutator.writtenMD[expectedID]
			Expect(ok).To(BeTrue())
			Expect(string(md)).To(ContainSubstring("Profile: bob.smith@example.com"))
		})

		It("should stamp wiki.authorization.acl.owner = login on the new page", func() {
			fm, ok := mutator.writtenFM[expectedID]
			Expect(ok).To(BeTrue())
			wikiSubtree, ok := fm["wiki"].(map[string]any)
			Expect(ok).To(BeTrue(), "wiki subtree should be present")
			authSubtree, ok := wikiSubtree["authorization"].(map[string]any)
			Expect(ok).To(BeTrue(), "wiki.authorization subtree should be present")
			aclSubtree, ok := authSubtree["acl"].(map[string]any)
			Expect(ok).To(BeTrue(), "wiki.authorization.acl subtree should be present")
			Expect(aclSubtree["owner"]).To(Equal("bob.smith@example.com"))
		})

		It("should default wiki.authorization.allow_agent_access to false on the new page", func() {
			fm, ok := mutator.writtenFM[expectedID]
			Expect(ok).To(BeTrue())
			wikiSubtree, ok := fm["wiki"].(map[string]any)
			Expect(ok).To(BeTrue())
			authSubtree, ok := wikiSubtree["authorization"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(authSubtree["allow_agent_access"]).To(Equal(false))
		})
	})

	When("the profile page is missing and the template page is missing too", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			user := tailscale.NewIdentity("ghost@example.com", "Ghost", "node")
			c, r := makeContext(user)
			rec = r
			resolveAndRedirectToProfile(c, user, mutator, index)
		})

		It("should respond 500", func() {
			Expect(rec.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should mention the template in the error", func() {
			Expect(rec.Body.String()).To(ContainSubstring("profile_template"))
		})
	})

	When("the profile page is missing and the template lacks wiki.template = true", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			user := tailscale.NewIdentity("user@example.com", "User", "node")
			mutator.frontmatter[profileTemplateIdentifier] = wikipage.FrontMatter{
				"identifier": "profile_template",
				"title":      "Profile Template (broken)",
				// no wiki.template flag
			}
			mutator.markdown[profileTemplateIdentifier] = wikipage.Markdown("body")
			c, r := makeContext(user)
			rec = r
			resolveAndRedirectToProfile(c, user, mutator, index)
		})

		It("should respond 500", func() {
			Expect(rec.Code).To(Equal(http.StatusInternalServerError))
		})

		It("should explain the missing template flag", func() {
			Expect(rec.Body.String()).To(ContainSubstring("wiki.template"))
		})
	})

	When("the profile page read errors with something other than not-found", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			user := tailscale.NewIdentity("user@example.com", "User", "node")
			erroringMutator := &readErrorMutator{permissionErr: errors.New("permission denied")}
			c, r := makeContext(user)
			rec = r
			resolveAndRedirectToProfile(c, user, erroringMutator, index)
		})

		It("should respond 500 (no silent assumption that any error is not-found)", func() {
			Expect(rec.Code).To(Equal(http.StatusInternalServerError))
		})
	})
})

// readErrorMutator returns a non-not-found error for every ReadFrontMatter
// call, exercising the discrimination requirement from CLAUDE.md.
type readErrorMutator struct {
	permissionErr error
}

func (m *readErrorMutator) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	return id, nil, m.permissionErr
}
func (*readErrorMutator) ReadMarkdown(_ wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	return "", "", nil
}
func (*readErrorMutator) WriteFrontMatter(_ wikipage.PageIdentifier, _ wikipage.FrontMatter) error {
	return nil
}
func (*readErrorMutator) WriteMarkdown(_ wikipage.PageIdentifier, _ wikipage.Markdown) error {
	return nil
}
func (*readErrorMutator) DeletePage(_ wikipage.PageIdentifier) error { return nil }
func (*readErrorMutator) ModifyMarkdown(_ wikipage.PageIdentifier, _ func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	return nil
}
