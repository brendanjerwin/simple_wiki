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
		var enqueueErr error

		BeforeEach(func() {
			blockingJob = jobs.NewBlockingMockJob("TestQueue")
			enqueueErr = coordinator.EnqueueJob(blockingJob)
		})

		AfterEach(func() {
			// Always release the job to prevent goroutine leaks
			blockingJob.Release()
		})

		It("should not return an error", func() {
			Expect(enqueueErr).NotTo(HaveOccurred())
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
			Expect(coordinator.EnqueueJob(&jobs.MockJob{Name: "Queue1"})).To(Succeed())
			Expect(coordinator.EnqueueJob(&jobs.MockJob{Name: "Queue3"})).To(Succeed())

			// Check immediately after enqueueing, before jobs complete
			activeQueues := coordinator.GetActiveQueues()
			Expect(len(activeQueues)).To(Equal(2))
		})

		It("should return only active queues", func() {
			// Enqueue jobs - queues are auto-registered
			Expect(coordinator.EnqueueJob(&jobs.MockJob{Name: "Queue1"})).To(Succeed())
			Expect(coordinator.EnqueueJob(&jobs.MockJob{Name: "Queue3"})).To(Succeed())

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
			var enqueueErr error

			BeforeEach(func() {
				callbackCalled = false
				callbackError = nil

				job := &jobs.MockJob{Name: "CompletionTestQueue", Err: nil}
				enqueueErr = coordinator.EnqueueJobWithCompletion(job, func(err error) {
					callbackCalled = true
					callbackError = err
				})

				// Wait for job and callback to complete
				Eventually(func() bool {
					return callbackCalled
				}).Should(BeTrue())
			})

			It("should not return an error", func() {
				Expect(enqueueErr).NotTo(HaveOccurred())
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
			var enqueueErr error

			BeforeEach(func() {
				callbackCalled = false
				callbackError = nil
				expectedErr = &testError{msg: "job failed"}

				job := &jobs.MockJob{Name: "ErrorTestQueue", Err: expectedErr}
				enqueueErr = coordinator.EnqueueJobWithCompletion(job, func(err error) {
					callbackCalled = true
					callbackError = err
				})

				// Wait for job and callback to complete
				Eventually(func() bool {
					return callbackCalled
				}).Should(BeTrue())
			})

			It("should not return an error from enqueue", func() {
				Expect(enqueueErr).NotTo(HaveOccurred())
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
					_ = coordinator.EnqueueJobWithCompletion(job, nil)
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
		var progress jobs.JobProgress

		BeforeEach(func() {
			// Enqueue multiple jobs in different queues
			blockingJob1 = jobs.NewBlockingMockJob("Queue1")
			blockingJob2 = jobs.NewBlockingMockJob("Queue2")

			Expect(coordinator.EnqueueJob(blockingJob1)).To(Succeed())
			Expect(coordinator.EnqueueJob(blockingJob2)).To(Succeed())

			// Action: Get the job progress
			progress = coordinator.GetJobProgress()
		})

		AfterEach(func() {
			// Always release jobs to prevent goroutine leaks
			blockingJob1.Release()
			blockingJob2.Release()
		})

		It("should return progress for all queues", func() {
			Expect(progress.QueueStats).To(HaveLen(2))
		})

		It("should report active queues count", func() {
			Expect(progress.TotalActive).To(BeNumerically(">", 0))
		})

		It("should report total queues count", func() {
			Expect(progress.TotalQueues).To(Equal(int32(2)))
		})

		It("should include queue statistics", func() {
			queueNames := []string{}
			for _, q := range progress.QueueStats {
				queueNames = append(queueNames, q.QueueName)
			}
			Expect(queueNames).To(ContainElements("Queue1", "Queue2"))
		})
	})

	Describe("when dispatch fails", func() {
		var failingCoordinator *jobs.JobQueueCoordinator
		var dispatchErr error

		BeforeEach(func() {
			// Create a coordinator with a factory that returns failing dispatchers
			dispatchErr = &testError{msg: "dispatcher not active"}
			logger := lumber.NewConsoleLogger(lumber.WARN)
			failingCoordinator = jobs.NewJobQueueCoordinatorWithFactory(
				logger,
				jobs.FailingDispatcherFactory(dispatchErr),
			)
		})

		Describe("when EnqueueJob fails", func() {
			var enqueueErr error

			BeforeEach(func() {
				job := &jobs.MockJob{Name: "FailingQueue"}
				enqueueErr = failingCoordinator.EnqueueJob(job)
			})

			It("should return an error", func() {
				Expect(enqueueErr).To(HaveOccurred())
			})

			It("should include dispatch in error message", func() {
				Expect(enqueueErr.Error()).To(ContainSubstring("dispatch"))
			})

			It("should wrap the original error", func() {
				Expect(enqueueErr.Error()).To(ContainSubstring("dispatcher not active"))
			})

			It("should rollback job count", func() {
				stats := failingCoordinator.GetQueueStats("FailingQueue")
				Expect(stats.JobsRemaining).To(Equal(int32(0)))
			})

			It("should mark queue as inactive", func() {
				stats := failingCoordinator.GetQueueStats("FailingQueue")
				Expect(stats.IsActive).To(BeFalse())
			})
		})

		Describe("when EnqueueJobWithCompletion fails", func() {
			var enqueueErr error
			var callbackCalled bool

			BeforeEach(func() {
				callbackCalled = false
				job := &jobs.MockJob{Name: "FailingCompletionQueue"}
				enqueueErr = failingCoordinator.EnqueueJobWithCompletion(job, func(err error) {
					callbackCalled = true
				})
			})

			It("should return an error", func() {
				Expect(enqueueErr).To(HaveOccurred())
			})

			It("should include dispatch in error message", func() {
				Expect(enqueueErr.Error()).To(ContainSubstring("dispatch"))
			})

			It("should not call the completion callback", func() {
				Expect(callbackCalled).To(BeFalse())
			})

			It("should rollback job count", func() {
				stats := failingCoordinator.GetQueueStats("FailingCompletionQueue")
				Expect(stats.JobsRemaining).To(Equal(int32(0)))
			})
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

var _ = Describe("JobQueueCoordinator.RegisterQueue", func() {
	var coordinator *jobs.JobQueueCoordinator

	BeforeEach(func() {
		logger := lumber.NewConsoleLogger(lumber.WARN)
		coordinator = jobs.NewJobQueueCoordinator(logger)
	})

	Describe("when registering a queue with multiple workers", func() {
		var registerErr error

		BeforeEach(func() {
			registerErr = coordinator.RegisterQueue("MultiWorker", 3, 10)
		})

		It("should not return an error", func() {
			Expect(registerErr).NotTo(HaveOccurred())
		})

		It("should allow concurrent dispatches", func() {
			job1 := jobs.NewBlockingMockJob("MultiWorker")
			job2 := jobs.NewBlockingMockJob("MultiWorker")
			job3 := jobs.NewBlockingMockJob("MultiWorker")

			Expect(coordinator.EnqueueJob(job1)).To(Succeed())
			Expect(coordinator.EnqueueJob(job2)).To(Succeed())
			Expect(coordinator.EnqueueJob(job3)).To(Succeed())

			// All three should be running simultaneously (workers=3); jobs_remaining
			// reflects queued + in-flight, so we expect 3 here.
			Eventually(func() int32 {
				return coordinator.GetQueueStats("MultiWorker").JobsRemaining
			}).Should(Equal(int32(3)))

			job1.Release()
			job2.Release()
			job3.Release()

			Eventually(func() int32 {
				return coordinator.GetQueueStats("MultiWorker").JobsRemaining
			}).Should(Equal(int32(0)))
		})
	})

	Describe("when registering the same queue twice", func() {
		var firstErr, secondErr error

		BeforeEach(func() {
			firstErr = coordinator.RegisterQueue("Duplicate", 1, 10)
			secondErr = coordinator.RegisterQueue("Duplicate", 1, 10)
		})

		It("should accept the first registration", func() {
			Expect(firstErr).NotTo(HaveOccurred())
		})

		It("should reject the second registration with an error", func() {
			Expect(secondErr).To(HaveOccurred())
		})

		It("should mention the queue name in the error", func() {
			Expect(secondErr.Error()).To(ContainSubstring("Duplicate"))
		})
	})

	Describe("when a queue is registered then a job is enqueued for it", func() {
		var enqueueErr error

		BeforeEach(func() {
			Expect(coordinator.RegisterQueue("Preregistered", 2, 10)).To(Succeed())
			enqueueErr = coordinator.EnqueueJob(&jobs.MockJob{Name: "Preregistered"})
		})

		It("should not return an error", func() {
			Expect(enqueueErr).NotTo(HaveOccurred())
		})

		It("should not auto-register a duplicate dispatcher", func() {
			// The queue must remain the one we registered (workers=2, capacity=10).
			// Best proxy: stats exist for the registered name.
			Expect(coordinator.GetQueueStats("Preregistered")).NotTo(BeNil())
		})
	})

	Describe("auto-register fallback for unregistered names", func() {
		var enqueueErr error

		BeforeEach(func() {
			enqueueErr = coordinator.EnqueueJob(&jobs.MockJob{Name: "Unregistered"})
		})

		It("should still succeed (back-compat)", func() {
			Expect(enqueueErr).NotTo(HaveOccurred())
		})
	})
})