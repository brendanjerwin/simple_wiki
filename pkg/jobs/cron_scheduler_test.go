package jobs_test

import (
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/jcelliott/lumber"
)

var _ = Describe("CronScheduler", func() {
	var (
		scheduler *jobs.CronScheduler
		logger    lumber.Logger
	)

	BeforeEach(func() {
		logger = lumber.NewConsoleLogger(lumber.ERROR)
		scheduler = jobs.NewCronScheduler(logger)
	})

	AfterEach(func() {
		scheduler.Stop()
	})

	Describe("NewCronScheduler", func() {
		It("should create a scheduler with the given logger", func() {
			Expect(scheduler).NotTo(BeNil())
		})
	})

	Describe("Schedule", func() {
		When("given a valid cron schedule and job", func() {
			var (
				executionCount atomic.Int32
				entryID        int
				scheduleErr    error
			)

			BeforeEach(func() {
				executionCount.Store(0)
				customJob := &countingJob{
					name:    "test-job",
					counter: &executionCount,
				}
				// Schedule to run every second
				entryID, scheduleErr = scheduler.Schedule("* * * * * *", customJob)
			})

			It("should not return an error", func() {
				Expect(scheduleErr).NotTo(HaveOccurred())
			})

			It("should return a non-zero entry ID", func() {
				Expect(entryID).To(BeNumerically(">", 0))
			})
		})

		When("given an invalid cron schedule", func() {
			var (
				entryID     int
				scheduleErr error
			)

			BeforeEach(func() {
				job := &jobs.MockJob{Name: "test-job"}
				entryID, scheduleErr = scheduler.Schedule("invalid cron", job)
			})

			It("should return an error", func() {
				Expect(scheduleErr).To(HaveOccurred())
			})

			It("should return zero entry ID", func() {
				Expect(entryID).To(Equal(0))
			})
		})
	})

	Describe("Start and Stop", func() {
		When("scheduler is started", func() {
			var executionCount atomic.Int32

			BeforeEach(func() {
				executionCount.Store(0)
				job := &countingJob{
					name:    "test-job",
					counter: &executionCount,
				}
				// Use @every which is simpler and more reliable
				_, err := scheduler.Schedule("@every 1s", job)
				Expect(err).NotTo(HaveOccurred())

				scheduler.Start()
			})

			It("should execute scheduled jobs", func() {
				Eventually(func() int32 {
					return executionCount.Load()
				}, 3*time.Second, 100*time.Millisecond).Should(BeNumerically(">=", 1))
			})
		})

		When("scheduler is stopped after running", func() {
			var (
				executionCount atomic.Int32
				countAtStop    int32
			)

			BeforeEach(func() {
				executionCount.Store(0)
				job := &countingJob{
					name:    "test-job",
					counter: &executionCount,
				}
				_, err := scheduler.Schedule("@every 1s", job)
				Expect(err).NotTo(HaveOccurred())

				scheduler.Start()

				// Wait for at least one execution
				Eventually(func() int32 {
					return executionCount.Load()
				}, 3*time.Second, 100*time.Millisecond).Should(BeNumerically(">=", 1))

				// Stop and record count
				scheduler.Stop()
				countAtStop = executionCount.Load()
			})

			It("should not execute more jobs after stop", func() {
				// Wait and verify no more executions
				Consistently(func() int32 {
					return executionCount.Load()
				}, 2*time.Second, 100*time.Millisecond).Should(BeNumerically("<=", countAtStop))
			})
		})
	})

	Describe("Remove", func() {
		When("removing a scheduled job", func() {
			var (
				executionCount  atomic.Int32
				countAtRemoval  int32
				entryID         int
			)

			BeforeEach(func() {
				executionCount.Store(0)
				job := &countingJob{
					name:    "test-job",
					counter: &executionCount,
				}
				var err error
				entryID, err = scheduler.Schedule("@every 1s", job)
				Expect(err).NotTo(HaveOccurred())

				scheduler.Start()

				// Wait for at least one execution
				Eventually(func() int32 {
					return executionCount.Load()
				}, 3*time.Second, 100*time.Millisecond).Should(BeNumerically(">=", 1))

				// Remove the job and record count
				scheduler.Remove(entryID)
				countAtRemoval = executionCount.Load()
			})

			It("should stop executing the removed job", func() {
				// Verify no more executions after removal
				Consistently(func() int32 {
					return executionCount.Load()
				}, 2*time.Second, 100*time.Millisecond).Should(BeNumerically("<=", countAtRemoval))
			})
		})
	})

	Describe("Entries", func() {
		When("jobs are scheduled", func() {
			var entryCount int

			BeforeEach(func() {
				job1 := &jobs.MockJob{Name: "job1"}
				job2 := &jobs.MockJob{Name: "job2"}

				_, err := scheduler.Schedule("@every 1h", job1)
				Expect(err).NotTo(HaveOccurred())
				_, err = scheduler.Schedule("@every 2h", job2)
				Expect(err).NotTo(HaveOccurred())

				entryCount = scheduler.Entries()
			})

			It("should return the count of scheduled entries", func() {
				Expect(entryCount).To(Equal(2))
			})
		})

		When("no jobs are scheduled", func() {
			var entryCount int

			BeforeEach(func() {
				entryCount = scheduler.Entries()
			})

			It("should return zero", func() {
				Expect(entryCount).To(Equal(0))
			})
		})
	})
})

// countingJob is a test job that counts executions
type countingJob struct {
	name    string
	counter *atomic.Int32
}

func (j *countingJob) Execute() error {
	j.counter.Add(1)
	return nil
}

func (j *countingJob) GetName() string {
	return j.name
}
