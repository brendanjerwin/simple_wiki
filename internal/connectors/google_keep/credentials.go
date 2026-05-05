// Package google_keep: this file owns the Google Keep gpsoauth
// credential state machine — the per-profile bundle (email,
// master_token, android_id, connected_at, last_verified_at) on the
// user's profile page.
//
// Per Phase 5-A of the SyncEngine extraction, the gpsoauth handling
// that used to live in internal/connectors/google_keep/sync/{connector.go,
// subscriptions.go} now lives here. Bind/Unbind/Resume/Sync algorithms
// belong to internal/connectors/engine; this file only reads, writes,
// and clears the per-profile credential bundle.
//
//revive:disable:var-naming // package name google_keep mirrors ConnectorKindGoogleKeep
package google_keep

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// CredentialBundle is the per-profile gpsoauth state for Google Keep.
// Carries the master token plus the human-facing metadata the gRPC
// GetState response needs (email, connected_at, last_verified_at).
//
// AndroidID is the stable 16-hex-char Android device id Google sees on
// the wire — the wiki persists one per profile so subsequent calls
// don't trip "new device" heuristics. Derived deterministically from
// the profile id when not yet stored, then rewritten on first connect
// so future deployments use the same value.
//
// Bindings (the per-checklist records) are owned by the engine's
// FrontmatterBindingStore, NOT this struct. Both live alongside each
// other under wiki.connectors.google_keep.* on the profile page.
type CredentialBundle struct {
	Email          string
	MasterToken    string
	AndroidID      string
	ConnectedAt    time.Time
	LastVerifiedAt time.Time
}

// IsConfigured reports whether the bundle carries a master token.
// Empty master_token = "not connected" (or Disconnect was called).
func (b CredentialBundle) IsConfigured() bool { return b.MasterToken != "" }

// Frontmatter leaf names. Mirrors the legacy package's same-named
// constants; kept here so the credential store doesn't depend on the
// engine package and the legacy package can be deleted.
const (
	credentialKeyEmail          = "email"
	credentialKeyMasterToken    = "master_token"
	credentialKeyAndroidID      = "android_id"
	credentialKeyConnectedAt    = "connected_at"
	credentialKeyLastVerifiedAt = "last_verified_at"

	// Path to the connector subtree. Matches engine.FrontmatterBindingStore's
	// same constants (intentionally duplicated so the engine package isn't
	// imported here).
	credentialKeyWiki       = "wiki"
	credentialKeyConnectors = "connectors"
	credentialKeyKeep       = "google_keep"
)

// PausedReasonAuthFailed is the canonical reason string written into
// a binding's PausedReason when Disconnect is invoked or when the
// gpsoauth bearer is rejected by Keep. Mirrors the Tasks package's
// constant for parity at the gRPC boundary.
const PausedReasonAuthFailed = "auth_failed"

// PauseAllBindingsHook is the engine-side hook ClearCredentials calls
// when Disconnect runs: every active binding for this profile must
// be transitioned to paused (PausedReason=auth_failed) so the
// scheduler stops driving Sync against an empty master token. The
// bootstrap-supplied closure walks BindingStore.LoadBindings and
// calls Engine.TransitionToPaused on each.
//
// A nil hook means "skip pause fan-out" — used by tests; the next
// scheduler tick still pauses bindings naturally via the adapter's
// ErrCredentialMissing → ErrorClassAuthFailed mapping.
type PauseAllBindingsHook func(ctx context.Context, profileID wikipage.PageIdentifier, reason string) error

// ResumeAllBindingsHook is the engine-side hook PersistMasterToken
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

// FrontmatterCredentialStore reads and writes the per-profile Keep
// credential bundle on the wiki's frontmatter. It also drives the
// engine-side pause-on-disconnect / resume-on-reconnect fan-outs via
// injected hooks.
//
// Concurrency: there is no per-profile mutex on this struct. The
// engine's FrontmatterBindingStore.WithProfileLock serializes bind
// flows; the gpsoauth-callback path serializes itself; GetState is a
// read-only path. If a future concern surfaces, a sync.Map of
// per-profile mutexes lands here.
type FrontmatterCredentialStore struct {
	pages     FrontmatterReadWriter
	clock     Clock
	logger    Logger
	pauseAll  PauseAllBindingsHook
	resumeAll ResumeAllBindingsHook
}

