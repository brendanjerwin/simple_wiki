package sync

import (
	"fmt"
	"sync"
	"time"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// JobEnqueuer is the subset of *jobs.JobQueueCoordinator the debouncer
// uses. Stated as an interface so tests can substitute a fake.
type JobEnqueuer interface {
	EnqueueJob(job jobs.Job) error
}

// SubscriberLogger is the lumber-style log surface the debouncer uses.
// Method shapes match jcelliott/lumber's ConsoleLogger so the wiki's
// existing logger plugs in directly.
type SubscriberLogger interface {
	Info(format string, args ...any)
	Error(format string, args ...any)
}

// SyncDebouncer turns a stream of "checklist X mutated" events into
// at-most-one outbound sync job per (profile, page, listName) per
// debounceWindow. Implements checklistmutator.Subscriber.
//
// Why debounce: a user toggling 50 items rapidly should produce ONE
// Keep push at the end, not 50 — Keep API quota and the per-account
// targetVersion both penalize chatter. The debounce window resets on
// every event, so a steady stream of edits keeps deferring the push
// until the user pauses for `debounceWindow`.
//
// Why per-key (not global): a user editing list A shouldn't have list
// B's pending sync delayed; each binding has independent freshness.
type SyncDebouncer struct {
	enqueuer        JobEnqueuer
	connector       *Connector
	pages           wikipage.PageReaderMutator
	logger          SubscriberLogger
	debounceWindow  time.Duration

	mu         sync.Mutex
	timers     map[string]*time.Timer // key: "<profileID>|<page>|<listName>"
	suppressed map[string]int        // refcount: nonzero = inbound-apply in progress
}

// NewSyncDebouncer constructs a debouncer.
func NewSyncDebouncer(
	enqueuer JobEnqueuer,
	connector *Connector,
	pages wikipage.PageReaderMutator,
	logger SubscriberLogger,
	debounceWindow time.Duration,
) *SyncDebouncer {
	return &SyncDebouncer{
		enqueuer:       enqueuer,
		connector:      connector,
		pages:          pages,
		logger:         logger,
		debounceWindow: debounceWindow,
		timers:         map[string]*time.Timer{},
		suppressed:     map[string]int{},
	}
}

// Suppress marks a (profile, page, list) as "inbound-apply in
// progress" — OnChecklistMutated calls for this key during the
// suppress window are ignored. Refcounted so nested apply windows
// (multiple bindings on the same page, etc.) compose cleanly.
//
// Always pair with Unsuppress in a defer; missing the unsuppress
// permanently blocks future sync events for that binding.
func (d *SyncDebouncer) Suppress(profileID wikipage.PageIdentifier, page, listName string) {
	key := fmt.Sprintf("%s|%s|%s", string(profileID), page, listName)
	d.mu.Lock()
	defer d.mu.Unlock()
	d.suppressed[key]++
}

// Unsuppress decrements the suppress refcount for the given key. The
// underlying entry is removed when the count returns to zero so the
// map doesn't accumulate stale keys for short-lived bindings.
func (d *SyncDebouncer) Unsuppress(profileID wikipage.PageIdentifier, page, listName string) {
	key := fmt.Sprintf("%s|%s|%s", string(profileID), page, listName)
	d.mu.Lock()
	defer d.mu.Unlock()
	d.suppressed[key]--
	if d.suppressed[key] <= 0 {
		delete(d.suppressed, key)
	}
}

// syncIdentityLoginName is the LoginName the inbound-sync code path
// uses for its synthetic identity. Hard-coded here (rather than
// imported from checklistmutator.SyncIdentity) to avoid a bridge →
// checklistmutator import cycle. Kept in sync with the const in
// server/checklistmutator/sync_helpers.go.
const syncIdentityLoginName = "system:keep-sync"

// OnChecklistMutated implements checklistmutator.Subscriber. Resolves
// the calling identity to a profileID, then debounces an outbound
// sync enqueue for (profileID, page, listName).
//
// Anonymous identities (no LoginName) are silently dropped — they
// can't have a binding because Bind requires a real user.
//
// The synthetic SyncIdentity is dropped explicitly: it's used only
// when applyInboundFromKeep is writing inbound state via the mutator,
// and we MUST NOT re-enqueue a sync for that. The suppressor is also
// in play during apply, but matches by (real-user-profileID, page,
// listName); the synthetic identity would resolve to a different
// profileID and slip past the suppressor. Filtering at the source
// is the simpler invariant.
func (d *SyncDebouncer) OnChecklistMutated(page, listName string, identity tailscale.IdentityValue) {
	if identity == nil {
		return
	}
	login := identity.LoginName()
	if login == "" {
		return
	}
	if login == syncIdentityLoginName {
		return
	}
	profileID, err := wikipage.ProfileIdentifierFor(login)
	if err != nil {
		d.logger.Error("SyncDebouncer: resolve profile for login %q: %v", login, err)
		return
	}
	d.scheduleSync(profileID, page, listName)
}

// scheduleSync resets the per-key debounce timer. Suppressed keys are
// dropped silently — the inbound-apply pass is mid-flight and another
// sync run would loop.
func (d *SyncDebouncer) scheduleSync(profileID wikipage.PageIdentifier, page, listName string) {
	key := fmt.Sprintf("%s|%s|%s", string(profileID), page, listName)

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.suppressed[key] > 0 {
		return
	}
	if existing, ok := d.timers[key]; ok {
		existing.Stop()
	}
	d.timers[key] = time.AfterFunc(d.debounceWindow, func() {
		d.fireSync(key, profileID, page, listName)
	})
}

// fireSync is invoked by the debounce timer; clears the timer entry
// and enqueues the sync job.
func (d *SyncDebouncer) fireSync(key string, profileID wikipage.PageIdentifier, page, listName string) {
	d.mu.Lock()
	delete(d.timers, key)
	d.mu.Unlock()

	job := NewKeepOutboundSyncJob(d.connector, profileID, page, listName)
	if err := d.enqueuer.EnqueueJob(job); err != nil {
		d.logger.Error("SyncDebouncer: enqueue Keep sync for profile=%s page=%s list=%s: %v",
			string(profileID), page, listName, err)
		return
	}
	d.logger.Info("SyncDebouncer: enqueued Keep sync profile=%s page=%s list=%s",
		string(profileID), page, listName)
}

// Compile-time interface check: SyncDebouncer satisfies the structural
// interface SetSubscriber expects on the mutator.
var _ subscriberShape = (*SyncDebouncer)(nil)

// subscriberShape mirrors checklistmutator.Subscriber to anchor the
// compile-time assertion above without importing checklistmutator
// (which depends on bridge would create a cycle once we hook the
// reader the other way).
type subscriberShape interface {
	OnChecklistMutated(page, listName string, identity tailscale.IdentityValue)
}

// Compile-time check: KeepOutboundSyncJob satisfies pkg/jobs.Job so
// the coordinator can route it. (Same shape but anchoring it here too
// guards against a future drift where the package-level interface
// adds a method.)
var _ jobs.Job = (*KeepOutboundSyncJob)(nil)
