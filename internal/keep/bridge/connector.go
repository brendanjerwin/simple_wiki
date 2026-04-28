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
	"time"

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
	authBuilder   func(deviceID string) AuthExchanger
	clientBuilder KeepClientFactory
}

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
// the wiki checklist and converts; the connector doesn't import apiv1.
//
// UID is the wiki ChecklistItem UID and is what the binding's
// ItemIDMap keys on after the bundled push completes — that's how
// future incremental syncs find "the Keep node corresponding to
// wiki item X."
type InitialItem struct {
	UID     string
	Text    string
	Checked bool
}

// Bind binds (page, listName) to a Keep note for the calling user. If
// keepNoteID is empty, a new note titled `listName` is created. When
// initialItems is non-empty AND a new note is being created, the items
// are pushed in the same Changes request as the list creation —
// Google's backend rejects two-step (create list → push items) with
// 500 Unknown Error, so bundling is the only path that works.
//
// initialItems is ignored when binding to an existing keepNoteID; in
// that case, the user is opting in to "merge into the existing list,"
// which the v1 sync engine handles via its incremental reconcile.
func (c *Connector) Bind(ctx context.Context, profileID wikipage.PageIdentifier, page, listName, keepNoteID string, initialItems []InitialItem) (Binding, error) {
	client, state, err := c.keepClientFor(ctx, profileID)
	if err != nil {
		return Binding{}, err
	}

	idMap := map[string]string{}
	if keepNoteID == "" {
		specs := make([]protocol.ListItemSpec, len(initialItems))
		for i, it := range initialItems {
			// Lower SortValues sort to the bottom in Keep, so map the
			// caller's intended top-to-bottom order into descending
			// values. (n-i)*1000 leaves room for future inserts.
			specs[i] = protocol.ListItemSpec{
				Text:      it.Text,
				Checked:   it.Checked,
				SortValue: fmt.Sprintf("%d", (len(initialItems)-i)*sortValueGap),
			}
		}
		result, err := client.CreateListWithItems(ctx, listName, specs)
		if err != nil {
			return Binding{}, err
		}
		keepNoteID = result.ListServerID
		// Populate the binding's wiki-uid → keep-serverID mapping from
		// what the bundled create returned. This is the seed every
		// future incremental sync uses to identify which Keep node
		// corresponds to which wiki item.
		for i, serverID := range result.ItemServerIDs {
			if i >= len(initialItems) || serverID == "" {
				continue
			}
			if uid := initialItems[i].UID; uid != "" {
				idMap[uid] = serverID
			}
		}
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
	_ = state // reserved for future per-binding bookkeeping during bind
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
	pull, err := client.Changes(ctx, protocol.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--syncpull-%s", now.UnixMilli(), binding.KeepNoteID),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		return fmt.Errorf("sync pull: %w", err)
	}

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
	covered := make(map[string]bool, len(binding.ItemIDMap))
	pushNodes := make([]protocol.Node, 0, len(checklist.GetItems()))
	freshUIDs := make([]string, 0)        // index-aligned with the appended fresh items below
	freshClientIDs := make([]string, 0)
	for i, item := range checklist.GetItems() {
		serverID := binding.ItemIDMap[item.GetUid()]
		node := WikiToKeep(item, binding.KeepNoteID, serverID)
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
			covered[item.GetUid()] = true
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
		pushNodes = append(pushNodes, protocol.Node{
			Kind:           "notes#node",
			ID:             serverID, // Keep accepts serverID as ID for in-place updates
			ServerID:       serverID,
			ParentID:       binding.KeepNoteID,
			ParentServerID: binding.KeepNoteID,
			Type:           protocol.NodeTypeListItem,
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
