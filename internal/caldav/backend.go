package caldav

import (
	"context"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/caldav/etag"
	"github.com/brendanjerwin/simple_wiki/internal/caldav/icalcodec"
	"github.com/brendanjerwin/simple_wiki/server/checklistmutator"
	"github.com/brendanjerwin/simple_wiki/tailscale"
)

// Errors returned by CalendarBackend implementations. The HTTP layer
// maps these to the appropriate CalDAV status codes.
var (
	// ErrCollectionNotFound is returned when the named (page, list)
	// pair does not exist on the wiki.
	ErrCollectionNotFound = errors.New("caldav: collection not found")
	// ErrItemNotFound is returned when the requested uid does not
	// exist in the named collection.
	ErrItemNotFound = errors.New("caldav: item not found")
	// ErrItemDeleted is returned when the requested uid is in the
	// tombstone list (HTTP 404 to clients; sync-collection emits the
	// uid in the deleted set instead).
	ErrItemDeleted = errors.New("caldav: item deleted")
	// ErrPreconditionFailed is returned when an If-Match or
	// If-None-Match precondition on PutItem/DeleteItem does not hold.
	// The HTTP layer maps this to 412 Precondition Failed.
	ErrPreconditionFailed = errors.New("caldav: precondition failed")
	// ErrInvalidBody is returned when a PUT body cannot be parsed as
	// a single VTODO (no VTODO, multiple VTODOs, missing UID, or any
	// other decoder error). The HTTP layer maps this to 400.
	ErrInvalidBody = errors.New("caldav: invalid request body")
	// ErrUIDMismatch is returned when the UID in the PUT body does
	// not match the uid in the request path. The HTTP layer maps
	// this to 400.
	ErrUIDMismatch = errors.New("caldav: path uid does not match body UID")
	// ErrDescriptionTooLarge is returned when a PUT body's
	// DESCRIPTION exceeds icalcodec.DescriptionMaxBytes. The HTTP
	// layer maps this to 413 Payload Too Large.
	ErrDescriptionTooLarge = errors.New("caldav: description too large")
	// ErrInvalidSyncToken is returned by SyncCollection when the
	// caller-supplied sync-token cannot be parsed (wrong prefix, non-
	// integer suffix, or otherwise malformed). The HTTP layer maps
	// this to 403 Forbidden with a `<DAV:valid-sync-token/>`
	// precondition element per RFC 6578 §3.2 so the client knows to
	// drop its state and replay an initial full sync.
	ErrInvalidSyncToken = errors.New("caldav: invalid sync token")
)

// CalendarCollection describes a single (page, list) pair. The HTTP
// PROPFIND handler maps this onto the WebDAV multistatus response.
type CalendarCollection struct {
	Page        string
	ListName    string
	DisplayName string
	UpdatedAt   time.Time
	SyncToken   string
	CTag        string
}

// CalendarItem describes a single VTODO resource. ICalBytes holds the
// pre-rendered iCalendar body so the HTTP layer can serve GET and
// embed calendar-data in PROPFIND/REPORT responses without re-running
// the codec.
type CalendarItem struct {
	UID       string
	ETag      string
	UpdatedAt time.Time
	CreatedAt time.Time
	ICalBytes []byte
}

