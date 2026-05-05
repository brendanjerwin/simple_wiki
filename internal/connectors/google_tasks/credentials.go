// Package google_tasks: this file owns the Google Tasks OAuth credential
// state machine — the per-profile bundle (email, refresh_token,
// connected_at, last_verified_at) on the user's profile page.
//
// Per Phase 4-3 of the SyncEngine extraction, the OAuth handling that
// used to live in internal/connectors/google_tasks/sync/{connector.go,
// lifecycle.go} now lives here. Bind/Unbind/Resume/Sync algorithms
// belong to internal/connectors/engine; this file only reads, writes,
// and clears the per-profile credential bundle.
//
//revive:disable:var-naming // package name google_tasks mirrors ConnectorKindGoogleTasks
package google_tasks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

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

// Frontmatter leaf names. Mirrors the legacy package's same-named
// constants; kept here so the credential store doesn't depend on the
// engine package and the legacy package can be deleted.
const (
	credentialKeyEmail          = "email"
	credentialKeyRefreshToken   = "refresh_token"
	credentialKeyConnectedAt    = "connected_at"
	credentialKeyLastVerifiedAt = "last_verified_at"

	// Path to the connector subtree. Matches engine.FrontmatterBindingStore's
	// same constants (intentionally duplicated so the engine package isn't
	// imported here).
	credentialKeyWiki       = "wiki"
	credentialKeyConnectors = "connectors"
	credentialKeyTasks      = "google_tasks"
)

// PausedReasonAuthFailed is the canonical reason string written into
// a binding's PausedReason when invalid_grant retry-once exhausts or
// when Disconnect is invoked. Mirrors the legacy package's constant.
const PausedReasonAuthFailed = "auth_failed"

// PauseAllBindingsHook is the engine-side hook ClearCredentials calls
// when Disconnect runs: every active binding for this profile must
// be transitioned to paused (PausedReason=auth_failed) so the
// scheduler stops driving Sync against an empty refresh token. The
// bootstrap-supplied closure walks BindingStore.LoadBindings and
// calls Engine.TransitionToPaused on each.
//
// A nil hook means "skip pause fan-out" — used by tests; the next
// scheduler tick still pauses bindings naturally via the adapter's
// ErrCredentialMissing → ErrorClassAuthFailed mapping.
type PauseAllBindingsHook func(ctx context.Context, profileID wikipage.PageIdentifier, reason string) error

// ResumeAllBindingsHook is the engine-side hook PersistRefreshToken
// calls after a successful reconnect. The bootstrap-supplied closure
// walks BindingStore.LoadBindings and calls Engine.Resume on each.
// engine.Resume is idempotent on active bindings and routes paused
// bindings >= 7d to ForceFullResync, so a blanket walk is safe.
//
// A nil hook means "skip resume fan-out" — used by tests; the next
// scheduler tick re-evaluates pause-state naturally.
type ResumeAllBindingsHook func(ctx context.Context, profileID wikipage.PageIdentifier) error

// FrontmatterReadWriter is the wiki-side seam this file uses. Mirrors
// engine.FrontmatterReadWriter; declared here so the credentials code
// doesn't import the engine package.
type FrontmatterReadWriter interface {
	ReadFrontMatter(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error)
	WriteFrontMatter(identifier wikipage.PageIdentifier, fm wikipage.FrontMatter) error
}

// Clock is the testable wall-clock seam.
type Clock interface {
	Now() time.Time
}

// SystemClock returns time.Now(). The production wiring uses this;
// tests inject a deterministic stub.
type SystemClock struct{}

// Now returns the current wall-clock time.
func (SystemClock) Now() time.Time { return time.Now() }

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
	pages    FrontmatterReadWriter
	clock    Clock
	logger   Logger
	pauseAll PauseAllBindingsHook
	resumeAll ResumeAllBindingsHook
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
	if pages == nil {
		return nil, errors.New("google_tasks: pages must not be nil")
	}
	if clock == nil {
		return nil, errors.New("google_tasks: clock must not be nil")
	}
	if logger == nil {
		return nil, errors.New("google_tasks: logger must not be nil")
	}
	return &FrontmatterCredentialStore{
		pages:     pages,
		clock:     clock,
		logger:    logger,
		pauseAll:  pauseAll,
		resumeAll: resumeAll,
	}, nil
}

