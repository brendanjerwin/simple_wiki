package bootstrap

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/jcelliott/lumber"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/engine"
	googlekeep "github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep"
	keepgateway "github.com/brendanjerwin/simple_wiki/internal/connectors/google_keep/gateway"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/server/checklistmutator"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// keepWiring bundles the engine-path collaborators the Keep gRPC
// handlers and lease-table boot rebuild need. setupGoogleKeep returns
// nil when the operator hasn't enabled the Keep connector — that's
// the documented opt-out (no env var configures the Keep path
// because Keep's gpsoauth flow is single-shot via CompleteAuth, not
// OAuth-redirect).
//
// In Phase 5-A the connector is *always* wired (no env-var gate) —
// the credential store handles the unconfigured-profile case via
// IsConfigured() and the gRPC layer maps that to FailedPrecondition.
// A future env-var disable knob can short-circuit setupGoogleKeep to
// return nil if needed.
type keepWiring struct {
	engine          *engine.Engine
	adapter         *googlekeep.KeepAdapter
	bindingStore    engine.BindingStore
	credentialStore *googlekeep.FrontmatterCredentialStore
	authVerifier    googlekeep.AuthVerifier
}

// keepOutboundSyncJobName is the queue name for the Keep outbound
// sync job. Distinct from the legacy Keep sync package's queue name —
// that legacy queue is no longer registered. The engine path's queue
// uses this dedicated name so on-disk job state from a prior process
// can't leak across the cutover.
const keepOutboundSyncJobName = "GoogleKeepOutboundSync"

// keepHTTPTimeoutSeconds is the timeout the Keep HTTP client uses for
// every Changes / gpsoauth call. 30s is comfortably above Keep's
// worst observed full-pull latency on a household-scale account.
const keepHTTPTimeoutSeconds = 30