// CalendarBackend is the boundary between the CalDAV protocol layer
// and the wiki's storage. Implementations are expected to:
//
//   - Resolve `(page, list)` to a CalendarCollection (or
//     ErrCollectionNotFound).
//   - Render every item as an iCalendar VTODO (typically via
//     icalcodec.RenderItem) so the HTTP layer can serve bytes.
//   - Pass writes through checklistmutator.Mutator so sync_token,
//     tombstones, and attribution stay correct.
//
// All methods take context.Context for cancellation and tracing. The
// HTTP layer extracts identity via tailscale.IdentityFromContext.
type CalendarBackend interface {
	// ListCollections returns every checklist on the named page.
	// Used by PROPFIND on the home-set URL with Depth:1.
	ListCollections(ctx context.Context, page string) ([]CalendarCollection, error)

	// GetCollection returns a single (page, list) pair. Used by
	// PROPFIND with Depth:0.
	GetCollection(ctx context.Context, page, listName string) (CalendarCollection, error)

	// ListItems returns the collection metadata plus every live item
	// in the collection. Used by PROPFIND Depth:1 on a collection
	// URL and by REPORT calendar-query without filters.
	ListItems(ctx context.Context, page, listName string) (CalendarCollection, []CalendarItem, error)

	// GetItem returns a single item. Returns ErrItemDeleted when uid
	// is in the tombstone list, ErrItemNotFound when uid is unknown.
	GetItem(ctx context.Context, page, listName, uid string) (CalendarItem, error)

	// PutItem creates or updates an item from an iCalendar body. The
	// returned ETag is the new per-item ETag the client should
	// remember. ifMatch and ifNoneMatch enforce RFC 7232 / 4918
	// preconditions; pass empty strings for "no precondition".
	PutItem(ctx context.Context, page, listName, uid string, body []byte, ifMatch, ifNoneMatch string, identity tailscale.IdentityValue) (newETag string, created bool, err error)

	// DeleteItem removes an item, writing a tombstone for sync-
	// collection replay. ifMatch enforces RFC 7232 preconditions.
	DeleteItem(ctx context.Context, page, listName, uid, ifMatch string, identity tailscale.IdentityValue) error

	// SyncCollection returns items changed and uids deleted since
	// clientToken. Empty token means initial sync (return all live
	// items, no tombstones).
	SyncCollection(ctx context.Context, page, listName, clientToken string) (newToken string, changed []CalendarItem, deletedUIDs []string, err error)
}

// MutatorBackend is the read+write surface defaultBackend depends on.
// Defined as an interface (rather than *checklistmutator.Mutator
// directly) so tests can inject a fake that doesn't need a real
// page-store. Production callers pass *checklistmutator.Mutator —
// it satisfies this interface implicitly.
type MutatorBackend interface {
	GetChecklists(ctx context.Context, page string) ([]*apiv1.Checklist, error)
	ListItems(ctx context.Context, page, listName string) (*apiv1.Checklist, error)
	UpsertFromCalDAV(ctx context.Context, page, listName, uid string, args checklistmutator.UpsertFromCalDAVArgs, ifMatch, ifNoneMatch string, identity tailscale.IdentityValue) (*apiv1.ChecklistItem, *apiv1.Checklist, error)
	DeleteItem(ctx context.Context, page, listName, uid string, expectedUpdatedAt *time.Time, identity tailscale.IdentityValue) (*apiv1.Checklist, error)
}

// defaultBackend is the production CalendarBackend implementation. It
// renders items via icalcodec and routes mutations through the
// checklistmutator funnel.
//
// NowFn is exposed so tests can inject a fixed clock without touching
// the real time package. Production callers pass time.Now.
//
// BaseURL is the wiki's externally-visible base (e.g.
// "https://wiki.example.com"); used to build the URL property in
// rendered VTODOs.
type defaultBackend struct {
	mutator MutatorBackend
	baseURL string
	nowFn   func() time.Time
}

// NewBackend constructs a defaultBackend with the given dependencies.
// In production, mutator is a *checklistmutator.Mutator and baseURL is
// derived per-request from the gateway middleware (so it can honor
// X-Forwarded-Host etc); pass an empty string here and the gateway
// will swap in the real value.
func NewBackend(mutator MutatorBackend, baseURL string, nowFn func() time.Time) CalendarBackend {
	if nowFn == nil {
		nowFn = time.Now
	}
	return &defaultBackend{mutator: mutator, baseURL: baseURL, nowFn: nowFn}
}

