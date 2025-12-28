//revive:disable:dot-imports
package eager

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/jcelliott/lumber"
)

var _ = Describe("FileShadowingMigrationScanJob", func() {
	var (
		job         *FileShadowingMigrationScanJob
		testDataDir string
		coordinator *jobs.JobQueueCoordinator
		deps        *MockMigrationDeps
	)

	BeforeEach(func() {
		// Create temporary test directory
		var err error
		testDataDir, err = os.MkdirTemp("", "file-shadowing-scan-test")
		Expect(err).NotTo(HaveOccurred())
		
		logger := lumber.NewConsoleLogger(lumber.WARN) // Quiet logger for tests
		coordinator = jobs.NewJobQueueCoordinator(logger)
		
		// Initialize mock deps for testing
		deps = NewMockMigrationDeps(testDataDir)
		job = NewFileShadowingMigrationScanJob(testDataDir, coordinator, deps)
	})

	AfterEach(func() {
		os.RemoveAll(testDataDir)
	})

	Describe("Execute", func() {
		When("directory contains PascalCase identifiers", func() {
			var (
				err          error
				activeQueues []*jobs.QueueStats
			)

			BeforeEach(func() {
				// Create PascalCase pages directly on filesystem to simulate legacy state
				// These would have been created before the munging system was implemented
				CreatePascalCasePage(testDataDir, "LabInventory", "# Lab Inventory")
				CreatePascalCasePage(testDataDir, "UserGuide", "# User Guide") 
				CreatePascalCasePage(testDataDir, "DeviceManual", "# Device Manual")
				
				// Create already-munged page using mock deps (should be ignored by scan)
				existingErr := deps.UpdatePageContent("lab_inventory", "# Existing Lab")
				Expect(existingErr).NotTo(HaveOccurred())
				
				// Non-page files (should be ignored)
				CreateTestFile(testDataDir, "sha256_somehash", "binary content")
				CreateTestFile(testDataDir, "random.txt", "text file")

				// Act
				err = job.Execute()
				activeQueues = coordinator.GetActiveQueues()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should enqueue migration jobs for each PascalCase identifier", func() {
				// Should have enqueued jobs for LabInventory, UserGuide, DeviceManual (3 identifiers)
				// Each gets its own queue based on job name, so check for 3 active queues
				Expect(len(activeQueues)).To(Equal(3))
				for _, queue := range activeQueues {
					Expect(queue.IsActive).To(BeTrue())
					Expect(queue.JobsRemaining).To(Equal(int32(1)))
				}
			})
		})

		When("directory has no PascalCase identifiers", func() {
			var (
				err          error
				activeQueues []*jobs.QueueStats
			)

			BeforeEach(func() {
				// Only munged identifiers using mock deps (creates base32-encoded files)
				labErr := deps.UpdatePageContent("lab_inventory", "# Lab Inventory")
				Expect(labErr).NotTo(HaveOccurred())
				
				userErr := deps.UpdatePageContent("user_guide", "# User Guide")
				Expect(userErr).NotTo(HaveOccurred())
				
				// Non-page files
				CreateTestFile(testDataDir, "sha256_somehash", "binary content")

				// Act
				err = job.Execute()
				activeQueues = coordinator.GetActiveQueues()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not enqueue any migration jobs", func() {
				// No PascalCase identifiers means no migration jobs enqueued
				Expect(len(activeQueues)).To(Equal(0))
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

	Describe("GetName", func() {
		It("should return the job name", func() {
			Expect(job.GetName()).To(Equal("FileShadowingMigrationScanJob"))
		})
	})

	Describe("FindPascalCaseIdentifiers", func() {
		When("directory contains mixed page types", func() {
			var pascalIdentifiers []string

			BeforeEach(func() {
				// Create PascalCase pages directly on filesystem to simulate legacy state
				CreatePascalCasePage(testDataDir, "LabInventory", "# Lab Inventory")
				CreatePascalCasePage(testDataDir, "UserGuide", "# User Guide")
				CreatePascalCasePage(testDataDir, "DeviceList", "# Device List")

				// Create already-munged page using mock deps (creates base32-encoded files)
				existingErr := deps.UpdatePageContent("existing_page", "# Existing Page")
				Expect(existingErr).NotTo(HaveOccurred())

				// Non-page files (should be ignored)
				CreateTestFile(testDataDir, "sha256_abcdef", "binary")
				CreateTestFile(testDataDir, "config.txt", "config")

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

		When("MD file has no frontmatter", func() {
			var pascalIdentifiers []string

			BeforeEach(func() {
				// Create an MD file without frontmatter - identifier derived from filename
				// Note: filename-derived identifiers are lowercase, so won't need migration
				CreateMDFileWithoutFrontmatter(testDataDir, "nofrontmatter", "# Just Content")

				// Act
				pascalIdentifiers = job.FindPascalCaseIdentifiers()
			})

			It("should not flag lowercase identifiers for migration", func() {
				// When derived from filename, identifier is lowercase and doesn't need migration
				Expect(pascalIdentifiers).To(BeEmpty())
			})
		})

		When("MD file has TOML frontmatter without identifier", func() {
			var pascalIdentifiers []string

			BeforeEach(func() {
				// Create an MD file with frontmatter but no identifier field
				// Note: filename-derived identifiers are lowercase, so won't need migration
				CreateMDFileWithFrontmatterNoIdentifier(testDataDir, "missingid", "title = 'Test'", "# Content")

				// Act
				pascalIdentifiers = job.FindPascalCaseIdentifiers()
			})

			It("should not flag lowercase identifiers for migration", func() {
				// When derived from filename, identifier is lowercase and doesn't need migration
				Expect(pascalIdentifiers).To(BeEmpty())
			})
		})

		When("MD file has invalid identifier that fails munging", func() {
			var pascalIdentifiers []string

			BeforeEach(func() {
				// Create an MD file with identifier that would fail MungeIdentifier
				CreateMDFileWithInvalidIdentifier(testDataDir, "testfile", "///")

				// Act
				pascalIdentifiers = job.FindPascalCaseIdentifiers()
			})

			It("should skip the invalid identifier", func() {
				Expect(pascalIdentifiers).NotTo(ContainElement("///"))
			})

			It("should return empty list", func() {
				Expect(pascalIdentifiers).To(BeEmpty())
			})
		})

		When("directory contains identifiers that would have same base32 encoding when munged", func() {
			var pascalIdentifiers []string

			BeforeEach(func() {
				// Create a page that has mixed case but would result in same base32 when munged
				// This should NOT be detected as PascalCase for migration since it'd cause file conflicts
				CreatePascalCasePage(testDataDir, "lab_smallparts_2B4", "# Lab Smallparts 2B4")
				
				// Create a true PascalCase identifier that would have different base32 encoding
				CreatePascalCasePage(testDataDir, "TruePascalCase", "# True Pascal Case")

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

