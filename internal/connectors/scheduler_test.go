//revive:disable:dot-imports
package connectors_test

import (
	"context"
	"errors"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
)

// fakeEnqueuer records every job dispatched to it.
type fakeEnqueuer struct {
	mu        sync.Mutex
	enqueued  []jobs.Job
	failOnAll bool
}

func (f *fakeEnqueuer) EnqueueJob(j jobs.Job) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failOnAll {
		return errors.New("queue full")
	}
	f.enqueued = append(f.enqueued, j)
	return nil
}

// recordingLogger captures Info/Error calls for assertions.
type recordingLogger struct {
	mu     sync.Mutex
	infos  []string
	errors []string
}

func (l *recordingLogger) Info(format string, _ ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.infos = append(l.infos, format)
}

func (l *recordingLogger) Error(format string, _ ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errors = append(l.errors, format)
}

// fakeConnector is a minimal Connector implementation for scheduler tests.
type fakeConnector struct {
	kind connectors.ConnectorKind
}

func (f *fakeConnector) Kind() connectors.ConnectorKind { return f.kind }

func (*fakeConnector) Sync(_ context.Context, _ connectors.SubscriptionKey) error { return nil }

func (*fakeConnector) PausedReason(_ connectors.SubscriptionKey) (string, bool) { return "", false }

func (*fakeConnector) ForceFullResync(_ context.Context, _ connectors.SubscriptionKey) error {
	return nil
}

// stubJob is a placeholder Job used by scheduler tests.
type stubJob struct {
	name string
	key  connectors.SubscriptionKey
}

func (j *stubJob) GetName() string { return j.name }
func (*stubJob) Execute() error    { return nil }

