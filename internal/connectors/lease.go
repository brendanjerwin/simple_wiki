package connectors

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ErrChecklistAlreadyLeased is returned by LeaseTable.Take when the
// requested checklist is already leased by a different owner. Callers
// at the gRPC boundary surface this as AlreadyExists with the current
// owner named in the message.
var ErrChecklistAlreadyLeased = errors.New("connectors: checklist already leased")

// ChecklistKey identifies a checklist independently of which user
// owns its binding. The (Page, ListName) tuple is the aggregate
// root per ADR-0011 — the at-most-one-Binding invariant is keyed
// on this tuple, not on (ProfileID, Page, ListName).
type ChecklistKey struct {
	Page     string
	ListName string
}

// LeaseOwner records which connector + profile currently owns a
// checklist's lease. Returned by LookupOwner so callers can render
// "this checklist is currently bound to <Kind> by <ProfileID>"
// in the UI without needing to resolve the lease themselves.
type LeaseOwner struct {
	Kind      ConnectorKind
	ProfileID string
}

// LeaseTable is the in-memory registry of which (page, list_name) is
// currently leased by which connector + profile. Per ADR-0011, the
// authoritative source is the per-profile Binding record on the
// user profile page; this table is a derived view rebuilt at boot
// from a fan-out scan of all profile pages.
//
// All lease mutations go through per-checklist mutexes so that the
// "bind ceremony" — fan-out re-read inside the mutex, then
// write profile + take lease — is atomic with respect to concurrent
// Bind calls on the same checklist.
//
// LeaseTable does NOT own multi-process coordination. Single-process
// deployment is assumed (ADR-0011); deploying multiple wiki processes
// against the same data dir would race the profile writes, which is
// outside the lease table's responsibility.
type LeaseTable struct {
	// mu guards leases and locks. Held briefly — never across
	// per-checklist callbacks.
	mu     sync.Mutex
	leases map[ChecklistKey]LeaseOwner
	locks  map[ChecklistKey]*sync.Mutex

	// readyMu + ready gate WaitReady(ctx). The boot-time fan-out
	// rebuild calls SignalReady when complete; gRPC handlers wrap
	// their entrypoints with WaitReady so requests block during
	// rebuild rather than seeing false-negatives from LookupOwner.
	readyMu sync.Mutex
	ready   chan struct{}
}

// NewLeaseTable constructs an empty lease table. Callers MUST call
// SignalReady once the boot-time fan-out rebuild has completed; until
// then WaitReady blocks.
func NewLeaseTable() *LeaseTable {
	return &LeaseTable{
		leases: map[ChecklistKey]LeaseOwner{},
		locks:  map[ChecklistKey]*sync.Mutex{},
		ready:  make(chan struct{}),
	}
}

// SignalReady marks the lease table as fully populated. Subsequent
// WaitReady calls return immediately. Idempotent — calling twice has
// no effect, matching the boot-rebuild's "fire once when scan completes"
// invariant.
func (lt *LeaseTable) SignalReady() {
	lt.readyMu.Lock()
	defer lt.readyMu.Unlock()
	select {
	case <-lt.ready:
		// Already closed; second SignalReady is a no-op.
	default:
		close(lt.ready)
	}
}

// WaitReady blocks until SignalReady has been called or ctx is
// cancelled. Returns ctx.Err() on cancellation, nil on success.
//
// gRPC handlers and the bind-button frontend gate their reads
// through WaitReady so the boot-rebuild window doesn't surface as
// false-negative LookupOwner results.
func (lt *LeaseTable) WaitReady(ctx context.Context) error {
	select {
	case <-lt.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Take attempts to take a lease on the given checklist for owner.
// Returns ErrChecklistAlreadyLeased if a different owner already holds
// the lease. Same-owner re-takes are idempotent (no error).
//
// Take does NOT acquire the per-checklist mutex — callers wrap Take
// in WithChecklistLock when they need the bind-ceremony's atomic
// fan-out re-read + lease take. Standalone Take calls are valid for
// the boot-rebuild path (single-threaded; no concurrent Bind).
func (lt *LeaseTable) Take(key ChecklistKey, owner LeaseOwner) error {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	if existing, ok := lt.leases[key]; ok {
		if existing == owner {
			return nil
		}
		return fmt.Errorf("%w: %s/%s held by %s/%s",
			ErrChecklistAlreadyLeased, key.Page, key.ListName,
			existing.Kind, existing.ProfileID)
	}
	lt.leases[key] = owner
	return nil
}

// Release drops the lease for the given checklist. No-op if the
// lease is unowned. Returns no error on owner-mismatch — release is
// "release-if-mine-or-already-released"; the bind ceremony is
// where the strong invariant lives.
func (lt *LeaseTable) Release(key ChecklistKey) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	delete(lt.leases, key)
}

// LookupOwner returns the current lease owner for the given checklist
// and a bool indicating presence. Used by the bind-button picker
// to filter out checklists already owned by another connector and by
// the tombstone GC walker to extend retention for paused bindings.
func (lt *LeaseTable) LookupOwner(key ChecklistKey) (LeaseOwner, bool) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	owner, ok := lt.leases[key]
	return owner, ok
}

// WithChecklistLock acquires the per-checklist mutex for key, runs fn
// while holding it, and releases the mutex before returning fn's
// error. Used by the bind ceremony to make the fan-out re-read +
// profile-write + lease-take sequence atomic against concurrent
// binders on the same checklist.
//
// The mutex is created lazily on first reference to that checklist
// and retained for the lifetime of the table — a household-scale
// wiki has bounded checklist cardinality, so the map doesn't grow
// without bound.
func (lt *LeaseTable) WithChecklistLock(key ChecklistKey, fn func() error) error {
	lt.mu.Lock()
	mu, ok := lt.locks[key]
	if !ok {
		mu = &sync.Mutex{}
		lt.locks[key] = mu
	}
	lt.mu.Unlock()
	mu.Lock()
	defer mu.Unlock()
	return fn()
}
