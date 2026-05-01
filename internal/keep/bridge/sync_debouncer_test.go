//revive:disable:dot-imports
//revive:disable:add-constant
package bridge_test

import (
	"fmt"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/keep/bridge"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

// debouncerFakeEnqueuer records EnqueueJob calls. Shared by
// sync_debouncer_test.go and cron_test.go (same bridge_test package).
type debouncerFakeEnqueuer struct {
	mu   sync.Mutex
	jobs []jobs.Job
	err  error
}

func (e *debouncerFakeEnqueuer) EnqueueJob(j jobs.Job) error {
	if e.err != nil {
		return e.err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.jobs = append(e.jobs, j)
	return nil
}

func (e *debouncerFakeEnqueuer) jobCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.jobs)
}

// debouncerFakeLogger records Info/Error calls. Also shared with cron_test.go.
type debouncerFakeLogger struct {
	mu        sync.Mutex
	infoMsgs  []string
	errorMsgs []string
}

func (l *debouncerFakeLogger) Info(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.infoMsgs = append(l.infoMsgs, fmt.Sprintf(format, args...))
}

func (l *debouncerFakeLogger) Error(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errorMsgs = append(l.errorMsgs, fmt.Sprintf(format, args...))
}

func (l *debouncerFakeLogger) infoCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.infoMsgs)
}

const debouncerTestLogin = "bob@example.com"