// setupGoogleKeep wires the engine path for Google Keep: credential
// store, adapter, engine, debouncer, scheduler registration. Mirrors
// setupGoogleTasks's shape.
//
// The leaseTable parameter is the cross-connector LeaseTable shared
// with Tasks; its boot-rebuild fan-out scan + SignalReady is the
// caller's responsibility, NOT this function's.
//
//revive:disable-next-line:function-length
func setupGoogleKeep(
	site *server.Site,
	syncScheduler *connectors.SyncScheduler,
	checklistMutator *checklistmutator.Mutator,
	leaseTable *connectors.LeaseTable,
	logger *lumber.ConsoleLogger,
) (*keepWiring, error) {
	// BindingStore (engine-owned). Reads/writes profile frontmatter
	// under wiki.connectors.google_keep.bindings[] (with legacy
	// subscriptions[] dual-read until Phase 7 migrates).
	bindingStore, bsErr := engine.NewFrontmatterBindingStore(
		site,
		&frontmatterIndexProfileLister{index: site.FrontmatterIndexQueryer},
		logger,
	)
	if bsErr != nil {
		return nil, fmt.Errorf("build keep binding store: %w", bsErr)
	}

	httpClient := &http.Client{Timeout: keepHTTPTimeoutSeconds * time.Second}

	// Forward-declared by closures: pause/resume hooks call into the
	// engine, which references the credential store via the adapter's
	// CredentialReader. Use late binding via *engine.Engine pointer
	// that's filled in below (mirrors the Tasks pattern).
	var keepEngine *engine.Engine
	pauseAll := func(ctx context.Context, profileID wikipage.PageIdentifier, reason string) error {
		if keepEngine == nil {
			return nil
		}
		return pauseAllKeepBindings(ctx, keepEngine, bindingStore, profileID, reason)
	}
	resumeAll := func(ctx context.Context, profileID wikipage.PageIdentifier) error {
		if keepEngine == nil {
			return nil
		}
		return resumeAllKeepBindings(ctx, keepEngine, bindingStore, profileID)
	}

	credentialStore, csErr := googlekeep.NewFrontmatterCredentialStore(
		site,
		googlekeep.SystemClock{},
		logger,
		pauseAll,
		resumeAll,
	)
	if csErr != nil {
		return nil, fmt.Errorf("build keep credential store: %w", csErr)
	}

	// Per-profile gateway client factory: each call constructs a
	// fresh gpsoauth Stage 2 round trip (master_token → bearer), then
	// builds a KeepClient on top. The adapter calls this once per
	// primitive invocation; the bearer is short-lived but the call
	// volume on household-scale accounts is low enough that
	// re-exchanging on every primitive is fine.
	authClient := newKeepAuthHTTPClient()
	keepClientFactory := googlekeep.KeepClientFactory(func(ctx context.Context, _ wikipage.PageIdentifier, masterToken, email string) (googlekeep.KeepClient, error) {
		auth := keepgateway.NewAuthenticator(authClient, keepgateway.AuthURL, "")
		bearer, err := auth.ExchangeMasterTokenForBearer(ctx, email, masterToken)
		if err != nil {
			return nil, fmt.Errorf("exchange master token for bearer: %w", err)
		}
		client := keepgateway.NewKeepClient(httpClient, keepgateway.DefaultKeepBaseURL, bearer)
		client.SetDebugLogger(logger)
		return client, nil
	})

	keepAdapter, aerr := googlekeep.NewKeepAdapter(credentialStore, keepClientFactory, googlekeep.SystemClock{}, logger)
	if aerr != nil {
		return nil, fmt.Errorf("build keep adapter: %w", aerr)
	}

	// Wiki-side bridge: wraps the wiki's checklistmutator subscriber
	// shape and the engine's SyncSuppressor / SyncDebouncer hookpoints
	// on the inbound apply path.
	bridge := newKeepMutatorBridge(logger)

	keepEng, eerr := engine.NewEngine(
		keepAdapter,
		leaseTable,
		checklistMutator,
		checklistMutator,
		bridge,
		logger,
		systemWallClock{},
		bindingStore,
	)
	if eerr != nil {
		return nil, fmt.Errorf("build keep sync engine: %w", eerr)
	}
	keepEngine = keepEng

	// Engine-owned outbound debouncer. The wiki mutator notifies the
	// bridge on every successful checklist mutation; the bridge
	// forwards to the engine debouncer's OnChecklistMutated; on
	// debounceWindow expiry the engine fires Sync via the SyncFunc.
	syncFn := func(ctx context.Context, key engine.SyncDebouncerKey) error {
		return keepEngine.Sync(ctx, connectors.BindingKey{
			ProfileID: key.ProfileID,
			Page:      key.Page,
			ListName:  key.ListName,
		})
	}
	debouncer, derr := engine.NewSyncDebouncer(
		systemWallClock{},
		realTimerScheduler{},
		syncFn,
		logger,
	)
	if derr != nil {
		return nil, fmt.Errorf("build keep engine debouncer: %w", derr)
	}
	bridge.attachDebouncer(debouncer)
	checklistMutator.AddSubscriber(bridge)

	keepSubscriptionLister := func() []connectors.BindingKey {
		out := make([]connectors.BindingKey, 0, 8)
		// Probe by master_token (the canonical "is connected?" leaf).
		// Mirrors the Tasks lister's refresh_token probe.
		for _, p := range site.FrontmatterIndexQueryer.QueryKeyExistence("wiki.connectors.google_keep.master_token") {
			bindings, err := bindingStore.LoadBindings(p, connectors.ConnectorKindGoogleKeep)
			if err != nil {
				continue
			}
			for _, b := range bindings {
				out = append(out, connectors.BindingKey{
					ProfileID: string(p),
					Page:      b.Page,
					ListName:  b.ListName,
				})
			}
		}
		return out
	}

	if regErr := syncScheduler.Register(
		keepEngine,
		keepSubscriptionLister,
		func(c connectors.Connector, key connectors.BindingKey) jobs.Job {
			return &engineSyncJob{connector: c, key: key, queueName: keepOutboundSyncJobName}
		},
	); regErr != nil {
		return nil, fmt.Errorf("register Keep with sync scheduler: %w", regErr)
	}

	// Single-worker queue: outbound order matters (Keep's targetVersion
	// is global per-account); concurrent pushes would race the cursor.
	if err := site.GetJobQueueCoordinator().RegisterQueue(
		keepOutboundSyncJobName, 1, keepOutboundSyncQueueDepth,
	); err != nil {
		return nil, fmt.Errorf("register Keep outbound sync queue: %w", err)
	}

	// Tombstone GC retention: when any Keep binding on a checklist is
	// paused, retain its tombstones beyond the default 7-day TTL so the
	// deletion replay on resume isn't undone by GC.
	checklistMutator.SetPausedChecker(&keepFannedOutPausedChecker{
		bindings: bindingStore,
		index:    site.FrontmatterIndexQueryer,
	})

	authVerifier := &keepAuthVerifierImpl{
		httpClient:     httpClient,
		authHTTPClient: authClient,
		debug:          logger,
	}

	logger.Info("Google Keep connector configured (engine path).")

	return &keepWiring{
		engine:          keepEngine,
		adapter:         keepAdapter,
		bindingStore:    bindingStore,
		credentialStore: credentialStore,
		authVerifier:    authVerifier,
	}, nil
}

