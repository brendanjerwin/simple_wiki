//revive:disable:dot-imports
package server

import (
	"fmt"
	"os"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
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
	queueName      string
	migrationJobs  []*BlockingJob
}

func NewMockScanJob(site *Site, coordinator *jobs.JobQueueCoordinator, queueName string) *MockScanJob {
	return &MockScanJob{
		site:        site,
		coordinator: coordinator,
		queueName:   queueName,
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
			j.coordinator.EnqueueJob(j.queueName, blockingJob)
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
		}
		
		coordinator = jobs.NewJobQueueCoordinator()
		service = NewFileShadowingService(coordinator, site)
	})

	AfterEach(func() {
		os.RemoveAll(testDataDir)
	})

	Describe("NewFileShadowingService", func() {
		It("should create a new service", func() {
			Expect(service).NotTo(BeNil())
			Expect(service.coordinator).To(Equal(coordinator))
			Expect(service.site).To(Equal(site))
		})
	})

	Describe("InitializeQueues", func() {
		BeforeEach(func() {
			service.InitializeQueues()
		})

		It("should register FileScan queue", func() {
			stats := coordinator.GetQueueStats("FileScan")
			Expect(stats).NotTo(BeNil())
			Expect(stats.QueueName).To(Equal("FileScan"))
		})

		It("should register FileMigration queue", func() {
			stats := coordinator.GetQueueStats("FileMigration")
			Expect(stats).NotTo(BeNil())
			Expect(stats.QueueName).To(Equal("FileMigration"))
		})
	})

	Describe("EnqueueScanJob", func() {
		BeforeEach(func() {
			service.InitializeQueues()
		})

		It("should enqueue scan job to FileScan queue", func() {
			service.EnqueueScanJob()
			
			// Check that FileScan queue has a job
			stats := coordinator.GetQueueStats("FileScan")
			Expect(stats).NotTo(BeNil())
			Expect(stats.JobsRemaining).To(BeNumerically(">", 0))
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
			service.InitializeQueues()
			
			// Create PascalCase pages that should be found by scan
			labPage, err := site.Open("LabInventory")
			Expect(err).NotTo(HaveOccurred())
			labPage.Text = versionedtext.NewVersionedText("# Lab Inventory")
			err = labPage.Save()
			Expect(err).NotTo(HaveOccurred())
			
			userPage, err := site.Open("UserGuide")
			Expect(err).NotTo(HaveOccurred())
			userPage.Text = versionedtext.NewVersionedText("# User Guide")
			err = userPage.Save()
			Expect(err).NotTo(HaveOccurred())
			
			// Also create a munged page that already exists to verify no migration
			existingPage, err := site.Open("existing_page")
			Expect(err).NotTo(HaveOccurred())
			existingPage.Text = versionedtext.NewVersionedText("# Already Munged")
			err = existingPage.Save()
			Expect(err).NotTo(HaveOccurred())
			
			// Create mock scan job instead of using the real one
			mockScanJob = NewMockScanJob(site, coordinator, "FileMigration")
		})

		When("mock scan job is executed", func() {
			var migrationJobs []*BlockingJob
			
			BeforeEach(func() {
				// Enqueue the mock scan job
				coordinator.EnqueueJob("FileScan", mockScanJob)
				
				// Wait for scan job to execute
				Eventually(func() bool {
					stats := coordinator.GetQueueStats("FileScan")
					return stats.JobsRemaining == 0
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
				// Check that FileMigration queue has migration jobs
				stats := coordinator.GetQueueStats("FileMigration")
				Expect(stats).NotTo(BeNil())
				// Should have 2 jobs remaining (they're blocked)
				Expect(stats.JobsRemaining).To(Equal(int32(2)))
				// Should have enqueued 2 migration jobs for LabInventory and UserGuide
				Expect(stats.HighWaterMark).To(Equal(int32(2)))
				
				// Verify the jobs were created for the right identifiers
				Expect(migrationJobs).To(HaveLen(2))
				jobNames := []string{}
				for _, job := range migrationJobs {
					jobNames = append(jobNames, job.GetName())
				}
				Expect(jobNames).To(ContainElements("MockMigration-LabInventory", "MockMigration-UserGuide"))
			})
			
			It("should maintain queue stats while jobs are blocked", func() {
				// Initial state - jobs are blocked
				stats := coordinator.GetQueueStats("FileMigration")
				Expect(stats.JobsRemaining).To(Equal(int32(2)))
				Expect(stats.IsActive).To(BeTrue())
				
				// Wait for at least one job to start executing
				Eventually(func() bool {
					return migrationJobs[0].IsExecuted() || migrationJobs[1].IsExecuted()
				}, "2s", "10ms").Should(BeTrue())
				
				// High water mark should remain at 2 while jobs are running
				stats = coordinator.GetQueueStats("FileMigration")
				Expect(stats.HighWaterMark).To(Equal(int32(2)))
				
				// Release all jobs
				for _, job := range migrationJobs {
					job.Release()
				}
				
				// Wait for all jobs to complete
				Eventually(func() int32 {
					return coordinator.GetQueueStats("FileMigration").JobsRemaining
				}, "2s", "10ms").Should(Equal(int32(0)))
				
				// After all jobs complete, high water mark should reset to 0
				stats = coordinator.GetQueueStats("FileMigration")
				Expect(stats.JobsRemaining).To(Equal(int32(0)))
				Expect(stats.HighWaterMark).To(Equal(int32(0))) // Reset when queue is empty
				Expect(stats.IsActive).To(BeFalse())
			})
		})
	})
})