// ListCollections returns every checklist on the named page as a
// CalendarCollection. Each entry's metadata (DisplayName, UpdatedAt,
// SyncToken, CTag) is derived from the checklist proto via the etag
// package so the values match what GetCollection / ListItems return.
func (b *defaultBackend) ListCollections(ctx context.Context, page string) ([]CalendarCollection, error) {
	checklists, err := b.mutator.GetChecklists(ctx, page)
	if err != nil {
		return nil, fmt.Errorf("caldav: list collections for page %q: %w", page, err)
	}
	if len(checklists) == 0 {
		return nil, nil
	}
	cols := make([]CalendarCollection, 0, len(checklists))
	for _, c := range checklists {
		cols = append(cols, collectionFromChecklist(page, c))
	}
	return cols, nil
}

// collectionFromChecklist maps a *apiv1.Checklist onto a
// CalendarCollection. Centralized so ListCollections, GetCollection,
// and ListItems all derive the metadata fields the same way.
func collectionFromChecklist(page string, c *apiv1.Checklist) CalendarCollection {
	col := CalendarCollection{
		Page:        page,
		ListName:    c.Name,
		DisplayName: c.Name,
		SyncToken:   etag.CollectionSyncToken(c),
		CTag:        etag.CollectionCTag(c),
	}
	if c.UpdatedAt != nil {
		col.UpdatedAt = c.UpdatedAt.AsTime()
	}
	return col
}

// GetCollection returns the metadata for a single (page, list) pair.
// Returns ErrCollectionNotFound when the page has no checklist with
// the requested name. The mutator's read-only ListItems returns an
// empty *apiv1.Checklist (Name set, UpdatedAt nil, no Items, no
// Tombstones) for both "page missing" and "list missing on the page";
// GetCollection treats that case uniformly as "not found".
func (b *defaultBackend) GetCollection(ctx context.Context, page, listName string) (CalendarCollection, error) {
	checklist, err := b.mutator.ListItems(ctx, page, listName)
	if err != nil {
		return CalendarCollection{}, fmt.Errorf("caldav: get collection %q/%q: %w", page, listName, err)
	}
	if !checklistExists(checklist) {
		return CalendarCollection{}, ErrCollectionNotFound
	}
	return collectionFromChecklist(page, checklist), nil
}

// checklistExists reports whether the mutator's ListItems result
// represents an actual checklist (vs the empty placeholder it returns
// for missing pages or unknown list names). A real checklist has
// either an UpdatedAt stamp, live items, or surviving tombstones.
func checklistExists(c *apiv1.Checklist) bool {
	if c == nil {
		return false
	}
	if c.UpdatedAt != nil {
		return true
	}
	if len(c.Items) > 0 || len(c.Tombstones) > 0 {
		return true
	}
	return false
}

// ListItems returns the collection metadata plus every live (non-
// tombstoned) item in the named collection. Each item is rendered to
// iCalendar bytes via icalcodec.RenderItem so the HTTP layer can serve
// GET / embed calendar-data without re-running the codec. Returns
// ErrCollectionNotFound when the page or list does not exist.
func (b *defaultBackend) ListItems(ctx context.Context, page, listName string) (CalendarCollection, []CalendarItem, error) {
	checklist, err := b.mutator.ListItems(ctx, page, listName)
	if err != nil {
		return CalendarCollection{}, nil, fmt.Errorf("caldav: list items %q/%q: %w", page, listName, err)
	}
	if !checklistExists(checklist) {
		return CalendarCollection{}, nil, ErrCollectionNotFound
	}

	col := collectionFromChecklist(page, checklist)
	items := make([]CalendarItem, 0, len(checklist.Items))
	for _, it := range checklist.Items {
		items = append(items, b.renderItem(it, page, listName))
	}
	return col, items, nil
}