// newKeepAuthHTTPClient returns an http.Client mirrored from the
// legacy package's same-named helper: the gpsoauth /auth endpoint
// rejects h2 ALPN with a 403 BadAuthentication body. We pin
// http/1.1 in TLS NextProtos to side-step that quirk.
func newKeepAuthHTTPClient() *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			NextProtos: []string{"http/1.1"},
		},
		ForceAttemptHTTP2: false,
	}
	return &http.Client{Transport: transport, Timeout: keepHTTPTimeoutSeconds * time.Second}
}

// pauseAllKeepBindings is the engine-side fan-out the credential
// store invokes from ClearCredentials. Walks every binding for the
// profile and transitions active ones to paused.
func pauseAllKeepBindings(ctx context.Context, eng *engine.Engine, store engine.BindingStore, profileID wikipage.PageIdentifier, reason string) error {
	_ = ctx // engine.TransitionToPaused has no context parameter (lock-bound work, no I/O).
	bindings, err := store.LoadBindings(profileID, connectors.ConnectorKindGoogleKeep)
	if err != nil {
		return fmt.Errorf("load bindings for %s: %w", profileID, err)
	}
	var firstErr error
	for _, b := range bindings {
		if b.IsPaused() {
			continue
		}
		if pauseErr := eng.TransitionToPaused(profileID, b.Page, b.ListName, reason); pauseErr != nil {
			if firstErr == nil {
				firstErr = pauseErr
			}
		}
	}
	return firstErr
}

// resumeAllKeepBindings is the engine-side fan-out the credential
// store invokes from PersistMasterToken. Walks every binding for the
// profile and offers each to engine.Resume — engine.Resume is
// idempotent on active bindings, so the blanket walk is safe.
func resumeAllKeepBindings(ctx context.Context, eng *engine.Engine, store engine.BindingStore, profileID wikipage.PageIdentifier) error {
	bindings, err := store.LoadBindings(profileID, connectors.ConnectorKindGoogleKeep)
	if err != nil {
		return fmt.Errorf("load bindings for %s: %w", profileID, err)
	}
	var firstErr error
	for _, b := range bindings {
		if resumeErr := eng.Resume(ctx, profileID, b.Page, b.ListName); resumeErr != nil {
			if firstErr == nil {
				firstErr = resumeErr
			}
		}
	}
	return firstErr
}

// keepFannedOutPausedChecker satisfies checklistmutator.PausedChecker
// by fanning out the per-profile binding-store query across every
// profile that has the Keep connector configured. The fan-out is
// keyed off the frontmatter index (same probe the binding lister
// uses), so it picks up profiles connected since process start.
//
// Returns true if any binding on (page, listName) on any profile is
// currently paused. Per ADR-0011 a checklist has at most one owner
// at a time, so the OR-fan-out is conservative-correct.
type keepFannedOutPausedChecker struct {
	bindings engine.BindingStore
	index    frontmatterKeyQueryer
}

// IsAnyChecklistBindingPaused fans out the per-profile pause
// check.
func (c *keepFannedOutPausedChecker) IsAnyChecklistBindingPaused(page, listName string) bool {
	for _, profileID := range c.index.QueryKeyExistence("wiki.connectors.google_keep.master_token") {
		bindings, err := c.bindings.LoadBindings(profileID, connectors.ConnectorKindGoogleKeep)
		if err != nil {
			continue
		}
		for _, b := range bindings {
			if b.Page == page && b.ListName == listName && b.IsPaused() {
				return true
			}
		}
	}
	return false
}

// keepMutatorBridge is the wiki-side glue between the checklistmutator
// notify shape and the engine's SyncDebouncer / SyncSuppressor.
// Mirrors tasksMutatorBridge for the Keep path.
//
// The bridge filters synthetic identities (system:keep-sync,
// system:connector-sync) so an inbound apply doesn't re-enqueue a
// sync against the same connector.
type keepMutatorBridge struct {
	logger    *lumber.ConsoleLogger
	debouncer *engine.SyncDebouncer

	mu         sync.Mutex
	suppressed map[string]int // refcount per "<profile>|<page>|<list>"
}

