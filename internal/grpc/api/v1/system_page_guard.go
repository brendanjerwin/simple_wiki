package v1

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// systemPageRedirect is the message users see when they try to mutate a
// system page through the public API. It points them at the source repo so
// the change can be made in a way that survives upgrades.
const systemPageRedirect = "page %q is a system page and cannot be edited via the API; it is sourced from the wiki binary — open an issue or pull request upstream to propose changes"

// requireUserMutable returns nil when the page is a normal user-owned page,
// or a `FailedPrecondition` status when the page is a system page (sourced
// from the embedded help corpus). Pages that don't yet exist are treated as
// user-mutable so first-time creates aren't blocked.
//
// On any other read error the guard fails closed: it returns a gRPC error
// rather than allowing the mutation to proceed. The alternative — letting
// the underlying handler observe the same transient failure and decide —
// risks overwriting a system page if a future handler implementation skips
// re-reading the frontmatter.
//
// Internal startup writes (via syspage.Sync) go through Site directly and
// bypass this guard — the guard lives only on the public gRPC surface.
func requireUserMutable(reader wikipage.PageReader, id wikipage.PageIdentifier) error {
	_, fm, err := reader.ReadFrontMatter(id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return status.Errorf(codes.Internal, "failed to read frontmatter for system-page check on %q: %v", string(id), err)
	}

	if wikipage.IsSystemPage(fm) {
		return status.Error(codes.FailedPrecondition, fmt.Sprintf(systemPageRedirect, string(id)))
	}
	return nil
}

// requireAuthorized enforces the page's wiki.authorization rules against
// the caller's identity (extracted from ctx). Pages that don't yet exist
// are treated as authorized — first-creator wins, and the new page's
// authorization stamps in via the write itself.
//
// Translates wikipage.ErrForbidden into gRPC PermissionDenied; any other
// read error fails closed with Internal.
//
// Internal callers (syspage.Sync, eager migrations, the indexer) bypass
// this guard by going through Site directly rather than through the gRPC
// surface.
func requireAuthorized(ctx context.Context, reader wikipage.PageReader, id wikipage.PageIdentifier) error {
	_, fm, err := reader.ReadFrontMatter(id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return status.Errorf(codes.Internal, "failed to read frontmatter for authorization check on %q: %v", string(id), err)
	}

	identity := tailscale.IdentityFromContext(ctx)
	if authErr := wikipage.Authorize(identity, fm); authErr != nil {
		if errors.Is(authErr, wikipage.ErrForbidden) {
			return status.Errorf(codes.PermissionDenied, "access to %q denied by wiki.authorization rules", string(id))
		}
		return status.Errorf(codes.Internal, "authorization check on %q failed: %v", string(id), authErr)
	}
	return nil
}
