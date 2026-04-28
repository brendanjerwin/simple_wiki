package bridge

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

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
	authBuilder   func(deviceID string) AuthExchanger
	clientBuilder KeepClientFactory
}

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

// Bind binds (page, listName) to a Keep note for the calling user. If
// keepNoteID is empty, a new note titled `listName` is created.
func (c *Connector) Bind(ctx context.Context, profileID wikipage.PageIdentifier, page, listName, keepNoteID string) (Binding, error) {
	client, state, err := c.keepClientFor(ctx, profileID)
	if err != nil {
		return Binding{}, err
	}

	if keepNoteID == "" {
		newID, err := client.CreateList(ctx, listName)
		if err != nil {
			return Binding{}, err
		}
		keepNoteID = newID
	}

	binding := Binding{
		Page:          page,
		ListName:      listName,
		KeepNoteID:    keepNoteID,
		KeepNoteTitle: listName,
		BoundAt:       c.clock.Now().UTC(),
	}
	if err := c.store.AddBinding(profileID, binding); err != nil {
		return Binding{}, err
	}
	_ = state // reserved for future per-binding bookkeeping during bind
	return binding, nil
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
