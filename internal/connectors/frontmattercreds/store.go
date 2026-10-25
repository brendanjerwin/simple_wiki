// Package frontmattercreds provides the generic read/write core for
// per-profile connector credential bundles stored in wiki frontmatter.
// Both googlekeep and googletasks use this package, parameterizing it
// over their connector-specific bundle types via the BundleCodec interface.
package frontmattercreds

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

const (
	// PausedReasonAuthFailed is the canonical pause reason when
	// credentials are cleared or authentication fails.
	PausedReasonAuthFailed = "auth_failed"

	credentialKeyWiki           = "wiki"
	credentialKeyConnectors     = "connectors"
	credentialKeyConnectedAt    = "connected_at"
	credentialKeyLastVerifiedAt = "last_verified_at"
)

// PauseAllBindingsHook is the engine-side hook called when credentials
// are cleared: every active binding for the profile must be transitioned
// to paused (PausedReason=auth_failed) so the scheduler stops driving
// Sync against an empty token.
//
// A nil hook means "skip pause fan-out" — used by tests.
type PauseAllBindingsHook func(ctx context.Context, profileID wikipage.PageIdentifier, reason string) error

// ResumeAllBindingsHook is the engine-side hook called after a
// successful reconnect. engine.Resume is idempotent on active bindings
// and routes paused bindings >= 7d to ForceFullResync.
//
// A nil hook means "skip resume fan-out" — used by tests.
type ResumeAllBindingsHook func(ctx context.Context, profileID wikipage.PageIdentifier) error

// FrontmatterReadWriter is the wiki-side seam this package uses.
type FrontmatterReadWriter interface {
	ReadFrontMatter(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error)
	WriteFrontMatter(identifier wikipage.PageIdentifier, fm wikipage.FrontMatter, identity wikipage.Identity) error
}

// Clocker is the testable wall-clock seam.
type Clocker interface {
	Now() time.Time
}

// SystemClock returns time.Now(). The production wiring uses this;
// tests inject a deterministic stub.
type SystemClock struct{}

// Now returns the current wall-clock time.
func (SystemClock) Now() time.Time { return time.Now() }

// Logger is the minimal log surface needed by the Store.
type Logger interface {
	Info(format string, args ...any)
	Error(format string, args ...any)
}

// BundleCodec maps the connector-specific token fields in a credential
// bundle to and from the frontmatter sub-map entries. Each connector
// supplies a concrete implementation that handles only its own fields;
// the Store owns the shared structural plumbing (timestamp parsing,
// read-modify-write, hook fan-out).
type BundleCodec[B any] interface {
	// ConnectorKey is the frontmatter key for this connector's subtree
	// under wiki.connectors.* (e.g., "google_keep", "google_tasks").
	ConnectorKey() string

	// ErrorPrefix is prepended to error strings for this connector
	// (e.g., "google_keep", "google_tasks").
	ErrorPrefix() string

	// AssembleBundle constructs a bundle from the frontmatter sub-map
	// and the already-parsed timestamp fields. The Store handles
	// timestamp parsing so codec implementations stay simple.
	AssembleBundle(m map[string]any, connectedAt, lastVerifiedAt time.Time) B

	// WriteTokenFields writes only the connector-specific (non-timestamp)
	// fields from b into m, deleting keys whose field is empty/zero.
	// The Store writes timestamps separately.
	WriteTokenFields(b B, m map[string]any)

	// Timestamps returns the bundle's ConnectedAt and LastVerifiedAt
	// fields so the Store can write them without knowing the concrete type.
	Timestamps(b B) (connectedAt, lastVerifiedAt time.Time)

	// IsConfigured reports whether b carries a valid primary token.
	IsConfigured(b B) bool

	// ClearForDisconnect blanks the primary token and clears
	// LastVerifiedAt in b, leaving other fields (e.g., email, android_id)
	// intact for a future reconnect.
	ClearForDisconnect(b *B)
}

// Store provides the generic frontmatter read/write core for
// connector credential bundles. It holds the shared plumbing;
// the connector-specific field handling is delegated to the BundleCodec.
//
// Concurrency: there is no per-profile mutex on this struct. The
// engine's FrontmatterBindingStore.WithProfileLock serializes bind
// flows; the auth-callback path serializes itself; GetState is a
// read-only path.
type Store[B any] struct {
	pages     FrontmatterReadWriter
	clock     Clocker
	logger    Logger
	pauseAll  PauseAllBindingsHook
	resumeAll ResumeAllBindingsHook
	codec     BundleCodec[B]
}

// NewStore creates a Store[B]. pages, clock, and logger are required;
// the pause/resume hooks may be nil for tests.
func NewStore[B any](
	pages FrontmatterReadWriter,
	clock Clocker,
	logger Logger,
	pauseAll PauseAllBindingsHook,
	resumeAll ResumeAllBindingsHook,
	codec BundleCodec[B],
) (*Store[B], error) {
	if pages == nil {
		return nil, fmt.Errorf("%s: pages must not be nil", codec.ErrorPrefix())
	}
	if clock == nil {
		return nil, fmt.Errorf("%s: clock must not be nil", codec.ErrorPrefix())
	}
	if logger == nil {
		return nil, fmt.Errorf("%s: logger must not be nil", codec.ErrorPrefix())
	}
	return &Store[B]{
		pages:     pages,
		clock:     clock,
		logger:    logger,
		pauseAll:  pauseAll,
		resumeAll: resumeAll,
		codec:     codec,
	}, nil
}

