// Package googletasks: this file owns the Google Tasks OAuth credential
// state machine — the per-profile bundle (email, refresh_token,
// connected_at, last_verified_at) on the user's profile page.
//
// Per Phase 4-3 of the SyncEngine extraction, the OAuth handling that
// used to live in internal/connectors/google_tasks/sync/{connector.go,
// lifecycle.go} now lives here. Bind/Unbind/Resume/Sync algorithms
// belong to internal/connectors/engine; this file only reads, writes,
// and clears the per-profile credential bundle.
package googletasks

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/internal/connectors/frontmattercreds"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// CredentialBundle is the per-profile OAuth state for Google Tasks.
// Carries the refresh token plus the human-facing metadata the gRPC
// GetState response needs (email, connected_at, last_verified_at).
//
// Bindings (the per-checklist records) are owned by the engine's
// FrontmatterBindingStore, NOT this struct. Both live alongside each
// other under wiki.connectors.google_tasks.* on the profile page.
type CredentialBundle struct {
	Email          string
	RefreshToken   string
	ConnectedAt    time.Time
	LastVerifiedAt time.Time
}

// IsConfigured reports whether the bundle carries a refresh token.
// Empty refresh_token = "not connected" (or Disconnect was called).
func (b CredentialBundle) IsConfigured() bool { return b.RefreshToken != "" }

// Frontmatter leaf names for the connector-specific fields.
const (
	credentialKeyEmail        = "email"
	credentialKeyRefreshToken = "refresh_token"
)

// PausedReasonAuthFailed is the canonical reason string written into
// a binding's PausedReason when invalid_grant retry-once exhausts or
// when Disconnect is invoked.
const PausedReasonAuthFailed = frontmattercreds.PausedReasonAuthFailed

// PauseAllBindingsHook is the engine-side hook ClearCredentials calls
// when Disconnect runs.
type PauseAllBindingsHook = frontmattercreds.PauseAllBindingsHook

// ResumeAllBindingsHook is the engine-side hook PersistRefreshToken
// calls after a successful reconnect.
type ResumeAllBindingsHook = frontmattercreds.ResumeAllBindingsHook

// FrontmatterReadWriter is the wiki-side seam this file uses.
type FrontmatterReadWriter = frontmattercreds.FrontmatterReadWriter

// Clock is the testable wall-clock seam.
type Clock = frontmattercreds.Clocker

// SystemClock returns time.Now(). The production wiring uses this;
// tests inject a deterministic stub.
type SystemClock = frontmattercreds.SystemClock

// tasksBundleCodec implements frontmattercreds.BundleCodec[CredentialBundle].
// It maps the Tasks-specific field (refresh_token) to/from the
// frontmatter sub-map, while the shared Store handles timestamps and
// the read-modify-write plumbing.
type tasksBundleCodec struct{}

func (tasksBundleCodec) ConnectorKey() string { return "google_tasks" }
func (tasksBundleCodec) ErrorPrefix() string  { return "google_tasks" }

func (tasksBundleCodec) AssembleBundle(m map[string]any, connectedAt, lastVerifiedAt time.Time) CredentialBundle {
	return CredentialBundle{
		Email:          frontmattercreds.GetStringFromMap(m, credentialKeyEmail),
		RefreshToken:   frontmattercreds.GetStringFromMap(m, credentialKeyRefreshToken),
		ConnectedAt:    connectedAt,
		LastVerifiedAt: lastVerifiedAt,
	}
}

func (tasksBundleCodec) WriteTokenFields(b CredentialBundle, m map[string]any) {
	if b.Email != "" {
		m[credentialKeyEmail] = b.Email
	} else {
		delete(m, credentialKeyEmail)
	}

	if b.RefreshToken != "" {
		m[credentialKeyRefreshToken] = b.RefreshToken
	} else {
		delete(m, credentialKeyRefreshToken)
	}
}

func (tasksBundleCodec) Timestamps(b CredentialBundle) (connectedAt, lastVerifiedAt time.Time) {
	return b.ConnectedAt, b.LastVerifiedAt
}

func (tasksBundleCodec) IsConfigured(b CredentialBundle) bool { return b.RefreshToken != "" }

func (tasksBundleCodec) ClearForDisconnect(b *CredentialBundle) {
	b.RefreshToken = ""
	b.LastVerifiedAt = time.Time{}
}

// FrontmatterCredentialStore reads and writes the per-profile Tasks
// credential bundle on the wiki's frontmatter. It also drives the
// engine-side pause-on-disconnect / resume-on-reconnect fan-outs via
// injected hooks.
//
// Concurrency: there is no per-profile mutex on this struct. The
// engine's FrontmatterBindingStore.WithProfileLock serializes bind
// flows; the OAuth-callback path serializes itself; GetState is a
// read-only path. If a future concern surfaces, a sync.Map of
// per-profile mutexes lands here.
type FrontmatterCredentialStore struct {
	core *frontmattercreds.Store[CredentialBundle]
}

// NewFrontmatterCredentialStore wires the production credential store.
// pages, clock, and logger are required; the pause/resume hooks may be
// nil for tests, in which case Disconnect/PersistRefreshToken skip the
// engine fan-out (the next scheduler tick reconciles state naturally).
func NewFrontmatterCredentialStore(
	pages FrontmatterReadWriter,
	clock Clock,
	logger Logger,
	pauseAll PauseAllBindingsHook,
	resumeAll ResumeAllBindingsHook,
) (*FrontmatterCredentialStore, error) {
	core, err := frontmattercreds.NewStore(pages, clock, logger, pauseAll, resumeAll, tasksBundleCodec{})
	if err != nil {
		return nil, err
	}
	return &FrontmatterCredentialStore{core: core}, nil
}

