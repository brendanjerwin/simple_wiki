package bridge

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/keep/protocol"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// Clock returns the current time. SystemClock for production, fake for tests.
type Clock interface{ Now() time.Time }

// SystemClock returns time.Now wrapped in a Clock.
type SystemClock struct{}

// Now returns wall-clock time.
func (SystemClock) Now() time.Time { return time.Now() }

// AuthExchanger is the subset of protocol.Authenticator the connector uses.
// Stated as an interface so tests can substitute a fake without spinning up
// a real httptest server for every Connector test.
type AuthExchanger interface {
	ExchangeOAuthTokenForMasterToken(ctx context.Context, email, oauthToken string) (string, error)
	ExchangeMasterTokenForBearer(ctx context.Context, email, masterToken string) (string, error)
}

// KeepClientFactory creates a KeepClient given a bearer. Real factory just
// constructs *protocol.KeepClient; tests inject a stub.
type KeepClientFactory func(bearer string) KeepClient

// KeepClient is the subset of *protocol.KeepClient the connector calls.
type KeepClient interface {
	Changes(ctx context.Context, req protocol.ChangesRequest) (protocol.ChangesResponse, error)
	CreateList(ctx context.Context, title string) (string, error)
	CreateListWithItems(ctx context.Context, title string, items []protocol.ListItemSpec) (protocol.CreateListResult, error)
}

// ChecklistReader is the read-side of checklistmutator the connector
// needs for outbound sync. The real type lives in server/checklistmutator
// but pulling it here as an interface keeps bridge from depending on the
// server package — bootstrap injects the concrete reader at startup.
type ChecklistReader interface {
	ListItems(ctx context.Context, page, listName string) (*apiv1.Checklist, error)
}

// ChecklistMutator is the write-side counterpart used by inbound sync
// to apply Keep-side changes to the wiki. The mutator's notify hook
// MUST be suppressed for the sync window (via SyncDebouncer.Suppress)
// or these calls trigger another sync job and loop forever.
//
// Args and signatures mirror the real mutator's. The bridge package
// intentionally ignores per-call expectedUpdatedAt (last-writer-wins
// from Keep at sync time) and identity (sync runs as a system actor,
// not a user; the wiki captures completed_by from a separate path).
type ChecklistMutator interface {
	AddItemForSync(ctx context.Context, page, listName, text string, checked bool, tags []string, description, sortValueHint string) (string, error)
	UpdateItemForSync(ctx context.Context, page, listName, uid, text string, checked bool, tags []string, description string) error
	DeleteItemForSync(ctx context.Context, page, listName, uid string) error
}

// Connector orchestrates the Keep bridge: per-user auth exchange,
// verification, binding management, and Keep client construction. Owns no
// long-running goroutines (those live in the scheduler) — every method
// completes in the caller's context.
type Connector struct {
	store         *BindingStore
	httpClient    *http.Client
	clock         Clock
	debug         protocol.DebugLogger
	checklistR    ChecklistReader
	checklistW    ChecklistMutator
	suppressor    SyncSuppressor
	enqueuer      JobEnqueuer
	authBuilder   func(deviceID string) AuthExchanger
	clientBuilder KeepClientFactory

	activeMu sync.Mutex
	active   *activeBindings
}

// SetJobEnqueuer wires the queue Bind enqueues a sync job into. Same
// JobQueueCoordinator used by the cron tick and the SyncDebouncer, so
// every sync — bind, save, cron — flows through the same single-worker
// queue and can never race the targetVersion cursor against itself.
func (c *Connector) SetJobEnqueuer(e JobEnqueuer) { c.enqueuer = e }

// SyncSuppressor is the notify-suppression interface SyncDebouncer
// satisfies. SyncToKeep wraps its inbound-apply pass in
// Suppress/Unsuppress so mutator notifies during apply don't enqueue
// another sync.
type SyncSuppressor interface {
	Suppress(profileID wikipage.PageIdentifier, page, listName string)
	Unsuppress(profileID wikipage.PageIdentifier, page, listName string)
}

// SetChecklistMutator injects the wiki-side mutator the inbound sync
// path uses to apply Keep-originated changes. Optional; nil disables
// inbound apply (SyncToKeep still does outbound).
func (c *Connector) SetChecklistMutator(w ChecklistMutator) { c.checklistW = w }

// SetSyncSuppressor injects the suppressor used during inbound apply
// to keep mutator notifies from looping back as new sync triggers.
// Optional; nil falls back to "outbound only" behavior to avoid an
// infinite loop in mis-wired test setups.
func (c *Connector) SetSyncSuppressor(s SyncSuppressor) { c.suppressor = s }

// SetChecklistReader injects the wiki-side reader the outbound sync
// path uses to diff wiki state against the binding's id_map. Pass at
// startup; nil disables outbound sync (SyncToKeep returns an error).
func (c *Connector) SetChecklistReader(r ChecklistReader) { c.checklistR = r }

// SetDebugLogger attaches a debug logger that the underlying KeepClient
// will use to dump response bodies on each Changes call. Production
// wires the wiki's lumber logger through here to chase response-shape
// regressions; pass nil to silence.
func (c *Connector) SetDebugLogger(l protocol.DebugLogger) { c.debug = l }

// NewConnector wires the production dependencies. Tests construct a
// Connector directly with stubbed builders.
//
// The auth-side http.Client forces HTTP/1.1 (no h2 ALPN advertisement)
// because gpsoauth's auth endpoint at android.clients.google.com/auth
// returns 403 Bad Authentication when an h2 ALPN protocol is offered.
// The Python gpsoauth library applies the same quirk via a custom
// HTTPAdapter; mirroring it here.
func NewConnector(store *BindingStore, httpClient *http.Client, clock Clock) *Connector {
	authClient := newAuthHTTPClient()
	c := &Connector{
		store:      store,
		httpClient: httpClient,
		clock:      clock,
		authBuilder: func(deviceID string) AuthExchanger {
			return protocol.NewAuthenticator(authClient, protocol.AuthURL, deviceID)
		},
	}
	c.clientBuilder = func(bearer string) KeepClient {
		kc := protocol.NewKeepClient(httpClient, protocol.DefaultKeepBaseURL, bearer)
		if c.debug != nil {
			kc.SetDebugLogger(c.debug)
		}
		return kc
	}
	return c
}