// NewFrontmatterCredentialStore wires the production credential store.
// pages, clock, and logger are required; the pause/resume hooks may be
// nil for tests, in which case Disconnect/PersistMasterToken skip the
// engine fan-out (the next scheduler tick reconciles state naturally).
func NewFrontmatterCredentialStore(
	pages FrontmatterReadWriter,
	clock Clock,
	logger Logger,
	pauseAll PauseAllBindingsHook,
	resumeAll ResumeAllBindingsHook,
) (*FrontmatterCredentialStore, error) {
	if pages == nil {
		return nil, errors.New("google_keep: pages must not be nil")
	}
	if clock == nil {
		return nil, errors.New("google_keep: clock must not be nil")
	}
	if logger == nil {
		return nil, errors.New("google_keep: logger must not be nil")
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
		return CredentialBundle{}, fmt.Errorf("google_keep: read frontmatter for %s: %w", profileID, err)
	}
	connector := readKeepCredentialMap(fm)
	if connector == nil {
		return CredentialBundle{}, nil
	}
	connectedAt, err := parseRFC3339(getStringFromMap(connector, credentialKeyConnectedAt))
	if err != nil {
		return CredentialBundle{}, fmt.Errorf("wiki.connectors.google_keep.connected_at: %w", err)
	}
	lastVerifiedAt, err := parseRFC3339(getStringFromMap(connector, credentialKeyLastVerifiedAt))
	if err != nil {
		return CredentialBundle{}, fmt.Errorf("wiki.connectors.google_keep.last_verified_at: %w", err)
	}
	return CredentialBundle{
		Email:          getStringFromMap(connector, credentialKeyEmail),
		MasterToken:    getStringFromMap(connector, credentialKeyMasterToken),
		AndroidID:      getStringFromMap(connector, credentialKeyAndroidID),
		ConnectedAt:    connectedAt,
		LastVerifiedAt: lastVerifiedAt,
	}, nil
}

// LoadMasterToken implements CredentialReader (used by KeepAdapter).
// Returns ErrCredentialMissing when the profile has no master token.
// Synthesizes a deterministic Android device id from the profile id
// when none is persisted, so the gateway always sees a stable value.
func (s *FrontmatterCredentialStore) LoadMasterToken(ctx context.Context, profileID wikipage.PageIdentifier) (MasterTokenBundle, error) {
	bundle, err := s.LoadCredentials(ctx, profileID)
	if err != nil {
		return MasterTokenBundle{}, err
	}
	if !bundle.IsConfigured() {
		return MasterTokenBundle{}, ErrCredentialMissing
	}
	deviceID := bundle.AndroidID
	if deviceID == "" {
		deviceID = deriveDeviceID(profileID)
	}
	return MasterTokenBundle{
		MasterToken: bundle.MasterToken,
		Email:       bundle.Email,
		AndroidID:   deviceID,
	}, nil
}

