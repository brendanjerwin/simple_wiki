package jobs_test

import (
	"testing"

	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
)

func TestJobsPackage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Jobs Package Suite")
}

var _ = Describe("JobQueueCoordinator", func() {
	var coordinator *jobs.JobQueueCoordinator

	BeforeEach(func() {
		logger := lumber.NewConsoleLogger(lumber.WARN) // Quiet logger for tests
		coordinator = jobs.NewJobQueueCoordinator(logger)
	})

	It("should exist", func() {
		Expect(coordinator).NotTo(BeNil())
	})


	Describe("when enqueueing a job", func() {
		var blockingJob *jobs.BlockingMockJob

		BeforeEach(func() {
			blockingJob = jobs.NewBlockingMockJob("TestQueue")
			coordinator.EnqueueJob(blockingJob)
		})

		AfterEach(func() {
			// Always release the job to prevent goroutine leaks
			blockingJob.Release()
		})

		It("should increase jobs remaining", func() {
			stats := coordinator.GetQueueStats("TestQueue")
			Expect(stats.JobsRemaining).To(Equal(int32(1)))
		})

		It("should update high water mark", func() {
			stats := coordinator.GetQueueStats("TestQueue")
			Expect(stats.HighWaterMark).To(Equal(int32(1)))
		})

		It("should mark queue as active", func() {
			stats := coordinator.GetQueueStats("TestQueue")
			Expect(stats.IsActive).To(BeTrue())
		})

		Describe("when the job completes", func() {
			BeforeEach(func() {
				// Release the job to let it complete
				blockingJob.Release()

				// Wait for job to complete
				Eventually(func() int32 {
					return coordinator.GetQueueStats("TestQueue").JobsRemaining
				}).Should(Equal(int32(0)))
			})

			It("should reset jobs remaining to zero", func() {
				stats := coordinator.GetQueueStats("TestQueue")
				Expect(stats.JobsRemaining).To(Equal(int32(0)))
			})

			It("should reset high water mark to zero when queue is empty", func() {
				stats := coordinator.GetQueueStats("TestQueue")
				Expect(stats.HighWaterMark).To(Equal(int32(0)))
			})

			It("should mark queue as inactive", func() {
				stats := coordinator.GetQueueStats("TestQueue")
				Expect(stats.IsActive).To(BeFalse())
			})
		})
	})

	Describe("when getting all active queues", func() {
		It("should return correct number of active queues", func() {
			// Enqueue jobs - queues are auto-registered
			coordinator.EnqueueJob(&jobs.MockJob{Name: "Queue1"})
			coordinator.EnqueueJob(&jobs.MockJob{Name: "Queue3"})

			// Check immediately after enqueueing, before jobs complete
			activeQueues := coordinator.GetActiveQueues()
			Expect(len(activeQueues)).To(Equal(2))
		})

		It("should return only active queues", func() {
			// Enqueue jobs - queues are auto-registered
			coordinator.EnqueueJob(&jobs.MockJob{Name: "Queue1"})
			coordinator.EnqueueJob(&jobs.MockJob{Name: "Queue3"})

			// Check immediately after enqueueing, before jobs complete
			activeQueues := coordinator.GetActiveQueues()
			queueNames := make([]string, len(activeQueues))
			for i, stats := range activeQueues {
				queueNames[i] = stats.QueueName
			}
			Expect(queueNames).To(ContainElements("Queue1", "Queue3"))
		})
	})

	Describe("EnqueueJobWithCompletion", func() {
		Describe("when job completes successfully", func() {
			var callbackCalled bool
			var callbackError error

			BeforeEach(func() {
				callbackCalled = false
				callbackError = nil

				job := &jobs.MockJob{Name: "CompletionTestQueue", Err: nil}
				coordinator.EnqueueJobWithCompletion(job, func(err error) {
					callbackCalled = true
					callbackError = err
				})

				// Wait for job and callback to complete
				Eventually(func() bool {
					return callbackCalled
				}).Should(BeTrue())
			})

			It("should call the completion callback", func() {
				Expect(callbackCalled).To(BeTrue())
			})

			It("should pass nil error to callback", func() {
				Expect(callbackError).To(BeNil())
			})
		})

		Describe("when job fails with error", func() {
			var callbackCalled bool
			var callbackError error
			var expectedErr error

			BeforeEach(func() {
				callbackCalled = false
				callbackError = nil
				expectedErr = &testError{msg: "job failed"}

				job := &jobs.MockJob{Name: "ErrorTestQueue", Err: expectedErr}
				coordinator.EnqueueJobWithCompletion(job, func(err error) {
					callbackCalled = true
					callbackError = err
				})

				// Wait for job and callback to complete
				Eventually(func() bool {
					return callbackCalled
				}).Should(BeTrue())
			})

			It("should call the completion callback", func() {
				Expect(callbackCalled).To(BeTrue())
			})

			It("should pass the error to callback", func() {
				Expect(callbackError).To(Equal(expectedErr))
			})
		})

		Describe("when callback is nil", func() {
			It("should not panic", func() {
				job := &jobs.MockJob{Name: "NilCallbackQueue", Err: nil}
				Expect(func() {
					coordinator.EnqueueJobWithCompletion(job, nil)
				}).NotTo(Panic())

				// Wait for job to complete
				Eventually(func() bool {
					stats := coordinator.GetQueueStats("NilCallbackQueue")
					return stats != nil && !stats.IsActive
				}).Should(BeTrue())
			})
		})
	})

	Describe("GetJobProgress", func() {
		var blockingJob1, blockingJob2 *jobs.BlockingMockJob

		BeforeEach(func() {
			// Enqueue multiple jobs in different queues
			blockingJob1 = jobs.NewBlockingMockJob("Queue1")
			blockingJob2 = jobs.NewBlockingMockJob("Queue2")
			
			coordinator.EnqueueJob(blockingJob1)
			coordinator.EnqueueJob(blockingJob2)
		})

		AfterEach(func() {
			// Always release jobs to prevent goroutine leaks
			blockingJob1.Release()
			blockingJob2.Release()
		})

		It("should return progress for all queues", func() {
			progress := coordinator.GetJobProgress()
			Expect(progress.QueueStats).To(HaveLen(2))
		})

		It("should report active queues count", func() {
			progress := coordinator.GetJobProgress()
			Expect(progress.TotalActive).To(BeNumerically(">", 0))
		})

		It("should report total queues count", func() {
			progress := coordinator.GetJobProgress()
			Expect(progress.TotalQueues).To(Equal(int32(2)))
		})

		It("should include queue statistics", func() {
			progress := coordinator.GetJobProgress()
			queueNames := []string{}
			for _, q := range progress.QueueStats {
				queueNames = append(queueNames, q.QueueName)
			}
			Expect(queueNames).To(ContainElements("Queue1", "Queue2"))
		})
	})
})

// testError is a simple error type for testing.
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}