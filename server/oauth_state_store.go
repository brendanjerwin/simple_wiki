// Package server — OAuth state-token store.
//
// The OAuth callback handler at /oauth/google/callback (see
// oauth_google_handler.go) uses this store to bind authorization-code
// callbacks to the originating browser session. The state token also
// carries the PKCE code_verifier that the handler will replay to
// Google's token endpoint.
//
// Security profile (per the plan's "OAuth security profile" section
// and RFCs 6749 §10.12, 7636, 9700):
//
//   - State value is ≥256 bits from crypto/rand, base64url-encoded.
//   - Entries are keyed BY the state value (not by profile ID), so a
//     stolen profile ID can't be replayed to drain the store.
//   - Single-use: Consume deletes the entry on first lookup. A second
//     Consume with the same state value returns ErrStateNotFound
//     (replay is indistinguishable from "expired" by design).
//   - 10-minute TTL with a periodic GC sweep. Consume rejects expired
//     entries even if GC hasn't run yet, so timing-of-GC isn't
//     security-load-bearing — it's just memory hygiene.
package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"
)

// oauthStateBytes is the entropy of a state token in bytes. 32 bytes →
// 256 bits, satisfying RFC 6749 §10.12 / 9700 minimum recommendation.
const oauthStateBytes = 32

// oauthStateTTL is how long a state entry remains valid. Per the plan,
// 10 minutes — long enough for the user to complete consent on Google's
// side, short enough that a leaked log entry isn't useful tomorrow.
const oauthStateTTL = 10 * time.Minute

// oauthStateGCInterval is how often the background GC sweeps expired
// entries. Roughly TTL/3 keeps the store small without spamming the
// scheduler.
const oauthStateGCInterval = 3 * time.Minute

// ErrStateNotFound is returned by Consume when the state value is not
// in the store — either because it was never issued, was already
// consumed (single-use), or has expired past its TTL.
//
// The handler MUST NOT distinguish "expired" from "never existed" in
// the user-facing error: doing so leaks information about the store's
// timing behavior. Both cases route to the same "session expired, try
// again" page.
var ErrStateNotFound = errors.New("oauth state: not found or expired")

// OAuthStateEntry is the per-state record persisted by the store. It
// binds a callback's `state` parameter to the originating profile and
// the PKCE code_verifier that must accompany the authorization-code
// exchange.
//
// CodeChallengeMethod is always "S256" in this codebase (no plain
// fallback per RFC 9700 §2.1.1) — surfaced in the entry anyway so a
// future iCloud/Microsoft connector that *also* uses S256 can share
// this store with the field still meaningful at a glance.
type OAuthStateEntry struct {
	ProfileID           string
	CodeVerifier        string
	CodeChallengeMethod string
	CreatedAt           time.Time
}

// OAuthStateStore is the abstraction the handler depends on. The
// in-memory implementation lives in this file; tests substitute a fake.
type OAuthStateStore interface {
	// Issue mints a new state value, persists an entry binding it to
	// profileID + codeVerifier (with method "S256"), and returns the
	// state value the caller embeds in the auth URL.
	Issue(ctx context.Context, profileID, codeVerifier string) (string, error)

	// Consume looks up and atomically deletes the entry for the given
	// state value. Returns ErrStateNotFound for unknown, already-used,
	// or expired entries.
	Consume(ctx context.Context, state string) (OAuthStateEntry, error)
}

// inMemoryOAuthStateStore is a process-local sync.Map-backed
// implementation of OAuthStateStore. It satisfies the single-process
// assumption documented in ADR-0011 — this wiki is not designed for
// multi-process coordination.
type inMemoryOAuthStateStore struct {
	mu      sync.Mutex
	entries map[string]OAuthStateEntry
	now     func() time.Time
	stop    chan struct{}
	stopped bool
}

// NewInMemoryOAuthStateStore constructs an in-memory store and kicks
// off a background GC goroutine. The returned OAuthStateStore is backed
// by an in-memory map and is suitable for single-process use only (see
// ADR-0011). Tests should call Close on the underlying implementation to
// stop the background GC goroutine; production callers rely on process
// lifetime.
func NewInMemoryOAuthStateStore() OAuthStateStore {
	return newInMemoryOAuthStateStoreWithClock(time.Now)
}

// newInMemoryOAuthStateStoreWithClock is the test seam — Ginkgo specs
// inject a controllable clock so they don't have to wait real time
// for TTL expiry.
func newInMemoryOAuthStateStoreWithClock(now func() time.Time) *inMemoryOAuthStateStore {
	s := &inMemoryOAuthStateStore{
		entries: make(map[string]OAuthStateEntry),
		now:     now,
		stop:    make(chan struct{}),
	}
	go s.gcLoop()
	return s
}

// Close stops the background GC goroutine. Safe to call multiple times.
func (s *inMemoryOAuthStateStore) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	s.stopped = true
	close(s.stop)
}

// Issue mints a fresh state value and persists the entry.
func (s *inMemoryOAuthStateStore) Issue(_ context.Context, profileID, codeVerifier string) (string, error) {
	if profileID == "" {
		return "", errors.New("oauth state: profileID is required")
	}
	if codeVerifier == "" {
		return "", errors.New("oauth state: codeVerifier is required")
	}

	state, err := generateOAuthState()
	if err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[state] = OAuthStateEntry{
		ProfileID:           profileID,
		CodeVerifier:        codeVerifier,
		CodeChallengeMethod: "S256",
		CreatedAt:           s.now(),
	}
	return state, nil
}

// Consume atomically looks up and removes the entry. Expired entries
// are treated as not-found (and removed as a side effect).
func (s *inMemoryOAuthStateStore) Consume(_ context.Context, state string) (OAuthStateEntry, error) {
	if state == "" {
		return OAuthStateEntry{}, ErrStateNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[state]
	if !ok {
		return OAuthStateEntry{}, ErrStateNotFound
	}
	delete(s.entries, state)

	if s.now().Sub(entry.CreatedAt) > oauthStateTTL {
		return OAuthStateEntry{}, ErrStateNotFound
	}
	return entry, nil
}

// gcLoop sweeps expired entries on a fixed interval. Memory hygiene
// only — Consume already rejects expired entries on the read path.
func (s *inMemoryOAuthStateStore) gcLoop() {
	ticker := time.NewTicker(oauthStateGCInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.gcOnce()
		}
	}
}

// gcOnce performs one sweep. Exported via package-level test helper
// only; production code should rely on the loop.
func (s *inMemoryOAuthStateStore) gcOnce() {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := s.now().Add(-oauthStateTTL)
	for k, v := range s.entries {
		if v.CreatedAt.Before(cutoff) {
			delete(s.entries, k)
		}
	}
}

// generateOAuthState returns a cryptographically random base64url
// string suitable for use as an OAuth state parameter.
func generateOAuthState() (string, error) {
	b := make([]byte, oauthStateBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("read random: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
