package connectors

import (
	"context"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// BackendAdapter is the contract every connector's per-tick
// implementation MUST honor. Per ADR-0015 + the user's directive
// (2026-05-03, "make it a required part of the interface"):
// structural primitives that all backends need to implement live
// here as a single interface, so adding a new backend (iCloud
// Reminders, etc.) without implementing every method is a compile
// error rather than a "Tasks forgot to do what Keep does"
// behavior gap.
//
// Each connector type asserts compliance via:
//
//	var _ connectors.BackendAdapter = (*Connector)(nil)
//
// Adding a method to BackendAdapter is therefore a focused way to
// require new behavior across every backend. Each connector's
// per-tick orchestration calls these methods (until the SyncEngine
// extraction lands and drives them centrally).
//
// The interface is deliberately small — only methods that are
// genuinely cross-backend belong here. Adapter-specific primitives
// (Tasks's PatchTask, Keep's Changes call) stay in their own
// packages.
type BackendAdapter interface {
	// FetchRemoteListTitle returns the current display title of the
	// remote list/note bound to the given remote handle. The cloud
	// is authoritative for the display name once bound; failing to
	// honor a rename is a parity bug, hence the interface
	// requirement.
	//
	// Per-backend implementation:
	//   - Google Keep: walks the latest pull's LIST nodes for the
	//     bound serverID and reads the title field. The connector
	//     caches this so the call is a map lookup, not a network
	//     round-trip.
	//   - Google Tasks: calls tasklists.list and matches by ID.
	//     ~10–20 entries for a typical household user, one HTTP
	//     call per tick (cheap).
	//   - iCloud Reminders (future): CalDAV PROPFIND on the
	//     collection's <displayname>.
	//
	// Returns:
	//   - title: the remote's current name. May be empty if the
	//     backend reported an empty title (caller decides whether
	//     to overwrite a non-empty cached title with empty).
	//   - ok: true if a fresh title was successfully observed this
	//     tick. False on transient API failure, list-not-found,
	//     or "no fresh data this tick" — caller preserves the
	//     prior title.
	//   - err: non-nil only on auth/permission failures the
	//     caller should surface. Transient errors return
	//     ("", false, nil) so a title-fetch hiccup doesn't fail
	//     an otherwise-successful tick.
	FetchRemoteListTitle(ctx context.Context, profileID wikipage.PageIdentifier, remoteListHandle string) (title string, ok bool, err error)
}