var _ = Describe("SyncDebouncer", func() {
	var (
		enqueuer  *debouncerFakeEnqueuer
		logger    *debouncerFakeLogger
		debouncer *bridge.SyncDebouncer
		profileID wikipage.PageIdentifier
	)

	const (
		debouncePage     = "TaskBoard"
		debounceListName = "sprint"
		// shortWindow is small enough to fire quickly in tests but large
		// enough that a fresh timer never fires during test setup (before
		// we've had a chance to call OnChecklistMutated).
		shortWindowMs = 10 * time.Millisecond
		// waitBeyond is how long we wait to confirm a job did NOT fire.
		waitBeyond = 3 * shortWindowMs
		// pollInterval is how often Eventually polls.
		pollInterval = 2 * time.Millisecond
		// pollTimeout is the max wait for an expected job to appear.
		pollTimeout = 20 * shortWindowMs
	)

	BeforeEach(func() {
		var err error
		profileID, err = wikipage.ProfileIdentifierFor(debouncerTestLogin)
		Expect(err).ToNot(HaveOccurred())

		enqueuer = &debouncerFakeEnqueuer{}
		logger = &debouncerFakeLogger{}
		// Connector is only used to construct the sync job, so nil is fine
		// for tests that only verify enqueue behaviour.
		debouncer = bridge.NewSyncDebouncer(enqueuer, nil, nil, logger, shortWindowMs)
	})

	// ------------------------------------------------------------------ Suppress / Unsuppress

	Describe("Suppress and Unsuppress", func() {

		Describe("when Suppress is called and OnChecklistMutated fires", func() {
			BeforeEach(func() {
				debouncer.Suppress(profileID, debouncePage, debounceListName)
			})

			It("should not enqueue a job", func() {
				identity := tailscale.NewIdentity(debouncerTestLogin, "Bob", "node-1")
				debouncer.OnChecklistMutated(debouncePage, debounceListName, identity)
				time.Sleep(waitBeyond)
				Expect(enqueuer.jobCount()).To(Equal(0))
			})
		})

		Describe("when Suppress then Unsuppress are called and OnChecklistMutated fires", func() {
			BeforeEach(func() {
				debouncer.Suppress(profileID, debouncePage, debounceListName)
				debouncer.Unsuppress(profileID, debouncePage, debounceListName)
			})

			It("should enqueue a job after the debounce window", func() {
				identity := tailscale.NewIdentity(debouncerTestLogin, "Bob", "node-1")
				debouncer.OnChecklistMutated(debouncePage, debounceListName, identity)
				Eventually(enqueuer.jobCount, pollTimeout, pollInterval).Should(Equal(1))
			})
		})

		Describe("when Suppress is called twice then Unsuppress once", func() {
			BeforeEach(func() {
				debouncer.Suppress(profileID, debouncePage, debounceListName)
				debouncer.Suppress(profileID, debouncePage, debounceListName)
				debouncer.Unsuppress(profileID, debouncePage, debounceListName)
			})

			It("should still suppress OnChecklistMutated (refcount > 0)", func() {
				identity := tailscale.NewIdentity(debouncerTestLogin, "Bob", "node-1")
				debouncer.OnChecklistMutated(debouncePage, debounceListName, identity)
				time.Sleep(waitBeyond)
				Expect(enqueuer.jobCount()).To(Equal(0))
			})
		})
	})

	// ------------------------------------------------------------------ OnChecklistMutated

	Describe("OnChecklistMutated", func() {

		Describe("when called with a nil identity", func() {
			BeforeEach(func() {
				debouncer.OnChecklistMutated(debouncePage, debounceListName, nil)
			})

			It("should not enqueue a job", func() {
				time.Sleep(waitBeyond)
				Expect(enqueuer.jobCount()).To(Equal(0))
			})
		})

		Describe("when called with an anonymous identity (empty LoginName)", func() {
			BeforeEach(func() {
				debouncer.OnChecklistMutated(debouncePage, debounceListName, tailscale.Anonymous)
			})

			It("should not enqueue a job", func() {
				time.Sleep(waitBeyond)
				Expect(enqueuer.jobCount()).To(Equal(0))
			})
		})

		Describe("when called with the synthetic keep-sync identity", func() {
			BeforeEach(func() {
				synthIdentity := tailscale.NewIdentity("system:keep-sync", "sync", "node-sync")
				debouncer.OnChecklistMutated(debouncePage, debounceListName, synthIdentity)
			})

			It("should not enqueue a job (prevents inbound-apply loop)", func() {
				time.Sleep(waitBeyond)
				Expect(enqueuer.jobCount()).To(Equal(0))
			})
		})

		Describe("when called with a real user identity", func() {
			BeforeEach(func() {
				identity := tailscale.NewIdentity(debouncerTestLogin, "Bob", "node-1")
				debouncer.OnChecklistMutated(debouncePage, debounceListName, identity)
			})

			It("should enqueue a job after the debounce window", func() {
				Eventually(enqueuer.jobCount, pollTimeout, pollInterval).Should(Equal(1))
			})

			It("should enqueue a KeepOutboundSync job", func() {
				Eventually(func() string {
					if enqueuer.jobCount() == 0 {
						return ""
					}
					enqueuer.mu.Lock()
					defer enqueuer.mu.Unlock()
					return enqueuer.jobs[0].GetName()
				}, pollTimeout, pollInterval).Should(Equal(bridge.KeepOutboundSyncJobName))
			})
		})

		Describe("when called three times rapidly for the same key (debounce coalescing)", func() {
			BeforeEach(func() {
				identity := tailscale.NewIdentity(debouncerTestLogin, "Bob", "node-1")
				debouncer.OnChecklistMutated(debouncePage, debounceListName, identity)
				debouncer.OnChecklistMutated(debouncePage, debounceListName, identity)
				debouncer.OnChecklistMutated(debouncePage, debounceListName, identity)
			})

			It("should enqueue exactly one job", func() {
				Eventually(enqueuer.jobCount, pollTimeout, pollInterval).Should(Equal(1))
				// Confirm no second job arrives.
				time.Sleep(waitBeyond)
				Expect(enqueuer.jobCount()).To(Equal(1))
			})
		})

		Describe("when called for two different list keys", func() {
			BeforeEach(func() {
				identity := tailscale.NewIdentity(debouncerTestLogin, "Bob", "node-1")
				debouncer.OnChecklistMutated(debouncePage, "list-A", identity)
				debouncer.OnChecklistMutated(debouncePage, "list-B", identity)
			})

			It("should enqueue two independent jobs", func() {
				Eventually(enqueuer.jobCount, pollTimeout, pollInterval).Should(Equal(2))
			})
		})
	})
})
