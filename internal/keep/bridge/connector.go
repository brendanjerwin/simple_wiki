package bridge

import (
	"context"
	"crypto/sha256"
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
	ExchangeASPForMasterToken(ctx context.Context, email, asp string) (string, error)
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
	authBuilder   func(deviceID string) AuthExchanger
	clientBuilder KeepClientFactory
}

// NewConnector wires the production dependencies. Tests construct a
// Connector directly with stubbed builders.
func NewConnector(store *BindingStore, httpClient *http.Client, clock Clock) *Connector {
	return &Connector{
		store:      store,
		httpClient: httpClient,
		clock:      clock,
		authBuilder: func(deviceID string) AuthExchanger {
			return protocol.NewAuthenticator(httpClient, protocol.AuthURL, deviceID)
		},
		clientBuilder: func(bearer string) KeepClient {
			return protocol.NewKeepClient(httpClient, protocol.DefaultKeepBaseURL, bearer)
		},
	}
}

// Connect performs the full connect flow: ASP → master token → bearer →
// verify with a Keep changes call → store. On any failure, no state is
// written.
func (c *Connector) Connect(ctx context.Context, profileID wikipage.PageIdentifier, email, asp string) (ConnectorState, error) {
	if email == "" {
		return ConnectorState{}, errors.New("keep bridge: email is required")
	}
	if asp == "" {
		return ConnectorState{}, errors.New("keep bridge: app-specific password is required")
	}

	auth := c.authBuilder(deriveDeviceID(profileID))
	masterToken, err := auth.ExchangeASPForMasterToken(ctx, email, asp)
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
		ClientTimestamp: fmt.Sprintf("%d", c.clock.Now().UnixMicro()),
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
func (c *Connector) Disconnect(ctx context.Context, profileID wikipage.PageIdentifier) (ConnectorState, error) {
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
func (c *Connector) GetState(ctx context.Context, profileID wikipage.PageIdentifier) (ConnectorState, error) {
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
		ClientTimestamp: fmt.Sprintf("%d", now.UnixMicro()),
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
func (c *Connector) Unbind(ctx context.Context, profileID wikipage.PageIdentifier, page, listName string) error {
	err := c.store.RemoveBinding(profileID, page, listName)
	if errors.Is(err, ErrBindingNotFound) {
		// Idempotent at the orchestrator boundary — UI calls this on
		// rebind/remove flows and shouldn't have to disambiguate.
		return nil
	}
	return err
}

// FindBinding mirrors BindingStore.FindBinding for handler convenience.
func (c *Connector) FindBinding(ctx context.Context, profileID wikipage.PageIdentifier, page, listName string) (Binding, bool, error) {
	return c.store.FindBinding(profileID, page, listName)
}

// deriveDeviceID returns a stable 16-hex-char device id derived from the
// profile id. Matches the gpsoauth requirement of a stable per-account
// android id without reusing any real device's id.
func deriveDeviceID(profileID wikipage.PageIdentifier) string {
	sum := sha256.Sum256([]byte(profileID))
	return hex.EncodeToString(sum[:8]) // 16 hex chars
}
