//revive:disable:dot-imports
package sync_test

import (
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	taskssync "github.com/brendanjerwin/simple_wiki/internal/connectors/google_tasks/sync"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// recordingEnqueuer captures every Job EnqueueJob is called with.
type recordingEnqueuer struct {
	mu   sync.Mutex
	jobs []jobs.Job
}

func (r *recordingEnqueuer) EnqueueJob(job jobs.Job) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs = append(r.jobs, job)
	return nil
}

func (r *recordingEnqueuer) Snapshot() []jobs.Job {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]jobs.Job, len(r.jobs))
	copy(out, r.jobs)
	return out
}

// fakeIdentity satisfies tailscale.IdentityValue minimally for tests.
type fakeIdentity struct{ login string }

func (f fakeIdentity) IsAnonymous() bool { return f.login == "" }
func (f fakeIdentity) LoginName() string { return f.login }
func (f fakeIdentity) DisplayName() string { return f.login }
func (fakeIdentity) NodeName() string    { return "" }
func (f fakeIdentity) ForLog() string    { return f.login }
func (f fakeIdentity) String() string    { return f.login }
func (fakeIdentity) IsAgent() bool       { return false }
func (f fakeIdentity) Name() string      { return f.login }

// Compile-time assertion: fakeIdentity satisfies tailscale.IdentityValue.
var _ tailscale.IdentityValue = fakeIdentity{}

func newDebouncerForTest(window time.Duration) (*taskssync.SyncDebouncer, *recordingEnqueuer) {
	enq := &recordingEnqueuer{}
	pages := newFakePages()
	store := newConfiguredStore(pages, nil)
	c, err := taskssync.NewConnector(store, readyLeaseTable(), stubFactoryThatReturns(newFakeTasksClient()), silentLogger{}, newFakeClock(time.Now()))
	Expect(err).ToNot(HaveOccurred())
	d, err := taskssync.NewSyncDebouncer(enq, c, silentLogger{}, window)
	Expect(err).ToNot(HaveOccurred())
	return d, enq
}

var _ = Describe("NewSyncDebouncer input validation", func() {
	When("enqueuer is nil", func() {
		var newErr error

		BeforeEach(func() {
			pages := newFakePages()
			store := newConfiguredStore(pages, nil)
			c, _ := taskssync.NewConnector(store, readyLeaseTable(), stubFactoryThatReturns(newFakeTasksClient()), silentLogger{}, newFakeClock(time.Now()))
			_, newErr = taskssync.NewSyncDebouncer(nil, c, silentLogger{}, 100*time.Millisecond)
		})

		It("should return an error", func() {
			Expect(newErr).To(MatchError(ContainSubstring("enqueuer must not be nil")))
		})
	})

	When("debounceWindow is zero", func() {
		var newErr error

		BeforeEach(func() {
			pages := newFakePages()
			store := newConfiguredStore(pages, nil)
			c, _ := taskssync.NewConnector(store, readyLeaseTable(), stubFactoryThatReturns(newFakeTasksClient()), silentLogger{}, newFakeClock(time.Now()))
			_, newErr = taskssync.NewSyncDebouncer(&recordingEnqueuer{}, c, silentLogger{}, 0)
		})

		It("should return an error", func() {
			Expect(newErr).To(MatchError(ContainSubstring("debounceWindow must be > 0")))
		})
	})
})

