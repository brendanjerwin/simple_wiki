// Package googlekeep: this file owns the Google Keep gpsoauth
// credential state machine — the per-profile bundle (email,
// master_token, android_id, connected_at, last_verified_at) on the
// user's profile page.
//
// Per the SyncEngine extraction (Phase 5), the gpsoauth handling
// that previously lived in the legacy Keep sync package now lives
// here. Bind/Unbind/Resume/Sync algorithms belong to
// internal/connectors/engine; this file only reads, writes, and
// clears the per-profile credential bundle.
package googlekeep

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/brendanjerwin/simple_wiki/internal/connectors/frontmattercreds"
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

// Frontmatter leaf names for the connector-specific fields.
const (
	credentialKeyEmail       = "email"
	credentialKeyMasterToken = "master_token"
	credentialKeyAndroidID   = "android_id"
)

// PausedReasonAuthFailed is the canonical reason string written into
// a binding's PausedReason when Disconnect is invoked or when the
// gpsoauth bearer is rejected by Keep.
const PausedReasonAuthFailed = frontmattercreds.PausedReasonAuthFailed

// PauseAllBindingsHook is the engine-side hook ClearCredentials calls
// when Disconnect runs.
type PauseAllBindingsHook = frontmattercreds.PauseAllBindingsHook

// ResumeAllBindingsHook is the engine-side hook PersistMasterToken
// calls after a successful reconnect.
type ResumeAllBindingsHook = frontmattercreds.ResumeAllBindingsHook

// FrontmatterReadWriter is the wiki-side seam this file uses.
type FrontmatterReadWriter = frontmattercreds.FrontmatterReadWriter

// keepBundleCodec implements frontmattercreds.BundleCodec[CredentialBundle].
// It maps the Keep-specific fields (master_token, android_id) to/from
// the frontmatter sub-map, while the shared Store handles timestamps and
// the read-modify-write plumbing.
type keepBundleCodec struct{}

func (keepBundleCodec) ConnectorKey() string { return "google_keep" }
func (keepBundleCodec) ErrorPrefix() string  { return "google_keep" }

func (keepBundleCodec) AssembleBundle(m map[string]any, connectedAt, lastVerifiedAt time.Time) CredentialBundle {
	return CredentialBundle{
		Email:          frontmattercreds.GetStringFromMap(m, credentialKeyEmail),
		MasterToken:    frontmattercreds.GetStringFromMap(m, credentialKeyMasterToken),
		AndroidID:      frontmattercreds.GetStringFromMap(m, credentialKeyAndroidID),
		ConnectedAt:    connectedAt,
		LastVerifiedAt: lastVerifiedAt,
	}
}

func (keepBundleCodec) WriteTokenFields(b CredentialBundle, m map[string]any) {
	if b.Email != "" {
		m[credentialKeyEmail] = b.Email
	} else {
		delete(m, credentialKeyEmail)
	}

	if b.MasterToken != "" {
		m[credentialKeyMasterToken] = b.MasterToken
	} else {
		delete(m, credentialKeyMasterToken)
	}

	if b.AndroidID != "" {
		m[credentialKeyAndroidID] = b.AndroidID
	} else {
		delete(m, credentialKeyAndroidID)
	}
}

func (keepBundleCodec) Timestamps(b CredentialBundle) (connectedAt, lastVerifiedAt time.Time) {
	return b.ConnectedAt, b.LastVerifiedAt
}

func (keepBundleCodec) IsConfigured(b CredentialBundle) bool { return b.MasterToken != "" }

func (keepBundleCodec) ClearForDisconnect(b *CredentialBundle) {
	b.MasterToken = ""
	b.LastVerifiedAt = time.Time{}
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
	core *frontmattercreds.Store[CredentialBundle]
}

// NewFrontmatterCredentialStore wires the production credential store.
// pages, clock, and logger are required; the pause/resume hooks may be
// nil for tests, in which case Disconnect/PersistMasterToken skip the
// engine fan-out (the next scheduler tick reconciles state naturally).
func NewFrontmatterCredentialStore(
	pages FrontmatterReadWriter,
	clock Clocker,
	logger Logger,
	pauseAll PauseAllBindingsHook,
	resumeAll ResumeAllBindingsHook,
) (*FrontmatterCredentialStore, error) {
	core, err := frontmattercreds.NewStore(pages, clock, logger, pauseAll, resumeAll, keepBundleCodec{})
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

	bundle, err := s.core.LoadCredentials(ctx, profileID)
	if err != nil {
		return fmt.Errorf("load credentials for %s: %w", profileID, err)
	}
	now := s.core.Clock().Now().UTC()
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

	if err := s.core.WriteCredentials(profileID, bundle); err != nil {
		return fmt.Errorf("persist credentials for %s: %w", profileID, err)
	}

	// Auto-resume paused bindings via the engine hook. Best-effort —
	// the next scheduler tick re-attempts naturally on failure.
	s.core.ResumeBindings(ctx, profileID)
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
	return s.core.ClearCredentials(ctx, profileID)
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
