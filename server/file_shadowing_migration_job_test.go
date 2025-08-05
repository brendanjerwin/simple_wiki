//revive:disable:dot-imports
package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/rollingmigrations"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/jcelliott/lumber"
	"github.com/schollz/versionedtext"
)

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
			Logger:     lumber.NewConsoleLogger(lumber.WARN),
			MigrationApplicator: rollingmigrations.NewEmptyApplicator(),
		}
		
		job = NewFileShadowingMigrationJob(site, "")
	})

	AfterEach(func() {
		os.RemoveAll(testDataDir)
	})

	Describe("Execute", func() {
		When("PascalCase page exists with shadowing conflict", func() {
			var (
				err        error
				mungedPage *Page
				content    string
			)

			BeforeEach(func() {
				// Create PascalCase page directly on filesystem to simulate legacy state
				createPascalCasePage(testDataDir, "LabInventory", "# Rich Pascal Lab Inventory\n\nThis has detailed content with multiple sections.\n\n## Equipment\n- Microscope\n- Centrifuge")
				
				// Create existing munged page with poor content using Site.Open()
				mungedPage, err = site.Open("lab_inventory")
				Expect(err).NotTo(HaveOccurred())
				mungedPage.Text = versionedtext.NewVersionedText("# Poor Munged Lab")
				err = site.UpdatePageContent(mungedPage.Identifier, mungedPage.Text.GetCurrent())
				Expect(err).NotTo(HaveOccurred())

				// Act
				job.logicalPageID = "LabInventory"
				err = job.Execute()
				
				// Capture result data after action
				if err == nil {
					mungedPage, _ = site.Open("lab_inventory")
					if mungedPage != nil {
						content = mungedPage.Text.GetCurrent()
					}
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should consolidate to munged name with richer content", func() {
				Expect(mungedPage.IsNew()).To(BeFalse())
				Expect(content).To(ContainSubstring("Rich Pascal Lab Inventory"))
				Expect(content).To(ContainSubstring("detailed content with multiple sections"))
				Expect(content).To(ContainSubstring("Microscope"))
			})

			It("should remove original PascalCase JSON file", func() {
				pascalJSONPath := filepath.Join(testDataDir, base32tools.EncodeToBase32("labinventory")+".json")
				_, statErr := os.Stat(pascalJSONPath)
				Expect(os.IsNotExist(statErr)).To(BeTrue())
			})

			It("should remove original PascalCase MD file", func() {
				pascalMdPath := filepath.Join(testDataDir, base32tools.EncodeToBase32("labinventory")+".md")
				_, statErr := os.Stat(pascalMdPath)
				Expect(os.IsNotExist(statErr)).To(BeTrue())
			})
		})

		When("PascalCase page exists without shadowing conflict", func() {
			var (
				err        error
				mungedPage *Page
				content    string
			)

			BeforeEach(func() {
				// Create PascalCase page directly on filesystem to simulate legacy state
				createPascalCasePage(testDataDir, "UserGuide", "# User Guide Content\n\nDetailed guide here.")

				// Act
				job.logicalPageID = "UserGuide"
				err = job.Execute()
				
				// Capture result data after action
				if err == nil {
					mungedPage, _ = site.Open("user_guide")
					if mungedPage != nil {
						content = mungedPage.Text.GetCurrent()
					}
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create munged page from PascalCase content", func() {
				Expect(mungedPage.IsNew()).To(BeFalse())
				Expect(content).To(ContainSubstring("User Guide Content"))
				Expect(content).To(ContainSubstring("Detailed guide here"))
			})

			It("should remove original PascalCase JSON file", func() {
				pascalJSONPath := filepath.Join(testDataDir, base32tools.EncodeToBase32("userguide")+".json")
				_, statErr := os.Stat(pascalJSONPath)
				Expect(os.IsNotExist(statErr)).To(BeTrue())
			})

			It("should remove original PascalCase MD file", func() {
				pascalMdPath := filepath.Join(testDataDir, base32tools.EncodeToBase32("userguide")+".md")
				_, statErr := os.Stat(pascalMdPath)
				Expect(os.IsNotExist(statErr)).To(BeTrue())
			})
		})

		When("munged page has richer content than PascalCase page", func() {
			var (
				err        error
				mungedPage *Page
				content    string
			)

			BeforeEach(func() {
				// Create PascalCase page with basic content directly on filesystem
				createPascalCasePage(testDataDir, "LabInventory", "# Basic Lab")
				
				// Create munged page with much richer content using Site.Open()
				mungedPage, err = site.Open("lab_inventory")
				Expect(err).NotTo(HaveOccurred())
				mungedPage.Text = versionedtext.NewVersionedText("# Rich Munged Lab Inventory\n\nThis munged version has extensive content:\n\n## Equipment List\n- Advanced Microscope\n- High-speed Centrifuge\n- Spectrophotometer\n\n## Procedures\nDetailed procedures here...")
				err = site.UpdatePageContent(mungedPage.Identifier, mungedPage.Text.GetCurrent())
				Expect(err).NotTo(HaveOccurred())

				// Act
				job.logicalPageID = "LabInventory"
				err = job.Execute()
				
				// Capture result data after action
				if err == nil {
					mungedPage, _ = site.Open("lab_inventory")
					if mungedPage != nil {
						content = mungedPage.Text.GetCurrent()
					}
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should keep the richer munged content", func() {
				Expect(mungedPage.IsNew()).To(BeFalse())
				Expect(content).To(ContainSubstring("Rich Munged Lab Inventory"))
				Expect(content).To(ContainSubstring("extensive content"))
				Expect(content).To(ContainSubstring("Advanced Microscope"))
				Expect(content).To(ContainSubstring("Spectrophotometer"))
			})

			It("should remove original PascalCase JSON file", func() {
				pascalJSONPath := filepath.Join(testDataDir, base32tools.EncodeToBase32("labinventory")+".json")
				_, statErr := os.Stat(pascalJSONPath)
				Expect(os.IsNotExist(statErr)).To(BeTrue())
			})

			It("should remove original PascalCase MD file", func() {
				pascalMdPath := filepath.Join(testDataDir, base32tools.EncodeToBase32("labinventory")+".md")
				_, statErr := os.Stat(pascalMdPath)
				Expect(os.IsNotExist(statErr)).To(BeTrue())
			})
		})

		When("PascalCase page exists without markdown file", func() {
			var (
				err        error
				mungedPage *Page
				content    string
			)

			BeforeEach(func() {
				// Create PascalCase page directly on filesystem to simulate legacy state
				createPascalCasePage(testDataDir, "DeviceManual", "# Device Manual\n\nOperating instructions.")

				// Act
				job.logicalPageID = "DeviceManual"
				err = job.Execute()
				
				// Capture result data after action
				if err == nil {
					mungedPage, _ = site.Open("device_manual")
					if mungedPage != nil {
						content = mungedPage.Text.GetCurrent()
					}
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create munged page from PascalCase content", func() {
				Expect(mungedPage.IsNew()).To(BeFalse())
				Expect(content).To(ContainSubstring("Device Manual"))
				Expect(content).To(ContainSubstring("Operating instructions"))
			})

			It("should remove original PascalCase JSON file", func() {
				pascalJSONPath := filepath.Join(testDataDir, base32tools.EncodeToBase32("devicemanual")+".json")
				_, statErr := os.Stat(pascalJSONPath)
				Expect(os.IsNotExist(statErr)).To(BeTrue())
			})

			It("should remove original PascalCase MD file", func() {
				pascalMdPath := filepath.Join(testDataDir, base32tools.EncodeToBase32("devicemanual")+".md")
				_, statErr := os.Stat(pascalMdPath)
				Expect(os.IsNotExist(statErr)).To(BeTrue())
			})
		})

		When("PascalCase page does not exist", func() {
			var err error

			BeforeEach(func() {
				// Act
				job.logicalPageID = "NonExistentPage"
				err = job.Execute()
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should indicate page not found", func() {
				Expect(err.Error()).To(ContainSubstring("no page found for PascalCase identifier"))
			})
		})
	})

	Describe("CheckForShadowing", func() {
		When("munged version exists", func() {
			var (
				hasShadowing bool
				mungedFiles  []string
			)

			BeforeEach(func() {
				// Create munged page using Site.Open() - this will store as base32-encoded files
				mungedPage, err := site.Open("lab_inventory")
				Expect(err).NotTo(HaveOccurred())
				mungedPage.Text = versionedtext.NewVersionedText("# Munged Version")
				err = site.UpdatePageContent(mungedPage.Identifier, mungedPage.Text.GetCurrent())
				Expect(err).NotTo(HaveOccurred())

				// Act
				hasShadowing, mungedFiles = job.CheckForShadowing("LabInventory")
			})

			It("should detect shadowing conflict", func() {
				Expect(hasShadowing).To(BeTrue())
			})

			It("should return munged files", func() {
				Expect(mungedFiles).To(HaveLen(2))
				
				base32JSONName := base32tools.EncodeToBase32("lab_inventory") + ".json"
				base32MdName := base32tools.EncodeToBase32("lab_inventory") + ".md"
				Expect(mungedFiles).To(ContainElements(
					filepath.Join(testDataDir, base32JSONName),
					filepath.Join(testDataDir, base32MdName),
				))
			})
		})

		When("no munged version exists", func() {
			var (
				hasShadowing bool
				mungedFiles  []string
			)

			BeforeEach(func() {
				// Act
				hasShadowing, mungedFiles = job.CheckForShadowing("UserGuide")
			})

			It("should not detect shadowing", func() {
				Expect(hasShadowing).To(BeFalse())
			})

			It("should return empty munged files list", func() {
				Expect(mungedFiles).To(BeEmpty())
			})
		})

		When("migration should use soft delete and delete before write", func() {
			var (
				err          error
				pascalID     string
				deletedDir   string
				originalPath string
			)

			BeforeEach(func() {
				pascalID = "TestPage"
				
				// Create PascalCase page that would have same base32 encoding when munged
				// This simulates the problematic case where original and munged have same filename
				createPascalCasePage(testDataDir, pascalID, "# Test Page Content")
				
				// Calculate expected paths
				encodedName := base32tools.EncodeToBase32(strings.ToLower(pascalID))
				originalPath = filepath.Join(testDataDir, encodedName+".json")
				deletedDir = filepath.Join(testDataDir, "__deleted__")
				
				// Set up the migration job
				job.logicalPageID = pascalID
				
				// Act
				err = job.Execute()
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should move original files to __deleted__ directory using soft delete", func() {
				// Original file should no longer exist in original location
				_, statErr := os.Stat(originalPath)
				Expect(os.IsNotExist(statErr)).To(BeTrue())
				
				// __deleted__ directory should exist
				Expect(deletedDir).To(BeADirectory())
				
				// Should contain a timestamped subdirectory with the deleted file
				entries, err := os.ReadDir(deletedDir)
				Expect(err).NotTo(HaveOccurred())
				Expect(entries).To(HaveLen(1))
				
				timestampDir := entries[0]
				Expect(timestampDir.IsDir()).To(BeTrue())
				
				// Check that the file exists in the timestamped directory
				timestampPath := filepath.Join(deletedDir, timestampDir.Name())
				encodedName := base32tools.EncodeToBase32(strings.ToLower(pascalID))
				deletedFilePath := filepath.Join(timestampPath, encodedName+".json")
				
				_, deletedStatErr := os.Stat(deletedFilePath)
				Expect(deletedStatErr).NotTo(HaveOccurred())
			})

			It("should preserve content in deleted file", func() {
				// Should contain a timestamped subdirectory with the deleted file
				entries, err := os.ReadDir(deletedDir)
				Expect(err).NotTo(HaveOccurred())
				
				timestampDir := entries[0]
				timestampPath := filepath.Join(deletedDir, timestampDir.Name())
				encodedName := base32tools.EncodeToBase32(strings.ToLower(pascalID))
				deletedFilePath := filepath.Join(timestampPath, encodedName+".json")
				
				// Verify content was preserved
				deletedContent, readErr := os.ReadFile(deletedFilePath)
				Expect(readErr).NotTo(HaveOccurred())
				Expect(string(deletedContent)).To(ContainSubstring("Test Page Content"))
			})

			It("should create new file with munged identifier after deletion", func() {
				// The new file should exist with the munged identifier (which is "test_page" in this case)
				mungedID := "test_page" // wikiidentifiers.MungeIdentifier(pascalID) 
				encodedMungedName := base32tools.EncodeToBase32(strings.ToLower(mungedID))
				newFilePath := filepath.Join(testDataDir, encodedMungedName+".json")
				
				_, newFileStatErr := os.Stat(newFilePath)
				Expect(newFileStatErr).NotTo(HaveOccurred())
			})

			It("should set correct identifier in new file", func() {
				// The new file should exist with the munged identifier (which is "test_page" in this case)
				mungedID := "test_page" // wikiidentifiers.MungeIdentifier(pascalID) 
				encodedMungedName := base32tools.EncodeToBase32(strings.ToLower(mungedID))
				newFilePath := filepath.Join(testDataDir, encodedMungedName+".json")
				
				// Verify the new file has the correct identifier
				newContent, readErr := os.ReadFile(newFilePath)
				Expect(readErr).NotTo(HaveOccurred())
				Expect(string(newContent)).To(ContainSubstring(`"Identifier": "test_page"`))
			})
		})
	})
})

const testFileTimestampMigration = 1609459200 // 2021-01-01 Unix timestamp

// Helper function to create PascalCase pages directly on filesystem
// This simulates the legacy state before munging was implemented
func createPascalCasePage(dir, pascalIdentifier, content string) {
	// Create the versioned text
	vText := versionedtext.NewVersionedText(content)
	
	// Create a basic page structure with PascalCase identifier
	pageData := map[string]any{
		"identifier": pascalIdentifier,
		"text":       vText,
	}
	
	// Encode the PascalCase identifier (lowercase) to base32 for filename
	encodedFilename := base32tools.EncodeToBase32(strings.ToLower(pascalIdentifier))
	jsonPath := filepath.Join(dir, encodedFilename+".json")
	
	// Marshal and write the JSON file
	jsonData, err := json.Marshal(pageData)
	if err != nil {
		panic(err)
	}
	
	err = os.WriteFile(jsonPath, jsonData, 0644)
	if err != nil {
		panic(err)
	}
	
	// Set consistent timestamp
	timestamp := time.Unix(testFileTimestampMigration, 0)
	if err := os.Chtimes(jsonPath, timestamp, timestamp); err != nil {
		panic(err)
	}
}