// newKeepMutatorBridge returns a fresh bridge.
func newKeepMutatorBridge(logger *lumber.ConsoleLogger) *keepMutatorBridge {
	return &keepMutatorBridge{
		logger:     logger,
		suppressed: map[string]int{},
	}
}

// attachDebouncer connects the bridge to the engine debouncer.
func (b *keepMutatorBridge) attachDebouncer(d *engine.SyncDebouncer) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.debouncer = d
}

// Suppress implements engine.SyncSuppressor.
func (b *keepMutatorBridge) Suppress(profileID wikipage.PageIdentifier, page, listName string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.suppressed[bridgeKey(profileID, page, listName)]++
}

// Unsuppress implements engine.SyncSuppressor.
func (b *keepMutatorBridge) Unsuppress(profileID wikipage.PageIdentifier, page, listName string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	key := bridgeKey(profileID, page, listName)
	b.suppressed[key]--
	if b.suppressed[key] <= 0 {
		delete(b.suppressed, key)
	}
}

// Synthetic identity strings the bridge drops. These are the system:*
// identities the checklistmutator stamps on inbound apply writes;
// without filtering, every inbound apply would re-trigger the same
// connector's outbound debounce loop.
const (
	keepSyncIdentityLogin           = "system:keep-sync"
	keepConnectorSyncIdentityLogin  = "system:connector-sync"
)

// OnChecklistMutated implements checklistmutator.Subscriber.
func (b *keepMutatorBridge) OnChecklistMutated(page, listName string, identity tailscale.IdentityValue) {
	if identity == nil {
		return
	}
	login := identity.LoginName()
	if login == "" {
		return
	}
	if login == keepSyncIdentityLogin || login == keepConnectorSyncIdentityLogin {
		return
	}
	profileID, err := wikipage.ProfileIdentifierFor(login)
	if err != nil {
		if b.logger != nil {
			b.logger.Error("keep bridge: resolve profile for login %q: %v", login, err)
		}
		return
	}

	b.mu.Lock()
	if b.suppressed[bridgeKey(profileID, page, listName)] > 0 {
		b.mu.Unlock()
		return
	}
	debouncer := b.debouncer
	b.mu.Unlock()

	if debouncer == nil {
		return
	}
	debouncer.OnChecklistMutated(engine.SyncDebouncerKey{
		ProfileID: string(profileID),
		Page:      page,
		ListName:  listName,
	})
}

// keepAuthVerifierImpl satisfies googlekeep.AuthVerifier by wrapping
// gateway.Authenticator + a transient KeepClient. Used by
// CompleteAuth(GOOGLE_KEEP) on the gRPC service to perform the
// gpsoauth round trip + Keep verify pull on the operator's behalf.
type keepAuthVerifierImpl struct {
	httpClient     *http.Client
	authHTTPClient *http.Client
	debug          keepgateway.DebugLogger
}

// VerifyOAuthToken performs Stage 1b (oauth_token → master_token),
// Stage 2 (master_token → bearer), and a no-mutation Changes call to
// confirm Keep accepts the bearer. Mirrors the legacy
// connector.Connect flow but factored into the per-call shape the
// credential store's AuthVerifier interface expects.
func (v *keepAuthVerifierImpl) VerifyOAuthToken(ctx context.Context, _ wikipage.PageIdentifier, email, oauthToken, androidID string) (string, error) {
	auth := keepgateway.NewAuthenticator(v.authHTTPClient, keepgateway.AuthURL, androidID)
	masterToken, err := auth.ExchangeOAuthTokenForMasterToken(ctx, email, oauthToken)
	if err != nil {
		return "", err
	}
	bearer, err := auth.ExchangeMasterTokenForBearer(ctx, email, masterToken)
	if err != nil {
		return "", err
	}
	client := keepgateway.NewKeepClient(v.httpClient, keepgateway.DefaultKeepBaseURL, bearer)
	if v.debug != nil {
		client.SetDebugLogger(v.debug)
	}
	now := time.Now().UTC()
	if _, err := client.Changes(ctx, keepgateway.ChangesRequest{
		SessionID:       fmt.Sprintf("s--%d--verify", now.UnixMilli()),
		ClientTimestamp: now.Format("2006-01-02T15:04:05.000000Z"),
	}); err != nil {
		return "", err
	}
	return masterToken, nil
}

// Compile-time check: keepAuthVerifierImpl satisfies the AuthVerifier
// contract the credential store's Connect path consumes.
var _ googlekeep.AuthVerifier = (*keepAuthVerifierImpl)(nil)