// newAuthHTTPClient returns an http.Client that mirrors the TLS quirks
// the Python gpsoauth library applies — specifically, do not advertise
// h2 in TLS ALPN. Google's /auth endpoint returns 403 (sometimes
// surfaced as the body-level "BadAuthentication") when it sees h2.
func newAuthHTTPClient() *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"http/1.1"},
		},
		ForceAttemptHTTP2: false,
	}
	return &http.Client{Transport: transport, Timeout: 30 * time.Second}
}

// Connect performs the full connect flow: oauth_token → master token →
// bearer → verify with a Keep changes call → store. The oauth_token is
// captured by the user from accounts.google.com after signing in via
// /EmbeddedSetup; see help_google_keep for instructions. On any failure,
// no state is written.
func (c *Connector) Connect(ctx context.Context, profileID wikipage.PageIdentifier, email, oauthToken string) (ConnectorState, error) {
	if email == "" {
		return ConnectorState{}, errors.New("keep bridge: email is required")
	}
	if oauthToken == "" {
		return ConnectorState{}, errors.New("keep bridge: oauth_token is required")
	}

	auth := c.authBuilder(deriveDeviceID(profileID))
	masterToken, err := auth.ExchangeOAuthTokenForMasterToken(ctx, email, oauthToken)
	if err != nil {
		return ConnectorState{}, err
	}
	bearer, err := auth.ExchangeMasterTokenForBearer(ctx, email, masterToken)
	if err != nil {
		return ConnectorState{}, err
	}
	client := c.clientBuilder(bearer)
	// Verify with a no-mutation pull. Empty TargetVersion = full pull, but
	// we only need shape — discard the response body.
	if _, err := client.Changes(ctx, protocol.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--verify", c.clock.Now().UnixMilli()),
		ClientTimestamp: c.clock.Now().UTC().Format("2006-01-02T15:04:05.000000Z"),
	}); err != nil {
		return ConnectorState{}, err
	}

	now := c.clock.Now().UTC()

	existing, err := c.store.LoadState(profileID)
	if err != nil {
		return ConnectorState{}, err
	}
	existing.Email = email
	existing.MasterToken = masterToken
	existing.ConnectedAt = now
	existing.LastVerifiedAt = now

	if err := c.store.SaveState(profileID, existing); err != nil {
		return ConnectorState{}, err
	}
	return existing, nil
}

// Disconnect wipes the master token from the calling user's profile but
// preserves the bindings list (paused). Reconnect resumes them.
func (c *Connector) Disconnect(_ context.Context, profileID wikipage.PageIdentifier) (ConnectorState, error) {
	state, err := c.store.LoadState(profileID)
	if err != nil {
		return ConnectorState{}, err
	}
	state.MasterToken = ""
	state.LastVerifiedAt = time.Time{}
	if err := c.store.SaveState(profileID, state); err != nil {
		return ConnectorState{}, err
	}
	return state, nil
}

// GetState returns the calling user's connector state.
func (c *Connector) GetState(_ context.Context, profileID wikipage.PageIdentifier) (ConnectorState, error) {
	return c.store.LoadState(profileID)
}

// keepClientFor refreshes a bearer for the user and returns an
// authenticated KeepClient. Used by ListNotes, Bind (when creating a new
// note), and the sync tick.
func (c *Connector) keepClientFor(ctx context.Context, profileID wikipage.PageIdentifier) (KeepClient, ConnectorState, error) {
	state, err := c.store.LoadState(profileID)
	if err != nil {
		return nil, ConnectorState{}, err
	}
	if !state.IsConfigured() {
		return nil, state, ErrConnectorNotConfigured
	}
	auth := c.authBuilder(deriveDeviceID(profileID))
	bearer, err := auth.ExchangeMasterTokenForBearer(ctx, state.Email, state.MasterToken)
	if err != nil {
		return nil, state, err
	}
	return c.clientBuilder(bearer), state, nil
}

// KeepNoteSummary is the shape ListNotes returns for the bind picker.
type KeepNoteSummary struct {
	KeepNoteID string
	Title      string
	ItemCount  int
}

