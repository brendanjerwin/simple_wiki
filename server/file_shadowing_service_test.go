//revive:disable:dot-imports
package server

import (
	"fmt"
	"os"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/rollingmigrations"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/jcelliott/lumber"
	"github.com/schollz/versionedtext"
)

// BlockingJob is a mock job that blocks until released
type BlockingJob struct {
	name     string
	blocker  chan struct{}
	executed bool
	mu       sync.Mutex
}

func NewBlockingJob(name string) *BlockingJob {
	return &BlockingJob{
		name:    name,
		blocker: make(chan struct{}),
	}
}

func (j *BlockingJob) Execute() error {
	j.mu.Lock()
	j.executed = true
	j.mu.Unlock()
	
	// Block until released
	<-j.blocker
	return nil
}

func (j *BlockingJob) GetName() string {
	return j.name
}

func (j *BlockingJob) Release() {
	select {
	case <-j.blocker:
		// Already closed
	default:
		close(j.blocker)
	}
}

func (j *BlockingJob) IsExecuted() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.executed
}

// MockScanJob that enqueues controllable migration jobs
type MockScanJob struct {
	site           *Site
	coordinator    *jobs.JobQueueCoordinator
	migrationJobs  []*BlockingJob
}

func NewMockScanJob(site *Site, coordinator *jobs.JobQueueCoordinator) *MockScanJob {
	return &MockScanJob{
		site:        site,
		coordinator: coordinator,
	}
}

func (j *MockScanJob) Execute() error {
	// Find PascalCase identifiers
	entries := j.site.DirectoryList()
	
	for _, entry := range entries {
		identifier := entry.Name()
		mungedVersion := wikiidentifiers.MungeIdentifier(identifier)
		if identifier != mungedVersion {
			// Create a blocking job for each PascalCase identifier
			blockingJob := NewBlockingJob(fmt.Sprintf("MockMigration-%s", identifier))
			j.migrationJobs = append(j.migrationJobs, blockingJob)
			j.coordinator.EnqueueJob(blockingJob)
		}
	}
	
	return nil
}

func (*MockScanJob) GetName() string {
	return "MockScanJob"
}

func (j *MockScanJob) GetMigrationJobs() []*BlockingJob {
	return j.migrationJobs
}

