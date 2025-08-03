//revive:disable:dot-imports
package server

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
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
		
		site = &Site{PathToData: testDataDir}
		job = NewFileShadowingMigrationScanJob(testDataDir, coordinator, queueName, site)
	})

	AfterEach(func() {
		os.RemoveAll(testDataDir)
	})

	Describe("Execute", func() {
		When("directory contains PascalCase files", func() {
			BeforeEach(func() {
				// Create PascalCase files that need migration
				createTestFile(testDataDir, "LabInventory.json", `{"text":{"current":"# Lab Inventory"}}`)
				createTestFile(testDataDir, "LabInventory.md", "# Lab Inventory")
				createTestFile(testDataDir, "UserGuide.json", `{"text":{"current":"# User Guide"}}`)
				createTestFile(testDataDir, "DeviceManual.md", "# Device Manual")
				
				// Create already-munged files (should be ignored)
				mungedLabName := base32tools.EncodeToBase32("lab_inventory") + ".json"
				createTestFile(testDataDir, mungedLabName, `{"text":{"current":"# Existing Lab"}}`)
				
				// Create non-page files (should be ignored)
				createTestFile(testDataDir, "sha256_somehash", "binary content")
				createTestFile(testDataDir, "random.txt", "text file")
			})

			It("should enqueue migration jobs for each PascalCase logical page", func() {
				err := job.Execute()
				Expect(err).NotTo(HaveOccurred())
				
				// Should have enqueued jobs for LabInventory, UserGuide, DeviceManual (3 logical pages)
				stats := coordinator.GetQueueStats(queueName)
				Expect(stats).NotTo(BeNil())
				Expect(stats.JobsRemaining).To(Equal(int32(3)))
				Expect(stats.IsActive).To(BeTrue())
			})
		})

		When("directory has no PascalCase files", func() {
			BeforeEach(func() {
				// Only munged files
				mungedName := base32tools.EncodeToBase32("lab_inventory") + ".json"
				createTestFile(testDataDir, mungedName, `{"text":{"current":"# Lab"}}`)
				
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

	Describe("FindPascalCaseFiles", func() {
		When("directory contains mixed file types", func() {
			BeforeEach(func() {
				// PascalCase files
				createTestFile(testDataDir, "LabInventory.json", `{"text":{"current":"# Lab"}}`)
				createTestFile(testDataDir, "UserGuide.md", "# Guide")
				createTestFile(testDataDir, "DeviceList.json", `{"text":{"current":"# Devices"}}`)
				
				// Already munged files
				mungedName := base32tools.EncodeToBase32("existing_page") + ".json"
				createTestFile(testDataDir, mungedName, `{"text":{"current":"# Existing"}}`)
				
				// Non-page files
				createTestFile(testDataDir, "sha256_abcdef", "binary")
				createTestFile(testDataDir, "config.txt", "config")
			})

			It("should identify only PascalCase page files", func() {
				pascalFiles := job.FindPascalCaseFiles()
				
				Expect(pascalFiles).To(HaveLen(3))
				Expect(pascalFiles).To(ContainElements("LabInventory.json", "UserGuide.md", "DeviceList.json"))
			})
		})
	})

	Describe("GroupFilesByLogicalPage", func() {
		When("files belong to same logical pages", func() {
			It("should group files by their base identifier", func() {
				pascalFiles := []string{
					"LabInventory.json", "LabInventory.md",
					"UserGuide.json",
					"DeviceList.md",
				}
				
				groups := job.GroupFilesByLogicalPage(pascalFiles)
				
				Expect(groups).To(HaveLen(3))
				Expect(groups["LabInventory"]).To(ConsistOf("LabInventory.json", "LabInventory.md"))
				Expect(groups["UserGuide"]).To(ConsistOf("UserGuide.json"))
				Expect(groups["DeviceList"]).To(ConsistOf("DeviceList.md"))
			})
		})
	})
})

var _ = Describe("FileShadowingMigrationJob", func() {
	var (
		job         *FileShadowingMigrationJob
		testDataDir string
		site        *Site
	)

	BeforeEach(func() {
		// Create temporary test directory
		var err error
		testDataDir, err = os.MkdirTemp("", "file-shadowing-migration-test")
		Expect(err).NotTo(HaveOccurred())
		
		// Create minimal site for testing
		site = &Site{
			PathToData: testDataDir,
		}
		
		job = NewFileShadowingMigrationJob(site, "")
	})

	AfterEach(func() {
		os.RemoveAll(testDataDir)
	})

	Describe("Execute", func() {
		When("PascalCase files exist with shadowing conflict", func() {
			BeforeEach(func() {
				// Create PascalCase files with rich content
				pascalJSONContent := `{"text":{"current":"# Rich Pascal Lab Inventory\n\nThis has detailed content with multiple sections.\n\n## Equipment\n- Microscope\n- Centrifuge","versions":[{"text":"# Lab Inventory","timestamp":1000000000000000000},{"text":"# Rich Pascal Lab Inventory\n\nThis has detailed content with multiple sections.\n\n## Equipment\n- Microscope\n- Centrifuge","timestamp":2000000000000000000}]}}`
				createTestFile(testDataDir, "LabInventory.json", pascalJSONContent)
				createTestFile(testDataDir, "LabInventory.md", "# Rich Pascal Lab Inventory\n\nThis has detailed content with multiple sections.\n\n## Equipment\n- Microscope\n- Centrifuge")
				
				// Create existing munged files with poor content
				mungedJSONName := base32tools.EncodeToBase32("lab_inventory") + ".json"
				mungedMdName := base32tools.EncodeToBase32("lab_inventory") + ".md"
				createTestFile(testDataDir, mungedJSONName, `{"text":{"current":"# Poor Munged Lab"}}`)
				createTestFile(testDataDir, mungedMdName, "# Poor Munged Lab")
			})

			It("should consolidate to munged name with richer content", func() {
				job.logicalPageID = "LabInventory"
				err := job.Execute()
				Expect(err).NotTo(HaveOccurred())
				
				// Verify munged files have the richer content from PascalCase
				mungedJSONName := base32tools.EncodeToBase32("lab_inventory") + ".json"
				mungedMdName := base32tools.EncodeToBase32("lab_inventory") + ".md"
				
				mungedJSONContent, err := os.ReadFile(filepath.Join(testDataDir, mungedJSONName))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(mungedJSONContent)).To(ContainSubstring("Rich Pascal Lab Inventory"))
				Expect(string(mungedJSONContent)).To(ContainSubstring("detailed content with multiple sections"))
				Expect(string(mungedJSONContent)).To(ContainSubstring("Microscope"))
				
				mungedMdContent, err := os.ReadFile(filepath.Join(testDataDir, mungedMdName))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(mungedMdContent)).To(ContainSubstring("Rich Pascal Lab Inventory"))
				Expect(string(mungedMdContent)).To(ContainSubstring("Equipment"))
				
				// Verify PascalCase files are removed
				_, err = os.Stat(filepath.Join(testDataDir, "LabInventory.json"))
				Expect(os.IsNotExist(err)).To(BeTrue())
				_, err = os.Stat(filepath.Join(testDataDir, "LabInventory.md"))
				Expect(os.IsNotExist(err)).To(BeTrue())
			})
		})

		When("PascalCase files exist without shadowing conflict", func() {
			BeforeEach(func() {
				createTestFile(testDataDir, "UserGuide.json", `{"text":{"current":"# User Guide Content\n\nDetailed guide here."}}`)
				createTestFile(testDataDir, "UserGuide.md", "# User Guide Content\n\nDetailed guide here.")
			})

			It("should create munged versions from PascalCase content", func() {
				job.logicalPageID = "UserGuide"
				err := job.Execute()
				Expect(err).NotTo(HaveOccurred())
				
				// Verify munged files are created with PascalCase content
				mungedJSONName := base32tools.EncodeToBase32("user_guide") + ".json"
				mungedMdName := base32tools.EncodeToBase32("user_guide") + ".md"
				
				mungedJSONContent, err := os.ReadFile(filepath.Join(testDataDir, mungedJSONName))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(mungedJSONContent)).To(ContainSubstring("User Guide Content"))
				Expect(string(mungedJSONContent)).To(ContainSubstring("Detailed guide here"))
				
				mungedMdContent, err := os.ReadFile(filepath.Join(testDataDir, mungedMdName))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(mungedMdContent)).To(ContainSubstring("User Guide Content"))
				
				// Verify PascalCase files are removed
				_, err = os.Stat(filepath.Join(testDataDir, "UserGuide.json"))
				Expect(os.IsNotExist(err)).To(BeTrue())
				_, err = os.Stat(filepath.Join(testDataDir, "UserGuide.md"))
				Expect(os.IsNotExist(err)).To(BeTrue())
			})
		})

		When("munged version has richer content than PascalCase", func() {
			BeforeEach(func() {
				// Create PascalCase files with basic content
				createTestFile(testDataDir, "LabInventory.json", `{"text":{"current":"# Basic Lab"}}`)
				createTestFile(testDataDir, "LabInventory.md", "# Basic Lab")
				
				// Create munged files with much richer content
				mungedJSONName := base32tools.EncodeToBase32("lab_inventory") + ".json"
				mungedMdName := base32tools.EncodeToBase32("lab_inventory") + ".md"
				richMungedContent := `{"text":{"current":"# Rich Munged Lab Inventory\n\nThis munged version has extensive content:\n\n## Equipment List\n- Advanced Microscope\n- High-speed Centrifuge\n- Spectrophotometer\n\n## Procedures\nDetailed procedures here...","versions":[{"text":"# Lab Inventory","timestamp":1000000000000000000},{"text":"# Rich Munged Lab Inventory\n\nThis munged version has extensive content:\n\n## Equipment List\n- Advanced Microscope\n- High-speed Centrifuge\n- Spectrophotometer\n\n## Procedures\nDetailed procedures here...","timestamp":2000000000000000000}]}}`
				createTestFile(testDataDir, mungedJSONName, richMungedContent)
				createTestFile(testDataDir, mungedMdName, "# Rich Munged Lab Inventory\n\nThis munged version has extensive content:\n\n## Equipment List\n- Advanced Microscope\n- High-speed Centrifuge\n- Spectrophotometer\n\n## Procedures\nDetailed procedures here...")
			})

			It("should keep the richer munged content and remove PascalCase files", func() {
				job.logicalPageID = "LabInventory"
				err := job.Execute()
				Expect(err).NotTo(HaveOccurred())
				
				// Verify munged files retain their richer content
				mungedJSONName := base32tools.EncodeToBase32("lab_inventory") + ".json"
				mungedMdName := base32tools.EncodeToBase32("lab_inventory") + ".md"
				
				mungedJSONContent, err := os.ReadFile(filepath.Join(testDataDir, mungedJSONName))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(mungedJSONContent)).To(ContainSubstring("Rich Munged Lab Inventory"))
				Expect(string(mungedJSONContent)).To(ContainSubstring("extensive content"))
				Expect(string(mungedJSONContent)).To(ContainSubstring("Advanced Microscope"))
				Expect(string(mungedJSONContent)).To(ContainSubstring("Spectrophotometer"))
				
				mungedMdContent, err := os.ReadFile(filepath.Join(testDataDir, mungedMdName))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(mungedMdContent)).To(ContainSubstring("Rich Munged Lab Inventory"))
				Expect(string(mungedMdContent)).To(ContainSubstring("Equipment List"))
				
				// Verify PascalCase files are removed
				_, err = os.Stat(filepath.Join(testDataDir, "LabInventory.json"))
				Expect(os.IsNotExist(err)).To(BeTrue())
				_, err = os.Stat(filepath.Join(testDataDir, "LabInventory.md"))
				Expect(os.IsNotExist(err)).To(BeTrue())
			})
		})

		When("only .json file exists for PascalCase page", func() {
			BeforeEach(func() {
				createTestFile(testDataDir, "DeviceManual.json", `{"text":{"current":"# Device Manual\n\nOperating instructions."}}`)
			})

			It("should create both munged .json and .md files", func() {
				job.logicalPageID = "DeviceManual"
				err := job.Execute()
				Expect(err).NotTo(HaveOccurred())
				
				// Verify both munged files are created
				mungedJSONName := base32tools.EncodeToBase32("device_manual") + ".json"
				mungedMdName := base32tools.EncodeToBase32("device_manual") + ".md"
				
				mungedJSONContent, err := os.ReadFile(filepath.Join(testDataDir, mungedJSONName))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(mungedJSONContent)).To(ContainSubstring("Device Manual"))
				
				mungedMdContent, err := os.ReadFile(filepath.Join(testDataDir, mungedMdName))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(mungedMdContent)).To(ContainSubstring("Device Manual"))
				
				// Verify PascalCase file is removed
				_, err = os.Stat(filepath.Join(testDataDir, "DeviceManual.json"))
				Expect(os.IsNotExist(err)).To(BeTrue())
			})
		})

		When("only .md file exists for PascalCase page", func() {
			BeforeEach(func() {
				createTestFile(testDataDir, "QuickStart.md", "# Quick Start Guide\n\nGetting started instructions.")
			})

			It("should create both munged .json and .md files", func() {
				job.logicalPageID = "QuickStart"
				err := job.Execute()
				Expect(err).NotTo(HaveOccurred())
				
				// Verify both munged files are created
				mungedJSONName := base32tools.EncodeToBase32("quick_start") + ".json"
				mungedMdName := base32tools.EncodeToBase32("quick_start") + ".md"
				
				mungedJSONContent, err := os.ReadFile(filepath.Join(testDataDir, mungedJSONName))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(mungedJSONContent)).To(ContainSubstring("Quick Start Guide"))
				
				mungedMdContent, err := os.ReadFile(filepath.Join(testDataDir, mungedMdName))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(mungedMdContent)).To(ContainSubstring("Quick Start Guide"))
				
				// Verify PascalCase file is removed
				_, err = os.Stat(filepath.Join(testDataDir, "QuickStart.md"))
				Expect(os.IsNotExist(err)).To(BeTrue())
			})
		})

		When("PascalCase identifier does not exist", func() {
			It("should return an error", func() {
				job.logicalPageID = "NonExistentPage"
				err := job.Execute()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no PascalCase files found for identifier"))
			})
		})
	})

	Describe("CheckForShadowing", func() {
		When("munged version exists", func() {
			BeforeEach(func() {
				mungedJSONName := base32tools.EncodeToBase32("lab_inventory") + ".json"
				mungedMdName := base32tools.EncodeToBase32("lab_inventory") + ".md"
				createTestFile(testDataDir, mungedJSONName, `{"text":{"current":"# Munged Version"}}`)
				createTestFile(testDataDir, mungedMdName, "# Munged Version")
			})

			It("should detect shadowing conflict", func() {
				hasShadowing, mungedFiles := job.CheckForShadowing("LabInventory")
				
				Expect(hasShadowing).To(BeTrue())
				Expect(mungedFiles).To(HaveLen(2))
				
				mungedJSONName := base32tools.EncodeToBase32("lab_inventory") + ".json"
				mungedMdName := base32tools.EncodeToBase32("lab_inventory") + ".md"
				Expect(mungedFiles).To(ContainElements(
					filepath.Join(testDataDir, mungedJSONName),
					filepath.Join(testDataDir, mungedMdName),
				))
			})
		})

		When("no munged version exists", func() {
			It("should not detect shadowing", func() {
				hasShadowing, mungedFiles := job.CheckForShadowing("UserGuide")
				
				Expect(hasShadowing).To(BeFalse())
				Expect(mungedFiles).To(BeEmpty())
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