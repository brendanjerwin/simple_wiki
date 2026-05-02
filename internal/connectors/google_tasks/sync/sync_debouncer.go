package sync

import (
	"fmt"
	"sync"
	"time"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// JobEnqueuer is the subset of *jobs.JobQueueCoordinator the
// debouncer uses. Stated as an interface so tests can substitute a
// fake.
type JobEnqueuer interface {
	EnqueueJob(job jobs.Job) error
}

// SyncDebouncer turns a stream of "checklist X mutated" events into
// at-most-one outbound sync job per (profile, page, listName) per
// debounceWindow. Implements checklistmutator.Subscriber.
//
// Why debounce: a user toggling 50 items rapidly should produce ONE
// Tasks push at the end, not 50 — Google's per-user write quota and
// the wiki's preference for atomic tick semantics both favor
// coalescing.
//
// Why per-key (not global): a user editing list A shouldn't have list
// B's pending sync delayed; each subscription has independent
// freshness.
//
// Note (Option A vs B): the Keep bridge has a near-identical
// SyncDebouncer in its sibling package. Per the plan, "if Tasks needs
// the same shape, a generalized version is extracted with two real
// implementations as evidence." The two implementations differ in the
// job type they enqueue (KeepOutboundSyncJob vs TasksOutboundSyncJob)
// and in nothing else of substance. We keep them separate (Option A)
// for now: extracting them shares the cleanup-agent's edit window
// (concurrent edits on the Keep file would race), and the duplication
// is shallow enough that a follow-up generalization can collapse them
// at low risk. Documented for the next reviewer / extractor.
type SyncDebouncer struct {
	enqueuer       JobEnqueuer
	connector      *Connector
	logger         Logger
	debounceWindow time.Duration

	mu         sync.Mutex
	timers     map[string]*time.Timer // key: "<profileID>|<page>|<listName>"
	suppressed map[string]int         // refcount: nonzero = inbound-apply in progress
}

// NewSyncDebouncer constructs a debouncer.
func NewSyncDebouncer(
	enqueuer JobEnqueuer,
	connector *Connector,
	logger Logger,
	debounceWindow time.Duration,
) (*SyncDebouncer, error) {
	if enqueuer == nil {
		return nil, fmt.Errorf("tasks bridge: enqueuer must not be nil")
	}
	if connector == nil {
		return nil, fmt.Errorf("tasks bridge: connector must not be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("tasks bridge: logger must not be nil")
	}
	if debounceWindow <= 0 {
		return nil, fmt.Errorf("tasks bridge: debounceWindow must be > 0")
	}
	return &SyncDebouncer{
		enqueuer:       enqueuer,
		connector:      connector,
		logger:         logger,
		debounceWindow: debounceWindow,
		timers:         map[string]*time.Timer{},
		suppressed:     map[string]int{},
	}, nil
}

// Suppress marks a (profile, page, list) as "inbound-apply in
// progress" — OnChecklistMutated calls for this key during the
// suppress window are ignored. Refcounted so nested apply windows
// compose cleanly. Always pair with Unsuppress in a defer.
func (d *SyncDebouncer) Suppress(profileID wikipage.PageIdentifier, page, listName string) {
	key := debounceKey(profileID, page, listName)
	d.mu.Lock()
	defer d.mu.Unlock()
	d.suppressed[key]++
}

// Unsuppress decrements the suppress refcount for the given key.
func (d *SyncDebouncer) Unsuppress(profileID wikipage.PageIdentifier, page, listName string) {
	key := debounceKey(profileID, page, listName)
	d.mu.Lock()
	defer d.mu.Unlock()
	d.suppressed[key]--
	if d.suppressed[key] <= 0 {
		delete(d.suppressed, key)
	}
}

// SyncIdentityLoginName is the LoginName the inbound-sync code path
// uses for its synthetic identity. Hard-coded here (rather than
// imported from checklistmutator) to avoid a sync → checklistmutator
// import cycle. Distinct from the Keep bridge's identity so the
// debouncer can drop notifies originating from either bridge's
// inbound apply.
const SyncIdentityLoginName = "system:tasks-sync"

// OnChecklistMutated implements checklistmutator.Subscriber.
// Resolves the calling identity to a profileID, then debounces an
// outbound sync enqueue for (profileID, page, listName).
//
// Anonymous identities (no LoginName) are silently dropped — they
// can't have a subscription because Subscribe requires a real user.
//
// The synthetic SyncIdentity is dropped explicitly: it's used only
// when applyInboundFromTasks is writing inbound state via the
// mutator, and we MUST NOT re-enqueue a sync for that.
func (d *SyncDebouncer) OnChecklistMutated(page, listName string, identity tailscale.IdentityValue) {
	if identity == nil {
		return
	}
	login := identity.LoginName()
	if login == "" {
		return
	}
	if login == SyncIdentityLoginName {
		return
	}
	profileID, err := wikipage.ProfileIdentifierFor(login)
	if err != nil {
		d.logger.Error("tasks SyncDebouncer: resolve profile for login %q: %v", login, err)
		return
	}
	d.scheduleSync(profileID, page, listName)
}

// scheduleSync resets the per-key debounce timer.
func (d *SyncDebouncer) scheduleSync(profileID wikipage.PageIdentifier, page, listName string) {
	key := debounceKey(profileID, page, listName)

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

	job := NewTasksOutboundSyncJob(d.connector, profileID, page, listName)
	if err := d.enqueuer.EnqueueJob(job); err != nil {
		d.logger.Error("tasks SyncDebouncer: enqueue Tasks sync for profile=%s page=%s list=%s: %v",
			string(profileID), page, listName, err)
		return
	}
	d.logger.Info("tasks SyncDebouncer: enqueued Tasks sync profile=%s page=%s list=%s",
		string(profileID), page, listName)
}

func debounceKey(profileID wikipage.PageIdentifier, page, listName string) string {
	return fmt.Sprintf("%s|%s|%s", string(profileID), page, listName)
}

// subscriberShape mirrors checklistmutator.Subscriber to anchor a
// compile-time assertion that SyncDebouncer satisfies the structural
// interface SetSubscriber expects on the mutator.
type subscriberShape interface {
	OnChecklistMutated(page, listName string, identity tailscale.IdentityValue)
}

// Compile-time interface checks.
var (
	_ subscriberShape = (*SyncDebouncer)(nil)
	_ SyncSuppressor  = (*SyncDebouncer)(nil)
	_ jobs.Job        = (*TasksOutboundSyncJob)(nil)
)
