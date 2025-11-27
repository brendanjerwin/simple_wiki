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
})