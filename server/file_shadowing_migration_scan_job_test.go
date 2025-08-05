//revive:disable:dot-imports
package server

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/rollingmigrations"
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
			MigrationApplicator: rollingmigrations.NewEmptyApplicator(),
		}
		job = NewFileShadowingMigrationScanJob(testDataDir, coordinator, queueName, site)
	})

	AfterEach(func() {
		os.RemoveAll(testDataDir)
	})

	Describe("Execute", func() {
		When("directory contains PascalCase identifiers", func() {
			var (
				err   error
				stats *jobs.QueueStats
			)

			BeforeEach(func() {
				// Create PascalCase pages directly on filesystem to simulate legacy state
				// These would have been created before the munging system was implemented
				createPascalCasePage(testDataDir, "LabInventory", "# Lab Inventory")
				createPascalCasePage(testDataDir, "UserGuide", "# User Guide") 
				createPascalCasePage(testDataDir, "DeviceManual", "# Device Manual")
				
				// Create already-munged page using Site.Open() (should be ignored by scan)
				existingPage, existingErr := site.Open("lab_inventory")
				Expect(existingErr).NotTo(HaveOccurred())
				existingPage.Text = versionedtext.NewVersionedText("# Existing Lab")
				site.UpdatePageContent(existingPage.Identifier, existingPage.Text.GetCurrent())
				
				// Non-page files (should be ignored)
				createTestFile(testDataDir, "sha256_somehash", "binary content")
				createTestFile(testDataDir, "random.txt", "text file")

				// Act
				err = job.Execute()
				stats = coordinator.GetQueueStats(queueName)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should enqueue migration jobs for each PascalCase identifier", func() {
				// Should have enqueued jobs for LabInventory, UserGuide, DeviceManual (3 identifiers)
				Expect(stats).NotTo(BeNil())
				Expect(stats.JobsRemaining).To(Equal(int32(3)))
				Expect(stats.IsActive).To(BeTrue())
			})
		})

		When("directory has no PascalCase identifiers", func() {
			var (
				err   error
				stats *jobs.QueueStats
			)

			BeforeEach(func() {
				// Only munged identifiers using Site.Open (creates base32-encoded files)
				labPage, labErr := site.Open("lab_inventory")
				Expect(labErr).NotTo(HaveOccurred())
				labPage.Text = versionedtext.NewVersionedText("# Lab Inventory")
				site.UpdatePageContent(labPage.Identifier, labPage.Text.GetCurrent())
				
				userPage, userErr := site.Open("user_guide")
				Expect(userErr).NotTo(HaveOccurred())
				userPage.Text = versionedtext.NewVersionedText("# User Guide")  
				site.UpdatePageContent(userPage.Identifier, userPage.Text.GetCurrent())
				
				// Non-page files
				createTestFile(testDataDir, "sha256_somehash", "binary content")

				// Act
				err = job.Execute()
				stats = coordinator.GetQueueStats(queueName)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not enqueue any migration jobs", func() {
				Expect(stats).NotTo(BeNil())
				Expect(stats.JobsRemaining).To(Equal(int32(0)))
				Expect(stats.IsActive).To(BeFalse())
			})
		})

		When("directory does not exist", func() {
			var err error

			BeforeEach(func() {
				os.RemoveAll(testDataDir)

				// Act
				err = job.Execute()
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should indicate directory not found", func() {
				Expect(err.Error()).To(ContainSubstring("no such file or directory"))
			})
		})
	})

	Describe("FindPascalCaseIdentifiers", func() {
		When("directory contains mixed page types", func() {
			var pascalIdentifiers []string

			BeforeEach(func() {
				// Create PascalCase pages directly on filesystem to simulate legacy state
				createPascalCasePage(testDataDir, "LabInventory", "# Lab Inventory")
				createPascalCasePage(testDataDir, "UserGuide", "# User Guide") 
				createPascalCasePage(testDataDir, "DeviceList", "# Device List")
				
				// Create already-munged page using Site.Open (creates base32-encoded files)
				existingPage, existingErr := site.Open("existing_page")
				Expect(existingErr).NotTo(HaveOccurred())
				existingPage.Text = versionedtext.NewVersionedText("# Existing Page")
				site.UpdatePageContent(existingPage.Identifier, existingPage.Text.GetCurrent())
				
				// Non-page files (should be ignored)
				createTestFile(testDataDir, "sha256_abcdef", "binary")
				createTestFile(testDataDir, "config.txt", "config")

				// Act
				pascalIdentifiers = job.FindPascalCaseIdentifiers()
			})

			It("should identify the correct number of PascalCase identifiers", func() {
				Expect(pascalIdentifiers).To(HaveLen(3))
			})

			It("should identify only PascalCase identifiers", func() {
				Expect(pascalIdentifiers).To(ContainElements("LabInventory", "UserGuide", "DeviceList"))
			})
		})

		When("directory contains identifiers that would have same base32 encoding when munged", func() {
			var pascalIdentifiers []string

			BeforeEach(func() {
				// Create a page that has mixed case but would result in same base32 when munged
				// This should NOT be detected as PascalCase for migration since it'd cause file conflicts
				createPascalCasePage(testDataDir, "lab_smallparts_2B4", "# Lab Smallparts 2B4")
				
				// Create a true PascalCase identifier that would have different base32 encoding
				createPascalCasePage(testDataDir, "TruePascalCase", "# True Pascal Case")

				// Act
				pascalIdentifiers = job.FindPascalCaseIdentifiers()
			})

			It("should detect only safe PascalCase identifiers", func() {
				// Should only find TruePascalCase, not lab_smallparts_2B4 (which would conflict)
				Expect(pascalIdentifiers).To(HaveLen(1))
			})

			It("should identify the safe PascalCase identifier", func() {
				Expect(pascalIdentifiers).To(ContainElement("TruePascalCase"))
			})

			It("should not detect identifiers that would conflict when munged", func() {
				Expect(pascalIdentifiers).NotTo(ContainElement("lab_smallparts_2B4"))
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