package jobs_test

import (
	"testing"

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
		coordinator = jobs.NewJobQueueCoordinator()
	})

	It("should exist", func() {
		Expect(coordinator).NotTo(BeNil())
	})

	Describe("when registering a new queue", func() {
		var queueName string

		BeforeEach(func() {
			queueName = "TestQueue"
			coordinator.RegisterQueue(queueName)
		})

		It("should have the queue registered", func() {
			stats := coordinator.GetQueueStats(queueName)
			Expect(stats).NotTo(BeNil())
		})

		It("should have zero jobs remaining initially", func() {
			stats := coordinator.GetQueueStats(queueName)
			Expect(stats.JobsRemaining).To(Equal(int32(0)))
		})

		It("should have zero high water mark initially", func() {
			stats := coordinator.GetQueueStats(queueName)
			Expect(stats.HighWaterMark).To(Equal(int32(0)))
		})

		It("should not be active initially", func() {
			stats := coordinator.GetQueueStats(queueName)
			Expect(stats.IsActive).To(BeFalse())
		})
	})

	Describe("when enqueueing a job", func() {
		var queueName string
		var mockJob *jobs.MockJob

		BeforeEach(func() {
			queueName = "TestQueue"
			coordinator.RegisterQueue(queueName)
			mockJob = &jobs.MockJob{Name: "test-job"}
			coordinator.EnqueueJob(queueName, mockJob)
		})

		It("should increase jobs remaining", func() {
			stats := coordinator.GetQueueStats(queueName)
			Expect(stats.JobsRemaining).To(Equal(int32(1)))
		})

		It("should update high water mark", func() {
			stats := coordinator.GetQueueStats(queueName)
			Expect(stats.HighWaterMark).To(Equal(int32(1)))
		})

		It("should mark queue as active", func() {
			stats := coordinator.GetQueueStats(queueName)
			Expect(stats.IsActive).To(BeTrue())
		})

		Describe("when the job completes", func() {
			BeforeEach(func() {
				// Allow job to complete by waiting briefly
				Eventually(func() int32 {
					return coordinator.GetQueueStats(queueName).JobsRemaining
				}).Should(Equal(int32(0)))
			})

			It("should reset jobs remaining to zero", func() {
				stats := coordinator.GetQueueStats(queueName)
				Expect(stats.JobsRemaining).To(Equal(int32(0)))
			})

			It("should reset high water mark to zero when queue is empty", func() {
				stats := coordinator.GetQueueStats(queueName)
				Expect(stats.HighWaterMark).To(Equal(int32(0)))
			})

			It("should mark queue as inactive", func() {
				stats := coordinator.GetQueueStats(queueName)
				Expect(stats.IsActive).To(BeFalse())
			})
		})
	})

	Describe("when getting all active queues", func() {
		BeforeEach(func() {
			coordinator.RegisterQueue("Queue1")
			coordinator.RegisterQueue("Queue2")
			coordinator.RegisterQueue("Queue3")
			
			// Enqueue jobs to make Queue1 and Queue3 active
			coordinator.EnqueueJob("Queue1", &jobs.MockJob{Name: "job1"})
			coordinator.EnqueueJob("Queue3", &jobs.MockJob{Name: "job3"})
		})

		var (
			activeQueues []*jobs.QueueStats
			queueNames   []string
		)

		BeforeEach(func() {
			activeQueues = coordinator.GetActiveQueues()
			queueNames = make([]string, len(activeQueues))
			for i, stats := range activeQueues {
				queueNames[i] = stats.QueueName
			}
		})

		It("should return correct number of active queues", func() {
			Expect(len(activeQueues)).To(Equal(2))
		})

		It("should return only active queues", func() {
			Expect(queueNames).To(ContainElements("Queue1", "Queue3"))
		})
	})
})