var _ = Describe("FileShadowingService", func() {
	var (
		service        *FileShadowingService
		coordinator    *jobs.JobQueueCoordinator
		testDataDir    string
		site           *Site
	)

	BeforeEach(func() {
		// Create temporary test directory
		var err error
		testDataDir, err = os.MkdirTemp("", "file-shadowing-service-test")
		Expect(err).NotTo(HaveOccurred())
		
		// Create minimal site for testing
		site = &Site{
			PathToData: testDataDir,
			Logger:     lumber.NewConsoleLogger(lumber.WARN),
			MigrationApplicator: rollingmigrations.NewEmptyApplicator(),
		}
		
		logger := lumber.NewConsoleLogger(lumber.WARN) // Quiet logger for tests
		coordinator = jobs.NewJobQueueCoordinator(logger)
		service = NewFileShadowingService(coordinator, site)
	})

	AfterEach(func() {
		os.RemoveAll(testDataDir)
	})

	Describe("NewFileShadowingService", func() {
		It("should create a new service", func() {
			Expect(service).NotTo(BeNil())
		})

		It("should set coordinator correctly", func() {
			Expect(service.coordinator).To(Equal(coordinator))
		})

		It("should set site correctly", func() {
			Expect(service.site).To(Equal(site))
		})
	})


	Describe("EnqueueScanJob", func() {
		It("should enqueue scan job", func() {
			service.EnqueueScanJob()
			
			// Check that a queue was auto-registered for the scan job
			activeQueues := coordinator.GetActiveQueues()
			Expect(len(activeQueues)).To(Equal(1))
			Expect(activeQueues[0].QueueName).To(Equal("FileShadowingMigrationScanJob"))
		})
	})

	Describe("GetJobQueueCoordinator", func() {
		It("should return the coordinator", func() {
			result := service.GetJobQueueCoordinator()
			Expect(result).To(Equal(coordinator))
		})
	})

	Describe("integration with mock scan job", func() {
		var mockScanJob *MockScanJob
		
		BeforeEach(func() {
			// Create PascalCase pages that should be found by scan
			labPage, err := site.Open("LabInventory")
			Expect(err).NotTo(HaveOccurred())
			labPage.Text = versionedtext.NewVersionedText("# Lab Inventory")
			err = site.UpdatePageContent(labPage.Identifier, labPage.Text.GetCurrent())
			Expect(err).NotTo(HaveOccurred())
			
			userPage, err := site.Open("UserGuide")
			Expect(err).NotTo(HaveOccurred())
			userPage.Text = versionedtext.NewVersionedText("# User Guide")
			err = site.UpdatePageContent(userPage.Identifier, userPage.Text.GetCurrent())
			Expect(err).NotTo(HaveOccurred())
			
			// Also create a munged page that already exists to verify no migration
			existingPage, err := site.Open("existing_page")
			Expect(err).NotTo(HaveOccurred())
			existingPage.Text = versionedtext.NewVersionedText("# Already Munged")
			err = site.UpdatePageContent(existingPage.Identifier, existingPage.Text.GetCurrent())
			Expect(err).NotTo(HaveOccurred())
			
			// Create mock scan job instead of using the real one
			mockScanJob = NewMockScanJob(site, coordinator)
		})

		When("mock scan job is executed", func() {
			var migrationJobs []*BlockingJob
			
			BeforeEach(func() {
				// Enqueue the mock scan job
				coordinator.EnqueueJob(mockScanJob)
				
				// Wait for scan job to execute
				Eventually(func() bool {
					stats := coordinator.GetQueueStats("MockScanJob")
					return stats != nil && stats.JobsRemaining == 0
				}, "2s", "10ms").Should(BeTrue())
				
				// Get the migration jobs created by the scan job
				migrationJobs = mockScanJob.GetMigrationJobs()
			})
			
			AfterEach(func() {
				// Release all blocking jobs to clean up
				for _, job := range migrationJobs {
					job.Release()
				}
			})

			It("should enqueue migration jobs for PascalCase pages", func() {
				// Each migration job gets its own queue, so check active queues
				activeQueues := coordinator.GetActiveQueues()
				Expect(len(activeQueues)).To(Equal(2)) // Should have 2 migration job queues
				
				// Verify the jobs were created for the right identifiers
				Expect(migrationJobs).To(HaveLen(2))
				jobNames := []string{}
				for _, job := range migrationJobs {
					jobNames = append(jobNames, job.GetName())
				}
				Expect(jobNames).To(ContainElements("MockMigration-LabInventory", "MockMigration-UserGuide"))
				
				// Each queue should have 1 job remaining (they're blocked)
				for _, queue := range activeQueues {
					Expect(queue.JobsRemaining).To(Equal(int32(1)))
					Expect(queue.HighWaterMark).To(Equal(int32(1)))
					Expect(queue.IsActive).To(BeTrue())
				}
			})
			
			It("should maintain queue stats while jobs are blocked", func() {
				// Initial state - 2 active queues with jobs blocked
				activeQueues := coordinator.GetActiveQueues()
				Expect(len(activeQueues)).To(Equal(2))
				for _, queue := range activeQueues {
					Expect(queue.JobsRemaining).To(Equal(int32(1)))
					Expect(queue.IsActive).To(BeTrue())
				}
				
				// Wait for at least one job to start executing
				Eventually(func() bool {
					return migrationJobs[0].IsExecuted() || migrationJobs[1].IsExecuted()
				}, "2s", "10ms").Should(BeTrue())
				
				// High water mark should remain at 1 while jobs are running
				activeQueues = coordinator.GetActiveQueues()
				for _, queue := range activeQueues {
					Expect(queue.HighWaterMark).To(Equal(int32(1)))
				}
				
				// Release all jobs
				for _, job := range migrationJobs {
					job.Release()
				}
				
				// Wait for all jobs to complete
				Eventually(func() int {
					return len(coordinator.GetActiveQueues())
				}, "2s", "10ms").Should(Equal(0))
				
				// After all jobs complete, no active queues should remain
				activeQueues = coordinator.GetActiveQueues()
				Expect(len(activeQueues)).To(Equal(0))
			})
		})
	})
})