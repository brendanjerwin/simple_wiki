package sync

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
	"sync"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/gateway"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/translator"
	"github.com/brendanjerwin/simple_wiki/internal/hashtags"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// Clock returns the current time. SystemClock for production, fake for tests.
type Clock interface{ Now() time.Time }

// SystemClock returns time.Now wrapped in a Clock.
type SystemClock struct{}

// Now returns wall-clock time.
func (SystemClock) Now() time.Time { return time.Now() }

// AuthExchanger is the subset of gateway.Authenticator the connector uses.
// Stated as an interface so tests can substitute a fake without spinning up
// a real httptest server for every Connector test.
type AuthExchanger interface {
	ExchangeOAuthTokenForMasterToken(ctx context.Context, email, oauthToken string) (string, error)
	ExchangeMasterTokenForBearer(ctx context.Context, email, masterToken string) (string, error)
}

// KeepClientFactory creates a KeepClient given a bearer. Real factory just
// constructs *gateway.KeepClient; tests inject a stub.
type KeepClientFactory func(bearer string) KeepClient

// KeepClient is the subset of *gateway.KeepClient the connector calls.
type KeepClient interface {
	Changes(ctx context.Context, req gateway.ChangesRequest) (gateway.ChangesResponse, error)
	CreateList(ctx context.Context, title string) (string, error)
	CreateListWithItems(ctx context.Context, title string, items []gateway.ListItemSpec) (gateway.CreateListResult, error)
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
	// ownerEmail is the binding owner's Google email — used for
	// completed_by attribution so wiki readers see who synced from
	// Keep, not an opaque "system" placeholder.
	AddItemForSync(ctx context.Context, page, listName, ownerEmail, text string, checked bool, tags []string, description, sortValueHint string) (string, error)
	UpdateItemForSync(ctx context.Context, page, listName, ownerEmail, uid, text string, checked bool, tags []string, description string) error
	DeleteItemForSync(ctx context.Context, page, listName, ownerEmail, uid string) error
}