// Clock returns the injected clock. Connector wrappers use this for
// timestamping in their custom Persist* methods.
func (s *Store[B]) Clock() Clocker { return s.clock }

// LoadCredentials reads the per-profile credential bundle. A missing
// page or absent connector subtree returns a zero B with no error —
// callers branch on the codec's IsConfigured.
func (s *Store[B]) LoadCredentials(_ context.Context, profileID wikipage.PageIdentifier) (B, error) {
	var zero B
	_, fm, err := s.pages.ReadFrontMatter(profileID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return zero, nil
		}
		return zero, fmt.Errorf("%s: read frontmatter for %s: %w", s.codec.ErrorPrefix(), profileID, err)
	}
	connector := readConnectorMap(fm, s.codec.ConnectorKey())
	if connector == nil {
		return zero, nil
	}
	connectedAt, err := parseRFC3339(GetStringFromMap(connector, credentialKeyConnectedAt))
	if err != nil {
		return zero, fmt.Errorf("wiki.connectors.%s.connected_at: %w", s.codec.ConnectorKey(), err)
	}
	lastVerifiedAt, err := parseRFC3339(GetStringFromMap(connector, credentialKeyLastVerifiedAt))
	if err != nil {
		return zero, fmt.Errorf("wiki.connectors.%s.last_verified_at: %w", s.codec.ConnectorKey(), err)
	}
	return s.codec.AssembleBundle(connector, connectedAt, lastVerifiedAt), nil
}

// WriteCredentials does the read-modify-write on the profile's
// frontmatter so unrelated fields under wiki.connectors.* are preserved.
func (s *Store[B]) WriteCredentials(profileID wikipage.PageIdentifier, bundle B) error {
	_, fm, err := s.pages.ReadFrontMatter(profileID)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read frontmatter: %w", err)
	}
	if fm == nil {
		fm = make(wikipage.FrontMatter)
	}
	connector := ensureConnectorMap(fm, s.codec.ConnectorKey())
	s.codec.WriteTokenFields(bundle, connector)
	connectedAt, lastVerifiedAt := s.codec.Timestamps(bundle)
	if !connectedAt.IsZero() {
		connector[credentialKeyConnectedAt] = connectedAt.UTC().Format(time.RFC3339)
	} else {
		delete(connector, credentialKeyConnectedAt)
	}
	if !lastVerifiedAt.IsZero() {
		connector[credentialKeyLastVerifiedAt] = lastVerifiedAt.UTC().Format(time.RFC3339)
	} else {
		delete(connector, credentialKeyLastVerifiedAt)
	}
	if err := s.pages.WriteFrontMatter(profileID, fm, wikipage.AnonymousIdentity); err != nil {
		return fmt.Errorf("write frontmatter: %w", err)
	}
	return nil
}

// ResumeBindings calls the resume hook (best-effort). Any hook error is
// logged but not returned — the next scheduler tick re-evaluates
// pause-state naturally.
func (s *Store[B]) ResumeBindings(ctx context.Context, profileID wikipage.PageIdentifier) {
	if s.resumeAll != nil {
		if err := s.resumeAll(ctx, profileID); err != nil {
			s.logger.Error("%s: auto-resume after reconnect failed for profile=%s: %v",
				s.codec.ErrorPrefix(), profileID, err)
		}
	}
}

// ClearCredentials wipes the primary token from the profile and
// transitions every active binding to paused. Bindings are preserved
// so a reconnect can resume them. Returns the post-clear bundle.
func (s *Store[B]) ClearCredentials(ctx context.Context, profileID wikipage.PageIdentifier) (B, error) {
	bundle, err := s.LoadCredentials(ctx, profileID)
	if err != nil {
		return bundle, err
	}
	s.codec.ClearForDisconnect(&bundle)
	if err := s.WriteCredentials(profileID, bundle); err != nil {
		return bundle, fmt.Errorf("clear credentials for %s: %w", profileID, err)
	}
	if s.pauseAll != nil {
		if pauseErr := s.pauseAll(ctx, profileID, PausedReasonAuthFailed); pauseErr != nil {
			s.logger.Error("%s: pause-all-bindings after Disconnect failed for profile=%s: %v",
				s.codec.ErrorPrefix(), profileID, pauseErr)
		}
	}
	return bundle, nil
}

// GetStringFromMap returns the string value for key in m, or "" if
// absent or not a string. Exported so BundleCodec implementations in
// connector packages can use it in their AssembleBundle methods.
func GetStringFromMap(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// readConnectorMap returns wiki.connectors.<connectorKey> (read-only),
// or nil when any path segment is missing or the wrong type.
func readConnectorMap(fm wikipage.FrontMatter, connectorKey string) map[string]any {
	wiki, ok := fm[credentialKeyWiki].(map[string]any)
	if !ok {
		return nil
	}
	conns, ok := wiki[credentialKeyConnectors].(map[string]any)
	if !ok {
		return nil
	}
	m, ok := conns[connectorKey].(map[string]any)
	if !ok {
		return nil
	}
	return m
}

// ensureConnectorMap returns wiki.connectors.<connectorKey>, creating
// any missing intermediate maps. Used by WriteCredentials so a fresh
// profile (no prior frontmatter) gets a proper subtree.
func ensureConnectorMap(fm wikipage.FrontMatter, connectorKey string) map[string]any {
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
	m, ok := conns[connectorKey].(map[string]any)
	if !ok {
		m = make(map[string]any)
		conns[connectorKey] = m
	}
	return m
}

// parseRFC3339 parses an RFC3339 timestamp string, returning a zero
// time.Time for an empty string.
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
