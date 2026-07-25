//revive:disable:dot-imports
package connectors_test

import (
	"context"
	"errors"
	"strings"
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

func (*fakeConnector) Sync(_ context.Context, _ connectors.BindingKey) error { return nil }

func (*fakeConnector) PausedReason(_ connectors.BindingKey) (string, bool) { return "", false }

func (*fakeConnector) ForceFullResync(_ context.Context, _ connectors.BindingKey) error {
	return nil
}

// stubJob is a placeholder Job used by scheduler tests.
type stubJob struct {
	name string
	key  connectors.BindingKey
}

func (j *stubJob) GetName() string { return j.name }
func (*stubJob) Execute() error    { return nil }

// errorJob always returns an error on Execute.
type errorJob struct {
	name string
}

func (j *errorJob) GetName() string { return j.name }
func (*errorJob) Execute() error    { return errors.New("sync failed") }

// runAll executes every enqueued job in place and discards errors.
func (f *fakeEnqueuer) RunAll() {
	f.mu.Lock()
	jobs := append([]jobs.Job(nil), f.enqueued...)
	f.mu.Unlock()
	for _, j := range jobs {
		_ = j.Execute()
	}
}

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
			err = s.Register(nil, func() []connectors.BindingKey { return nil },
				func(_ connectors.Connector, _ connectors.BindingKey) jobs.Job { return &stubJob{} })
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
				func(_ connectors.Connector, _ connectors.BindingKey) jobs.Job { return &stubJob{} })
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
				func() []connectors.BindingKey { return nil },
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
				func() []connectors.BindingKey {
					return []connectors.BindingKey{{ProfileID: "p", Page: "pg", ListName: "ln"}}
				},
				func(_ connectors.Connector, k connectors.BindingKey) jobs.Job {
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
			wrapped, ok := enq.enqueued[0].(*connectors.ReportingJob)
			Expect(ok).To(BeTrue())
			j, ok := wrapped.Unwrap().(*stubJob)
			Expect(ok).To(BeTrue())
			Expect(j.key).To(Equal(connectors.BindingKey{ProfileID: "p", Page: "pg", ListName: "ln"}))
		})
	})

	When("multiple connectors are registered, each with subscriptions", func() {
		var enq *fakeEnqueuer

		BeforeEach(func() {
			enq = &fakeEnqueuer{}
			s, _ := connectors.NewSyncScheduler(enq, &recordingLogger{})
			Expect(s.Register(
				&fakeConnector{kind: connectors.ConnectorKindGoogleKeep},
				func() []connectors.BindingKey {
					return []connectors.BindingKey{{ProfileID: "p1", Page: "a", ListName: "l1"}}
				},
				func(_ connectors.Connector, k connectors.BindingKey) jobs.Job {
					return &stubJob{name: "keep", key: k}
				},
			)).To(Succeed())
			Expect(s.Register(
				&fakeConnector{kind: connectors.ConnectorKindGoogleTasks},
				func() []connectors.BindingKey {
					return []connectors.BindingKey{{ProfileID: "p2", Page: "b", ListName: "l2"}}
				},
				func(_ connectors.Connector, k connectors.BindingKey) jobs.Job {
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
				func() []connectors.BindingKey {
					return []connectors.BindingKey{{ProfileID: "p", Page: "pg", ListName: "ln"}}
				},
				func(_ connectors.Connector, k connectors.BindingKey) jobs.Job {
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
				func() []connectors.BindingKey {
					return []connectors.BindingKey{{ProfileID: "p", Page: "pg", ListName: "ln"}}
				},
				func(_ connectors.Connector, k connectors.BindingKey) jobs.Job {
					return &stubJob{key: k}
				},
			)).To(Succeed())
			Expect(s.Tick(context.Background())).To(Succeed())
		})

		It("should dispatch via the same path as Execute", func() {
			Expect(enq.enqueued).To(HaveLen(1))
		})
	})

	When("a connector kind exceeds the failure threshold", func() {
		var enq *fakeEnqueuer
		var rl *recordingLogger
		var s *connectors.SyncScheduler

		BeforeEach(func() {
			enq = &fakeEnqueuer{}
			rl = &recordingLogger{}
			s, _ = connectors.NewSyncScheduler(enq, rl)
			Expect(s.Register(
				&fakeConnector{kind: connectors.ConnectorKindGoogleKeep},
				func() []connectors.BindingKey {
					return []connectors.BindingKey{{ProfileID: "p", Page: "pg", ListName: "ln"}}
				},
				func(_ connectors.Connector, _ connectors.BindingKey) jobs.Job {
					return &errorJob{name: "failing-keep"}
				},
			)).To(Succeed())

			// Five ticks with one binding each = five failures, opening the breaker.
			for range 5 {
				Expect(s.Execute()).To(Succeed())
			}
			// Run every enqueued job so its reporting callback updates the breaker.
			enq.RunAll()
		})

		It("stops enqueuing jobs once the breaker opens", func() {
			enq.mu.Lock()
			count := len(enq.enqueued)
			enq.mu.Unlock()
			Expect(count).To(Equal(5))

			// A sixth tick should not enqueue anything because the breaker is open.
			Expect(s.Execute()).To(Succeed())

			rl.mu.Lock()
			hasSkipLog := false
			for _, info := range rl.infos {
				if strings.Contains(info, "circuit breaker open") {
					hasSkipLog = true
					break
				}
			}
			rl.mu.Unlock()
			Expect(hasSkipLog).To(BeTrue())
		})
	})

	Describe("GetName", func() {
		It("should return a stable name suitable for cron logs", func() {
			s, _ := connectors.NewSyncScheduler(&fakeEnqueuer{}, &recordingLogger{})
			Expect(s.GetName()).To(Equal("ConnectorSyncScheduler"))
		})
	})
})