// Connector orchestrates the Keep bridge: per-user auth exchange,
// verification, binding management, and Keep client construction. Owns no
// long-running goroutines (those live in the scheduler) — every method
// completes in the caller's context.
type Connector struct {
	store         *SubscriptionStore
	leaseTable    *connectors.LeaseTable
	httpClient    *http.Client
	clock         Clock
	debug         gateway.DebugLogger
	checklistR    ChecklistReader
	checklistW    ChecklistMutator
	suppressor    SyncSuppressor
	enqueuer      JobEnqueuer
	authBuilder   func(deviceID string) AuthExchanger
	clientBuilder KeepClientFactory

	activeMu sync.Mutex
	active   *activeSubscriptions

	// loggedSkip records bindings whose un-migrated SyncToKeep skip has
	// already produced an INFO log this process. Keys are BindingKey
	// values; presence means "skip already logged, stay silent". Reset
	// on process restart (next process logs fresh on first skip). The
	// rationale lives in plan §"Log throttling on the skip path": a
	// stuck-un-migrated binding would otherwise spam the journal once
	// per cron tick (~30s) until the migration job catches up.
	loggedSkip sync.Map
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
func (c *Connector) SetDebugLogger(l gateway.DebugLogger) { c.debug = l }

// SetClientBuilder overrides the KeepClient factory. Test-only —
// production wires the real factory in NewConnector. Used by sync
// matrix tests to swap in a fake KeepClient that records pushes and
// returns canned pull responses.
func (c *Connector) SetClientBuilder(f KeepClientFactory) { c.clientBuilder = f }

// SetAuthBuilder overrides the AuthExchanger factory. Test-only —
// production wires the real factory in NewConnector. Used by sync
// matrix tests to bypass the gpsoauth round trip.
func (c *Connector) SetAuthBuilder(f func(deviceID string) AuthExchanger) {
	c.authBuilder = f
}

// NewConnector wires the production dependencies. Tests construct a
// Connector directly with stubbed builders.
//
// leaseTable is the cross-connector exclusivity registry. Keep's
// subscribe-ceremony (Bind) takes the lease through this table so a
// checklist can have at most one Subscription across all connectors
// (Keep + Tasks + future iCloud) per ADR-0011's
// ChecklistSubscription aggregate invariant.
//
// The auth-side http.Client forces HTTP/1.1 (no h2 ALPN advertisement)
// because gpsoauth's auth endpoint at android.clients.google.com/auth
// returns 403 Bad Authentication when an h2 ALPN protocol is offered.
// The Python gpsoauth library applies the same quirk via a custom
// HTTPAdapter; mirroring it here.
func NewConnector(store *SubscriptionStore, leaseTable *connectors.LeaseTable, httpClient *http.Client, clock Clock) (*Connector, error) {
	if store == nil {
		return nil, errors.New("keep bridge: store must not be nil")
	}
	if leaseTable == nil {
		return nil, errors.New("keep bridge: leaseTable must not be nil")
	}
	if clock == nil {
		return nil, errors.New("keep bridge: clock must not be nil")
	}
	authClient := newAuthHTTPClient()
	c := &Connector{
		store:      store,
		leaseTable: leaseTable,
		httpClient: httpClient,
		clock:      clock,
		authBuilder: func(deviceID string) AuthExchanger {
			return gateway.NewAuthenticator(authClient, gateway.AuthURL, deviceID)
		},
	}
	c.clientBuilder = func(bearer string) KeepClient {
		kc := gateway.NewKeepClient(httpClient, gateway.DefaultKeepBaseURL, bearer)
		if c.debug != nil {
			kc.SetDebugLogger(c.debug)
		}
		return kc
	}
	return c, nil
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
	return &http.Client{Transport: transport, Timeout: keepHTTPClientTimeoutSeconds * time.Second}
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
	if _, err := client.Changes(ctx, gateway.ChangesRequest{
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
	resp, err := client.Changes(ctx, gateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--listnotes", now.UnixMilli()),
		ClientTimestamp: now.UTC().Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	for _, n := range resp.Nodes {
		if n.Type == gateway.NodeTypeListItem && n.ParentID != "" && n.Timestamps.Trashed.IsZero() && n.Timestamps.Deleted.IsZero() {
			counts[n.ParentID]++
		}
	}

	out := make([]KeepNoteSummary, 0)
	for _, n := range resp.Nodes {
		if n.Type != gateway.NodeTypeList {
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
//
// Per ADR-0011's ChecklistSubscription aggregate invariant, this
// implements the subscribe-ceremony:
//
//  1. Block until the LeaseTable has finished its boot rebuild.
//  2. Acquire the per-checklist mutex.
//  3. Cross-connector existence check via LookupOwner — if any
//     connector (Keep, Tasks, …) already owns this (page, list_name)
//     return ErrChecklistAlreadyLeased.
//  4. Persist the Subscription on the profile.
//  5. Take the lease (best-effort rollback on Take failure).
//  6. Release mutex; emit EventSubscriptionEstablished.
//
//revive:disable-next-line:function-length // single-purpose subscribe ceremony; splitting hurts readability
func (c *Connector) Bind(ctx context.Context, profileID wikipage.PageIdentifier, page, listName, keepNoteID string, initialItems []InitialItem) (Subscription, error) {
	if page == "" {
		return Subscription{}, errors.New("keep bridge: page is required")
	}
	if listName == "" {
		return Subscription{}, errors.New("keep bridge: list_name is required")
	}

	if err := c.leaseTable.WaitReady(ctx); err != nil {
		return Subscription{}, fmt.Errorf("await lease-table ready: %w", err)
	}

	idMap := map[string]ItemMapping{}
	labelIDs := map[string]string{}
	var keepNoteClientID string
	if keepNoteID != "" {
		client, _, err := c.keepClientFor(ctx, profileID)
		if err != nil {
			return Subscription{}, err
		}
		idMap, labelIDs, keepNoteClientID = c.seedIDMapFromExistingList(ctx, client, keepNoteID, initialItems)
	}

	binding := Subscription{
		Page:             page,
		ListName:         listName,
		KeepNoteID:       keepNoteID,
		KeepNoteTitle:    listName,
		KeepNoteClientID: keepNoteClientID,
		SubscribedAt:          c.clock.Now().UTC(),
		// Born-migrated: a fresh binding has nothing to migrate; its
		// fingerprints will be populated by the bootstrap push response
		// or by the per-item synced_fp updates on subsequent syncs.
		MigratedFingerprints: true,
		ItemIDMap:            idMap,
		LabelIDs:             labelIDs,
	}

	checklistKey := connectors.ChecklistKey{Page: page, ListName: listName}
	owner := connectors.LeaseOwner{Kind: connectors.ConnectorKindGoogleKeep, ProfileID: string(profileID)}

	cerErr := c.leaseTable.WithChecklistLock(checklistKey, func() error {
		// Cross-profile/cross-connector existence check via the
		// LeaseTable — boot-rebuild + the Take-on-Subscribe contract
		// guarantee this is the authoritative cross-profile view.
		if existing, exists := c.leaseTable.LookupOwner(checklistKey); exists {
			return fmt.Errorf("%w: %s/%s held by %s/%s",
				connectors.ErrChecklistAlreadyLeased, page, listName,
				existing.Kind, existing.ProfileID)
		}

		if err := c.store.AddSubscription(profileID, binding); err != nil {
			return err
		}
		if err := c.leaseTable.Take(checklistKey, owner); err != nil {
			// Profile already updated — best-effort rollback so the
			// next subscribe on the same checklist sees a clean
			// LeaseTable + clean profile state.
			_ = c.store.RemoveSubscription(profileID, page, listName)
			return err
		}
		return nil
	})
	if cerErr != nil {
		return Subscription{}, cerErr
	}

	c.noteSubscriptionAdded(BindingKey{ProfileID: profileID, Page: page, ListName: listName})

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

// keepHTTPClientTimeoutSeconds is the request timeout for the
// dedicated Keep HTTP client. 30s is comfortably above Keep's worst
// observed full-pull latency on the household-scale account but
// short enough that a stuck request doesn't park a sync worker
// indefinitely.
const keepHTTPClientTimeoutSeconds = 30

// keepSessionIDOffset and keepSessionIDSpan together produce a 10-digit
// session-id suffix in [1000000000, 9999999999], matching gkeepapi.
// Pulled out so the magic numbers don't repeat in generatePushSessionID.
const (
	keepSessionIDOffset = 1000000000
	keepSessionIDSpan   = 9000000000
)

// truncationResyncThreshold is the streak length of consecutive
// truncated pulls (per binding) at which — combined with no observed
// progress (cursor not advancing AND no synced_fp updates) — the
// connector drops the cursor and forces a full resync on the next
// tick. The two-condition trigger distinguishes chronic-but-progressing
// truncation (legitimate large-account pagination) from chronic-and-
// stuck truncation (real masking risk). Source: plan §"Truncation
// escape hatch".
const truncationResyncThreshold = 5

// deadLetterThreshold is the per-item consecutive-push-failure count at
// which the connector stops attempting to push the item. After 10
// failures the item is dead-lettered: surfaced in the KeepConnect macro
// (task #83) and skipped on subsequent ticks until the user clears it
// (gRPC ClearDeadLetter, task #83) or re-edits the wiki side (which
// resets the failure count automatically — see SyncToKeep). Inbound
// apply still operates on dead-lettered items; only the outbound push
// is gated. Source: plan §"Bounded retry + dead-letter".
const deadLetterThreshold = 10

// pushFailureBackoffBaseSeconds and pushFailureBackoffMaxSeconds bound
// the exponential per-item retry schedule. After the n-th consecutive
// failure, the connector waits min(60 * 2^(n-1), 3600) seconds before
// the next push attempt for that item. NextAttemptAt on the
// ItemMapping records the absolute wall-clock deadline; the diff loop
// skips items whose NextAttemptAt is in the future. Source: plan
// §"Bounded retry + dead-letter".
const (
	pushFailureBackoffBaseSeconds = 60
	pushFailureBackoffMaxSeconds  = 3600
)

// noResponseStatusCode is the LastFailureCode used when Keep's response
// has no WriteResults entry for a pushed item. Distinguishes "Keep
// rejected" (we have a status code) from "Keep didn't tell us anything"
// (which can mean the response shape changed, the item was silently
// dropped, or our matcher failed). Inspect the journal for this code
// and check the raw push response next to the request body.
const noResponseStatusCode = "no_response_status"

// writeResultStatusSuccess is the per-node WriteResults Status string
// Keep returns for a successfully accepted push. Anything else (or
// missing) counts as failure for the purposes of synced_fp gating.
const writeResultStatusSuccess = "SUCCESS"

// writeResultStatusKeepDeletedOnPush is the synthetic LastFailureCode
// for the case where our push for a LIST_ITEM came back with the same
// item tombstoned in response.Nodes — Keep silently rejected the
// update by deleting it server-side (observed live when an UPDATE
// without baseVersion races concurrent edits, or on a few other edge
// cases). Distinct from `noResponseStatusCode` because the connector
// has positive evidence Keep saw the push and chose to delete.
const writeResultStatusKeepDeletedOnPush = "keep_deleted_on_push"

// ErrChecklistReaderUnavailable is returned by SyncToKeep when the
// connector wasn't given a ChecklistReader at startup. Indicates a
// wiring bug in bootstrap; callers should fail loudly so it gets
// noticed rather than silently dropping sync events.
var ErrChecklistReaderUnavailable = errors.New("keep bridge: checklist reader not configured (bootstrap bug)")

// logSkipOnce records a per-process "we already logged the skip for
// this binding" signal. Returns true exactly once per (profileID,
// page, listName) per Connector instance — the caller logs an INFO
// line on `true` and stays silent on `false`. The state persists
// only for the life of this process; a restart re-logs each
// still-un-migrated binding once. See the field comment on
// Connector.loggedSkip and plan §"Log throttling on the skip path"
// for the rationale (avoid 30s-cadence journal spam while still
// surfacing the transition into "stuck un-migrated").
func (c *Connector) logSkipOnce(profileID wikipage.PageIdentifier, page, listName string) bool {
	key := BindingKey{ProfileID: profileID, Page: page, ListName: listName}
	_, alreadyLogged := c.loggedSkip.LoadOrStore(key, struct{}{})
	return !alreadyLogged
}

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
//
// The function is long because it orchestrates a single atomic tick
// (read state → pull → apply inbound → diff outbound → push → persist
// outcomes) and splitting that into helpers without losing readability
// is a follow-up refactor; the inline structure makes the production-
// debugging trace much easier to follow against the live wire log.
//
//revive:disable-next-line:function-length,cognitive-complexity,cyclomatic
func (c *Connector) SyncToKeep(ctx context.Context, profileID wikipage.PageIdentifier, page, listName string) error {
	if c.checklistR == nil {
		return ErrChecklistReaderUnavailable
	}

	binding, found, err := c.store.FindSubscription(profileID, page, listName)
	if err != nil {
		return fmt.Errorf("load binding: %w", err)
	}
	if !found {
		// No binding for this checklist — nothing to sync. Not an error;
		// the mutator hook fires on every checklist edit and shouldn't
		// require pre-checking whether a binding exists.
		return nil
	}

	// Migration gate: legacy bindings created before the fingerprint
	// rewrite have a flat id_map (just serverIDs, no synced_fp) and
	// MigratedFingerprints=false. The sync engine's divergence rule
	// requires a real synced_fp baseline; running it on a zero-valued
	// baseline would silently rebaseline on the first tick (the "lazy
	// first-tick" approach the plan rejected). Skip — eagerly — until
	// the KeepBridgeFingerprintMigrationJob populates synced_fp and
	// stamps the flag. Throttled INFO log on the first skip per binding
	// per process so we see the transition into "stuck un-migrated"
	// without spamming the journal at the cron cadence.
	if !binding.MigratedFingerprints {
		if c.logSkipOnce(profileID, page, listName) && c.debug != nil {
			c.debug.Info("SyncToKeep skipping un-migrated binding profile=%s page=%s list=%s",
				profileID, page, listName)
		}
		return nil
	}

	checklist, err := c.checklistR.ListItems(ctx, page, listName)
	if err != nil {
		return fmt.Errorf("read wiki checklist: %w", err)
	}

	client, state, err := c.keepClientFor(ctx, profileID)
	if err != nil {
		return err
	}
	ownerEmail := state.Email

	now := c.clock.Now().UTC()

	// Fresh-bind branch: binding has no Keep serverID yet. Use the
	// bundled CreateListWithItems path (Keep rejects two-step
	// create-list + push-items with 500 Unknown Error). After this,
	// the binding is fully initialized and subsequent syncs go down
	// the steady-state pull/apply/push path below.
	if binding.KeepNoteID == "" {
		return c.bootstrapKeepListForBinding(ctx, profileID, binding, checklist, client, now)
	}

	pull, err := client.Changes(ctx, gateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--syncpull-%s", now.UnixMilli(), binding.KeepNoteID),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
		TargetVersion:   binding.KeepCursor,
	})
	if err != nil {
		return fmt.Errorf("sync pull: %w", err)
	}

	// Self-heal KeepNoteClientID for legacy bindings that were persisted
	// before this field existed. If the LIST node is in this pull (full
	// pulls always include it; incremental pulls only include it when
	// the LIST itself changed), capture its client-side `id`. Outbound
	// LIST pushes need `id != serverId` — Keep 500s on equal values.
	keepNoteClientIDMissingAfterPull := false
	if binding.KeepNoteClientID == "" {
		for _, n := range pull.Nodes {
			if n.Type == gateway.NodeTypeList && n.ServerID == binding.KeepNoteID && n.ID != "" {
				binding.KeepNoteClientID = n.ID
				break
			}
		}
		if binding.KeepNoteClientID == "" {
			keepNoteClientIDMissingAfterPull = true
		}
	}

	// Title sync: Keep is authoritative for the note name once bound.
	// If the pull echoes the LIST node with a non-empty Title that
	// differs from the binding's stored value, the user renamed the
	// note in the Keep app — mirror that into binding.KeepNoteTitle
	// so the macro UI surfaces the current name. The wiki list
	// identity (Page, ListName) is unchanged; only the cosmetic
	// title moves.
	for _, n := range pull.Nodes {
		if n.Type == gateway.NodeTypeList && n.ServerID == binding.KeepNoteID && n.Title != "" && n.Title != binding.KeepNoteTitle {
			binding.KeepNoteTitle = n.Title
			break
		}
	}

	// Capture the prior cursor so the truncation escape hatch can
	// distinguish "cursor advanced this tick" (progress) from "cursor
	// stuck at the same value" (no progress). A truncated pull never
	// advances the cursor below, so without this snapshot we can't
	// tell whether a later push advanced it.
	priorCursor := binding.KeepCursor

	// Track whether ANY synced_fp moved this tick (inbound apply,
	// outbound push success, applyInboundNewFromKeep adoption / fresh-
	// from-Keep imports). Combined with a cursor advance below, this
	// is the "progress" signal used by the truncation escape hatch:
	// chronic truncation that's still advancing items isn't masking
	// deletes, just paginating.
	progressed := false

	// Cursor advance: a successful non-truncated pull (even one with an
	// empty Nodes slice) advances the binding's KeepCursor to pull.ToVersion.
	// Skip advance only when pull.Truncated == true — leave the cursor
	// stale so the next tick re-pulls from the same point. Without this,
	// an empty incremental pull would re-fetch the same delta forever.
	if !pull.Truncated && pull.ToVersion != "" {
		binding.KeepCursor = pull.ToVersion
	}

	// forceFullResync: Keep is telling us our cursor is unusable and the
	// next pull must start from scratch. Drop our cursor; the next tick
	// will issue a full pull (TargetVersion=""), which is the only state
	// in which the class-4 hard-delete pass can correctly observe Keep's
	// complete current state.
	if pull.ForceFullResync {
		binding.KeepCursor = ""
	}

	// Self-heal continued: if KeepNoteClientID is still empty after this
	// pull, drop the cursor so the next tick's pull is full — full pulls
	// always echo the LIST node, which lets us populate the field
	// without operator intervention. Done AFTER cursor advance above so
	// this reset isn't overwritten.
	if keepNoteClientIDMissingAfterPull && binding.KeepNoteClientID == "" {
		binding.KeepCursor = ""
	}

	// Truncation streak: bump on truncated pull, reset on clean pull.
	// The two-condition escape hatch fires AFTER the rest of the tick
	// (so we can observe whether any synced_fp / cursor advance happened
	// despite the truncation). Source: plan §"Truncation escape hatch".
	if pull.Truncated {
		binding.TruncatedTickStreak++
	} else {
		binding.TruncatedTickStreak = 0
	}
	getMetrics().recordTruncationStreak(ctx, profileID, page, listName, int64(binding.TruncatedTickStreak))

	// Inbound apply: bring Keep-side changes into the wiki BEFORE
	// computing the outbound diff. Suppressed via the SyncSuppressor
	// so the mutator notifies emitted by AddItemForSync /
	// UpdateItemForSync / DeleteItemForSync don't loop back as fresh
	// sync triggers. updatedBinding carries any id_map changes (new
	// uid → serverID for items that arrived from Keep). inboundProgressed
	// is true if any synced_fp advance happened during the apply.
	updatedBinding, freshChecklist, inboundProgressed, err := c.applyInboundFromKeep(ctx, profileID, ownerEmail, binding, pull, checklist)
	if err != nil {
		return fmt.Errorf("inbound apply: %w", err)
	}
	binding = updatedBinding
	checklist = freshChecklist
	if inboundProgressed {
		progressed = true
	}

	// Resolve which Keep labels the bound LIST should carry. Reads the
	// host page's frontmatter tags and reconciles with the user's
	// existing Keep labels: existing names map by id; missing names
	// generate fresh Label CRUD entries that ride in the same push.
	pageTags, err := c.readPageTags(binding.Page)
	if err != nil {
		return fmt.Errorf("read page tags: %w", err)
	}
	// Absorb this pull's labels into the persisted name → MainID map
	// BEFORE resolving tags. Two effects:
	//   1. New labels (Keep just learned about them) become available
	//      to translator.MergeKeepLabels as the secondary lookup.
	//   2. Tombstoned labels (Deleted != zero) get evicted so the
	//      next push re-creates them with a fresh MainID instead of
	//      reusing a dead one.
	// On incremental pulls with no labels (the common case), this
	// loop is a no-op and the persisted map carries forward unchanged
	// — exactly what we want to stop the per-tick label-spam.
	if binding.LabelIDs == nil {
		binding.LabelIDs = map[string]string{}
	}
	for _, l := range pull.Labels {
		if l.Name == "" {
			continue
		}
		if !l.Deleted.IsZero() {
			delete(binding.LabelIDs, l.Name)
			continue
		}
		if l.MainID == "" {
			continue
		}
		binding.LabelIDs[l.Name] = l.MainID
	}
	labelPushEntries, listLabelIDs, err := translator.MergeKeepLabels(pageTags, binding.LabelIDs, pull.Labels, now)
	if err != nil {
		return fmt.Errorf("resolve labels: %w", err)
	}
	// Record any freshly-minted labels in the persisted map too, so
	// the next tick's MergeKeepLabels sees them as primary even
	// before Keep echoes them back in a pull.
	for _, l := range labelPushEntries {
		if l.Name == "" || l.MainID == "" {
			continue
		}
		binding.LabelIDs[l.Name] = l.MainID
	}

	// Walk wiki items: classify each as fresh or update; track which
	// existing map entries we covered so we can soft-delete the rest.
	//
	// baseVersions / originalClientIDs are derived from the JUST-
	// COMPLETED pull and are inherently sparse on incremental pulls
	// (only items that CHANGED since our last cursor appear). The
	// authoritative source for outbound updates is the persisted
	// ItemMapping.BaseVersion / ClientID — we use the per-pull maps
	// here only to UPDATE the persisted values when an item is in
	// the pull. Items absent from this pull keep whatever they had
	// from the last successful pull that included them.
	//
	// Why persisted values are required: Keep's optimistic-
	// concurrency-control 500s "Unknown Error" if a LIST_ITEM update
	// is sent without the baseVersion the server currently holds.
	// gkeepapi sets it via the loaded-from-server flow (node.py:
	// self._base_version = raw["baseVersion"]; emitted by Node.save()).
	// Before this fix, incremental pulls dropped baseVersion for any
	// item not changed since our last cursor → push 500s.
	baseVersions := make(map[string]string, len(pull.Nodes))
	originalClientIDs := make(map[string]string, len(pull.Nodes))
	// keepNodes is retained for inbound bookkeeping (see soft-delete
	// loop below: skip pushing a soft-delete for items Keep has
	// already removed). The outbound push gate, however, is the
	// fingerprint divergence rule — see the per-item decision in
	// the loop below.
	keepNodes := make(map[string]gateway.Node, len(pull.Nodes))
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
		keepNodes[n.ServerID] = n
	}
	// Update the persisted BaseVersion / ClientID on each ItemMapping
	// for serverIDs that DID appear in this pull. Items absent from
	// the pull retain whatever was previously persisted — this is the
	// fix for the incremental-pull regression that made every push
	// after the second tick 500 with empty baseVersion.
	if binding.ItemIDMap == nil {
		binding.ItemIDMap = map[string]ItemMapping{}
	}
	for uid, ib := range binding.ItemIDMap {
		serverID := ib.ServerID
		if serverID == "" {
			continue
		}
		mutated := false
		if bv, ok := baseVersions[serverID]; ok && bv != "" && ib.BaseVersion != bv {
			ib.BaseVersion = bv
			mutated = true
		}
		if cid, ok := originalClientIDs[serverID]; ok && cid != "" && ib.ClientID != cid {
			ib.ClientID = cid
			mutated = true
		}
		if mutated {
			binding.ItemIDMap[uid] = ib
		}
	}

	covered := make(map[string]bool, len(binding.ItemIDMap))
	pushNodes := make([]gateway.Node, 0, len(checklist.GetItems()))
	freshUIDs := make([]string, 0)        // index-aligned with the appended fresh items below
	freshClientIDs := make([]string, 0)
	// pushedFP[uid] = the Fingerprint we sent for that item. After a
	// successful push, each ItemMapping's synced_fp is advanced to its
	// entry here ONLY for nodes Keep reports as Status=SUCCESS in
	// WriteResults. Items missing from WriteResults or marked non-
	// SUCCESS leave synced_fp at its prior value so the next tick re-
	// pushes. Indexed by wiki uid so the response walk can match.
	pushedFP := make(map[string]translator.Fingerprint, len(checklist.GetItems()))
	// pushedClientID[uid] = the client_id we sent in the push for that
	// uid. Used to match Keep's per-node WriteResults entries (Keep
	// echoes the request's client-side `id` field, NOT array index).
	pushedClientID := make(map[string]string, len(checklist.GetItems()))
	for i, item := range checklist.GetItems() {
		uid := item.GetUid()
		ib := binding.ItemIDMap[uid]

		// Wiki-side re-edit auto-resets the failure count. If the user
		// edited this item in the wiki since the previous tick, the
		// current wiki fingerprint differs from LastObservedWiki* and
		// PushFailureCount/LastFailureCode/NextAttemptAt are cleared.
		// Done BEFORE the dead-letter check so a re-edit unblocks an
		// item that would otherwise be skipped.
		wikiFPNow := translator.FingerprintWiki(item)
		if ib.LastObservedWikiText != "" {
			observedFP := translator.Fingerprint{
				Text:      ib.LastObservedWikiText,
				Checked:   ib.LastObservedWikiChecked,
				SortValue: ib.LastObservedWikiSortValue,
			}
			if wikiFPNow != observedFP {
				ib.PushFailureCount = 0
				ib.LastFailureCode = ""
				ib.NextAttemptAt = time.Time{}
				binding.ItemIDMap[uid] = ib
			}
		}

		// Dead-letter skip: after deadLetterThreshold consecutive push
		// failures, stop attempting the item until the user clears the
		// dead-letter (gRPC ClearDeadLetter, task #83) or re-edits the
		// wiki side (handled above). Inbound apply still operates on
		// dead-lettered items; only the outbound push is gated.
		if ib.PushFailureCount >= deadLetterThreshold {
			covered[uid] = true
			getMetrics().recordPushAttempt(ctx, profileID, page, listName, PushAttemptStatusDeadLetteredSkip, 1)
			continue
		}

		// Backoff gate: skip until NextAttemptAt. Successful pushes and
		// wiki re-edits clear NextAttemptAt; only failures populate it
		// (now + min(60 * 2^(n-1), 3600) seconds). Items in their
		// backoff window are still considered "covered" so the soft-
		// delete loop below doesn't misinterpret them as wiki-deleted.
		if !ib.NextAttemptAt.IsZero() && now.Before(ib.NextAttemptAt) {
			covered[uid] = true
			continue
		}

		serverID := ib.ServerID
		node := translator.WikiToKeep(item, binding.KeepNoteID, serverID)
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
			// Fresh item (W1): no synced baseline — wiki is offering
			// it for the first time, so push unconditionally. Assign
			// a client_id and remember the uid so we can update the
			// map after the response.
			cid, idErr := generatePushClientID(now, i)
			if idErr != nil {
				return fmt.Errorf("generate client_id: %w", idErr)
			}
			node.ID = cid
			freshClientIDs = append(freshClientIDs, cid)
			freshUIDs = append(freshUIDs, uid)
			// Mark fresh uids as covered too — without this, the
			// post-push "drop uncovered id_map entries" loop deletes
			// the just-created entry for fresh items, losing both
			// the server-assigned ServerID and the synced_fp advance.
			covered[uid] = true
		} else {
			// Source `id` and `baseVersion` from the persisted
			// ItemMapping. The just-completed pull's per-pull maps
			// are sparse on incremental pulls — they ONLY contain
			// items that changed since our last cursor. Reading from
			// the persisted value keeps both fields populated for
			// items that were quiet on this tick. (The pre-loop
			// update above already pulled in any new values from
			// this pull's nodes.)
			if ib.ClientID != "" {
				node.ID = ib.ClientID
			}
			node.BaseVersion = ib.BaseVersion
			covered[uid] = true

			// Per-item decision rule (fingerprint divergence):
			//   wiki_fp   = translator.FingerprintWiki(item)
			//   synced_fp = last successful sync's content for this uid
			//   wiki_fp == synced_fp → skip (W0 / no-op tick: wiki
			//     hasn't been edited since the last sync).
			//   wiki_fp != synced_fp → push (W2/W3/W4/W5/W6 cases).
			//
			// Concurrent-edit case (wiki_fp != synced_fp AND
			// keep_fp != synced_fp): handled by applyInboundFromKeep
			// before this loop. If Keep diverged, the inbound apply
			// either rewrote wiki to match Keep (B1: keep wins) or
			// silently rebaselined synced_fp (when wiki_fp ==
			// keep_fp). Either way, by the time we reach this loop
			// wiki_fp == synced_fp again and we skip the push — no
			// special-case branch needed here.
			//
			// Why this replaces the old content-equality-against-
			// Keep skip: byte-equality only catches "wiki and Keep
			// happen to match", not "wiki was never edited since
			// last sync". The latter is the actual no-op condition
			// we want — and Keep's backend still 500s on no-op
			// multi-item updates, so the fingerprint check is also
			// the production-correctness gate.
			syncedFP := translator.FingerprintFromSyncedFields(ib.SyncedText, ib.SyncedChecked, ib.SyncedSortValue)
			if wikiFPNow == syncedFP {
				continue
			}
		}
		pushNodes = append(pushNodes, node)
		pushedFP[uid] = translator.Fingerprint{
			Text:      node.Text,
			Checked:   node.Checked,
			SortValue: node.SortValue,
		}
		pushedClientID[uid] = node.ID
	}

	// Soft-delete: any binding map entry whose uid isn't in the current
	// wiki items list got deleted wiki-side. Push Deleted=now so Keep
	// moves it to the trash on the user's phone.
	for uid, ib := range binding.ItemIDMap {
		serverID := ib.ServerID
		if covered[uid] || serverID == "" {
			continue
		}
		// Drop-vs-push decision: if a FULL pull confirms Keep no
		// longer has this item (absent from pull.Nodes), it was
		// already removed Keep-side — drop the stale id_map entry
		// instead of pushing a redundant soft-delete (Keep 500s on
		// "delete an already-deleted item").
		//
		// On INCREMENTAL pulls the heuristic is wrong: incremental
		// pulls only contain items that changed since the last
		// cursor, so unchanged-but-still-alive items are also absent.
		// Reading "absent from incremental pull" as "Keep deleted
		// it" silently swallowed every steady-state wiki delete in
		// production — the connector dropped the id_map entry and
		// the soft-delete never reached Keep.
		//
		// On incremental pulls, push the soft-delete unconditionally;
		// if Keep already deleted it, the post-push handler observes
		// a tombstone echo (or no echo) and the entry exits id_map
		// via the failure-handling branch.
		if _, present := keepNodes[serverID]; !present && !pull.Incremental {
			delete(binding.ItemIDMap, uid)
			continue
		}
		// Same id-vs-serverId distinction as the update path above:
		// prefer the persisted client_id (or fall back to serverID
		// if we never observed one). Reading from the persisted
		// ItemMapping rather than per-pull maps so soft-deletes
		// also survive incremental pulls without 500ing.
		clientID := serverID
		if ib.ClientID != "" {
			clientID = ib.ClientID
		}
		// Use `Deleted` not `Trashed` — verified via cmd/keep-debug
		// against a fresh sandbox: setting `trashed` on a LIST_ITEM
		// update returns stage3 HTTP 500 "Unknown Error", but
		// setting `deleted` accepts and removes the item. gkeepapi
		// exposes both as separate methods (trash/delete), but only
		// `deleted` makes it through Keep's Changes API on incremental
		// updates. Trashed-marked items in pulls are still recognized
		// as soft-deleted on the inbound side.
		pushNodes = append(pushNodes, gateway.Node{
			Kind:           "notes#node",
			ID:             clientID,
			ServerID:       serverID,
			ParentID:       binding.KeepNoteID,
			ParentServerID: binding.KeepNoteID,
			Type:           gateway.NodeTypeListItem,
			BaseVersion:    ib.BaseVersion,
			Timestamps: gateway.Timestamps{
				Deleted: now,
				Updated: now,
			},
		})
	}

	// Always include the LIST node in the push when we have label
	// assignments to record — Keep ignores label CRUD without a node
	// referencing the new IDs. Cheap: a labelIds-only push on an
	// existing LIST is one extra wireNode.
	if len(listLabelIDs) > 0 || len(labelPushEntries) > 0 {
		// Outbound LIST node MUST send `id != serverId`. Keep returns
		// stage3 HTTP 500 "Unknown Error" when they match. KeepNoteClientID
		// is captured at bind/bootstrap/migration time and re-captured
		// from full pulls via the self-heal block above. The fallback
		// to KeepNoteID exists only so logs surface the bug rather than
		// silently dropping the LIST node from the push — when this
		// branch fires the push will 500 and the next tick's full-pull
		// self-heal will repair the binding.
		listClientID := binding.KeepNoteClientID
		if listClientID == "" {
			listClientID = binding.KeepNoteID
		}
		// Title intentionally omitted on outbound LIST updates: once
		// bound, Keep is authoritative for the note name. Sending
		// binding.KeepNoteTitle here would clobber any rename the
		// user did in the Keep app (the binding only refreshes from
		// pulls, so the wiki-side value is stale by definition).
		// The initial create-note path in Bind/bootstrap is where
		// the title is set; from then on, inbound apply mirrors
		// Keep's value back into binding.KeepNoteTitle.
		listNode := gateway.Node{
			Kind:       "notes#node",
			ID:         listClientID,
			ServerID:   binding.KeepNoteID,
			Type:       gateway.NodeTypeList,
			LabelIDs:   listLabelIDs,
			Timestamps: gateway.Timestamps{Updated: now},
		}
		pushNodes = append(pushNodes, listNode)
	}

	if len(pushNodes) == 0 && len(labelPushEntries) == 0 {
		// No diff — wiki and binding map are in sync. Stamp end-of-
		// tick LastObservedWiki* (so a wiki re-edit after this tick is
		// detectable on the next one) and exit.
		c.maybeForceFullResyncOnTruncation(ctx, &binding, priorCursor, progressed, profileID)
		stampLastObservedWiki(&binding, checklist)
		getMetrics().recordDeadLetterCount(ctx, profileID, page, listName, countDeadLetters(binding.ItemIDMap))
		return c.markBindingSynced(profileID, binding, now)
	}

	pushSession, err := generatePushSessionID(now)
	if err != nil {
		return fmt.Errorf("generate session_id: %w", err)
	}
	resp, err := client.Changes(ctx, gateway.ChangesRequest{
		Nodes:           pushNodes,
		Labels:          labelPushEntries,
		TargetVersion:   pull.ToVersion,
		SessionID:       pushSession,
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		return fmt.Errorf("sync push: %w", err)
	}

	// Successful push: prefer the push response's ToVersion over the
	// pull's — the push is the more recent server state.
	if resp.ToVersion != "" {
		binding.KeepCursor = resp.ToVersion
	}

	// Update id_map: pick up server-assigned IDs for fresh items;
	// advance synced_fp ONLY for nodes Keep reports as SUCCESS in
	// WriteResults; bump PushFailureCount + record LastFailureCode for
	// the rest. Match WriteResults entries to pushed nodes by client-
	// side ID (Keep echoes the request's `id` field, NOT array index).
	// An item with no WriteResults entry counts as a failure with
	// LastFailureCode=noResponseStatusCode.
	if binding.ItemIDMap == nil {
		binding.ItemIDMap = map[string]ItemMapping{}
	}
	for _, n := range resp.Nodes {
		if n.Type != gateway.NodeTypeListItem {
			continue
		}
		// Match echoed-back fresh items by their client_id (n.ID).
		for i, cid := range freshClientIDs {
			if n.ID == cid && n.ServerID != "" {
				ib := binding.ItemIDMap[freshUIDs[i]]
				ib.ServerID = n.ServerID
				// Persist the synthetic client_id we generated for the
				// push so subsequent UPDATEs send {id: client_id,
				// serverId: server_id} (different values) instead of
				// {id: server_id, serverId: server_id} (same value).
				// Keep silently no-ops the same-value shape on
				// LIST_ITEM updates — verified live by missing
				// writeResults entries and last_failure_code =
				// no_response_status on items added wiki-side.
				ib.ClientID = cid
				binding.ItemIDMap[freshUIDs[i]] = ib
			}
		}
	}
	// Build a clientID → status map so the per-uid walk below can
	// look up Keep's verdict in O(1). Note: Keep emits writeResults
	// ONLY for top-level NOTE/LIST nodes — LIST_ITEM children are
	// NEVER in writeResults regardless of whether the push succeeded.
	// Verified live: every successful production push response has a
	// writeResults array containing exactly one entry (the LIST node)
	// no matter how many LIST_ITEMs were pushed. So for LIST_ITEMs
	// we must infer success from response.Nodes echo instead.
	statusByClientID := make(map[string]string, len(resp.WriteResults))
	for _, wr := range resp.WriteResults {
		statusByClientID[wr.ID] = wr.Status
	}
	// echoedAliveByClientID / echoedAliveByServerID: a LIST_ITEM is
	// echoed alive (no Trashed/Deleted timestamp) if Keep accepted
	// the push and the item still exists. echoedDeletedByClientID:
	// Keep rejected our update by tombstoning the item — that's a
	// failure, equivalent to a non-SUCCESS writeResults entry.
	echoedAliveByClientID := make(map[string]bool, len(resp.Nodes))
	echoedAliveByServerID := make(map[string]bool, len(resp.Nodes))
	echoedDeletedByClientID := make(map[string]bool, len(resp.Nodes))
	echoedDeletedByServerID := make(map[string]bool, len(resp.Nodes))
	for _, n := range resp.Nodes {
		if n.Type != gateway.NodeTypeListItem {
			continue
		}
		alive := n.Timestamps.Trashed.IsZero() && n.Timestamps.Deleted.IsZero()
		if alive {
			if n.ID != "" {
				echoedAliveByClientID[n.ID] = true
			}
			if n.ServerID != "" {
				echoedAliveByServerID[n.ServerID] = true
			}
		} else {
			if n.ID != "" {
				echoedDeletedByClientID[n.ID] = true
			}
			if n.ServerID != "" {
				echoedDeletedByServerID[n.ServerID] = true
			}
		}
	}
	// Advance synced_fp + reset failure state for SUCCESS uids; bump
	// PushFailureCount + set LastFailureCode + populate NextAttemptAt
	// for the rest. SyncedText / SyncedChecked / SyncedSortValue use
	// the fingerprint we SENT (not what Keep echoed back) — Keep
	// doesn't necessarily echo content fields on a successful update,
	// but the wire-state it now holds for that item is what we sent.
	for uid, fp := range pushedFP {
		clientID := pushedClientID[uid]
		status, hasStatus := statusByClientID[clientID]
		ib, ok := binding.ItemIDMap[uid]
		if !ok {
			// Fresh item whose response wasn't echoed back — no
			// id_map entry was created above. Skip; the next sync
			// will treat it as fresh again (idempotent).
			continue
		}
		// Success/failure decision tree:
		//  - writeResults SUCCESS → success
		//  - writeResults non-SUCCESS → failure (Keep explicitly rejected)
		//  - response echoes the node alive (by clientID or by the
		//    serverID we sent / now hold) → success (Keep accepted but
		//    didn't emit a writeResults entry, the production-normal
		//    path for LIST_ITEMs)
		//  - response echoes the node tombstoned → failure
		//  - neither in writeResults nor echoed → failure (we have no
		//    evidence Keep accepted the push)
		echoedAlive := echoedAliveByClientID[clientID] || echoedAliveByServerID[ib.ServerID]
		echoedDeleted := echoedDeletedByClientID[clientID] || echoedDeletedByServerID[ib.ServerID]
		isSuccess := false
		failureCode := ""
		switch {
		case hasStatus && status == writeResultStatusSuccess:
			isSuccess = true
		case hasStatus:
			failureCode = status
		case echoedAlive && !echoedDeleted:
			isSuccess = true
		case echoedDeleted:
			failureCode = writeResultStatusKeepDeletedOnPush
		default:
			failureCode = noResponseStatusCode
		}
		if isSuccess {
			ib.SyncedText = fp.Text
			ib.SyncedChecked = fp.Checked
			ib.SyncedSortValue = fp.SortValue
			ib.PushFailureCount = 0
			ib.LastFailureCode = ""
			ib.NextAttemptAt = time.Time{}
			progressed = true
			getMetrics().recordPushAttempt(ctx, profileID, page, listName, PushAttemptStatusSuccess, 1)
		} else {
			ib.PushFailureCount++
			ib.LastFailureCode = failureCode
			ib.NextAttemptAt = now.Add(pushFailureBackoff(ib.PushFailureCount))
			getMetrics().recordPushAttempt(ctx, profileID, page, listName, PushAttemptStatusFailure, 1)
		}
		binding.ItemIDMap[uid] = ib
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

	// End-of-tick: stamp LastObservedWiki* for every uid in id_map
	// that's also present in the current wiki snapshot. Drives the
	// next tick's "did the user re-edit the wiki side?" check.
	c.maybeForceFullResyncOnTruncation(ctx, &binding, priorCursor, progressed, profileID)
	stampLastObservedWiki(&binding, checklist)
	getMetrics().recordDeadLetterCount(ctx, profileID, page, listName, countDeadLetters(binding.ItemIDMap))

	return c.markBindingSynced(profileID, binding, now)
}

// countDeadLetters reports how many entries in idMap are currently
// at-or-above the dead-letter threshold. Used as the snapshot value
// for the keep_bridge_dead_letter_count gauge at end of each
// SyncToKeep tick.
func countDeadLetters(idMap map[string]ItemMapping) int64 {
	var n int64
	for _, ib := range idMap {
		if ib.PushFailureCount >= deadLetterThreshold {
			n++
		}
	}
	return n
}

// maybeForceFullResyncOnTruncation implements the two-condition escape
// hatch from the plan §"Truncation escape hatch": if the binding has
// hit `truncationResyncThreshold` consecutive truncated pulls (condition
// A) AND no progress was made on this tick — neither the cursor
// advanced nor any synced_fp updates landed (condition B) — drop the
// cursor to force a full resync on the next tick and reset the streak
// counter.
//
// Why both conditions: chronic-but-progressing truncation is just
// pagination of a large account (legitimate); chronic-and-stuck is
// the actual masking-deletes risk this guard protects against. The
// reset gives the next tick a clean baseline regardless of which
// branch fires.
//
// `progressed` is intentionally a boolean — it's the per-tick
// summary of whether *any* synced_fp moved during inbound apply.
// The caller computes it from the apply pass's iteration; turning
// it into a struct or sentinel value would be ceremony with no
// information added.
//
//revive:disable-next-line:flag-parameter
func (c *Connector) maybeForceFullResyncOnTruncation(ctx context.Context, binding *Subscription, priorCursor string, progressed bool, profileID wikipage.PageIdentifier) {
	if binding.TruncatedTickStreak < truncationResyncThreshold {
		return
	}
	cursorAdvanced := binding.KeepCursor != "" && binding.KeepCursor != priorCursor
	if cursorAdvanced || progressed {
		// Condition A fired but B did not: pagination, not masking.
		// Leave streak elevated; another non-truncated tick will reset
		// it via the streak-reset branch in SyncToKeep.
		return
	}
	binding.KeepCursor = ""
	binding.TruncatedTickStreak = 0
	getMetrics().recordCursorReset(ctx, profileID, binding.Page, binding.ListName, CursorResetReasonChronicTruncation)
	if c.debug != nil {
		c.debug.Info("keep bridge: chronic truncation without progress (streak=%d) — dropping cursor to force full resync next tick (page=%s list=%s)",
			truncationResyncThreshold, binding.Page, binding.ListName)
	}
}

// stampLastObservedWiki records the current wiki fingerprint for every
// uid still in the id_map at the end of a SyncToKeep tick. The next
// tick compares the fresh wiki_fp against this snapshot to detect
// "user re-edited locally since the last tick" — which resets
// PushFailureCount and clears any backoff (the obvious user-fix path
// after a dead-letter). Captured at end-of-tick, not start, so
// intra-tick wiki edits aren't missed.
func stampLastObservedWiki(binding *Subscription, checklist *apiv1.Checklist) {
	pairedUIDs := make(map[string]struct{}, len(binding.ItemIDMap))
	for uid := range binding.ItemIDMap {
		pairedUIDs[uid] = struct{}{}
	}
	fingerprints := translator.LastObservedWikiFingerprints(pairedUIDs, checklist)
	for uid, fp := range fingerprints {
		ib, ok := binding.ItemIDMap[uid]
		if !ok {
			continue
		}
		ib.LastObservedWikiText = fp.Text
		ib.LastObservedWikiChecked = fp.Checked
		ib.LastObservedWikiSortValue = fp.SortValue
		binding.ItemIDMap[uid] = ib
	}
}

// pushFailureBackoff returns the wait duration after the n-th
// consecutive per-item push failure: min(60 * 2^(n-1), 3600) seconds.
// n is the post-increment PushFailureCount (so n=1 returns 60s, n=2
// returns 120s, ..., capped at 3600s = 1h).
func pushFailureBackoff(n int) time.Duration {
	if n < 1 {
		return 0
	}
	seconds := pushFailureBackoffBaseSeconds
	for i := 1; i < n; i++ {
		seconds *= 2
		if seconds >= pushFailureBackoffMaxSeconds {
			seconds = pushFailureBackoffMaxSeconds
			break
		}
	}
	return time.Duration(seconds) * time.Second
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
func (c *Connector) bootstrapKeepListForBinding(ctx context.Context, profileID wikipage.PageIdentifier, binding Subscription, checklist *apiv1.Checklist, client KeepClient, now time.Time) error {
	wikiItems := checklist.GetItems()
	specs := make([]gateway.ListItemSpec, len(wikiItems))
	for i, it := range wikiItems {
		// Same encoder the steady-state SyncToKeep uses, so the
		// initial push and subsequent diff-pushes produce
		// byte-identical wire shapes for the same wiki state.
		head := translator.EncodeTextWithTags(it.GetText(), it.GetTags())
		text := head
		if d := it.GetDescription(); d != "" {
			text = head + translator.DescriptionSeparator + d
		}
		specs[i] = gateway.ListItemSpec{
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
	// Capture the LIST node's client-side `id` so subsequent outbound
	// LIST updates send `id != serverId` (Keep 500s when they match).
	binding.KeepNoteClientID = result.ListClientID
	if binding.ItemIDMap == nil {
		binding.ItemIDMap = map[string]ItemMapping{}
	}
	for i, serverID := range result.ItemServerIDs {
		if i >= len(wikiItems) || serverID == "" {
			continue
		}
		if uid := wikiItems[i].GetUid(); uid != "" {
			ib := binding.ItemIDMap[uid]
			ib.ServerID = serverID
			binding.ItemIDMap[uid] = ib
		}
	}

	// Born-migrated: a fresh bootstrap-push is a complete sync; mark
	// the binding as having complete fingerprint state so the gate at
	// the top of SyncToKeep doesn't reject it on next tick.
	binding.MigratedFingerprints = true
	_ = now // reserved for future per-item synced_fp stamping

	// Suppress is unnecessary here — CreateListWithItems doesn't
	// touch wiki state, just reads — but persistBindingMap uses the
	// store's per-profile mutex so concurrent bind/cron interleavings
	// can't shred the id_map.
	return c.persistBindingMap(profileID, binding)
}

// markBindingSynced persists the updated binding (id_map changes,
// KeepCursor advance, per-item synced_fp updates). Holds the per-
// profile mutex via SubscriptionStore so concurrent syncs serialize.
//
// `now` is reserved for future per-item synced_fp stamping; not
// consulted by the gate logic, which uses content fingerprints
// instead of timestamps.
func (c *Connector) markBindingSynced(profileID wikipage.PageIdentifier, binding Subscription, _ time.Time) error {
	state, err := c.store.LoadState(profileID)
	if err != nil {
		return fmt.Errorf("reload state for sync persist: %w", err)
	}
	for i, b := range state.Subscriptions {
		if b.Page == binding.Page && b.ListName == binding.ListName {
			state.Subscriptions[i] = binding
			break
		}
	}
	return c.store.SaveState(profileID, state)
}

// applyInboundFromKeep reconciles Keep-side changes into the wiki
// before the outbound diff runs. Returns the (possibly updated)
// binding (id_map gains entries for fresh-from-Keep items, synced_fp
// advances on every successful apply) and a freshly-loaded checklist
// reflecting the post-apply state.
//
// The classification is the per-item divergence rule from the plan:
//
//	wiki_fp   = translator.FingerprintWiki(wiki item) | NIL if absent
//	keep_fp   = translator.FingerprintKeep(node)      | NIL if trashed/deleted/absent
//	synced_fp = FingerprintFromItemBinding | NIL if not in id_map
//
//	wiki_diverged := wiki_fp != synced_fp
//	keep_diverged := keep_fp != synced_fp
//
// Cells handled here (inbound):
//
//	¬W ∧  K ∧ M=none                           → adopt-by-text else AddItemForSync
//	 W ∧  K ∧ M=correct ∧ ¬wd ∧ ¬kd            → no-op
//	 W ∧  K ∧ M=correct ∧  wd ∧ ¬kd            → defer to outbound (push wiki)
//	 W ∧  K ∧ M=correct ∧ ¬wd ∧  kd            → apply Keep
//	 W ∧  K ∧ M=correct ∧  wd ∧  kd            → conflict; "Keep wins" (B1)
//	 K trashed/deleted ∧ M=correct             → apply delete + drop id_map (K4/K5)
//	 W ∧ ¬K ∧ M=correct ∧ ¬wd                  → apply Keep hard-delete (bug 1 fix)
//	 W ∧ ¬K ∧ M=correct ∧  wd                  → defer to outbound (push wiki as fresh)
//	¬W ∧ ¬K                                    → drop stale id_map (no-op)
//
// Special case "wd ∧ kd ∧ wiki_fp == keep_fp": both sides drifted to
// the same content (typical of a bridge-restart-recovery race or a
// first-tick rebaseline where wiki and Keep happen to agree). The
// unified Keep-wins branch would call UpdateItemForSync with the same
// content the wiki already has — operationally a no-op but a wasted
// mutator call. Detect this and silent-rebaseline: advance synced_fp
// without invoking the mutator.
//
// Suppression: each inbound mutation fires the mutator's notify hook,
// which would normally enqueue another sync. The suppressor blocks
// scheduleSync for this binding's key for the duration of the apply
// pass (refcounted). Outbound push then runs unsuppressed; if it
// itself produces new wiki state via the response (e.g. new Keep
// serverIDs), those land via the response-walk in the caller.
//revive:disable-next-line:function-result-limit,function-length,cognitive-complexity,cyclomatic
func (c *Connector) applyInboundFromKeep(ctx context.Context, profileID wikipage.PageIdentifier, ownerEmail string, binding Subscription, pull gateway.ChangesResponse, currentChecklist *apiv1.Checklist) (Subscription, *apiv1.Checklist, bool, error) {
	if c.checklistW == nil {
		// No mutator wired — outbound-only mode. Return inputs as-is.
		return binding, currentChecklist, false, nil
	}
	if c.suppressor != nil {
		c.suppressor.Suppress(profileID, binding.Page, binding.ListName)
		defer c.suppressor.Unsuppress(profileID, binding.Page, binding.ListName)
	}

	// Reverse map: serverID → wiki uid, from the binding's id_map.
	rev := make(map[string]string, len(binding.ItemIDMap))
	for uid, ib := range binding.ItemIDMap {
		rev[ib.ServerID] = uid
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
		// Render the wiki item the same way translator.WikiToKeep would so the
		// match key aligns with what's actually flowing on the wire.
		head := translator.EncodeTextWithTags(it.GetText(), it.GetTags())
		full := head
		if d := it.GetDescription(); d != "" {
			full = head + translator.DescriptionSeparator + d
		}
		// First wiki item wins on duplicate-text ties — same convention
		// as seedIDMapFromExistingList.
		if _, exists := wikiByText[full]; !exists {
			wikiByText[full] = it.GetUid()
		}
	}

	if binding.ItemIDMap == nil {
		binding.ItemIDMap = map[string]ItemMapping{}
	}

	// progressed tracks whether ANY synced_fp moved during this apply
	// — counts apply-Keep updates, silent rebaselines (B1 same-content),
	// and adopt/AddItemForSync via applyInboundNewFromKeep. Surfaces
	// to SyncToKeep via the return value so the truncation escape
	// hatch can distinguish stuck from progressing.
	progressed := false

	// Walk the pull. Each list item produces one decision under the
	// per-item divergence rule.
	for _, n := range pull.Nodes {
		serverID := n.ServerID
		if serverID == "" {
			continue
		}
		isAlive := n.Timestamps.Trashed.IsZero() && n.Timestamps.Deleted.IsZero()
		uid, knownToWiki := rev[serverID]
		// Type filter: alive nodes must declare LIST_ITEM. Tombstones
		// strip the type field entirely (verified live: Keep emits
		// {id, serverId, timestamps.deleted=epoch+1ms} with no type
		// for soft-deleted LIST_ITEMs on incremental pulls). For
		// tombstones, identify by ServerID-in-id_map — that's our
		// pairing handle and tells us this is a LIST_ITEM we care
		// about.
		if isAlive {
			if n.Type != gateway.NodeTypeListItem {
				continue
			}
		} else if !knownToWiki {
			// Tombstone for a serverID this binding never paired with;
			// not our concern.
			continue
		}
		// Parent filter for alive nodes (tombstones strip parent
		// fields). Tombstones already gated above by id_map match.
		if isAlive {
			if n.ParentID != binding.KeepNoteID && n.ParentServerID != binding.KeepNoteID {
				continue
			}
		}

		// K trashed/deleted ∧ M=correct → apply delete + drop id_map.
		if !isAlive {
			if knownToWiki {
				if delErr := c.checklistW.DeleteItemForSync(ctx, binding.Page, binding.ListName, ownerEmail, uid); delErr != nil && c.debug != nil {
					c.debug.Info("applyInboundFromKeep: DeleteItemForSync(uid=%s, serverID=%s) for Keep tombstone failed: %v",
						uid, serverID, delErr)
				}
				delete(binding.ItemIDMap, uid)
				progressed = true
			}
			// ¬W ∧ ¬K (id_map doesn't have it either) → nothing to do.
			continue
		}

		// K alive from here on.
		if !knownToWiki {
			// ¬W relative to id_map; not yet paired. Either adopt by
			// text (boomeranged push) or apply as a fresh Keep add.
			if c.applyInboundNewFromKeep(ctx, ownerEmail, &binding, n, wikiByText) {
				progressed = true
			}
			continue
		}

		// W ∧ K ∧ M=correct: compute divergence.
		wikiItem, hasWiki := wikiByUID[uid]
		if !hasWiki {
			// id_map says we have a uid that the wiki actually doesn't
			// have. Treat as ¬W ∧ K ∧ M=correct → outbound will push
			// soft-delete (W7). Inbound: nothing to do here.
			continue
		}
		ib := binding.ItemIDMap[uid]
		wikiFP := translator.FingerprintWiki(wikiItem)
		keepFP := translator.FingerprintKeep(n)
		syncedFP := translator.FingerprintFromSyncedFields(ib.SyncedText, ib.SyncedChecked, ib.SyncedSortValue)
		wikiDiverged := wikiFP != syncedFP
		keepDiverged := keepFP != syncedFP

		switch {
		case !wikiDiverged && !keepDiverged:
			// W0 — no-op.
			continue

		case wikiDiverged && !keepDiverged:
			// W2/W3/W4/W5/W6 — wiki edited from baseline; outbound push
			// will carry it. Inbound does nothing.
			continue

		case !wikiDiverged && keepDiverged:
			// K2/K3 — only Keep changed. Apply Keep; advance synced_fp.
			if err := c.applyKeepUpdate(ctx, ownerEmail, binding.Page, binding.ListName, uid, n); err == nil {
				ib.SyncedText = keepFP.Text
				ib.SyncedChecked = keepFP.Checked
				ib.SyncedSortValue = keepFP.SortValue
				binding.ItemIDMap[uid] = ib
				progressed = true
			}

		default:
			// wikiDiverged && keepDiverged — conflict. "Keep wins" (B1).
			// Special case: wiki_fp == keep_fp means both happened to
			// drift to the same content (or the synced_fp was empty
			// and both sides have the same value). Don't burn a mutator
			// call — silent-rebaseline.
			if wikiFP == keepFP {
				ib.SyncedText = keepFP.Text
				ib.SyncedChecked = keepFP.Checked
				ib.SyncedSortValue = keepFP.SortValue
				binding.ItemIDMap[uid] = ib
				progressed = true
				continue
			}
			if err := c.applyKeepUpdate(ctx, ownerEmail, binding.Page, binding.ListName, uid, n); err == nil {
				ib.SyncedText = keepFP.Text
				ib.SyncedChecked = keepFP.Checked
				ib.SyncedSortValue = keepFP.SortValue
				binding.ItemIDMap[uid] = ib
				progressed = true
			}
		}
	}

	// Class 4 — Keep hard-delete pass: Keep clients (notably the mobile
	// app's swipe-to-delete) remove items entirely instead of flipping
	// a Trashed/Deleted timestamp — the item is just absent from the
	// next pull. The walk above only fires for items that are still
	// in the pull (alive or tombstoned), so without this pass, hard-
	// deleted items orphan in the id_map and stay alive wiki-side.
	//
	// **Critical invariant**: this pass deletes ONLY wiki items that
	// were previously paired with a Keep node (id_map entry exists
	// with non-empty serverID). Wiki-only items that were never pushed
	// to Keep have no id_map entry and are never touched here. Keep
	// is NOT authoritative over wiki state — only over the lifecycle
	// of items established as paired.
	//
	// Per-item rule (W ∧ ¬K ∧ M=correct):
	//   - wiki_fp == synced_fp (¬wiki_diverged) → apply Keep hard-delete
	//     (DeleteItemForSync + drop id_map). This is bug-1 fix:
	//     wiki state matches the synced baseline, so the wiki has no
	//     uncommitted edit — it's safe to mirror Keep's deletion.
	//   - wiki_fp != synced_fp (wiki_diverged) → defer to outbound;
	//     the outbound diff will push the wiki edit as fresh, re-creating
	//     the item on Keep under a new serverID. Don't drop id_map
	//     here — outbound owns that.
	//
	// Guards (any one false → skip the entire pass for safety):
	//   - pull.Truncated: pagination, not deletion.
	//   - pull.Incremental: response only contains items that CHANGED
	//     since the request's TargetVersion. Unchanged items are simply
	//     omitted — their absence from pull.Nodes is NOT a deletion
	//     signal. This guard is the fix for the post-deploy mass-delete
	//     regression: the cursor-induced incremental pull previously
	//     made class-4 fire against id_map entries whose items were
	//     merely unchanged.
	//   - !binding.MigratedFingerprints: legacy binding without seeded
	//     synced_fp; the migration job will rebaseline first.
	//   - sanity check: if the pull contained ZERO of id_map's serverIDs,
	//     the response is suspect (auth blip, server-side filter, etc.)
	//     — refuse to mass-delete.
	if !pull.Truncated && !pull.Incremental && binding.MigratedFingerprints {
		keepHas := make(map[string]bool, len(pull.Nodes))
		for _, n := range pull.Nodes {
			if n.Type != gateway.NodeTypeListItem {
				continue
			}
			if n.ParentID != binding.KeepNoteID && n.ParentServerID != binding.KeepNoteID {
				continue
			}
			if n.ServerID == "" {
				continue
			}
			keepHas[n.ServerID] = true
		}
		// Sanity: confirm Keep returned at least one of the items
		// we expect to be there. Otherwise refuse to act on absence.
		anyExpectedSeen := false
		for _, ib := range binding.ItemIDMap {
			if keepHas[ib.ServerID] {
				anyExpectedSeen = true
				break
			}
		}
		if anyExpectedSeen || len(binding.ItemIDMap) == 0 {
			for uid, ib := range binding.ItemIDMap {
				serverID := ib.ServerID
				if serverID == "" || keepHas[serverID] {
					continue
				}
				wikiItem, hasWiki := wikiByUID[uid]
				if !hasWiki {
					// Wiki doesn't have it either; nothing to delete.
					// Drop the stale id_map entry.
					delete(binding.ItemIDMap, uid)
					progressed = true
					continue
				}
				// W ∧ ¬K ∧ M=correct: branch on wiki_diverged.
				wikiFP := translator.FingerprintWiki(wikiItem)
				syncedFP := translator.FingerprintFromSyncedFields(ib.SyncedText, ib.SyncedChecked, ib.SyncedSortValue)
				if wikiFP != syncedFP {
					// Wiki edited concurrently with Keep delete; defer
					// to outbound which pushes wiki as fresh (re-create
					// on Keep under a new serverID). Don't drop id_map
					// here — outbound owns that decision.
					continue
				}
				// ¬wiki_diverged: safe to apply Keep's hard-delete.
				if delErr := c.checklistW.DeleteItemForSync(ctx, binding.Page, binding.ListName, ownerEmail, uid); delErr != nil {
					if c.debug != nil {
						c.debug.Info("applyInboundFromKeep: DeleteItemForSync(uid=%s, serverID=%s) for Keep-side hard-delete failed: %v",
							uid, serverID, delErr)
					}
					continue
				}
				delete(binding.ItemIDMap, uid)
				progressed = true
			}
		}
	}

	// Reload wiki state — the apply mutated it.
	freshChecklist, err := c.checklistR.ListItems(ctx, binding.Page, binding.ListName)
	if err != nil {
		return binding, currentChecklist, progressed, fmt.Errorf("reload after inbound apply: %w", err)
	}

	// Persist binding's updated id_map (delete + new uid entries +
	// synced_fp advances).
	if persistErr := c.persistBindingMap(profileID, binding); persistErr != nil {
		// Persist failure is recoverable — outbound still works against
		// the in-memory map for this run; next run reloads from store.
		_ = persistErr
	}

	return binding, freshChecklist, progressed, nil
}

// applyInboundNewFromKeep handles ¬W ∧ K ∧ M=none: a Keep node that
// isn't yet paired with a wiki item. Adopt-by-text if a wiki item has
// byte-identical rendered text (typically a wiki-pushed copy that
// boomeranged back); else AddItemForSync to create a new wiki item.
//
// Adopt records the wiki uid → keep serverID mapping in id_map and
// seeds synced_fp from the Keep node's fingerprint (the adopted wiki
// item is, by definition, content-equal to Keep). AddItemForSync also
// seeds synced_fp from the Keep node, since we just imported its
// content verbatim into the wiki.
//
// Returns true if a new id_map entry was created (synced_fp seeded);
// the caller uses this signal as part of the "progress made this tick"
// computation for the truncation escape hatch.
func (c *Connector) applyInboundNewFromKeep(ctx context.Context, ownerEmail string, binding *Subscription, n gateway.Node, wikiByText map[string]string) bool {
	if existingUID, found := wikiByText[n.Text]; found {
		if _, alreadyMapped := binding.ItemIDMap[existingUID]; !alreadyMapped {
			keepFP := translator.FingerprintKeep(n)
			binding.ItemIDMap[existingUID] = ItemMapping{
				ServerID:        n.ServerID,
				SyncedText:      keepFP.Text,
				SyncedChecked:   keepFP.Checked,
				SyncedSortValue: keepFP.SortValue,
				BaseVersion:     n.BaseVersion,
				ClientID:        n.ID,
			}
			return true
		}
		return false
	}
	wikiItem, err := translator.KeepToWiki(n)
	if err != nil {
		// Skip malformed nodes rather than fail the whole sync.
		return false
	}
	desc := ""
	if wikiItem.Description != nil {
		desc = *wikiItem.Description
	}
	newUID, err := c.checklistW.AddItemForSync(ctx, binding.Page, binding.ListName, ownerEmail,
		wikiItem.GetText(), wikiItem.GetChecked(), wikiItem.GetTags(), desc, n.SortValue)
	if err != nil {
		return false
	}
	keepFP := translator.FingerprintKeep(n)
	binding.ItemIDMap[newUID] = ItemMapping{
		ServerID:        n.ServerID,
		SyncedText:      keepFP.Text,
		SyncedChecked:   keepFP.Checked,
		SyncedSortValue: keepFP.SortValue,
		BaseVersion:     n.BaseVersion,
		ClientID:        n.ID,
	}
	return true
}

// applyKeepUpdate dispatches an UpdateItemForSync for a Keep node whose
// content has diverged from the wiki/synced baseline. Returns any
// mutator error so the caller can decide whether to advance synced_fp
// (only on success).
func (c *Connector) applyKeepUpdate(ctx context.Context, ownerEmail, page, listName, uid string, n gateway.Node) error {
	converted, err := translator.KeepToWiki(n)
	if err != nil {
		return err
	}
	desc := ""
	if converted.Description != nil {
		desc = *converted.Description
	}
	if updateErr := c.checklistW.UpdateItemForSync(ctx, page, listName, ownerEmail, uid,
		converted.GetText(), converted.GetChecked(), converted.GetTags(), desc); updateErr != nil {
		if c.debug != nil {
			// Stop swallowing — when this fails the wiki silently
			// drifts out of sync with Keep, and the next outbound
			// push reverts Keep to wiki's stale state. Visibility
			// in journalctl is critical for diagnosing.
			c.debug.Info("applyInboundFromKeep: UpdateItemForSync(uid=%s, checked=%v) failed: %v",
				uid, converted.GetChecked(), updateErr)
		}
		return updateErr
	}
	return nil
}

// persistBindingMap writes the updated binding back to the store.
// Used after inbound apply to lock in id_map changes (new uids,
// dropped trashed entries) before the outbound push runs.
func (c *Connector) persistBindingMap(profileID wikipage.PageIdentifier, binding Subscription) error {
	state, err := c.store.LoadState(profileID)
	if err != nil {
		return err
	}
	for i, b := range state.Subscriptions {
		if b.Page == binding.Page && b.ListName == binding.ListName {
			state.Subscriptions[i] = binding
			break
		}
	}
	return c.store.SaveState(profileID, state)
}

// seedIDMapFromExistingList reconciles wiki items against an existing
// Keep list at bind time so future syncs don't duplicate items.
//
// Strategy: pull the bound list's existing LIST_ITEMs from Keep, then
// delegate the pure matching to translator.MatchWikiItemsToKeepNodes
// (exact head-line match: text + inline tags, before any "\n— description"
// suffix). Match-by-text is imperfect (collisions on identical-text
// items) but it's the only signal we have at bind time — Keep doesn't
// know wiki UIDs and wiki has no record of Keep serverIDs yet.
// Subsequent edits round-trip cleanly through the id_map.
//
// Returns:
//   - idMap: per-wiki-uid ItemMapping with ServerID, BaseVersion, and
//     ClientID populated from the bind-time pull. Seeding all three
//     here is what lets the very first outbound push from a fresh
//     bind avoid the "missing baseVersion → 500 Unknown Error" path.
//   - labelIDs: per-name → MainID map captured from the bind-time
//     pull's userInfo.labels. Without this, the first sync re-pulls
//     incrementally (no labels) and re-creates every label with a
//     fresh MainID, spamming the user's account.
//   - keepNoteClientID: the LIST node's client-generated `id` (distinct
//     from its server-assigned `serverId`). Captured so subsequent
//     outbound LIST updates carry `id != serverId` — Keep 500s when
//     the two are equal.
//
// Errors are swallowed — a failed pull at bind time should not block
// the bind itself; the next mutation will trigger a sync that
// re-discovers items via CreateListWithItems-style new-item pushes
// (which will dedupe-by-text on Keep's side... no, actually they
// won't — Keep accepts duplicate-text items. So this DOES matter, but
// we'd rather have a working bind than a hard failure if the pull
// flakes.).
func (c *Connector) seedIDMapFromExistingList(ctx context.Context, client KeepClient, listServerID string, wikiItems []InitialItem) (map[string]ItemMapping, map[string]string, string) {
	idMap := map[string]ItemMapping{}
	labelIDs := map[string]string{}
	var keepNoteClientID string
	now := c.clock.Now().UTC()
	pull, err := client.Changes(ctx, gateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--bindseed-%s", now.UnixMilli(), listServerID),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		return idMap, labelIDs, keepNoteClientID
	}
	// Capture the LIST node's client-side `id` so subsequent outbound
	// LIST updates send `id != serverId`. Keep 500s when they match.
	keepNoteClientID = translator.FindListClientID(pull.Nodes, listServerID)

	// Translator owns the pairing logic: build a normalized-text index
	// over live LIST_ITEMs under our list, then match wiki items by
	// text. We construct ItemMapping from the gateway-level seed match
	// fields here because ItemMapping is a sync-package type.
	seeds := translator.MatchWikiItemsToKeepNodes(pull.Nodes, listServerID, toSeedWikiItems(wikiItems))
	for uid, m := range seeds {
		idMap[uid] = ItemMapping{
			ServerID:    m.ServerID,
			BaseVersion: m.BaseVersion,
			ClientID:    m.ClientID,
		}
	}

	// Seed the per-name label MainID map from the bind-time pull so
	// MergeKeepLabels on the first post-bind sync uses persisted FKs
	// instead of emitting fresh label CRUD entries every tick.
	labelIDs = translator.IndexLabelsByName(pull.Labels)
	return idMap, labelIDs, keepNoteClientID
}

// toSeedWikiItems projects []InitialItem to the translator's
// SeedWikiItem shape (UID + Text only). The other InitialItem fields
// (Tags, Description, Checked) aren't consulted by base-text matching;
// they round-trip via SyncToKeep once pairing is established.
func toSeedWikiItems(items []InitialItem) []translator.SeedWikiItem {
	out := make([]translator.SeedWikiItem, len(items))
	for i, w := range items {
		out[i] = translator.SeedWikiItem{UID: w.UID, Text: w.Text}
	}
	return out
}

// readPageTags returns the union of (1) the host page's frontmatter
// tags array and (2) inline `#hashtag` markers in the page's markdown
// body. The inline source matters because most wiki pages use the
// `#tag` content syntax (extracted via internal/hashtags) rather than
// duplicating the same set in frontmatter. Returning only the
// frontmatter list silently dropped Keep-Label sync for every page
// that didn't maintain a parallel `tags = [...]` block.
//
// Returns an empty slice for "no tags" (not an error). Errors only on
// actual store failures; missing pages return ([], nil) — the binding
// might outlive the host page and we'd rather sync no labels than
// blow up the sync job.
func (c *Connector) readPageTags(page string) ([]string, error) {
	pageID := wikipage.PageIdentifier(page)
	out := make([]string, 0)
	seen := make(map[string]struct{})
	add := func(tag string) {
		if tag == "" {
			return
		}
		normalized := hashtags.Normalize(tag)
		if _, exists := seen[normalized]; exists {
			return
		}
		seen[normalized] = struct{}{}
		out = append(out, tag)
	}

	// 1. Frontmatter tags (explicit list).
	if _, fm, err := c.store.pages.ReadFrontMatter(pageID); err == nil {
		if raw, ok := fm["tags"]; ok {
			if arr, ok := raw.([]any); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok {
						add(s)
					}
				}
			}
		}
	}

	// 2. Inline #hashtag markers in the page body. The hashtags parser
	// already strips fenced code blocks and inline code so #-mentions
	// inside examples don't pollute the result.
	if _, body, err := c.store.pages.ReadMarkdown(pageID); err == nil {
		for _, tag := range hashtags.Extract(string(body)) {
			add(tag)
		}
	}

	return out, nil
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

// Unbind removes the calling user's binding for (page, listName) and
// releases the LeaseTable claim. Per ADR-0011's unsubscribe contract:
// per-checklist mutex acquire → write profile → release lease →
// release mutex.
func (c *Connector) Unbind(ctx context.Context, profileID wikipage.PageIdentifier, page, listName string) error {
	if err := c.leaseTable.WaitReady(ctx); err != nil {
		return fmt.Errorf("await lease-table ready: %w", err)
	}

	checklistKey := connectors.ChecklistKey{Page: page, ListName: listName}
	cerErr := c.leaseTable.WithChecklistLock(checklistKey, func() error {
		err := c.store.RemoveSubscription(profileID, page, listName)
		if errors.Is(err, ErrSubscriptionNotFound) {
			// Idempotent at the orchestrator boundary — UI calls this
			// on rebind/remove flows and shouldn't have to
			// disambiguate. Still release the lease in case the
			// LeaseTable carries a stale entry whose profile-side
			// record was already gone.
			c.leaseTable.Release(checklistKey)
			return nil
		}
		if err != nil {
			return err
		}
		c.leaseTable.Release(checklistKey)
		return nil
	})
	c.noteSubscriptionRemoved(BindingKey{ProfileID: profileID, Page: page, ListName: listName})
	return cerErr
}

// FindSubscription mirrors SubscriptionStore.FindSubscription for handler convenience.
func (c *Connector) FindSubscription(_ context.Context, profileID wikipage.PageIdentifier, page, listName string) (Subscription, bool, error) {
	return c.store.FindSubscription(profileID, page, listName)
}

// VerifyBinding pings Keep with the user's bearer to confirm the bound
// note still exists. Updates LastVerifiedAt on success. This is the v1
// sync stub — actual data round-trip lands as a follow-up; see the help
// page section "What sync does today".
//
// Pulled out as its own method so the scheduler can call it on a tick
// without knowing about Keep's wire shape.
func (c *Connector) VerifyBinding(ctx context.Context, profileID wikipage.PageIdentifier, binding Subscription) error {
	client, _, err := c.keepClientFor(ctx, profileID)
	if err != nil {
		return err
	}
	now := c.clock.Now()
	resp, err := client.Changes(ctx, gateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--verify-%s", now.UnixMilli(), binding.KeepNoteID),
		ClientTimestamp: now.UTC().Format("2006-01-02T15:04:05.000000Z"),
	})
	if err != nil {
		return err
	}
	for _, n := range resp.Nodes {
		if n.Type == gateway.NodeTypeList && n.ServerID == binding.KeepNoteID {
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

// ErrBoundNoteDeletedLocal mirrors gateway.ErrBoundNoteDeleted but is
// surfaced from VerifyBinding when the bound note simply isn't in the
// user's account anymore (e.g., they deleted it from the Keep app).
var ErrBoundNoteDeletedLocal = gateway.ErrBoundNoteDeleted

// MigrateSubscriptionFingerprints rebases a legacy binding's per-item
// synced_fp baseline so the new sync engine has a real merge-base to
// test divergence against. Called by the eager migration job (one
// per legacy binding) at startup; idempotent on already-migrated
// bindings.
//
// Algorithm (under the per-profile mutex for the entire cycle):
//
//  1. Load state. Find the binding. If MigratedFingerprints is
//     already true, return nil — concurrent migration jobs and
//     restart-resumed scans both land here harmlessly.
//  2. Authenticated full pull from Keep (empty target_version).
//  3. For each id_map entry whose serverID is in the pull AND wiki
//     has the uid:
//     - wiki_fp == keep_fp: silent rebaseline. Set synced_fp ←
//       wiki_fp. No mutator call (wiki content already matches Keep).
//     - wiki_fp != keep_fp: "Keep wins". UpdateItemForSync with
//       Keep's content; set synced_fp ← keep_fp.
//     - Either branch logs a forensic INFO line via c.debug with the
//       full payload (profileID, binding key, uid, serverID,
//       fingerprints, prior cursor) so an operator can audit what
//     migration did.
//  4. id_map entries whose serverID is NOT in the pull: drop the
//     entry. Keep already deleted that item; the pairing is gone.
//  5. Clear KeepCursor (set to "") so the FIRST post-migration sync
//     does a full pull. The class-4 hard-delete pass fires only on
//     full pulls, and the first post-migration tick must observe
//     Keep's complete state to reconcile the rebaselined id_map.
//  6. Stamp MigratedFingerprints = true.
//  7. Persist state.
//
// On error (network failure, auth_revoked, persist failure), the
// binding stays MigratedFingerprints=false; the eager migration's
// queue retries with backoff. SyncToKeep keeps skipping that
// binding via the gate until migration finally succeeds.
//
//revive:disable-next-line:function-length,cognitive-complexity,cyclomatic
func (c *Connector) MigrateSubscriptionFingerprints(ctx context.Context, profileID wikipage.PageIdentifier, page, listName string) error {
	if c.checklistR == nil {
		return ErrChecklistReaderUnavailable
	}

	// Pull happens BEFORE acquiring the lock — Keep API calls can
	// take seconds; holding the per-profile mutex across the network
	// round-trip would block concurrent Bind/Unbind/macro calls for
	// the entire pull. The lock-window is just the read-rebaseline-
	// write cycle below.
	client, state, err := c.keepClientFor(ctx, profileID)
	if err != nil {
		return fmt.Errorf("keep client for migration: %w", err)
	}
	ownerEmail := state.Email

	// Pre-check under the lock: the binding might already be migrated
	// by a concurrent run, or might not exist anymore. Check before
	// the (potentially expensive) pull.
	var preBinding Subscription
	var preFound bool
	preCheckErr := c.store.WithProfileLock(profileID, func() error {
		st, lerr := c.store.LoadStateLocked(profileID)
		if lerr != nil {
			return lerr
		}
		for _, b := range st.Subscriptions {
			if b.Page == page && b.ListName == listName {
				preBinding = b
				preFound = true
				return nil
			}
		}
		return nil
	})
	if preCheckErr != nil {
		return fmt.Errorf("pre-check binding for migration: %w", preCheckErr)
	}
	if !preFound {
		// Subscription was removed (e.g., user unbound between scan-time
		// and now). Nothing to migrate; succeed silently.
		return nil
	}
	if preBinding.MigratedFingerprints {
		// Already migrated — idempotent no-op. The flag is the
		// sole gate that enrolls a binding into the steady-state
		// sync path, so re-running this method on a migrated
		// binding does no work and emits no journal noise.
		return nil
	}

	// Read the wiki checklist OUTSIDE the lock — checklist reads do
	// not touch the binding store and can be slow. We re-read inside
	// the lock immediately before the rebaseline write so the
	// fingerprints we compute reflect the wiki state at write time.
	now := c.clock.Now().UTC()

	pull, pullErr := client.Changes(ctx, gateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--migrate-%s", now.UnixMilli(), preBinding.KeepNoteID),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
		// Empty TargetVersion → full pull. Migration must see the
		// authoritative current state of every paired item; an
		// incremental pull would miss items that haven't changed
		// since the legacy binding was first written.
		TargetVersion: "",
	})
	if pullErr != nil {
		return fmt.Errorf("migration pull: %w", pullErr)
	}

	// Build serverID → keep node for the rebaseline below. Filter
	// to LIST_ITEMs parented under the binding's note, alive only
	// (trashed/deleted items represent items Keep removed; the
	// "drop missing" branch handles them).
	keepByServerID := make(map[string]gateway.Node, len(pull.Nodes))
	// Capture the LIST node's client-side `id` so the migration can
	// persist it alongside the rebaseline. Without this, the first
	// post-migration push would reuse serverID for both `id` and
	// `serverId` and Keep 500s.
	var listClientIDFromPull string
	for _, n := range pull.Nodes {
		if n.Type == gateway.NodeTypeList && n.ServerID == preBinding.KeepNoteID {
			listClientIDFromPull = n.ID
			continue
		}
		if n.Type != gateway.NodeTypeListItem {
			continue
		}
		if n.ParentID != preBinding.KeepNoteID && n.ParentServerID != preBinding.KeepNoteID {
			continue
		}
		if !n.Timestamps.Trashed.IsZero() || !n.Timestamps.Deleted.IsZero() {
			continue
		}
		if n.ServerID == "" {
			continue
		}
		keepByServerID[n.ServerID] = n
	}

	// Rebaseline write window: read fresh state under the lock,
	// re-read wiki, walk id_map, persist. The lock spans every
	// store touch so concurrent macro actions / cron ticks
	// observe a consistent view.
	return c.store.WithProfileLock(profileID, func() error {
		st, lerr := c.store.LoadStateLocked(profileID)
		if lerr != nil {
			return fmt.Errorf("load state for migration: %w", lerr)
		}

		// Locate the binding by (page, listName); check the flag
		// once more under the lock — concurrent migration runs and
		// Bind/Unbind churn could have changed it.
		idx := -1
		for i, b := range st.Subscriptions {
			if b.Page == page && b.ListName == listName {
				idx = i
				break
			}
		}
		if idx < 0 {
			// Subscription was removed between pull and write. Not an
			// error — the user unbound and that's authoritative.
			return nil
		}
		binding := st.Subscriptions[idx]
		if binding.MigratedFingerprints {
			return nil
		}

		// Re-read wiki under the lock so wiki_fp reflects the
		// state we're about to baseline against. Wiki reads don't
		// hit the binding store, so this is safe under the lock.
		checklist, rerr := c.checklistR.ListItems(ctx, page, listName)
		if rerr != nil {
			return fmt.Errorf("read wiki checklist for migration: %w", rerr)
		}
		wikiByUID := make(map[string]*apiv1.ChecklistItem, len(checklist.GetItems()))
		for _, it := range checklist.GetItems() {
			wikiByUID[it.GetUid()] = it
		}

		priorCursor := binding.KeepCursor
		newIDMap := make(map[string]ItemMapping, len(binding.ItemIDMap))
		for uid, ib := range binding.ItemIDMap {
			serverID := ib.ServerID
			if serverID == "" {
				// Malformed legacy entry (no serverID) — drop it.
				continue
			}
			keepNode, hasKeep := keepByServerID[serverID]
			if !hasKeep {
				// Keep already removed this item. Pairing is gone;
				// drop the entry so subsequent syncs don't try to
				// touch a non-existent serverID.
				if c.debug != nil {
					c.debug.Info("MigrateSubscriptionFingerprints: dropping unpaired entry profile=%s page=%s list=%s uid=%s serverID=%s prior_cursor=%s",
						profileID, page, listName, uid, serverID, priorCursor)
				}
				continue
			}
			wikiItem, hasWiki := wikiByUID[uid]
			if !hasWiki {
				// id_map says we have a uid the wiki doesn't have —
				// mirrors the W7-style "id_map ahead of wiki"
				// state. Keep the entry as-is; the regular sync
				// engine will see ¬W ∧ K ∧ M=correct and push a
				// soft-delete on the next tick. Seed synced_fp
				// from Keep so the divergence rule has something
				// non-zero to test against.
				ib.SyncedText = keepNode.Text
				ib.SyncedChecked = keepNode.Checked
				ib.SyncedSortValue = keepNode.SortValue
				if keepNode.BaseVersion != "" {
					ib.BaseVersion = keepNode.BaseVersion
				}
				if keepNode.ID != "" {
					ib.ClientID = keepNode.ID
				}
				newIDMap[uid] = ib
				if c.debug != nil {
					c.debug.Info("MigrateSubscriptionFingerprints: orphan-uid baseline-from-keep profile=%s page=%s list=%s uid=%s serverID=%s keep_fp_text=%q prior_cursor=%s",
						profileID, page, listName, uid, serverID, keepNode.Text, priorCursor)
				}
				continue
			}
			wikiFP := translator.FingerprintWiki(wikiItem)
			keepFP := translator.FingerprintKeep(keepNode)
			if wikiFP == keepFP {
				// Silent rebaseline: wiki content already matches
				// Keep, just need a synced_fp to anchor the merge-
				// base. No mutator call.
				ib.SyncedText = wikiFP.Text
				ib.SyncedChecked = wikiFP.Checked
				ib.SyncedSortValue = wikiFP.SortValue
				if keepNode.BaseVersion != "" {
					ib.BaseVersion = keepNode.BaseVersion
				}
				if keepNode.ID != "" {
					ib.ClientID = keepNode.ID
				}
				newIDMap[uid] = ib
				getMetrics().recordSilentRebaseline(ctx, profileID, page, listName)
				if c.debug != nil {
					c.debug.Info("MigrateSubscriptionFingerprints: silent-rebaseline profile=%s page=%s list=%s uid=%s serverID=%s fp_text=%q fp_checked=%t fp_sort=%q prior_cursor=%s",
						profileID, page, listName, uid, serverID, wikiFP.Text, wikiFP.Checked, wikiFP.SortValue, priorCursor)
				}
				continue
			}
			// "Keep wins": apply Keep to wiki, then baseline to keep_fp.
			if c.checklistW != nil {
				keepItemForApply, kerr := translator.KeepToWiki(keepNode)
				if kerr != nil {
					return fmt.Errorf("keep→wiki conversion for migration uid=%s: %w", uid, kerr)
				}
				if uerr := c.checklistW.UpdateItemForSync(
					ctx, page, listName, ownerEmail, uid,
					keepItemForApply.GetText(),
					keepItemForApply.GetChecked(),
					keepItemForApply.GetTags(),
					keepItemForApply.GetDescription(),
				); uerr != nil {
					return fmt.Errorf("update item for migration uid=%s: %w", uid, uerr)
				}
			}
			ib.SyncedText = keepFP.Text
			ib.SyncedChecked = keepFP.Checked
			ib.SyncedSortValue = keepFP.SortValue
			if keepNode.BaseVersion != "" {
				ib.BaseVersion = keepNode.BaseVersion
			}
			if keepNode.ID != "" {
				ib.ClientID = keepNode.ID
			}
			newIDMap[uid] = ib
			if c.debug != nil {
				c.debug.Info("MigrateSubscriptionFingerprints: keep-wins profile=%s page=%s list=%s uid=%s serverID=%s wiki_fp_text=%q keep_fp_text=%q prior_cursor=%s",
					profileID, page, listName, uid, serverID, wikiFP.Text, keepFP.Text, priorCursor)
			}
		}

		binding.ItemIDMap = newIDMap
		// Seed the per-binding label name → MainID map from the full
		// migration pull. Without this, the very first post-migration
		// sync would see persisted_label_ids={} and fall back to the
		// per-pull byName index — which is fine on this tick (the
		// migration is a full pull) but for subsequent incremental
		// pulls (which return no labels at all) the map would still
		// be empty and the connector would re-mint a fresh MainID per
		// label per tick, spamming Keep with duplicate labels.
		// Tombstoned labels are skipped — re-create rather than revive.
		labelIDs := make(map[string]string, len(pull.Labels))
		for _, l := range pull.Labels {
			if l.Name == "" || l.MainID == "" {
				continue
			}
			if !l.Deleted.IsZero() {
				continue
			}
			labelIDs[l.Name] = l.MainID
		}
		binding.LabelIDs = labelIDs
		// Persist the LIST node's client-side `id` captured from the
		// migration's full pull. Future outbound LIST updates send
		// `id != serverId`; without this, Keep 500s on every push.
		if listClientIDFromPull != "" {
			binding.KeepNoteClientID = listClientIDFromPull
		}
		// Clear the cursor so the FIRST post-migration sync performs a
		// full (non-incremental) pull. This is essential: the class-4
		// hard-delete pass fires only on full pulls, and the first
		// post-migration tick must see Keep's complete current state to
		// reconcile against the rebaselined id_map. Once that tick lands
		// a fresh ToVersion, subsequent ticks resume incremental pulls.
		// Source: post-deploy mass-delete bug remediation.
		binding.KeepCursor = ""
		binding.MigratedFingerprints = true
		st.Subscriptions[idx] = binding
		if perr := c.store.SaveStateLocked(profileID, st); perr != nil {
			return fmt.Errorf("persist migrated binding: %w", perr)
		}
		// Clear the migration-pending gauge: the binding is now in
		// the new shape and SyncToKeep will start operating on it.
		getMetrics().setMigrationPending(ctx, profileID, page, listName, false)
		return nil
	})
}

// deriveDeviceID returns a stable 16-hex-char device id derived from the
// profile id. Matches the gpsoauth requirement of a stable per-account
// android id without reusing any real device's id.
func deriveDeviceID(profileID wikipage.PageIdentifier) string {
	sum := sha256.Sum256([]byte(profileID))
	return hex.EncodeToString(sum[:8]) // 16 hex chars
}

// DeadLetterEntry is one dead-lettered item surfaced to gRPC clients.
// Mirrors the api/v1.DeadLetterItem proto shape so the handler is a
// thin translation layer; the bridge package never imports proto types.
type DeadLetterEntry struct {
	// ItemUID is the wiki-side stable item identifier.
	ItemUID string
	// Text is the most recent observed wiki text, used by the macro
	// UI to render a recognizable row label.
	Text string
	// PushFailureCount is the consecutive failure count.
	PushFailureCount int
	// LastFailureCode is Keep's per-node WriteResults Status string
	// (or our internal classifier) for the most recent failure.
	LastFailureCode string
}

// ListDeadLetters returns every ItemMapping in the (profileID, page,
// listName) binding whose PushFailureCount is at-or-above the dead-
// letter threshold. Used by the <keep-connect> macro UI to render
// per-item failure rows. Returns an empty slice (not an error) when
// the binding has no dead-lettered items, and ErrSubscriptionNotFound when
// the binding itself doesn't exist.
func (c *Connector) ListDeadLetters(_ context.Context, profileID wikipage.PageIdentifier, page, listName string) ([]DeadLetterEntry, error) {
	binding, found, err := c.store.FindSubscription(profileID, page, listName)
	if err != nil {
		return nil, fmt.Errorf("load binding for dead-letter list: %w", err)
	}
	if !found {
		return nil, ErrSubscriptionNotFound
	}
	out := make([]DeadLetterEntry, 0)
	for uid, ib := range binding.ItemIDMap {
		if ib.PushFailureCount < deadLetterThreshold {
			continue
		}
		out = append(out, DeadLetterEntry{
			ItemUID:          uid,
			Text:             ib.LastObservedWikiText,
			PushFailureCount: ib.PushFailureCount,
			LastFailureCode:  ib.LastFailureCode,
		})
	}
	return out, nil
}

// ClearDeadLetter resets the failure-tracking fields on one
// ItemMapping so the next sync tick re-attempts the push.
// Specifically: PushFailureCount → 0, LastFailureCode → "",
// NextAttemptAt → zero. Synced{Text,Checked,SortValue} (the merge-
// base baseline) and ServerID are preserved — clearing only undoes
// the failure state, not the divergence rule's anchor. Returns
// ErrSubscriptionNotFound if the binding doesn't exist and
// ErrDeadLetterItemNotFound if the binding has no ItemMapping for
// the given uid.
//
// Per the plan §"Concurrent macro action vs sync tick", a Clear
// that races against an in-flight SyncToKeep takes effect on the
// next tick (≤30s) — last-writer-wins on the binding store mutex.
func (c *Connector) ClearDeadLetter(_ context.Context, profileID wikipage.PageIdentifier, page, listName, itemUID string) error {
	return c.store.WithProfileLock(profileID, func() error {
		st, err := c.store.LoadStateLocked(profileID)
		if err != nil {
			return fmt.Errorf("load state for clear dead-letter: %w", err)
		}
		idx := -1
		for i, b := range st.Subscriptions {
			if b.Page == page && b.ListName == listName {
				idx = i
				break
			}
		}
		if idx == -1 {
			return ErrSubscriptionNotFound
		}
		ib, ok := st.Subscriptions[idx].ItemIDMap[itemUID]
		if !ok {
			return ErrDeadLetterItemNotFound
		}
		ib.PushFailureCount = 0
		ib.LastFailureCode = ""
		ib.NextAttemptAt = time.Time{}
		st.Subscriptions[idx].ItemIDMap[itemUID] = ib
		if perr := c.store.SaveStateLocked(profileID, st); perr != nil {
			return fmt.Errorf("persist cleared dead-letter: %w", perr)
		}
		return nil
	})
}
