package server

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/templating"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/gin-gonic/gin"
)

// profileTemplateIdentifier is the page identifier of the system-shipped
// template used to seed brand-new profile pages. The file lives at
// internal/syspage/embedded/profile_template.md and is synced to the page
// store on startup by internal/syspage.Sync.
const profileTemplateIdentifier wikipage.PageIdentifier = "profile_template"

// profileNotAuthenticatedMessage is the 403 body returned to anonymous and
// agent callers. Profile pages are scoped to real human users.
const profileNotAuthenticatedMessage = "profile pages require an authenticated user"

// handleProfile resolves the current user's identity and redirects to their
// personal profile page. The page is auto-created on first visit from the
// shipped profile_template system page.
//
// Anonymous callers and the Tailscale agent identity are rejected with 403:
// profile pages are scoped to real human users and there is no meaningful
// "agent profile."
//
// See internal/syspage/embedded/help_profile.md for user-facing docs.
func (s *Site) handleProfile(c *gin.Context) {
	identity := tailscale.IdentityFromContext(c.Request.Context())
	resolveAndRedirectToProfile(c, identity, s, s.FrontmatterIndexQueryer)
}

// resolveAndRedirectToProfile is the testable core of handleProfile. It is
// a free function so unit tests can drive it with fake mutator + query
// implementations rather than constructing a full Site.
func resolveAndRedirectToProfile(
	c *gin.Context,
	identity tailscale.IdentityValue,
	mutator wikipage.PageReaderMutator,
	query wikipage.IQueryFrontmatterIndex,
) {
	if identity.IsAnonymous() || identity.IsAgent() {
		c.String(http.StatusForbidden, profileNotAuthenticatedMessage)
		return
	}

	id, err := wikipage.ProfileIdentifierFor(identity.LoginName())
	if err != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("could not derive profile identifier: %v", err))
		return
	}

	if _, _, readErr := mutator.ReadFrontMatter(id); readErr == nil {
		c.Redirect(http.StatusFound, "/"+string(id)+"/view")
		return
	} else if !errors.Is(readErr, os.ErrNotExist) {
		c.String(http.StatusInternalServerError, fmt.Sprintf("failed to read profile page %q: %v", string(id), readErr))
		return
	}

	if createErr := createProfileFromTemplate(id, identity.LoginName(), mutator, query); createErr != nil {
		c.String(http.StatusInternalServerError, fmt.Sprintf("failed to create profile page %q: %v", string(id), createErr))
		return
	}

	c.Redirect(http.StatusFound, "/"+string(id)+"/view")
}

// createProfileFromTemplate materializes a profile page at id by cloning
// frontmatter from the profile_template system page (excluding the
// reserved-namespace flags via wikipage.ApplyTemplate) and rendering its
// markdown body through the existing template engine.
func createProfileFromTemplate(
	id wikipage.PageIdentifier,
	login string,
	mutator wikipage.PageReaderMutator,
	query wikipage.IQueryFrontmatterIndex,
) error {
	_, templateFm, err := mutator.ReadFrontMatter(profileTemplateIdentifier)
	if err != nil {
		return fmt.Errorf("read profile_template frontmatter: %w", err)
	}
	if !wikipage.IsTemplatePage(templateFm) {
		return fmt.Errorf("profile_template is missing wiki.template = true; the system page sync may not have run yet")
	}
	_, templateMd, err := mutator.ReadMarkdown(profileTemplateIdentifier)
	if err != nil {
		return fmt.Errorf("read profile_template markdown: %w", err)
	}

	newFm := wikipage.FrontMatter{}
	wikipage.ApplyTemplate(newFm, templateFm)
	newFm[wikipage.IdentifierKey] = string(id)
	newFm["title"] = "Profile: " + login

	rendered, err := templating.ExecuteTemplate(string(templateMd), newFm, mutator, query)
	if err != nil {
		return fmt.Errorf("render profile_template body: %w", err)
	}

	if err := mutator.WriteFrontMatter(id, newFm); err != nil {
		return fmt.Errorf("write new profile frontmatter: %w", err)
	}
	if err := mutator.WriteMarkdown(id, wikipage.Markdown(rendered)); err != nil {
		return fmt.Errorf("write new profile markdown: %w", err)
	}
	return nil
}