var _ = Describe("SyncScheduler", func() {
	When("constructed with a nil enqueuer", func() {
		var err error

		BeforeEach(func() {
			_, err = connectors.NewSyncScheduler(nil, &recordingLogger{})
		})

		It("should error", func() {
			Expect(err).To(HaveOccurred())
		})
	})

	When("constructed with a nil logger", func() {
		var err error

		BeforeEach(func() {
			_, err = connectors.NewSyncScheduler(&fakeEnqueuer{}, nil)
		})

		It("should error", func() {
			Expect(err).To(HaveOccurred())
		})
	})

	When("Register is called with a nil connector", func() {
		var err error

		BeforeEach(func() {
			s, _ := connectors.NewSyncScheduler(&fakeEnqueuer{}, &recordingLogger{})
			err = s.Register(nil, func() []connectors.SubscriptionKey { return nil },
				func(_ connectors.Connector, _ connectors.SubscriptionKey) jobs.Job { return &stubJob{} })
		})

		It("should error", func() {
			Expect(err).To(HaveOccurred())
		})
	})

	When("Register is called with a nil lister", func() {
		var err error

		BeforeEach(func() {
			s, _ := connectors.NewSyncScheduler(&fakeEnqueuer{}, &recordingLogger{})
			err = s.Register(&fakeConnector{kind: connectors.ConnectorKindGoogleKeep}, nil,
				func(_ connectors.Connector, _ connectors.SubscriptionKey) jobs.Job { return &stubJob{} })
		})

		It("should error", func() {
			Expect(err).To(HaveOccurred())
		})
	})

	When("Register is called with a nil jobMaker", func() {
		var err error

		BeforeEach(func() {
			s, _ := connectors.NewSyncScheduler(&fakeEnqueuer{}, &recordingLogger{})
			err = s.Register(
				&fakeConnector{kind: connectors.ConnectorKindGoogleKeep},
				func() []connectors.SubscriptionKey { return nil },
				nil,
			)
		})

		It("should error", func() {
			Expect(err).To(HaveOccurred())
		})
	})

	When("Execute is called with no registered connectors", func() {
		var enq *fakeEnqueuer
		var execErr error

		BeforeEach(func() {
			enq = &fakeEnqueuer{}
			s, _ := connectors.NewSyncScheduler(enq, &recordingLogger{})
			execErr = s.Execute()
		})

		It("should not error", func() {
			Expect(execErr).NotTo(HaveOccurred())
		})

		It("should not enqueue anything", func() {
			Expect(enq.enqueued).To(BeEmpty())
		})
	})

	When("a connector reports a single subscription", func() {
		var enq *fakeEnqueuer
		var execErr error

		BeforeEach(func() {
			enq = &fakeEnqueuer{}
			s, _ := connectors.NewSyncScheduler(enq, &recordingLogger{})
			c := &fakeConnector{kind: connectors.ConnectorKindGoogleKeep}
			Expect(s.Register(
				c,
				func() []connectors.SubscriptionKey {
					return []connectors.SubscriptionKey{{ProfileID: "p", Page: "pg", ListName: "ln"}}
				},
				func(_ connectors.Connector, k connectors.SubscriptionKey) jobs.Job {
					return &stubJob{name: "fakeJob", key: k}
				},
			)).To(Succeed())
			execErr = s.Execute()
		})

		It("should not error", func() {
			Expect(execErr).NotTo(HaveOccurred())
		})

		It("should enqueue exactly one job", func() {
			Expect(enq.enqueued).To(HaveLen(1))
		})

		It("should pass the subscription key to the job maker", func() {
			j, ok := enq.enqueued[0].(*stubJob)
			Expect(ok).To(BeTrue())
			Expect(j.key).To(Equal(connectors.SubscriptionKey{ProfileID: "p", Page: "pg", ListName: "ln"}))
		})
	})

	When("multiple connectors are registered, each with subscriptions", func() {
		var enq *fakeEnqueuer

		BeforeEach(func() {
			enq = &fakeEnqueuer{}
			s, _ := connectors.NewSyncScheduler(enq, &recordingLogger{})
			Expect(s.Register(
				&fakeConnector{kind: connectors.ConnectorKindGoogleKeep},
				func() []connectors.SubscriptionKey {
					return []connectors.SubscriptionKey{{ProfileID: "p1", Page: "a", ListName: "l1"}}
				},
				func(_ connectors.Connector, k connectors.SubscriptionKey) jobs.Job {
					return &stubJob{name: "keep", key: k}
				},
			)).To(Succeed())
			Expect(s.Register(
				&fakeConnector{kind: connectors.ConnectorKindGoogleTasks},
				func() []connectors.SubscriptionKey {
					return []connectors.SubscriptionKey{{ProfileID: "p2", Page: "b", ListName: "l2"}}
				},
				func(_ connectors.Connector, k connectors.SubscriptionKey) jobs.Job {
					return &stubJob{name: "tasks", key: k}
				},
			)).To(Succeed())
			Expect(s.Execute()).To(Succeed())
		})

		It("should enqueue one job per connector", func() {
			Expect(enq.enqueued).To(HaveLen(2))
		})
	})

	When("the enqueuer fails on every call", func() {
		var enq *fakeEnqueuer
		var rl *recordingLogger
		var execErr error

		BeforeEach(func() {
			enq = &fakeEnqueuer{failOnAll: true}
			rl = &recordingLogger{}
			s, _ := connectors.NewSyncScheduler(enq, rl)
			Expect(s.Register(
				&fakeConnector{kind: connectors.ConnectorKindGoogleKeep},
				func() []connectors.SubscriptionKey {
					return []connectors.SubscriptionKey{{ProfileID: "p", Page: "pg", ListName: "ln"}}
				},
				func(_ connectors.Connector, k connectors.SubscriptionKey) jobs.Job {
					return &stubJob{name: "j", key: k}
				},
			)).To(Succeed())
			execErr = s.Execute()
		})

		It("should not return an error itself (errors logged per-job)", func() {
			Expect(execErr).NotTo(HaveOccurred())
		})

		It("should log an error per failed enqueue", func() {
			rl.mu.Lock()
			defer rl.mu.Unlock()
			Expect(rl.errors).NotTo(BeEmpty())
		})
	})

	When("Tick is invoked", func() {
		var enq *fakeEnqueuer

		BeforeEach(func() {
			enq = &fakeEnqueuer{}
			s, _ := connectors.NewSyncScheduler(enq, &recordingLogger{})
			Expect(s.Register(
				&fakeConnector{kind: connectors.ConnectorKindGoogleKeep},
				func() []connectors.SubscriptionKey {
					return []connectors.SubscriptionKey{{ProfileID: "p", Page: "pg", ListName: "ln"}}
				},
				func(_ connectors.Connector, k connectors.SubscriptionKey) jobs.Job {
					return &stubJob{key: k}
				},
			)).To(Succeed())
			Expect(s.Tick(context.Background())).To(Succeed())
		})

		It("should dispatch via the same path as Execute", func() {
			Expect(enq.enqueued).To(HaveLen(1))
		})
	})

	Describe("GetName", func() {
		It("should return a stable name suitable for cron logs", func() {
			s, _ := connectors.NewSyncScheduler(&fakeEnqueuer{}, &recordingLogger{})
			Expect(s.GetName()).To(Equal("ConnectorSyncScheduler"))
		})
	})
})