// renderItem maps an *apiv1.ChecklistItem onto a CalendarItem,
// including the per-item ETag and the pre-rendered iCalendar bytes.
// Centralized so ListItems and GetItem agree on the representation.
func (b *defaultBackend) renderItem(item *apiv1.ChecklistItem, page, listName string) CalendarItem {
	ci := CalendarItem{
		UID:       item.Uid,
		ETag:      etag.ItemETag(item),
		ICalBytes: icalcodec.RenderItem(item, page, listName, b.baseURL, b.nowFn),
	}
	if item.UpdatedAt != nil {
		ci.UpdatedAt = item.UpdatedAt.AsTime()
	}
	if item.CreatedAt != nil {
		ci.CreatedAt = item.CreatedAt.AsTime()
	}
	return ci
}

// GetItem returns the iCalendar representation of a single item.
// Searches the live items first; on miss, consults the tombstone list
// so a recently-deleted uid surfaces as ErrItemDeleted (mapped to a
// 404 by the HTTP layer, but distinguishable from "never existed" so
// sync-collection can replay the deletion). Unknown uids return
// ErrItemNotFound.
func (b *defaultBackend) GetItem(ctx context.Context, page, listName, uid string) (CalendarItem, error) {
	checklist, err := b.mutator.ListItems(ctx, page, listName)
	if err != nil {
		return CalendarItem{}, fmt.Errorf("caldav: get item %q/%q/%q: %w", page, listName, uid, err)
	}
	item, deleted := findItemOrTombstone(checklist, uid)
	if deleted {
		return CalendarItem{}, ErrItemDeleted
	}
	if item == nil {
		return CalendarItem{}, ErrItemNotFound
	}
	return b.renderItem(item, page, listName), nil
}

// PutItem creates or updates a checklist item from an inbound CalDAV
// PUT body. The body is decoded via icalcodec.ParseVTODO, validated
// against the path uid, and routed through Mutator.UpsertFromCalDAV so
// the funnel does the OCC and attribution work atomically. Returns
// the new per-item ETag and whether the resource was created (HTTP 201
// vs 204) for the HTTP layer to translate into the response.
func (b *defaultBackend) PutItem(ctx context.Context, page, listName, uid string, body []byte, ifMatch, ifNoneMatch string, identity tailscale.IdentityValue) (string, bool, error) {
	parsed, err := icalcodec.ParseVTODO(body)
	if err != nil {
		if errors.Is(err, icalcodec.ErrDescriptionTooLarge) {
			return "", false, ErrDescriptionTooLarge
		}
		return "", false, ErrInvalidBody
	}
	if parsed.UID != uid {
		return "", false, ErrUIDMismatch
	}

	created, err := b.uidIsNew(ctx, page, listName, uid)
	if err != nil {
		return "", false, fmt.Errorf("caldav: put item %q/%q/%q: %w", page, listName, uid, err)
	}

	args := checklistmutator.UpsertFromCalDAVArgs{
		Text:         parsed.Text,
		Tags:         parsed.Tags,
		Description:  parsed.Description,
		Due:          parsed.Due,
		AlarmPayload: parsed.AlarmPayload,
		Checked:      parsed.Checked,
		CompletedAt:  parsed.CompletedAt,
		Created:      parsed.Created,
	}

	item, _, err := b.mutator.UpsertFromCalDAV(ctx, page, listName, uid, args, ifMatch, ifNoneMatch, identity)
	if err != nil {
		if status.Code(err) == codes.FailedPrecondition {
			return "", false, ErrPreconditionFailed
		}
		return "", false, fmt.Errorf("caldav: put item %q/%q/%q: %w", page, listName, uid, err)
	}
	return etag.ItemETag(item), created, nil
}

// uidIsNew reports whether a uid is unknown in the named (page, list).
// The PUT handler needs this to decide between HTTP 201 Created and
// HTTP 204 No Content on success. A "page/list does not exist yet"
// counts as new — the funnel will create the collection on demand.
func (b *defaultBackend) uidIsNew(ctx context.Context, page, listName, uid string) (bool, error) {
	checklist, err := b.mutator.ListItems(ctx, page, listName)
	if err != nil {
		return false, err
	}
	if checklist == nil {
		return true, nil
	}
	for _, it := range checklist.Items {
		if it.Uid == uid {
			return false, nil
		}
	}
	return true, nil
}