// ListNotes enumerates the calling user's list-typed Keep notes. Used to
// populate the bind picker. Item counts are best-effort — they reflect
// whatever the most recent changes pull surfaced.
func (c *Connector) ListNotes(ctx context.Context, profileID wikipage.PageIdentifier) ([]KeepNoteSummary, error) {
	client, _, err := c.keepClientFor(ctx, profileID)
	if err != nil {
		return nil, err
	}
	now := c.clock.Now()
	resp, err := client.Changes(ctx, protocol.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--listnotes", now.UnixMilli()),
		ClientTimestamp: now.UTC().Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	for _, n := range resp.Nodes {
		if n.Type == protocol.NodeTypeListItem && n.ParentID != "" && n.Timestamps.Trashed.IsZero() && n.Timestamps.Deleted.IsZero() {
			counts[n.ParentID]++
		}
	}

	out := make([]KeepNoteSummary, 0)
	for _, n := range resp.Nodes {
		if n.Type != protocol.NodeTypeList {
			continue
		}
		if !n.Timestamps.Trashed.IsZero() || !n.Timestamps.Deleted.IsZero() {
			continue
		}
		out = append(out, KeepNoteSummary{
			KeepNoteID: n.ServerID,
			Title:      firstNonEmpty(n.Title, n.Text),
			ItemCount:  counts[n.ServerID],
		})
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// InitialItem is the input shape for items to push into a brand-new
// Keep note as part of Bind. Caller (the gRPC handler) reads them from
// the wiki checklist and forwards every field that survives a sync
// round trip; the bridge encodes tags + description into the Keep
// LIST_ITEM Text via the same encoder the cron-driven sync uses, so
// the initial-bind push and subsequent incremental syncs produce
// byte-identical wire shapes for the same wiki state.
//
// UID is the wiki ChecklistItem UID and is what the binding's
// ItemIDMap keys on after the bundled push completes — that's how
// future incremental syncs find "the Keep node corresponding to
// wiki item X."
type InitialItem struct {
	UID         string
	Text        string
	Tags        []string
	Description string
	Checked     bool
}

// Bind records a (page, listName) ↔ Keep-note binding for the calling
// user, then enqueues a SyncToKeep job. The actual data push (creating
// the Keep LIST when binding fresh, or reconciling against an existing
// LIST) happens inside SyncToKeep — not inline here. This is the
// "one method to sync" invariant: every trigger (bind, save, cron)
// converges on Connector.SyncToKeep.
//
// keepNoteID == "" means "create a new Keep note titled listName."
// SyncToKeep notices the empty keep_note_id and runs the bundled
// CreateListWithItems wire-protocol oddity (Keep rejects two-step
// create-list-then-push-items with 500). For an existing keepNoteID
// the binding is recorded with a base-text-match seed of the
// item_id_map so the first sync diff doesn't duplicate items
// already present on Keep.
func (c *Connector) Bind(ctx context.Context, profileID wikipage.PageIdentifier, page, listName, keepNoteID string, initialItems []InitialItem) (Binding, error) {
	idMap := map[string]string{}
	if keepNoteID != "" {
		client, _, err := c.keepClientFor(ctx, profileID)
		if err != nil {
			return Binding{}, err
		}
		idMap = c.seedIDMapFromExistingList(ctx, client, keepNoteID, initialItems)
	}

	binding := Binding{
		Page:          page,
		ListName:      listName,
		KeepNoteID:    keepNoteID,
		KeepNoteTitle: listName,
		BoundAt:       c.clock.Now().UTC(),
		ItemIDMap:     idMap,
	}
	if err := c.store.AddBinding(profileID, binding); err != nil {
		return Binding{}, err
	}
	c.noteBindingAdded(BindingKey{ProfileID: profileID, Page: page, ListName: listName})

	// Enqueue the unified sync — same job the cron tick and the
	// SyncDebouncer enqueue. SyncToKeep reads wiki state, creates the
	// Keep LIST if needed (fresh-bind case), pushes any item diff,
	// pulls Keep state and applies it back, and persists the binding's
	// updated id_map. We don't run it inline because the bind RPC
	// should return promptly; the queue worker will pick this up
	// within milliseconds.
	if c.enqueuer != nil {
		_ = c.enqueuer.EnqueueJob(NewKeepOutboundSyncJob(c, profileID, page, listName))
	}
	return binding, nil
}

// sortValueGap is the spacing we leave between adjacent initial items'
// Keep sort values so future inserts can land between them without a
// global re-numbering. 1000 is gkeepapi's gap of choice.
const sortValueGap = 1000

// keepSessionIDOffset and keepSessionIDSpan together produce a 10-digit
// session-id suffix in [1000000000, 9999999999], matching gkeepapi.
// Pulled out so the magic numbers don't repeat in generatePushSessionID.
const (
	keepSessionIDOffset = 1000000000
	keepSessionIDSpan   = 9000000000
)

// ErrChecklistReaderUnavailable is returned by SyncToKeep when the
// connector wasn't given a ChecklistReader at startup. Indicates a
// wiring bug in bootstrap; callers should fail loudly so it gets
// noticed rather than silently dropping sync events.
var ErrChecklistReaderUnavailable = errors.New("keep bridge: checklist reader not configured (bootstrap bug)")

// SyncToKeep diffs the wiki checklist (page, listName) against the
// bound Keep note and pushes the delta in a single Changes request.
//
// Algorithm:
//
//  1. Load binding + wiki items.
//  2. Pull Keep to acquire a fresh targetVersion (Keep rejects pushes
//     with empty targetVersion as 500 "Unknown Error" — proven via
//     keep-debug round trip).
//  3. Build the push:
//     - wiki uid found in ItemIDMap → update existing Keep node by
//       serverID (sets both ParentID and ParentServerID; required by
//       Keep on incremental edits).
//     - wiki uid absent from ItemIDMap → fresh push with new client_id.
//     - ItemIDMap entry whose uid no longer exists wiki-side → push
//       Trashed timestamp = now (Keep's soft-delete protocol).
//  4. POST one Changes request.
//  5. Walk the response; collect new uid → serverID mappings for items
//     pushed brand-new; remove map entries for trashed items (the
//     Keep response will echo them back with a non-zero Trashed).
//  6. Persist updated binding. Last-write-wins per binding via the
//     store's per-profile mutex.
//
// Conflict handling for v1 is "we win": if Keep has changes the wiki
// hasn't seen yet, our push still goes through (Keep's CRDT-ish backend
// merges by node). The bidirectional reconcile that preserves Keep-side
// edits is the inbound-sync follow-up (#1007).
func (c *Connector) SyncToKeep(ctx context.Context, profileID wikipage.PageIdentifier, page, listName string) error {
	if c.checklistR == nil {
		return ErrChecklistReaderUnavailable
	}

	binding, found, err := c.store.FindBinding(profileID, page, listName)
	if err != nil {
		return fmt.Errorf("load binding: %w", err)
	}
	if !found {
		// No binding for this checklist — nothing to sync. Not an error;
		// the mutator hook fires on every checklist edit and shouldn't
		// require pre-checking whether a binding exists.
		return nil
	}

	checklist, err := c.checklistR.ListItems(ctx, page, listName)
	if err != nil {
		return fmt.Errorf("read wiki checklist: %w", err)
	}

	client, _, err := c.keepClientFor(ctx, profileID)
	if err != nil {
		return err
	}

	now := c.clock.Now().UTC()

	// Fresh-bind branch: binding has no Keep serverID yet. Use the
	// bundled CreateListWithItems path (Keep rejects two-step
	// create-list + push-items with 500 Unknown Error). After this,
	// the binding is fully initialized and subsequent syncs go down
	// the steady-state pull/apply/push path below.
	if binding.KeepNoteID == "" {
		return c.bootstrapKeepListForBinding(ctx, profileID, binding, checklist, client, now)
	}

	pull, err := client.Changes(ctx, protocol.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--syncpull-%s", now.UnixMilli(), binding.KeepNoteID),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		return fmt.Errorf("sync pull: %w", err)
	}

	// Inbound apply: bring Keep-side changes into the wiki BEFORE
	// computing the outbound diff. Suppressed via the SyncSuppressor
	// so the mutator notifies emitted by AddItemForSync /
	// UpdateItemForSync / DeleteItemForSync don't loop back as fresh
	// sync triggers. updatedBinding carries any id_map changes (new
	// uid → serverID for items that arrived from Keep).
	updatedBinding, freshChecklist, err := c.applyInboundFromKeep(ctx, profileID, binding, pull, checklist)
	if err != nil {
		return fmt.Errorf("inbound apply: %w", err)
	}
	binding = updatedBinding
	checklist = freshChecklist

	// Resolve which Keep labels the bound LIST should carry. Reads the
	// host page's frontmatter tags and reconciles with the user's
	// existing Keep labels: existing names map by id; missing names
	// generate fresh Label CRUD entries that ride in the same push.
	pageTags, err := c.readPageTags(binding.Page)
	if err != nil {
		return fmt.Errorf("read page tags: %w", err)
	}
	labelPushEntries, listLabelIDs, err := resolveLabelsForTags(pageTags, pull.Labels, now)
	if err != nil {
		return fmt.Errorf("resolve labels: %w", err)
	}

	// Walk wiki items: classify each as fresh or update; track which
	// existing map entries we covered so we can soft-delete the rest.
	// baseVersions: serverID → BaseVersion captured from the just-
	// completed pull. Keep's optimistic-concurrency-control 500s
	// "Unknown Error" if a LIST_ITEM update is sent without the
	// baseVersion the server currently holds. gkeepapi sets it via
	// the loaded-from-server flow (node.py: self._base_version =
	// raw["baseVersion"]; emitted by Node.save()).
	baseVersions := make(map[string]string, len(pull.Nodes))
	// Also capture the original client_id for each server_id so
	// updates can carry the right `id` field (Keep distinguishes
	// `id` = client-generated stable ID, `serverId` = server-assigned).
	originalClientIDs := make(map[string]string, len(pull.Nodes))
	// And the Keep-side Updated timestamp per serverID — used by
	// shouldPushWikiUpdate to skip items where Keep's view is fresher
	// than wiki's. Symmetric to shouldPullKeepUpdate on the inbound
	// side. Without this gate every cron tick pushes EVERY wiki item,
	// which (combined with stale wiki state vs a just-edited Keep
	// item) reverts the user's phone-side edit on the next push.
	keepUpdated := make(map[string]time.Time, len(pull.Nodes))
	for _, n := range pull.Nodes {
		if n.ServerID == "" {
			continue
		}
		if n.BaseVersion != "" {
			baseVersions[n.ServerID] = n.BaseVersion
		}
		if n.ID != "" {
			originalClientIDs[n.ServerID] = n.ID
		}
		// Use the LATER of `updated` and `userEdited`. Keep stamps
		// `updated` with millisecond-offset epoch sentinels for items
		// it created server-side, even after the user has toggled
		// them; the actual "user touched this" recency lives in
		// `userEdited`. Verified via cmd/keep-debug dump on the
		// user's bound list — items toggled via the phone app had
		// userEdited=2026-04-28T23:44Z but updated=epoch+2ms.
		t := latestKeepTimestamp(n.Timestamps.Updated, n.Timestamps.UserEdited)
		if !t.IsZero() {
			keepUpdated[n.ServerID] = t
		}
	}
	if c.debug != nil {
		c.debug.Info("SyncToKeep diff prep: pull.Nodes=%d clientIDs=%d baseVersions=%d",
			len(pull.Nodes), len(originalClientIDs), len(baseVersions))
	}

	covered := make(map[string]bool, len(binding.ItemIDMap))
	pushNodes := make([]protocol.Node, 0, len(checklist.GetItems()))
	freshUIDs := make([]string, 0)        // index-aligned with the appended fresh items below
	freshClientIDs := make([]string, 0)
	for i, item := range checklist.GetItems() {
		serverID := binding.ItemIDMap[item.GetUid()]
		node := WikiToKeep(item, binding.KeepNoteID, serverID)
		// Stamp Updated to sync-now: gkeepapi touch() sets
		// timestamps.updated = now() on every dirty mutation, and
		// Keep's backend 500s ("Unknown Error") if we send an older
		// `updated` than what's on the server. Wiki UpdatedAt is the
		// wiki-side last-touch, which can be days stale relative to
		// Keep's record.
		node.Timestamps.Updated = now
		// Lower SortValues sort to the bottom; preserve wiki ordering
		// by mapping (n-i)*1000 in absence of an explicit SortOrder.
		if item.GetSortOrder() == 0 {
			node.SortValue = fmt.Sprintf("%d", (len(checklist.GetItems())-i)*sortValueGap)
		}
		if serverID == "" {
			// Fresh item: assign a client_id, remember the uid so we
			// can update the map after the response.
			cid, idErr := generatePushClientID(now, i)
			if idErr != nil {
				return fmt.Errorf("generate client_id: %w", idErr)
			}
			node.ID = cid
			freshClientIDs = append(freshClientIDs, cid)
			freshUIDs = append(freshUIDs, item.GetUid())
		} else {
			// Existing item: prefer the original client_id (the one
			// captured at the item's first arrival on this client) so
			// the wire shape mirrors gkeepapi exactly. Fall back to
			// serverID-as-id if the pull didn't carry a client_id
			// (some Keep responses elide it).
			if originalID, ok := originalClientIDs[serverID]; ok {
				node.ID = originalID
			}
			node.BaseVersion = baseVersions[serverID]
			covered[item.GetUid()] = true

			// Only push items where wiki's view is more recent than
			// Keep's. Symmetric to inbound's shouldPullKeepUpdate.
			// Skipping no-op items (or items where Keep is fresher)
			// avoids two corruption modes:
			//   1. Reverting a just-applied phone-side edit because
			//      wiki's stale state was pushed back.
			//   2. Bumping Keep timestamps on every cron tick, which
			//      makes future inbound updates think Keep is always
			//      newer than wiki and skip them.
			if !shouldPushWikiUpdate(item.GetUpdatedAt(), keepUpdated[serverID]) {
				continue
			}
			if c.debug != nil {
				wt := time.Time{}
				if item.GetUpdatedAt() != nil {
					wt = item.GetUpdatedAt().AsTime()
				}
				c.debug.Info("push gate let through: uid=%s wiki.updated=%s keep.updated=%s",
					item.GetUid(), wt.Format(time.RFC3339Nano), keepUpdated[serverID].Format(time.RFC3339Nano))
			}
		}
		pushNodes = append(pushNodes, node)
	}

	// Soft-delete: any binding map entry whose uid isn't in the current
	// wiki items list got deleted wiki-side. Push Trashed=now so Keep
	// moves it to the trash on the user's phone.
	for uid, serverID := range binding.ItemIDMap {
		if covered[uid] || serverID == "" {
			continue
		}
		// Same id-vs-serverId distinction as the update path above:
		// prefer the captured client_id when we have one.
		clientID := serverID
		if originalID, ok := originalClientIDs[serverID]; ok {
			clientID = originalID
		}
		pushNodes = append(pushNodes, protocol.Node{
			Kind:           "notes#node",
			ID:             clientID,
			ServerID:       serverID,
			ParentID:       binding.KeepNoteID,
			ParentServerID: binding.KeepNoteID,
			Type:           protocol.NodeTypeListItem,
			BaseVersion:    baseVersions[serverID],
			Timestamps: protocol.Timestamps{
				Trashed: now,
				Updated: now,
			},
		})
	}

	// Always include the LIST node in the push when we have label
	// assignments to record — Keep ignores label CRUD without a node
	// referencing the new IDs. Cheap: a labelIds-only push on an
	// existing LIST is one extra wireNode.
	if len(listLabelIDs) > 0 || len(labelPushEntries) > 0 {
		listNode := protocol.Node{
			Kind:       "notes#node",
			ID:         binding.KeepNoteID,
			ServerID:   binding.KeepNoteID,
			Type:       protocol.NodeTypeList,
			Title:      binding.KeepNoteTitle,
			LabelIDs:   listLabelIDs,
			Timestamps: protocol.Timestamps{Updated: now},
		}
		pushNodes = append(pushNodes, listNode)
	}

	if len(pushNodes) == 0 && len(labelPushEntries) == 0 {
		// No diff — wiki and binding map are in sync. Update verified
		// timestamp and exit.
		return c.markBindingSynced(profileID, binding, now)
	}

	pushSession, err := generatePushSessionID(now)
	if err != nil {
		return fmt.Errorf("generate session_id: %w", err)
	}
	resp, err := client.Changes(ctx, protocol.ChangesRequest{
		Nodes:           pushNodes,
		Labels:          labelPushEntries,
		TargetVersion:   pull.ToVersion,
		SessionID:       pushSession,
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		return fmt.Errorf("sync push: %w", err)
	}

	// Update id_map: pick up server-assigned IDs for fresh items;
	// drop entries whose corresponding node came back trashed.
	if binding.ItemIDMap == nil {
		binding.ItemIDMap = map[string]string{}
	}
	for _, n := range resp.Nodes {
		if n.Type != protocol.NodeTypeListItem {
			continue
		}
		// Match echoed-back fresh items by their client_id (n.ID).
		for i, cid := range freshClientIDs {
			if n.ID == cid && n.ServerID != "" {
				binding.ItemIDMap[freshUIDs[i]] = n.ServerID
			}
		}
	}
	// Drop trashed-confirmed entries.
	for uid := range binding.ItemIDMap {
		if covered[uid] {
			continue
		}
		// Wiki side dropped this; remove from id_map regardless of
		// whether the response echoed it (Keep's soft-delete is
		// eventually consistent).
		delete(binding.ItemIDMap, uid)
	}

	return c.markBindingSynced(profileID, binding, now)
}

// bootstrapKeepListForBinding creates the Keep LIST for a binding that
// was recorded with KeepNoteID="". Uses the bundled CreateListWithItems
// path because Keep rejects two-step (create-list, push-items) with
// stage3 HTTP 500. Pulls wiki items, encodes them with the same tags
// + description encoder the steady-state push uses, sends one Changes
// request, then persists the binding with the server-assigned LIST
// serverID + the per-item id_map echoed back.
//
// On success the binding is fully initialized; the next sync (cron or
// edit-triggered) falls through to the normal pull/apply/push path.
func (c *Connector) bootstrapKeepListForBinding(ctx context.Context, profileID wikipage.PageIdentifier, binding Binding, checklist *apiv1.Checklist, client KeepClient, now time.Time) error {
	wikiItems := checklist.GetItems()
	specs := make([]protocol.ListItemSpec, len(wikiItems))
	for i, it := range wikiItems {
		// Same encoder the steady-state SyncToKeep uses, so the
		// initial push and subsequent diff-pushes produce
		// byte-identical wire shapes for the same wiki state.
		head := encodeTextWithTags(it.GetText(), it.GetTags())
		text := head
		if d := it.GetDescription(); d != "" {
			text = head + descriptionSeparator + d
		}
		specs[i] = protocol.ListItemSpec{
			Text:      text,
			Checked:   it.GetChecked(),
			SortValue: fmt.Sprintf("%d", (len(wikiItems)-i)*sortValueGap),
		}
	}

	result, err := client.CreateListWithItems(ctx, binding.ListName, specs)
	if err != nil {
		return fmt.Errorf("bootstrap create-list: %w", err)
	}

	binding.KeepNoteID = result.ListServerID
	binding.KeepNoteTitle = binding.ListName
	if binding.ItemIDMap == nil {
		binding.ItemIDMap = map[string]string{}
	}
	for i, serverID := range result.ItemServerIDs {
		if i >= len(wikiItems) || serverID == "" {
			continue
		}
		if uid := wikiItems[i].GetUid(); uid != "" {
			binding.ItemIDMap[uid] = serverID
		}
	}

	// Suppress is unnecessary here — CreateListWithItems doesn't
	// touch wiki state, just reads — but persistBindingMap uses the
	// store's per-profile mutex so concurrent bind/cron interleavings
	// can't shred the id_map.
	return c.persistBindingMap(profileID, binding)
}

// markBindingSynced persists the updated binding (id_map changes) and
// stamps the binding's BoundAt — used as a "last successful sync" marker
// pending a more explicit field. Holds the per-profile mutex via
// BindingStore so concurrent syncs serialize.
func (c *Connector) markBindingSynced(profileID wikipage.PageIdentifier, binding Binding, _ time.Time) error {
	state, err := c.store.LoadState(profileID)
	if err != nil {
		return fmt.Errorf("reload state for sync persist: %w", err)
	}
	for i, b := range state.Bindings {
		if b.Page == binding.Page && b.ListName == binding.ListName {
			state.Bindings[i] = binding
			break
		}
	}
	return c.store.SaveState(profileID, state)
}

// applyInboundFromKeep reconciles Keep-side changes into the wiki
// before the outbound diff runs. Returns the (possibly updated)
// binding (id_map gains entries for fresh-from-Keep items) and a
// freshly-loaded checklist reflecting the post-apply state.
//
// Three classes of inbound change:
//
//  1. New on Keep, not in wiki — a Keep node whose serverID isn't in
//     binding.ItemIDMap (reverse). Apply via AddItemForSync; record
//     the new uid in the id_map.
//
//  2. Updated on Keep — a Keep node whose serverID IS in id_map and
//     whose Updated timestamp is newer than the wiki item's
//     UpdatedAt. Apply via UpdateItemForSync (replaces text/tags/
//     description, reconciles checked).
//
//  3. Trashed/Deleted on Keep — a Keep node whose serverID is in
//     id_map and whose Trashed/Deleted timestamp is non-zero. Apply
//     via DeleteItemForSync; remove from id_map.
//
// Suppression: each inbound mutation fires the mutator's notify hook,
// which would normally enqueue another sync. The suppressor blocks
// scheduleSync for this binding's key for the duration of the apply
// pass (refcounted). Outbound push then runs unsuppressed; if it
// itself produces new wiki state via the response (e.g. new Keep
// serverIDs), those land via the response-walk in the caller.
func (c *Connector) applyInboundFromKeep(ctx context.Context, profileID wikipage.PageIdentifier, binding Binding, pull protocol.ChangesResponse, currentChecklist *apiv1.Checklist) (Binding, *apiv1.Checklist, error) {
	if c.checklistW == nil {
		// No mutator wired — outbound-only mode. Return inputs as-is.
		return binding, currentChecklist, nil
	}
	if c.suppressor != nil {
		c.suppressor.Suppress(profileID, binding.Page, binding.ListName)
		defer c.suppressor.Unsuppress(profileID, binding.Page, binding.ListName)
	}

	// Reverse map: serverID → wiki uid, from the binding's id_map.
	rev := make(map[string]string, len(binding.ItemIDMap))
	for uid, sid := range binding.ItemIDMap {
		rev[sid] = uid
	}

	// Wiki items by uid (for find-by-uid update path) AND by exact
	// rendered text (for the adopt-don't-add path on inbound items
	// that match wiki state — typically wiki-pushed copies that
	// boomeranged back through a fresh pull). Adoption avoids
	// duplicating items wiki already has under a different uid.
	wikiByUID := make(map[string]*apiv1.ChecklistItem, len(currentChecklist.GetItems()))
	wikiByText := make(map[string]string, len(currentChecklist.GetItems()))
	for _, it := range currentChecklist.GetItems() {
		wikiByUID[it.GetUid()] = it
		// Render the wiki item the same way WikiToKeep would so the
		// match key aligns with what's actually flowing on the wire.
		head := encodeTextWithTags(it.GetText(), it.GetTags())
		full := head
		if d := it.GetDescription(); d != "" {
			full = head + descriptionSeparator + d
		}
		// First wiki item wins on duplicate-text ties — same convention
		// as seedIDMapFromExistingList.
		if _, exists := wikiByText[full]; !exists {
			wikiByText[full] = it.GetUid()
		}
	}

	if binding.ItemIDMap == nil {
		binding.ItemIDMap = map[string]string{}
	}

	for _, n := range pull.Nodes {
		if n.Type != protocol.NodeTypeListItem {
			continue
		}
		if n.ParentID != binding.KeepNoteID && n.ParentServerID != binding.KeepNoteID {
			continue
		}
		serverID := n.ServerID
		if serverID == "" {
			continue
		}
		isAlive := n.Timestamps.Trashed.IsZero() && n.Timestamps.Deleted.IsZero()
		uid, knownToWiki := rev[serverID]

		switch {
		case isAlive && !knownToWiki:
			// Class 1: arrived from Keep, not yet known to id_map.
			// Two sub-cases:
			//
			//   1a. Wiki has an item with byte-identical text — that's
			//       almost certainly a wiki-pushed copy that came back
			//       around. Adopt it: record uid → serverID in id_map
			//       and DON'T create a duplicate. This is the most
			//       common case after re-bind: the wiki-pushed enriched
			//       items live on Keep alongside the user's plain
			//       originals; the seed already mapped the plain ones,
			//       and on the next sync the enriched copies look
			//       "new" but actually correspond to wiki items we
			//       already have.
			//
			//   1b. No text match in wiki. Genuinely new from Keep
			//       (typically: phone-side add). Apply via AddItemForSync.
			if existingUID, found := wikiByText[n.Text]; found {
				if _, alreadyMapped := binding.ItemIDMap[existingUID]; !alreadyMapped {
					binding.ItemIDMap[existingUID] = serverID
				}
				continue
			}
			wikiItem, err := KeepToWiki(n)
			if err != nil {
				// Skip malformed nodes rather than fail the whole sync.
				continue
			}
			desc := ""
			if wikiItem.Description != nil {
				desc = *wikiItem.Description
			}
			newUID, err := c.checklistW.AddItemForSync(ctx, binding.Page, binding.ListName,
				wikiItem.GetText(), wikiItem.GetChecked(), wikiItem.GetTags(), desc, n.SortValue)
			if err != nil {
				continue
			}
			binding.ItemIDMap[newUID] = serverID

		case !isAlive && knownToWiki:
			// Class 3: trashed on Keep. Delete from wiki.
			_ = c.checklistW.DeleteItemForSync(ctx, binding.Page, binding.ListName, uid)
			delete(binding.ItemIDMap, uid)

		case isAlive && knownToWiki:
			// Class 2: maybe updated on Keep. Compare Keep's freshness
			// (later of updated/userEdited — see latestKeepTimestamp)
			// to wiki's UpdatedAt; only pull if Keep is newer.
			wikiItem, ok := wikiByUID[uid]
			if !ok {
				continue
			}
			keepFreshness := latestKeepTimestamp(n.Timestamps.Updated, n.Timestamps.UserEdited)
			if !shouldPullKeepUpdate(keepFreshness, wikiItem.GetUpdatedAt()) {
				continue
			}
			converted, err := KeepToWiki(n)
			if err != nil {
				continue
			}
			desc := ""
			if converted.Description != nil {
				desc = *converted.Description
			}
			if updateErr := c.checklistW.UpdateItemForSync(ctx, binding.Page, binding.ListName, uid,
				converted.GetText(), converted.GetChecked(), converted.GetTags(), desc); updateErr != nil && c.debug != nil {
				// Stop swallowing — when this fails the wiki silently
				// drifts out of sync with Keep, and the next outbound
				// push reverts Keep to wiki's stale state. Visibility
				// in journalctl is critical for diagnosing.
				c.debug.Info("applyInboundFromKeep: UpdateItemForSync(uid=%s, checked=%v) failed: %v",
					uid, converted.GetChecked(), updateErr)
			}
		}
	}

	// Reload wiki state — the apply mutated it.
	freshChecklist, err := c.checklistR.ListItems(ctx, binding.Page, binding.ListName)
	if err != nil {
		return binding, currentChecklist, fmt.Errorf("reload after inbound apply: %w", err)
	}

	// Persist binding's updated id_map (delete + new uid entries).
	if persistErr := c.persistBindingMap(profileID, binding); persistErr != nil {
		// Persist failure is recoverable — outbound still works against
		// the in-memory map for this run; next run reloads from store.
		_ = persistErr
	}

	return binding, freshChecklist, nil
}

// persistBindingMap writes the updated binding back to the store.
// Used after inbound apply to lock in id_map changes (new uids,
// dropped trashed entries) before the outbound push runs.
func (c *Connector) persistBindingMap(profileID wikipage.PageIdentifier, binding Binding) error {
	state, err := c.store.LoadState(profileID)
	if err != nil {
		return err
	}
	for i, b := range state.Bindings {
		if b.Page == binding.Page && b.ListName == binding.ListName {
			state.Bindings[i] = binding
			break
		}
	}
	return c.store.SaveState(profileID, state)
}

// latestKeepTimestamp picks whichever of `updated` and `userEdited`
// is more recent. Keep stamps `updated` with millisecond-offset
// epoch sentinels (1970-01-01T00:00:00.001/.002Z) for items it
// created server-side; the actual "user touched this" recency lives
// in `userEdited`. Using the max keeps the gate comparisons honest.
func latestKeepTimestamp(updated, userEdited time.Time) time.Time {
	if userEdited.After(updated) {
		return userEdited
	}
	return updated
}

// shouldPullKeepUpdate decides whether a Keep node's Updated
// timestamp is newer enough than the wiki item's UpdatedAt to warrant
// pulling the Keep state. Returns false if the wiki UpdatedAt is
// missing or already at-or-after Keep's Updated.
func shouldPullKeepUpdate(keepUpdated time.Time, wikiUpdatedAt *timestamppb.Timestamp) bool {
	if keepUpdated.IsZero() {
		return false
	}
	if wikiUpdatedAt == nil {
		return true
	}
	return keepUpdated.After(wikiUpdatedAt.AsTime())
}

// shouldPushWikiUpdate is the symmetric outbound version of
// shouldPullKeepUpdate. Returns true when wiki's view is strictly
// newer than Keep's — we have wiki-side edits that haven't reached
// Keep yet. Returns false when Keep's view is at or ahead of wiki's
// (the state has already round-tripped, or Keep was just edited and
// the inbound apply will catch up next tick).
//
// keepUpdated may be zero (Keep node missing or no Updated value) —
// treat that as "Keep is empty, push our state."
func shouldPushWikiUpdate(wikiUpdatedAt *timestamppb.Timestamp, keepUpdated time.Time) bool {
	if wikiUpdatedAt == nil {
		return false
	}
	if keepUpdated.IsZero() {
		return true
	}
	return wikiUpdatedAt.AsTime().After(keepUpdated)
}

// seedIDMapFromExistingList reconciles wiki items against an existing
// Keep list at bind time so future syncs don't duplicate items.
//
// Strategy: pull the bound list's existing LIST_ITEMs from Keep, then
// for each wiki item find a Keep item by exact head-line match (text +
// inline tags, before any "\n— description" suffix). Match-by-text is
// imperfect (collisions on identical-text items) but it's the only
// signal we have at bind time — Keep doesn't know wiki UIDs and wiki
// has no record of Keep serverIDs yet. Subsequent edits round-trip
// cleanly through the id_map.
//
// Errors are swallowed — a failed pull at bind time should not block
// the bind itself; the next mutation will trigger a sync that
// re-discovers items via CreateListWithItems-style new-item pushes
// (which will dedupe-by-text on Keep's side... no, actually they
// won't — Keep accepts duplicate-text items. So this DOES matter, but
// we'd rather have a working bind than a hard failure if the pull
// flakes.).
func (c *Connector) seedIDMapFromExistingList(ctx context.Context, client KeepClient, listServerID string, wikiItems []InitialItem) map[string]string {
	out := map[string]string{}
	now := c.clock.Now().UTC()
	pull, err := client.Changes(ctx, protocol.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--bindseed-%s", now.UnixMilli(), listServerID),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		return out
	}
	// Build base-text → serverID index for live LIST_ITEMs whose
	// parent matches our bound list. Base text strips inline #tags
	// and any "\n— description" suffix so a plain Keep "Apples"
	// matches an enriched wiki "Apples #produce\n— Deal: ...". This
	// is the bind-time reconcile: the user is opting in to "wiki is
	// the source of truth, but reuse Keep IDs where possible to
	// avoid duplicating items already there."
	keepByBase := make(map[string]string, len(pull.Nodes))
	for _, n := range pull.Nodes {
		if n.Type != protocol.NodeTypeListItem {
			continue
		}
		if n.ParentID != listServerID && n.ParentServerID != listServerID {
			continue
		}
		if !n.Timestamps.Trashed.IsZero() || !n.Timestamps.Deleted.IsZero() {
			continue
		}
		base := normalizeForSeedMatch(n.Text)
		if base == "" {
			continue
		}
		if _, exists := keepByBase[base]; exists {
			// Duplicate-base Keep items: first match wins; second
			// will look fresh to the sync engine and get a new
			// client_id. User can manually clean up.
			continue
		}
		keepByBase[base] = n.ServerID
	}
	for _, w := range wikiItems {
		if w.UID == "" {
			continue
		}
		base := normalizeForSeedMatch(w.Text)
		if base == "" {
			continue
		}
		if serverID, ok := keepByBase[base]; ok {
			out[w.UID] = serverID
		}
	}
	return out
}

// normalizeForSeedMatch reduces a LIST_ITEM text to its bare item-name
// portion: strips any "\n— description" suffix and any inline " #tag"
// markers, lowercases, and trims surrounding whitespace. Used only at
// bind time to find loose matches between wiki items (typically tagged
// + described) and existing Keep items (typically plain text).
func normalizeForSeedMatch(text string) string {
	head, _, _ := strings.Cut(text, "\n— ")
	// Walk word-by-word; drop tokens that start with '#'.
	fields := strings.Fields(head)
	cleaned := make([]string, 0, len(fields))
	for _, f := range fields {
		if strings.HasPrefix(f, "#") {
			continue
		}
		cleaned = append(cleaned, f)
	}
	return strings.ToLower(strings.Join(cleaned, " "))
}

// readPageTags reads the host page's frontmatter tags. Returns an
// empty slice for "no tags" (not an error). Errors only on actual
// store failures; missing pages return ([], nil) — the binding might
// outlive the host page in edge cases and we'd rather sync no labels
// than blow up the sync job.
func (c *Connector) readPageTags(page string) ([]string, error) {
	_, fm, err := c.store.pages.ReadFrontMatter(wikipage.PageIdentifier(page))
	if err != nil {
		// Treat missing page as "no tags" — sync engine continues
		// rather than wedging on an irrelevant lookup.
		return nil, nil
	}
	raw, ok := fm["tags"]
	if !ok {
		return nil, nil
	}
	arr, ok := raw.([]any)
	if !ok {
		// Frontmatter has a "tags" field but it isn't a list — the
		// page likely has a different convention. Skip rather than
		// failing the entire sync.
		return nil, nil
	}
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out, nil
}

// resolveLabelsForTags reconciles the host page's tags against the
// user's existing Keep labels. Returns:
//   - labelPush: Label CRUD entries for tags that don't have a Keep
//     label yet (one fresh MainID per missing tag).
//   - listLabelIDs: the set of Keep label MainIDs that should be
//     assigned to the bound LIST after the push.
//
// Existing Keep labels matched by exact name (case-sensitive); Keep's
// label uniqueness is per-name. Tombstoned labels (Deleted != zero)
// are skipped — we re-create rather than reviving.
func resolveLabelsForTags(tags []string, existingLabels []protocol.LabelEntry, now time.Time) (labelPush []protocol.LabelEntry, listLabelIDs []string, err error) {
	if len(tags) == 0 {
		return nil, nil, nil
	}
	byName := make(map[string]string, len(existingLabels))
	for _, l := range existingLabels {
		if !l.Deleted.IsZero() {
			continue
		}
		byName[l.Name] = l.MainID
	}
	listLabelIDs = make([]string, 0, len(tags))
	for _, tag := range tags {
		if existingID, ok := byName[tag]; ok {
			listLabelIDs = append(listLabelIDs, existingID)
			continue
		}
		newID, err := generateLabelMainID(now, len(labelPush))
		if err != nil {
			return nil, nil, err
		}
		labelPush = append(labelPush, protocol.LabelEntry{
			MainID:  newID,
			Name:    tag,
			Created: now,
			Updated: now,
		})
		listLabelIDs = append(listLabelIDs, newID)
		// Cache the just-created mapping so a duplicate tag in the
		// input list reuses the same MainID rather than creating
		// another (Keep tolerates duplicates but we'd rather not).
		byName[tag] = newID
	}
	return labelPush, listLabelIDs, nil
}

// generateLabelMainID makes a Keep-style label MainID. Same shape as a
// Node ID ("ms-hex.16-hex"); gkeepapi node.py:1077-1085 (Label._gen_id).
func generateLabelMainID(now time.Time, idx int) (string, error) {
	var entropy [8]byte
	if _, err := io.ReadFull(rand.Reader, entropy[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x.%016x", now.UnixMilli()+int64(idx), binary.BigEndian.Uint64(entropy[:])), nil
}

// generatePushClientID makes a Keep-style client id for a brand-new
// item being pushed during sync. Bumps the ms component by index so
// repeated calls within the same sync don't collide.
func generatePushClientID(now time.Time, idx int) (string, error) {
	var entropy [8]byte
	if _, err := io.ReadFull(rand.Reader, entropy[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x.%016x", now.UnixMilli()+int64(idx), binary.BigEndian.Uint64(entropy[:])), nil
}

// generatePushSessionID makes a Keep-style session id (s--<ms>--<10digit>)
// for a single sync push, identifying the outbound batch in Keep logs.
func generatePushSessionID(now time.Time) (string, error) {
	var entropy [8]byte
	if _, err := io.ReadFull(rand.Reader, entropy[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("s--%d--%010d", now.UnixMilli(),
		(binary.BigEndian.Uint64(entropy[:])%keepSessionIDSpan)+keepSessionIDOffset), nil
}

// Unbind removes the calling user's binding for (page, listName).
func (c *Connector) Unbind(_ context.Context, profileID wikipage.PageIdentifier, page, listName string) error {
	err := c.store.RemoveBinding(profileID, page, listName)
	c.noteBindingRemoved(BindingKey{ProfileID: profileID, Page: page, ListName: listName})
	if errors.Is(err, ErrBindingNotFound) {
		// Idempotent at the orchestrator boundary — UI calls this on
		// rebind/remove flows and shouldn't have to disambiguate.
		return nil
	}
	return err
}

// FindBinding mirrors BindingStore.FindBinding for handler convenience.
func (c *Connector) FindBinding(_ context.Context, profileID wikipage.PageIdentifier, page, listName string) (Binding, bool, error) {
	return c.store.FindBinding(profileID, page, listName)
}

// VerifyBinding pings Keep with the user's bearer to confirm the bound
// note still exists. Updates LastVerifiedAt on success. This is the v1
// sync stub — actual data round-trip lands as a follow-up; see the help
// page section "What sync does today".
//
// Pulled out as its own method so the scheduler can call it on a tick
// without knowing about Keep's wire shape.
func (c *Connector) VerifyBinding(ctx context.Context, profileID wikipage.PageIdentifier, binding Binding) error {
	client, _, err := c.keepClientFor(ctx, profileID)
	if err != nil {
		return err
	}
	now := c.clock.Now()
	resp, err := client.Changes(ctx, protocol.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--verify-%s", now.UnixMilli(), binding.KeepNoteID),
		ClientTimestamp: now.UTC().Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		return err
	}
	for _, n := range resp.Nodes {
		if n.Type == protocol.NodeTypeList && n.ServerID == binding.KeepNoteID {
			// Found — record verified-at.
			state, lerr := c.store.LoadState(profileID)
			if lerr != nil {
				return lerr
			}
			state.LastVerifiedAt = now.UTC()
			return c.store.SaveState(profileID, state)
		}
	}
	return ErrBoundNoteDeletedLocal
}

// ErrBoundNoteDeletedLocal mirrors protocol.ErrBoundNoteDeleted but is
// surfaced from VerifyBinding when the bound note simply isn't in the
// user's account anymore (e.g., they deleted it from the Keep app).
var ErrBoundNoteDeletedLocal = protocol.ErrBoundNoteDeleted

// deriveDeviceID returns a stable 16-hex-char device id derived from the
// profile id. Matches the gpsoauth requirement of a stable per-account
// android id without reusing any real device's id.
func deriveDeviceID(profileID wikipage.PageIdentifier) string {
	sum := sha256.Sum256([]byte(profileID))
	return hex.EncodeToString(sum[:8]) // 16 hex chars
}