// PersistMasterToken stamps the credential bundle after a successful
// gpsoauth round-trip (oauth_token → master_token via Stage 1b, then
// master_token → bearer via Stage 2 to verify). Records connected_at
// on first connect and last_verified_at every time. Persists the
// device id (synthesizing one if missing) so subsequent calls use a
// stable value.
//
// After the write succeeds, fans out engine.Resume across every
// binding for the profile so paused bindings transition back to
// active (within-horizon resume keeps the cursor; >7d routes through
// ForceFullResync per ADR-0011).
func (s *FrontmatterCredentialStore) PersistMasterToken(ctx context.Context, profileID wikipage.PageIdentifier, masterToken, androidID, email string) error {
	if profileID == "" {
		return errors.New("google_keep: profileID is required")
	}
	if masterToken == "" {
		return errors.New("google_keep: master_token is required")
	}

	bundle, err := s.LoadCredentials(ctx, profileID)
	if err != nil {
		return fmt.Errorf("load credentials for %s: %w", profileID, err)
	}
	now := s.clock.Now().UTC()
	if email != "" {
		bundle.Email = email
	}
	bundle.MasterToken = masterToken
	if androidID != "" {
		bundle.AndroidID = androidID
	}
	if bundle.AndroidID == "" {
		bundle.AndroidID = deriveDeviceID(profileID)
	}
	if bundle.ConnectedAt.IsZero() {
		bundle.ConnectedAt = now
	}
	bundle.LastVerifiedAt = now

	if err := s.writeCredentials(profileID, bundle); err != nil {
		return fmt.Errorf("persist credentials for %s: %w", profileID, err)
	}

	// Auto-resume paused bindings via the engine hook. Best-effort —
	// the next scheduler tick re-attempts naturally on failure.
	if s.resumeAll != nil {
		if resumeErr := s.resumeAll(ctx, profileID); resumeErr != nil {
			s.logger.Error("google_keep: auto-resume after reconnect failed for profile=%s: %v",
				profileID, resumeErr)
		}
	}
	return nil
}

// ClearCredentials wipes the master token from the profile and
// transitions every active binding to paused. Mirrors the legacy
// Disconnect semantics: bindings are preserved (a reconnect resumes
// them) but the scheduler stops syncing against a missing token.
//
// The Android device id is preserved across Disconnect so a
// subsequent reconnect uses the same value (no "new device"
// heuristic on Google's side).
//
// Returns the post-clear CredentialBundle (master_token == "") so
// the gRPC handler can render a "not configured" state response.
func (s *FrontmatterCredentialStore) ClearCredentials(ctx context.Context, profileID wikipage.PageIdentifier) (CredentialBundle, error) {
	bundle, err := s.LoadCredentials(ctx, profileID)
	if err != nil {
		return CredentialBundle{}, err
	}
	bundle.MasterToken = ""
	bundle.LastVerifiedAt = time.Time{}
	if err := s.writeCredentials(profileID, bundle); err != nil {
		return CredentialBundle{}, fmt.Errorf("clear credentials for %s: %w", profileID, err)
	}
	if s.pauseAll != nil {
		if pauseErr := s.pauseAll(ctx, profileID, PausedReasonAuthFailed); pauseErr != nil {
			s.logger.Error("google_keep: pause-all-bindings after Disconnect failed for profile=%s: %v",
				profileID, pauseErr)
		}
	}
	return bundle, nil
}