var _ = Describe("SyncDebouncer.OnChecklistMutated", func() {
	When("identity is nil", func() {
		var enq *recordingEnqueuer

		BeforeEach(func() {
			d, e := newDebouncerForTest(50 * time.Millisecond)
			enq = e
			d.OnChecklistMutated("p", "l", nil)
			time.Sleep(80 * time.Millisecond)
		})

		It("should not enqueue any job", func() {
			Expect(enq.Snapshot()).To(BeEmpty())
		})
	})

	When("identity has no LoginName", func() {
		var enq *recordingEnqueuer

		BeforeEach(func() {
			d, e := newDebouncerForTest(50 * time.Millisecond)
			enq = e
			d.OnChecklistMutated("p", "l", fakeIdentity{login: ""})
			time.Sleep(80 * time.Millisecond)
		})

		It("should not enqueue any job", func() {
			Expect(enq.Snapshot()).To(BeEmpty())
		})
	})

	When("identity is the synthetic Tasks-sync identity", func() {
		var enq *recordingEnqueuer

		BeforeEach(func() {
			d, e := newDebouncerForTest(50 * time.Millisecond)
			enq = e
			d.OnChecklistMutated("p", "l", fakeIdentity{login: taskssync.SyncIdentityLoginName})
			time.Sleep(80 * time.Millisecond)
		})

		It("should not enqueue (drop self-loops)", func() {
			Expect(enq.Snapshot()).To(BeEmpty())
		})
	})

	When("identity is a real user", func() {
		var enq *recordingEnqueuer

		BeforeEach(func() {
			d, e := newDebouncerForTest(30 * time.Millisecond)
			enq = e
			d.OnChecklistMutated("p", "l", fakeIdentity{login: "alice@example.com"})
			Eventually(func() int { return len(enq.Snapshot()) }, 500*time.Millisecond, 10*time.Millisecond).Should(Equal(1))
		})

		It("should enqueue exactly one job after the debounce window", func() {
			Expect(enq.Snapshot()).To(HaveLen(1))
		})

		It("should enqueue a TasksOutboundSyncJob", func() {
			Expect(enq.Snapshot()[0].GetName()).To(Equal(taskssync.TasksOutboundSyncJobName))
		})
	})

	When("the same key is mutated twice within the debounce window", func() {
		var enq *recordingEnqueuer

		BeforeEach(func() {
			d, e := newDebouncerForTest(60 * time.Millisecond)
			enq = e
			d.OnChecklistMutated("p", "l", fakeIdentity{login: "alice@example.com"})
			time.Sleep(20 * time.Millisecond)
			d.OnChecklistMutated("p", "l", fakeIdentity{login: "alice@example.com"})
			Eventually(func() int { return len(enq.Snapshot()) }, 500*time.Millisecond, 10*time.Millisecond).Should(Equal(1))
		})

		It("should coalesce into one enqueue", func() {
			Expect(enq.Snapshot()).To(HaveLen(1))
		})
	})

	When("the key is suppressed", func() {
		var enq *recordingEnqueuer

		BeforeEach(func() {
			d, e := newDebouncerForTest(40 * time.Millisecond)
			enq = e
			profileID, err := wikipage.ProfileIdentifierFor("alice@example.com")
			Expect(err).ToNot(HaveOccurred())
			d.Suppress(profileID, "p", "l")
			defer d.Unsuppress(profileID, "p", "l")
			d.OnChecklistMutated("p", "l", fakeIdentity{login: "alice@example.com"})
			time.Sleep(80 * time.Millisecond)
		})

		It("should not enqueue (suppressed)", func() {
			Expect(enq.Snapshot()).To(BeEmpty())
		})
	})
})

var _ = Describe("TasksOutboundSyncJob.GetName", func() {
	It("should return the queue-name constant", func() {
		job := taskssync.NewTasksOutboundSyncJob(nil, "profile_alice", "p", "l")
		Expect(job.GetName()).To(Equal(taskssync.TasksOutboundSyncJobName))
	})
})

// ActiveSubscriptions test (the SyncScheduler-facing tracker).
var _ = Describe("ActiveSubscriptions", func() {
	When("a key is added", func() {
		var snap []connectors.SubscriptionKey

		BeforeEach(func() {
			a := taskssync.NewActiveSubscriptions()
			a.Add(connectors.SubscriptionKey{ProfileID: "p1", Page: "x", ListName: "y"})
			snap = a.Snapshot()
		})

		It("should appear in Snapshot", func() {
			Expect(snap).To(HaveLen(1))
		})
	})

	When("a key is added then removed", func() {
		var snap []connectors.SubscriptionKey

		BeforeEach(func() {
			a := taskssync.NewActiveSubscriptions()
			k := connectors.SubscriptionKey{ProfileID: "p1", Page: "x", ListName: "y"}
			a.Add(k)
			a.Remove(k)
			snap = a.Snapshot()
		})

		It("should be empty", func() {
			Expect(snap).To(BeEmpty())
		})
	})
})