// LoadCredentials reads the per-profile credential bundle. A missing
// page or absent connector subtree returns a zero CredentialBundle
// with no error — callers branch on IsConfigured().
func (s *FrontmatterCredentialStore) LoadCredentials(_ context.Context, profileID wikipage.PageIdentifier) (CredentialBundle, error) {
	_, fm, err := s.pages.ReadFrontMatter(profileID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return CredentialBundle{}, nil
		}
		return CredentialBundle{}, fmt.Errorf("google_tasks: read frontmatter for %s: %w", profileID, err)
	}
	connector := readCredentialMap(fm)
	if connector == nil {
		return CredentialBundle{}, nil
	}
	connectedAt, err := parseRFC3339(getStringFromMap(connector, credentialKeyConnectedAt))
	if err != nil {
		return CredentialBundle{}, fmt.Errorf("wiki.connectors.google_tasks.connected_at: %w", err)
	}
	lastVerifiedAt, err := parseRFC3339(getStringFromMap(connector, credentialKeyLastVerifiedAt))
	if err != nil {
		return CredentialBundle{}, fmt.Errorf("wiki.connectors.google_tasks.last_verified_at: %w", err)
	}
	return CredentialBundle{
		Email:          getStringFromMap(connector, credentialKeyEmail),
		RefreshToken:   getStringFromMap(connector, credentialKeyRefreshToken),
		ConnectedAt:    connectedAt,
		LastVerifiedAt: lastVerifiedAt,
	}, nil
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

	bundle, err := s.LoadCredentials(ctx, pid)
	if err != nil {
		return fmt.Errorf("load credentials for %s: %w", pid, err)
	}
	now := s.clock.Now().UTC()
	if accountEmail != "" {
		bundle.Email = accountEmail
	}
	bundle.RefreshToken = refreshToken
	if bundle.ConnectedAt.IsZero() {
		bundle.ConnectedAt = now
	}
	bundle.LastVerifiedAt = now

	if err := s.writeCredentials(pid, bundle); err != nil {
		return fmt.Errorf("persist credentials for %s: %w", pid, err)
	}

	// Auto-resume paused bindings via the engine hook. Best-effort —
	// the next scheduler tick re-attempts naturally on failure.
	if s.resumeAll != nil {
		if resumeErr := s.resumeAll(ctx, pid); resumeErr != nil {
			s.logger.Error("google_tasks: auto-resume after reconnect failed for profile=%s: %v",
				pid, resumeErr)
		}
	}
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
	bundle, err := s.LoadCredentials(ctx, profileID)
	if err != nil {
		return CredentialBundle{}, err
	}
	bundle.RefreshToken = ""
	bundle.LastVerifiedAt = time.Time{}
	if err := s.writeCredentials(profileID, bundle); err != nil {
		return CredentialBundle{}, fmt.Errorf("clear credentials for %s: %w", profileID, err)
	}
	if s.pauseAll != nil {
		if pauseErr := s.pauseAll(ctx, profileID, PausedReasonAuthFailed); pauseErr != nil {
			s.logger.Error("google_tasks: pause-all-bindings after Disconnect failed for profile=%s: %v",
				profileID, pauseErr)
		}
	}
	return bundle, nil
}

// writeCredentials does the read-modify-write on the profile's
// frontmatter so unrelated fields under wiki.connectors.* (Keep,
// bindings, etc.) are preserved untouched.
func (s *FrontmatterCredentialStore) writeCredentials(profileID wikipage.PageIdentifier, bundle CredentialBundle) error {
	_, fm, err := s.pages.ReadFrontMatter(profileID)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read frontmatter: %w", err)
	}
	if fm == nil {
		fm = make(wikipage.FrontMatter)
	}
	connector := ensureCredentialMap(fm)
	if bundle.Email != "" {
		connector[credentialKeyEmail] = bundle.Email
	} else {
		delete(connector, credentialKeyEmail)
	}
	if bundle.RefreshToken != "" {
		connector[credentialKeyRefreshToken] = bundle.RefreshToken
	} else {
		delete(connector, credentialKeyRefreshToken)
	}
	if !bundle.ConnectedAt.IsZero() {
		connector[credentialKeyConnectedAt] = bundle.ConnectedAt.UTC().Format(time.RFC3339)
	} else {
		delete(connector, credentialKeyConnectedAt)
	}
	if !bundle.LastVerifiedAt.IsZero() {
		connector[credentialKeyLastVerifiedAt] = bundle.LastVerifiedAt.UTC().Format(time.RFC3339)
	} else {
		delete(connector, credentialKeyLastVerifiedAt)
	}
	if err := s.pages.WriteFrontMatter(profileID, fm); err != nil {
		return fmt.Errorf("write frontmatter: %w", err)
	}
	return nil
}

// CreateRemoteCollection creates a fresh Google Tasks tasklist named
// after listName and returns its id and friendly title. The
// "Bind to a new Tasks list" gRPC path calls this BEFORE engine.Bind:
// the bind ceremony itself takes a non-empty remote_handle (the engine
// doesn't manage remote-list creation). Mirrors the legacy
// Connector.subscribeWithLock empty-remoteListID branch.
func (a *TasksAdapter) CreateRemoteCollection(ctx context.Context, profileID wikipage.PageIdentifier, listName string) (handle, title string, err error) {
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

// readCredentialMap returns wiki.connectors.google_tasks (read-only),
// or nil when any segment is missing or the wrong type.
func readCredentialMap(fm wikipage.FrontMatter) map[string]any {
	wiki, ok := fm[credentialKeyWiki].(map[string]any)
	if !ok {
		return nil
	}
	conns, ok := wiki[credentialKeyConnectors].(map[string]any)
	if !ok {
		return nil
	}
	gt, ok := conns[credentialKeyTasks].(map[string]any)
	if !ok {
		return nil
	}
	return gt
}

// ensureCredentialMap returns wiki.connectors.google_tasks, creating
// any missing intermediate maps. Used by writeCredentials so a fresh
// profile (no prior frontmatter) gets a proper subtree.
func ensureCredentialMap(fm wikipage.FrontMatter) map[string]any {
	wiki, ok := fm[credentialKeyWiki].(map[string]any)
	if !ok {
		wiki = make(map[string]any)
		fm[credentialKeyWiki] = wiki
	}
	conns, ok := wiki[credentialKeyConnectors].(map[string]any)
	if !ok {
		conns = make(map[string]any)
		wiki[credentialKeyConnectors] = conns
	}
	gt, ok := conns[credentialKeyTasks].(map[string]any)
	if !ok {
		gt = make(map[string]any)
		conns[credentialKeyTasks] = gt
	}
	return gt
}

func getStringFromMap(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func parseRFC3339(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("not a valid RFC3339 timestamp: %w", err)
	}
	return t.UTC(), nil
}

// Compile-time check: FrontmatterCredentialStore satisfies the
// CredentialReader contract the TasksAdapter consumes.
var _ CredentialReader = (*FrontmatterCredentialStore)(nil)