// writeCredentials does the read-modify-write on the profile's
// frontmatter so unrelated fields under wiki.connectors.* (Tasks,
// bindings, etc.) are preserved untouched.
func (s *FrontmatterCredentialStore) writeCredentials(profileID wikipage.PageIdentifier, bundle CredentialBundle) error {
	_, fm, err := s.pages.ReadFrontMatter(profileID)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read frontmatter: %w", err)
	}
	if fm == nil {
		fm = make(wikipage.FrontMatter)
	}
	connector := ensureKeepCredentialMap(fm)
	if bundle.Email != "" {
		connector[credentialKeyEmail] = bundle.Email
	} else {
		delete(connector, credentialKeyEmail)
	}
	if bundle.MasterToken != "" {
		connector[credentialKeyMasterToken] = bundle.MasterToken
	} else {
		delete(connector, credentialKeyMasterToken)
	}
	if bundle.AndroidID != "" {
		connector[credentialKeyAndroidID] = bundle.AndroidID
	} else {
		delete(connector, credentialKeyAndroidID)
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

// readKeepCredentialMap returns wiki.connectors.google_keep
// (read-only), or nil when any segment is missing or wrong-typed.
func readKeepCredentialMap(fm wikipage.FrontMatter) map[string]any {
	wiki, ok := fm[credentialKeyWiki].(map[string]any)
	if !ok {
		return nil
	}
	conns, ok := wiki[credentialKeyConnectors].(map[string]any)
	if !ok {
		return nil
	}
	gk, ok := conns[credentialKeyKeep].(map[string]any)
	if !ok {
		return nil
	}
	return gk
}

// ensureKeepCredentialMap returns wiki.connectors.google_keep,
// creating any missing intermediate maps. Used by writeCredentials so
// a fresh profile (no prior frontmatter) gets a proper subtree.
func ensureKeepCredentialMap(fm wikipage.FrontMatter) map[string]any {
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
	gk, ok := conns[credentialKeyKeep].(map[string]any)
	if !ok {
		gk = make(map[string]any)
		conns[credentialKeyKeep] = gk
	}
	return gk
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

// deriveDeviceID returns a stable 16-hex-char device id derived from
// the profile id. Matches the gpsoauth requirement of a stable per-
// account android id without reusing any real device's id. Mirrors
// the legacy package's same-named helper.
func deriveDeviceID(profileID wikipage.PageIdentifier) string {
	sum := sha256.Sum256([]byte(profileID))
	return hex.EncodeToString(sum[:8]) // 16 hex chars
}

// Compile-time check: FrontmatterCredentialStore satisfies the
// CredentialReader contract the KeepAdapter consumes.
var _ CredentialReader = (*FrontmatterCredentialStore)(nil)

// AuthVerifier performs the full gpsoauth round-trip + Keep verify
// pull on the operator's behalf. Mirrors the legacy package's Connect
// flow but factored out so the credential store stays the sole owner
// of profile-side credential state.
//
// Implementations are responsible for:
//
//  1. Exchanging the captured oauth_token for a long-lived master
//     token (Stage 1b).
//  2. Exchanging the master token for a short-lived bearer (Stage 2)
//     to confirm the token Google issued is acceptable to Keep.
//  3. Calling the Keep Changes endpoint with that bearer to confirm
//     the master token works against the actual API (a no-mutation
//     pull).
//
// The bootstrap-supplied implementation wraps gateway.Authenticator
// and a transient KeepClient; tests inject a fake.
type AuthVerifier interface {
	VerifyOAuthToken(ctx context.Context, profileID wikipage.PageIdentifier, email, oauthToken, androidID string) (masterToken string, err error)
}

// Connect performs the full reconnect flow: oauth_token → master
// token → bearer → verify with a Keep changes call → persist. Mirrors
// the legacy connector.Connect semantics but uses the engine path's
// credential store.
//
// The injected AuthVerifier owns the gpsoauth + Keep round trip; on
// success this method persists the new master token via
// PersistMasterToken (which fans out engine.Resume across paused
// bindings). On any failure, no state is written.
func (s *FrontmatterCredentialStore) Connect(ctx context.Context, profileID wikipage.PageIdentifier, email, oauthToken string, verifier AuthVerifier) (CredentialBundle, error) {
	if email == "" {
		return CredentialBundle{}, errors.New("google_keep: email is required")
	}
	if oauthToken == "" {
		return CredentialBundle{}, errors.New("google_keep: oauth_token is required")
	}
	if verifier == nil {
		return CredentialBundle{}, errors.New("google_keep: verifier is required")
	}

	// Re-use the existing android_id when one is persisted so a
	// reconnect doesn't trip Google's "new device" heuristic.
	bundle, err := s.LoadCredentials(ctx, profileID)
	if err != nil {
		return CredentialBundle{}, err
	}
	androidID := bundle.AndroidID
	if androidID == "" {
		androidID = deriveDeviceID(profileID)
	}

	masterToken, err := verifier.VerifyOAuthToken(ctx, profileID, email, oauthToken, androidID)
	if err != nil {
		return CredentialBundle{}, err
	}
	if err := s.PersistMasterToken(ctx, profileID, masterToken, androidID, email); err != nil {
		return CredentialBundle{}, err
	}
	return s.LoadCredentials(ctx, profileID)
}
