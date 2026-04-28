package bridge

import (
	"sync"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// BindingKey is the (profile, page, list) tuple identifying one
// binding for the cron tick. Used both for in-memory tracking and as
// the work unit the tick enqueues into the sync queue.
type BindingKey struct {
	ProfileID wikipage.PageIdentifier
	Page      string
	ListName  string
}

// activeBindings tracks every binding the current process knows about
// so the cron tick can enumerate them without re-scanning the page
// store on each tick. Populated at bootstrap by RegisterActiveBindings
// (one-time scan of profile_* pages) and updated on Bind/Unbind.
type activeBindings struct {
	mu    sync.Mutex
	known map[BindingKey]struct{}
}

func newActiveBindings() *activeBindings {
	return &activeBindings{known: map[BindingKey]struct{}{}}
}

func (a *activeBindings) add(k BindingKey) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.known[k] = struct{}{}
}

func (a *activeBindings) remove(k BindingKey) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.known, k)
}

func (a *activeBindings) snapshot() []BindingKey {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]BindingKey, 0, len(a.known))
	for k := range a.known {
		out = append(out, k)
	}
	return out
}

// RegisterActiveBindings is called at bootstrap (after a one-time scan
// of profile_* pages) to seed the cron-tick's enumeration set with
// every binding that already exists on disk. Subsequent Bind/Unbind
// calls keep the set fresh in-process.
func (c *Connector) RegisterActiveBindings(keys []BindingKey) {
	c.activeMu.Lock()
	if c.active == nil {
		c.active = newActiveBindings()
	}
	c.activeMu.Unlock()
	for _, k := range keys {
		c.active.add(k)
	}
}

// ActiveBindingsSnapshot returns a copy of the currently-tracked
// binding set. Used by the cron tick to enumerate sync targets.
func (c *Connector) ActiveBindingsSnapshot() []BindingKey {
	c.activeMu.Lock()
	if c.active == nil {
		c.active = newActiveBindings()
	}
	c.activeMu.Unlock()
	return c.active.snapshot()
}

// noteBindingAdded is called from Bind after a successful AddBinding
// to keep the active-set in sync with on-disk state.
func (c *Connector) noteBindingAdded(k BindingKey) {
	c.activeMu.Lock()
	if c.active == nil {
		c.active = newActiveBindings()
	}
	c.activeMu.Unlock()
	c.active.add(k)
}

// noteBindingRemoved is called from Unbind for the same reason.
func (c *Connector) noteBindingRemoved(k BindingKey) {
	c.activeMu.Lock()
	if c.active == nil {
		c.active = newActiveBindings()
	}
	c.activeMu.Unlock()
	c.active.remove(k)
}

// BindingsLister returns the currently-known set of bindings. Called
// by the cron tick on every fire so the result reflects on-disk state
// (handles pre-existing bindings that weren't bound during this
// process's lifetime). Bootstrap satisfies this via the frontmatter
// index's QueryKeyExistence + per-profile decode.
type BindingsLister func() []BindingKey

// KeepCronTickJob enumerates active bindings via the BindingsLister
// callback and enqueues a sync job per binding. Cron schedule of
// "every 30 seconds" gives Keep-side edits a worst-case 30s latency
// to flow into the wiki, even when no wiki-side trigger fires the
// SyncDebouncer.
//
// The job's Execute returns nil even on partial enqueue failures;
// individual sync errors land in the sync queue's per-job log line.
type KeepCronTickJob struct {
	connector *Connector
	enqueuer  JobEnqueuer
	logger    SubscriberLogger
	lister    BindingsLister
}

// NewKeepCronTickJob constructs the periodic tick job. The lister is
// called fresh on every Execute so changes to on-disk binding state
// (including pre-existing bindings discovered after startup) are
// picked up without needing a process restart.
func NewKeepCronTickJob(connector *Connector, enqueuer JobEnqueuer, logger SubscriberLogger, lister BindingsLister) *KeepCronTickJob {
	return &KeepCronTickJob{
		connector: connector,
		enqueuer:  enqueuer,
		logger:    logger,
		lister:    lister,
	}
}

// GetName satisfies the jobs.Job interface — also doubles as the cron
// schedule's job name in scheduler logs.
func (*KeepCronTickJob) GetName() string { return "KeepCronTick" }

// Execute fires off one sync job per active binding. Errors from
// EnqueueJob (queue full, worker stopped) are logged and counted as
// the job's overall failure but don't interrupt enqueuing the rest.
//
// If a BindingsLister is wired, its result is the source of truth
// (re-read each tick). Otherwise we fall back to the in-memory active
// set populated by Bind/Unbind. The lister-driven path ensures cold
// pre-existing bindings get picked up without a wiki-side trigger.
func (j *KeepCronTickJob) Execute() error {
	if j.connector == nil || j.enqueuer == nil {
		return nil
	}
	var keys []BindingKey
	if j.lister != nil {
		keys = j.lister()
	} else {
		keys = j.connector.ActiveBindingsSnapshot()
	}
	// TEMP: log every tick (even zero-binding ticks) while we're
	// debugging the post-restart "lister returns 0 keys" symptom.
	// Strip back to the >0 guard once the index-readiness issue is
	// understood.
	j.logger.Info("KeepCronTick: tick fired, %d active binding(s)", len(keys))
	if len(keys) == 0 {
		return nil
	}
	for _, k := range keys {
		job := NewKeepOutboundSyncJob(j.connector, k.ProfileID, k.Page, k.ListName)
		if err := j.enqueuer.EnqueueJob(job); err != nil {
			j.logger.Error("KeepCronTick: enqueue for %s/%s/%s failed: %v",
				string(k.ProfileID), k.Page, k.ListName, err)
		}
	}
	return nil
}
