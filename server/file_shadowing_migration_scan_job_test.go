//revive:disable:dot-imports
package server

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/jcelliott/lumber"
	"github.com/schollz/versionedtext"
)

var _ = Describe("FileShadowingMigrationScanJob", func() {
	var (
		job         *FileShadowingMigrationScanJob
		testDataDir string
		coordinator *jobs.JobQueueCoordinator
		site        *Site
		queueName   string
	)

	BeforeEach(func() {
		// Create temporary test directory
		var err error
		testDataDir, err = os.MkdirTemp("", "file-shadowing-scan-test")
		Expect(err).NotTo(HaveOccurred())
		
		coordinator = jobs.NewJobQueueCoordinator()
		queueName = "FileShadowingMigration"
		coordinator.RegisterQueue(queueName)
		
		// Initialize Site with Logger for DirectoryList() to work
		site = &Site{
			PathToData: testDataDir,
			Logger:     lumber.NewConsoleLogger(lumber.WARN),
		}
		job = NewFileShadowingMigrationScanJob(testDataDir, coordinator, queueName, site)
	})

	AfterEach(func() {
		os.RemoveAll(testDataDir)
	})

	Describe("Execute", func() {
		When("directory contains PascalCase identifiers", func() {
			BeforeEach(func() {
				// Create PascalCase pages using Site.Open() - they will be stored as base32-encoded files
				// but the Page.Identifier will remain PascalCase
				labPage, err := site.Open("LabInventory")
				Expect(err).NotTo(HaveOccurred())
				labPage.Text = versionedtext.NewVersionedText("# Lab Inventory")
				labPage.Save()
				
				userPage, err := site.Open("UserGuide")
				Expect(err).NotTo(HaveOccurred()) 
				userPage.Text = versionedtext.NewVersionedText("# User Guide")
				userPage.Save()
				
				devicePage, err := site.Open("DeviceManual")
				Expect(err).NotTo(HaveOccurred())
				devicePage.Text = versionedtext.NewVersionedText("# Device Manual")
				devicePage.Save()
				
				// Create already-munged page (should be ignored by scan)
				existingPage, err := site.Open("lab_inventory")
				Expect(err).NotTo(HaveOccurred())
				existingPage.Text = versionedtext.NewVersionedText("# Existing Lab")
				existingPage.Save()
				
				// Non-page files (should be ignored)
				createTestFile(testDataDir, "sha256_somehash", "binary content")
				createTestFile(testDataDir, "random.txt", "text file")
			})

			It("should enqueue migration jobs for each PascalCase identifier", func() {
				err := job.Execute()
				Expect(err).NotTo(HaveOccurred())
				
				// Should have enqueued jobs for LabInventory, UserGuide, DeviceManual (3 identifiers)
				stats := coordinator.GetQueueStats(queueName)
				Expect(stats).NotTo(BeNil())
				Expect(stats.JobsRemaining).To(Equal(int32(3)))
				Expect(stats.IsActive).To(BeTrue())
			})
		})

		When("directory has no PascalCase identifiers", func() {
			BeforeEach(func() {
				// Only munged identifiers using Site.Open (creates base32-encoded files)
				labPage, err := site.Open("lab_inventory")
				Expect(err).NotTo(HaveOccurred())
				labPage.Text = versionedtext.NewVersionedText("# Lab Inventory")
				labPage.Save()
				
				userPage, err := site.Open("user_guide")
				Expect(err).NotTo(HaveOccurred())
				userPage.Text = versionedtext.NewVersionedText("# User Guide")  
				userPage.Save()
				
				// Non-page files
				createTestFile(testDataDir, "sha256_somehash", "binary content")
			})

			It("should not enqueue any migration jobs", func() {
				err := job.Execute()
				Expect(err).NotTo(HaveOccurred())
				
				stats := coordinator.GetQueueStats(queueName)
				Expect(stats).NotTo(BeNil())
				Expect(stats.JobsRemaining).To(Equal(int32(0)))
				Expect(stats.IsActive).To(BeFalse())
			})
		})

		When("directory does not exist", func() {
			BeforeEach(func() {
				os.RemoveAll(testDataDir)
			})

			It("should return an error", func() {
				err := job.Execute()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no such file or directory"))
			})
		})
	})

	Describe("FindPascalCaseIdentifiers", func() {
		When("directory contains mixed page types", func() {
			BeforeEach(func() {
				// Create PascalCase pages using Site.Open() - stored as base32 but identifiers remain PascalCase
				labPage, err := site.Open("LabInventory")
				Expect(err).NotTo(HaveOccurred())
				labPage.Text = versionedtext.NewVersionedText("# Lab Inventory")
				labPage.Save()
				
				userPage, err := site.Open("UserGuide")
				Expect(err).NotTo(HaveOccurred()) 
				userPage.Text = versionedtext.NewVersionedText("# User Guide")
				userPage.Save()
				
				devicePage, err := site.Open("DeviceList")
				Expect(err).NotTo(HaveOccurred())
				devicePage.Text = versionedtext.NewVersionedText("# Device List")
				devicePage.Save()
				
				// Create already-munged page using Site.Open (creates base32-encoded files)
				existingPage, err := site.Open("existing_page")
				Expect(err).NotTo(HaveOccurred())
				existingPage.Text = versionedtext.NewVersionedText("# Existing Page")
				existingPage.Save()
				
				// Non-page files (should be ignored)
				createTestFile(testDataDir, "sha256_abcdef", "binary")
				createTestFile(testDataDir, "config.txt", "config")
			})

			It("should identify only PascalCase identifiers", func() {
				pascalIdentifiers := job.FindPascalCaseIdentifiers()
				
				Expect(pascalIdentifiers).To(HaveLen(3))
				Expect(pascalIdentifiers).To(ContainElements("LabInventory", "UserGuide", "DeviceList"))
			})
		})
	})
})

const testFileTimestamp = 1609459200 // 2021-01-01 Unix timestamp

// Helper function to create test files
func createTestFile(dir, filename, content string) {
	filePath := filepath.Join(dir, filename)
	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		panic(err)
	}
	// Set a consistent timestamp for testing
	timestamp := time.Unix(testFileTimestamp, 0)
	if err := os.Chtimes(filePath, timestamp, timestamp); err != nil {
		panic(err)
	}
}