// DeleteItem removes an item from the named collection, writing a
// tombstone for sync-collection replay. When ifMatch is empty, the
// delete proceeds unconditionally. When ifMatch is non-empty, the
// backend resolves the current item, compares its ETag against ifMatch,
// and stamps expectedUpdatedAt onto the funnel call as a defense-in-
// depth OCC check (the funnel re-validates against its own snapshot).
//
// Tombstoned uids surface as ErrItemDeleted so the HTTP layer can
// distinguish "never existed" (ErrItemNotFound → 404) from "deleted
// recently" (also 404 to clients, but still in the sync-collection
// stream). FailedPrecondition from the funnel maps to
// ErrPreconditionFailed (412). Other funnel errors are wrapped.
func (b *defaultBackend) DeleteItem(ctx context.Context, page, listName, uid, ifMatch string, identity tailscale.IdentityValue) error {
	var expectedUpdatedAt *time.Time
	if ifMatch != "" {
		checklist, err := b.mutator.ListItems(ctx, page, listName)
		if err != nil {
			return fmt.Errorf("caldav: delete item %q/%q/%q: %w", page, listName, uid, err)
		}
		item, deleted := findItemOrTombstone(checklist, uid)
		if deleted {
			return ErrItemDeleted
		}
		if item == nil {
			return ErrItemNotFound
		}
		if etag.ItemETag(item) != ifMatch {
			return ErrPreconditionFailed
		}
		updated := item.UpdatedAt.AsTime()
		expectedUpdatedAt = &updated
	}

	if _, err := b.mutator.DeleteItem(ctx, page, listName, uid, expectedUpdatedAt, identity); err != nil {
		if errors.Is(err, checklistmutator.ErrItemNotFound) {
			return ErrItemNotFound
		}
		if status.Code(err) == codes.FailedPrecondition {
			return ErrPreconditionFailed
		}
		return fmt.Errorf("caldav: delete item %q/%q/%q: %w", page, listName, uid, err)
	}
	return nil
}

// findItemOrTombstone scans a checklist for uid, returning the live
// item when found, or signaling deleted=true when the uid is in the
// tombstone list. Both nil and false mean "uid is unknown".
func findItemOrTombstone(checklist *apiv1.Checklist, uid string) (*apiv1.ChecklistItem, bool) {
	if checklist == nil {
		return nil, false
	}
	for _, it := range checklist.Items {
		if it.Uid == uid {
			return it, false
		}
	}
	for _, t := range checklist.Tombstones {
		if t.Uid == uid {
			return nil, true
		}
	}
	return nil, false
}

// SyncCollection returns the items changed and uids deleted since the
// caller's last sync (RFC 6578). The wire flow:
//
//  1. Empty clientToken means "initial sync" — emit every live item
//     and no deletes (the client has nothing yet).
//  2. Non-empty clientToken parses to an int64 counter (see
//     etag.ParseSyncToken). A parse failure returns ErrInvalidSyncToken
//     so the HTTP layer can answer the report with the
//     `<DAV:valid-sync-token/>` precondition response.
//  3. If the named collection does not exist, return
//     ErrCollectionNotFound.
//  4. Build the changed slice. The proto has no per-item sync-token,
//     only a collection-level counter that bumps on every mutation.
//     Phase 3 takes a pragmatic approximation: when clientToken
//     parsed value differs from the collection's current sync_token,
//     emit every live item. This over-emits — a client that's only
//     missing one mutation gets the whole collection — but is correct
//     and trivial to reason about. A future phase can refine this by
//     tracking per-item sync-token if real-world traffic warrants it.
//  5. Build deletedUIDs:
//     - Initial sync (clientToken empty): no deletes — the client
//       has nothing to drop.
//     - Subsequent sync: emit tombstones whose SyncToken exceeds the
//       client's parsed counter. Legacy tombstones (SyncToken=0)
//       lack the Phase 0 stamping; emit them only when the client's
//       parsed counter is also 0.
//  6. Return the collection's current sync-token URI as newToken.
//
// revive:disable-next-line:max-control-nesting,function-result-limit  // CalendarBackend.SyncCollection's 4-return shape (newToken, changed, deleted, err) is part of the interface contract.
func (b *defaultBackend) SyncCollection(ctx context.Context, page, listName, clientToken string) (string, []CalendarItem, []string, error) { //nolint:revive // 4 returns are interface-mandated
	clientCounter, err := etag.ParseSyncToken(clientToken)
	if err != nil {
		return "", nil, nil, ErrInvalidSyncToken
	}
	checklist, err := b.mutator.ListItems(ctx, page, listName)
	if err != nil {
		return "", nil, nil, fmt.Errorf("caldav: sync collection %q/%q: %w", page, listName, err)
	}
	if !checklistExists(checklist) {
		return "", nil, nil, ErrCollectionNotFound
	}

	newToken := etag.CollectionSyncToken(checklist)
	changed := b.collectChanged(checklist, page, listName, clientToken, clientCounter)
	deletedUIDs := collectDeletedUIDs(checklist, clientToken, clientCounter)
	return newToken, changed, deletedUIDs, nil
}