// LoadCredentials reads the per-profile credential bundle. A missing
// page or absent connector subtree returns a zero CredentialBundle
// with no error — callers branch on IsConfigured().
func (s *FrontmatterCredentialStore) LoadCredentials(ctx context.Context, profileID wikipage.PageIdentifier) (CredentialBundle, error) {
	return s.core.LoadCredentials(ctx, profileID)
}

// LoadRefreshToken implements CredentialReader (used by TasksAdapter).
// Returns ErrCredentialMissing when the profile has no refresh token.
func (s *FrontmatterCredentialStore) LoadRefreshToken(ctx context.Context, profileID wikipage.PageIdentifier) (string, error) {
	bundle, err := s.LoadCredentials(ctx, profileID)
	if err != nil {
		return "", err
	}
	if !bundle.IsConfigured() {
		return "", ErrCredentialMissing
	}
	return bundle.RefreshToken, nil
}

// PersistRefreshToken implements server.RefreshTokenPersister. The
// OAuth callback handler invokes this after the auth-code exchange
// succeeds (state/PKCE/iss already verified server-side). Persists
// the fresh refresh token onto the profile, stamps connected_at /
// last_verified_at, and fans out engine.Resume across every binding
// for the profile so paused subscriptions transition back to active
// (within-horizon resume keeps the cursor; >7d routes through
// ForceFullResync per ADR-0011).
//
// accountEmail is optional — Google's /oauth2/v3/token response does
// not include the user's address, so the OAuth callback handler
// passes "" here. Existing email on the bundle is preserved.
func (s *FrontmatterCredentialStore) PersistRefreshToken(ctx context.Context, profileID, accountEmail, refreshToken string) error {
	if profileID == "" {
		return errors.New("google_tasks: profileID is required")
	}
	if refreshToken == "" {
		return errors.New("google_tasks: refresh_token is required")
	}
	pid := wikipage.PageIdentifier(profileID)

	bundle, err := s.core.LoadCredentials(ctx, pid)
	if err != nil {
		return fmt.Errorf("load credentials for %s: %w", pid, err)
	}
	now := s.core.Clock().Now().UTC()
	if accountEmail != "" {
		bundle.Email = accountEmail
	}
	bundle.RefreshToken = refreshToken
	if bundle.ConnectedAt.IsZero() {
		bundle.ConnectedAt = now
	}
	bundle.LastVerifiedAt = now

	if err := s.core.WriteCredentials(pid, bundle); err != nil {
		return fmt.Errorf("persist credentials for %s: %w", pid, err)
	}

	// Auto-resume paused bindings via the engine hook. Best-effort —
	// the next scheduler tick re-attempts naturally on failure.
	s.core.ResumeBindings(ctx, pid)
	return nil
}

// ClearCredentials wipes the refresh token from the profile and
// transitions every active binding to paused. Mirrors the legacy
// Disconnect semantics: bindings are preserved (a reconnect resumes
// them) but the scheduler stops syncing against a missing token.
//
// Returns the post-clear CredentialBundle (refresh_token == "") so
// the gRPC handler can render a "not configured" state response.
func (s *FrontmatterCredentialStore) ClearCredentials(ctx context.Context, profileID wikipage.PageIdentifier) (CredentialBundle, error) {
	return s.core.ClearCredentials(ctx, profileID)
}

// CreateRemoteCollection creates a fresh Google Tasks tasklist named
// after listName and returns its id and friendly title. The
// "Bind to a new Tasks list" gRPC path calls this BEFORE engine.Bind:
// the bind ceremony itself takes a non-empty remote_handle (the engine
// doesn't manage remote-list creation). Mirrors the legacy
// Connector.subscribeWithLock empty-remoteListID branch.
//
// initialItems is intentionally ignored: Tasks creates an empty
// tasklist, and the engine's first reconcile tick will Insert each
// wiki item via the per-item primitives. Pre-seeding via the Tasks API
// would require N+1 RPCs (one CreateList + N Insert), which is exactly
// what the engine does anyway — so we accept the parameter for
// connectors.BackendAdapter conformance and discard it. Keep, by
// contrast, can stage initial items in the same Changes call that
// mints the LIST node (see (*KeepAdapter).CreateRemoteCollection).
func (a *TasksAdapter) CreateRemoteCollection(ctx context.Context, profileID wikipage.PageIdentifier, listName string, _ []connectors.WikiItem) (handle, title string, err error) {
	client, err := a.buildClientForProfile(ctx, profileID)
	if err != nil {
		return "", "", fmt.Errorf("build client for profile %s: %w", profileID, err)
	}
	created, err := client.CreateTaskList(ctx, listName)
	if err != nil {
		return "", "", fmt.Errorf("create remote tasklist %q for profile %s: %w", listName, profileID, err)
	}
	return created.ID, created.Title, nil
}

// Compile-time check: FrontmatterCredentialStore satisfies the
// CredentialReader contract the TasksAdapter consumes.
var _ CredentialReader = (*FrontmatterCredentialStore)(nil)
