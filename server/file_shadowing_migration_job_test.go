//revive:disable:dot-imports
package server

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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
		}
		
		job = NewFileShadowingMigrationJob(site, "")
	})

	AfterEach(func() {
		os.RemoveAll(testDataDir)
	})

	Describe("Execute", func() {
		When("PascalCase page exists with shadowing conflict", func() {
			BeforeEach(func() {
				// Create PascalCase page using Site.Open() - this will store as base32-encoded files
				labPage := site.Open("LabInventory")
				labPage.Text = versionedtext.NewVersionedText("# Rich Pascal Lab Inventory\n\nThis has detailed content with multiple sections.\n\n## Equipment\n- Microscope\n- Centrifuge")
				err := labPage.Save()
				Expect(err).NotTo(HaveOccurred())
				
				// Create existing munged page with poor content using Site.Open()
				mungedPage := site.Open("lab_inventory")
				mungedPage.Text = versionedtext.NewVersionedText("# Poor Munged Lab")
				err = mungedPage.Save()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should consolidate to munged name with richer content", func() {
				job.logicalPageID = "LabInventory"
				err := job.Execute()
				Expect(err).NotTo(HaveOccurred())
				
				// Verify munged page now has the richer content from PascalCase page
				mungedPage := site.Open("lab_inventory")
				Expect(mungedPage.IsNew()).To(BeFalse())
				content := mungedPage.Text.GetCurrent()
				Expect(content).To(ContainSubstring("Rich Pascal Lab Inventory"))
				Expect(content).To(ContainSubstring("detailed content with multiple sections"))
				Expect(content).To(ContainSubstring("Microscope"))
				
				// Verify original PascalCase base32-encoded files are removed
				pascalJSONPath := filepath.Join(testDataDir, base32tools.EncodeToBase32("labinventory")+".json")
				pascalMdPath := filepath.Join(testDataDir, base32tools.EncodeToBase32("labinventory")+".md")
				_, err = os.Stat(pascalJSONPath)
				Expect(os.IsNotExist(err)).To(BeTrue())
				_, err = os.Stat(pascalMdPath)
				Expect(os.IsNotExist(err)).To(BeTrue())
			})
		})

		When("PascalCase page exists without shadowing conflict", func() {
			BeforeEach(func() {
				// Create PascalCase page using Site.Open() - this will store as base32-encoded files
				userPage := site.Open("UserGuide")
				userPage.Text = versionedtext.NewVersionedText("# User Guide Content\n\nDetailed guide here.")
				err := userPage.Save()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create munged page from PascalCase content", func() {
				job.logicalPageID = "UserGuide"
				err := job.Execute()
				Expect(err).NotTo(HaveOccurred())
				
				// Verify munged page is created with PascalCase content
				mungedPage := site.Open("user_guide")
				Expect(mungedPage.IsNew()).To(BeFalse())
				content := mungedPage.Text.GetCurrent()
				Expect(content).To(ContainSubstring("User Guide Content"))
				Expect(content).To(ContainSubstring("Detailed guide here"))
				
				// Verify original PascalCase base32-encoded files are removed
				pascalJSONPath := filepath.Join(testDataDir, base32tools.EncodeToBase32("userguide")+".json")
				pascalMdPath := filepath.Join(testDataDir, base32tools.EncodeToBase32("userguide")+".md")
				_, err = os.Stat(pascalJSONPath)
				Expect(os.IsNotExist(err)).To(BeTrue())
				_, err = os.Stat(pascalMdPath)
				Expect(os.IsNotExist(err)).To(BeTrue())
			})
		})

		When("munged page has richer content than PascalCase page", func() {
			BeforeEach(func() {
				// Create PascalCase page with basic content using Site.Open()
				labPage := site.Open("LabInventory")
				labPage.Text = versionedtext.NewVersionedText("# Basic Lab")
				err := labPage.Save()
				Expect(err).NotTo(HaveOccurred())
				
				// Create munged page with much richer content using Site.Open()
				mungedPage := site.Open("lab_inventory")
				mungedPage.Text = versionedtext.NewVersionedText("# Rich Munged Lab Inventory\n\nThis munged version has extensive content:\n\n## Equipment List\n- Advanced Microscope\n- High-speed Centrifuge\n- Spectrophotometer\n\n## Procedures\nDetailed procedures here...")
				err = mungedPage.Save()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should keep the richer munged content and remove PascalCase page files", func() {
				job.logicalPageID = "LabInventory"
				err := job.Execute()
				Expect(err).NotTo(HaveOccurred())
				
				// Verify munged page retains its richer content
				mungedPage := site.Open("lab_inventory")
				Expect(mungedPage.IsNew()).To(BeFalse())
				content := mungedPage.Text.GetCurrent()
				Expect(content).To(ContainSubstring("Rich Munged Lab Inventory"))
				Expect(content).To(ContainSubstring("extensive content"))
				Expect(content).To(ContainSubstring("Advanced Microscope"))
				Expect(content).To(ContainSubstring("Spectrophotometer"))
				
				// Verify original PascalCase base32-encoded files are removed
				pascalJSONPath := filepath.Join(testDataDir, base32tools.EncodeToBase32("labinventory")+".json")
				pascalMdPath := filepath.Join(testDataDir, base32tools.EncodeToBase32("labinventory")+".md")
				_, err = os.Stat(pascalJSONPath)
				Expect(os.IsNotExist(err)).To(BeTrue())
				_, err = os.Stat(pascalMdPath)
				Expect(os.IsNotExist(err)).To(BeTrue())
			})
		})

		When("PascalCase page exists without markdown file", func() {
			BeforeEach(func() {
				// Create PascalCase page using Site.Open() and save it
				devicePage := site.Open("DeviceManual")
				devicePage.Text = versionedtext.NewVersionedText("# Device Manual\n\nOperating instructions.")
				err := devicePage.Save()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create munged page from PascalCase content", func() {
				job.logicalPageID = "DeviceManual"
				err := job.Execute()
				Expect(err).NotTo(HaveOccurred())
				
				// Verify munged page is created with PascalCase content
				mungedPage := site.Open("device_manual")
				Expect(mungedPage.IsNew()).To(BeFalse())
				content := mungedPage.Text.GetCurrent()
				Expect(content).To(ContainSubstring("Device Manual"))
				Expect(content).To(ContainSubstring("Operating instructions"))
				
				// Verify original PascalCase base32-encoded files are removed
				pascalJSONPath := filepath.Join(testDataDir, base32tools.EncodeToBase32("devicemanual")+".json")
				pascalMdPath := filepath.Join(testDataDir, base32tools.EncodeToBase32("devicemanual")+".md")
				_, err = os.Stat(pascalJSONPath)
				Expect(os.IsNotExist(err)).To(BeTrue())
				_, err = os.Stat(pascalMdPath)
				Expect(os.IsNotExist(err)).To(BeTrue())
			})
		})

		When("PascalCase page does not exist", func() {
			It("should return an error", func() {
				job.logicalPageID = "NonExistentPage"
				err := job.Execute()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no page found for PascalCase identifier"))
			})
		})
	})

	Describe("CheckForShadowing", func() {
		When("munged version exists", func() {
			BeforeEach(func() {
				// Create munged page using Site.Open() - this will store as base32-encoded files
				mungedPage := site.Open("lab_inventory")
				mungedPage.Text = versionedtext.NewVersionedText("# Munged Version")
				err := mungedPage.Save()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should detect shadowing conflict", func() {
				hasShadowing, mungedFiles := job.CheckForShadowing("LabInventory")
				
				Expect(hasShadowing).To(BeTrue())
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
			It("should not detect shadowing", func() {
				hasShadowing, mungedFiles := job.CheckForShadowing("UserGuide")
				
				Expect(hasShadowing).To(BeFalse())
				Expect(mungedFiles).To(BeEmpty())
			})
		})
	})
})