// collectChanged returns the items the client should re-fetch given the
// supplied (clientToken, clientCounter) pair. See SyncCollection's doc
// comment for the over-emission rationale.
func (b *defaultBackend) collectChanged(checklist *apiv1.Checklist, page, listName, clientToken string, clientCounter int64) []CalendarItem {
	// Initial sync: emit every live item.
	if clientToken == "" {
		return b.renderAllItems(checklist, page, listName)
	}
	// Up-to-date: emit nothing.
	if clientCounter == checklist.SyncToken {
		return nil
	}
	// Behind: emit every live item (Phase 3 approximation).
	return b.renderAllItems(checklist, page, listName)
}

// renderAllItems renders every live item on the checklist into the
// CalendarItem shape the report handler emits. Used by both initial-
// sync and the "client is behind" branches of collectChanged.
func (b *defaultBackend) renderAllItems(checklist *apiv1.Checklist, page, listName string) []CalendarItem {
	if len(checklist.Items) == 0 {
		return nil
	}
	out := make([]CalendarItem, 0, len(checklist.Items))
	for _, it := range checklist.Items {
		out = append(out, b.renderItem(it, page, listName))
	}
	return out
}

// collectDeletedUIDs returns the tombstoned uids the client should
// drop from its local cache. The wire rule:
//
//   - Initial sync (empty clientToken): no deletes — the client has
//     nothing to drop.
//   - Subsequent sync: emit every tombstone whose SyncToken exceeds
//     the client's parsed counter. Legacy tombstones (SyncToken=0)
//     pre-date the Phase 0 stamping work; they only emit when the
//     client's token is itself the zero counter, which is the one
//     and only safe replay window.
func collectDeletedUIDs(checklist *apiv1.Checklist, clientToken string, clientCounter int64) []string {
	if clientToken == "" || len(checklist.Tombstones) == 0 {
		return nil
	}
	out := make([]string, 0, len(checklist.Tombstones))
	for _, t := range checklist.Tombstones {
		if shouldEmitTombstone(t, clientCounter) {
			out = append(out, t.GetUid())
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// shouldEmitTombstone applies the per-tombstone visibility rules for
// the subsequent-sync branch. A post-Phase-0 tombstone with SyncToken
// > clientCounter emits. Legacy tombstones (SyncToken=0) emit only
// when the client's parsed counter is also 0; on later incremental
// syncs we suppress them to avoid replaying every legacy delete on
// every poll.
func shouldEmitTombstone(t *apiv1.Tombstone, clientCounter int64) bool {
	if t == nil {
		return false
	}
	stamped := t.GetSyncToken()
	if stamped > 0 {
		return stamped > clientCounter
	}
	// Legacy (unstamped) tombstone: only emit when clientCounter is 0.
	return clientCounter == 0
}
