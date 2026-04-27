package wikipage

import "errors"

// ErrForbidden is the sentinel error returned by Authorize when the caller
// is not permitted to access the page. API-surface wrappers translate this
// into the right transport-level status (HTTP 403 / gRPC PermissionDenied).
var ErrForbidden = errors.New("authorization denied by wiki.authorization rules")

// Identity is the minimal view of a caller's identity that Authorize needs.
// `tailscale.IdentityValue` satisfies it structurally, so callers pass the
// identity straight through. Defined here (rather than imported from the
// tailscale package) to keep wikipage free of inbound dependencies on
// upper-layer packages — the existing observability → wikipage edge would
// otherwise close a cycle.
type Identity interface {
	LoginName() string
	IsAgent() bool
	IsAnonymous() bool
}

// Authorize reports whether identity is allowed to read or write a page
// whose frontmatter is fm.
//
// Pages with no `wiki.authorization` subtree are public — every caller
// (human or agent, including anonymous) is allowed. The wiki has historically
// been default-public; this preserves that behavior for every existing page.
//
// Pages with a `wiki.authorization` subtree are gated by two orthogonal
// dimensions:
//
//   - Humans: a page with `wiki.authorization.acl.owner` set is restricted
//     to that owner. A page with no acl (or no acl.owner) is open to any
//     authenticated human. Anonymous callers are humans without a login,
//     so they cannot match a specific owner.
//   - Agents: governed entirely by `wiki.authorization.allow_agent_access`.
//     The flag is independent of the human acl — agent access does not
//     fall through to the owner check.
//
// Returns nil when allowed, ErrForbidden when denied. Internal callers
// that must bypass authorization (syspage.Sync, eager migrations, the
// indexer, etc.) simply do not invoke Authorize at all.
func Authorize(identity Identity, fm FrontMatter) error {
	auth, hasAuth := readAuthorizationSubtree(fm)
	if !hasAuth {
		// Default-public: pages without wiki.authorization are open to all.
		return nil
	}

	if identity.IsAgent() {
		if auth.AllowAgentAccess {
			return nil
		}
		return ErrForbidden
	}

	// Human path. The acl gates non-agent callers; an empty owner means
	// no specific gate, so any authenticated human passes.
	owner := auth.ACLOwner
	if owner == "" {
		if identity.IsAnonymous() {
			// No owner gate, but the page has explicitly opted into the
			// authorization regime — anonymous callers cannot pass because
			// they have no login by which the wiki could ever attribute
			// their access.
			return ErrForbidden
		}
		return nil
	}
	if !identity.IsAnonymous() && identity.LoginName() == owner {
		return nil
	}
	return ErrForbidden
}

// authorizationSubtree is a flattened view of wiki.authorization for the
// authorization decision. We don't reuse templating.WikiAuthorization here
// to keep wikipage free of upstream dependencies.
type authorizationSubtree struct {
	ACLOwner         string
	AllowAgentAccess bool
}

// readAuthorizationSubtree pulls the wiki.authorization subtree out of fm.
// The bool return distinguishes "not present" from "present but empty"
// because those two cases have different default-public semantics.
func readAuthorizationSubtree(fm FrontMatter) (authorizationSubtree, bool) {
	wikiSubtree, ok := fm["wiki"].(map[string]any)
	if !ok {
		return authorizationSubtree{}, false
	}
	authSubtree, ok := wikiSubtree["authorization"].(map[string]any)
	if !ok {
		return authorizationSubtree{}, false
	}
	out := authorizationSubtree{}
	if aclSubtree, ok := authSubtree["acl"].(map[string]any); ok {
		if owner, ok := aclSubtree["owner"].(string); ok {
			out.ACLOwner = owner
		}
	}
	if allow, ok := authSubtree["allow_agent_access"].(bool); ok {
		out.AllowAgentAccess = allow
	}
	return out, true
}
