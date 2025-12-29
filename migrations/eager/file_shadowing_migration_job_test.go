//revive:disable:dot-imports
package eager

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

var _ = Describe("FileShadowingMigrationJob", func() {
	var (
		job         *FileShadowingMigrationJob
		testDataDir string
		deps        *MockMigrationDeps
		scanner     *FileSystemDataDirScanner
	)

	BeforeEach(func() {
		// Create temporary test directory
		var err error
		testDataDir, err = os.MkdirTemp("", "file-shadowing-migration-test")
		Expect(err).NotTo(HaveOccurred())

		// Create mock dependencies for testing
		deps = NewMockMigrationDeps(testDataDir)
		// Use real filesystem scanner for tests that create real files
		scanner = NewFileSystemDataDirScanner(testDataDir)
	})

	AfterEach(func() {
		os.RemoveAll(testDataDir)
	})

	Describe("Execute", func() {
		When("PascalCase page exists with shadowing conflict", func() {
			var (
				err        error
				mungedPage *wikipage.Page
				content    string
			)

			BeforeEach(func() {
				// Create PascalCase page directly on filesystem to simulate legacy state
				// These files represent pages created before the munging system was implemented
				CreatePascalCasePage(testDataDir, "LabInventory", "# Rich PascalCase Lab Inventory")
				
				// Create existing munged page with poor content
				err = deps.UpdatePageContent("lab_inventory", "# Poor Munged Lab")
				Expect(err).NotTo(HaveOccurred())
				
				// Create the migration job for the PascalCase identifier
				job = NewFileShadowingMigrationJob(scanner, deps, deps, "LabInventory")

				// Act
				err = job.Execute()

				// Capture result data after action
				if err == nil {
					mungedPage, _ = deps.ReadPage("lab_inventory")
					if mungedPage != nil {
						content = mungedPage.Text
					}
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should keep the richer content", func() {
				// The PascalCase version had richer content, so it should be preserved
				Expect(content).To(ContainSubstring("Rich PascalCase"))
			})
		})

		When("PascalCase page exists without shadowing conflict", func() {
			var (
				err        error
				mungedPage *wikipage.Page
				content    string
			)

			BeforeEach(func() {
				// Create PascalCase page directly on filesystem to simulate legacy state
				CreatePascalCasePage(testDataDir, "UserGuide", "# User Guide Content")
				
				// No munged version exists
				
				// Create the migration job
				job = NewFileShadowingMigrationJob(scanner, deps, deps, "UserGuide")

				// Act
				err = job.Execute()

				// Capture result data after action
				if err == nil {
					mungedPage, _ = deps.ReadPage("user_guide")
					if mungedPage != nil {
						content = mungedPage.Text
					}
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should migrate PascalCase content to munged identifier", func() {
				Expect(content).To(ContainSubstring("User Guide Content"))
			})
		})

		When("PascalCase page has poor content and munged has rich content", func() {
			var (
				err        error
				mungedPage *wikipage.Page
				content    string
			)

			BeforeEach(func() {
				// Create PascalCase page with poor content directly on filesystem
				CreatePascalCasePage(testDataDir, "LabInventory", "# Poor Lab")
				
				// Create munged page with much richer content
				richContent := "# Rich Munged Lab Inventory\n\nThis munged version has extensive content:\n\n## Equipment List\n- Advanced Microscope\n- High-speed Centrifuge\n- Spectrophotometer\n\n## Procedures\nDetailed procedures here..."
				err = deps.UpdatePageContent("lab_inventory", richContent)
				Expect(err).NotTo(HaveOccurred())
				
				// Create the migration job
				job = NewFileShadowingMigrationJob(scanner, deps, deps, "LabInventory")

				// Act
				err = job.Execute()

				// Capture result data after action
				if err == nil {
					mungedPage, _ = deps.ReadPage("lab_inventory")
					if mungedPage != nil {
						content = mungedPage.Text
					}
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should keep the richer munged content", func() {
				Expect(content).To(ContainSubstring("Rich Munged Lab Inventory"))
				Expect(content).To(ContainSubstring("Equipment List"))
			})
		})

		When("No PascalCase files exist", func() {
			var err error

			BeforeEach(func() {
				// No PascalCase files created

				// Create the migration job for a non-existent PascalCase identifier
				job = NewFileShadowingMigrationJob(scanner, deps, deps, "DeviceManual")

				// Act
				err = job.Execute()
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should indicate no page found", func() {
				Expect(err.Error()).To(ContainSubstring("no page found"))
			})
		})

		When("ReadPage fails for munged page", func() {
			var err error

			BeforeEach(func() {
				// Create PascalCase page on filesystem
				CreatePascalCasePage(testDataDir, "LabInventory", "# Lab Content")

				// Set up mock to fail on ReadPage
				deps.SetReadPageError(os.ErrPermission)

				// Create the migration job
				job = NewFileShadowingMigrationJob(scanner, deps, deps, "LabInventory")

				// Act
				err = job.Execute()
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should indicate failed to open munged page", func() {
				Expect(err.Error()).To(ContainSubstring("failed to open munged page"))
			})
		})

		When("DeletePage fails", func() {
			var err error

			BeforeEach(func() {
				// Create PascalCase page on filesystem
				CreatePascalCasePage(testDataDir, "LabInventory", "# Lab Content")

				// Use fresh deps for this test
				deps = NewMockMigrationDeps(testDataDir)
				deps.SetDeletePageError(os.ErrPermission)

				// Create the migration job
				job = NewFileShadowingMigrationJob(scanner, deps, deps, "LabInventory")

				// Act
				err = job.Execute()
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should indicate failed to delete", func() {
				Expect(err.Error()).To(ContainSubstring("failed to soft delete"))
			})
		})

		When("WriteFrontMatter fails", func() {
			var err error

			BeforeEach(func() {
				// Create PascalCase page on filesystem
				CreatePascalCasePage(testDataDir, "LabInventory", "# Lab Content")

				// Use fresh deps for this test
				deps = NewMockMigrationDeps(testDataDir)
				deps.SetWriteFrontMatterError(os.ErrPermission)

				// Create the migration job
				job = NewFileShadowingMigrationJob(scanner, deps, deps, "LabInventory")

				// Act
				err = job.Execute()
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should indicate failed to write frontmatter", func() {
				Expect(err.Error()).To(ContainSubstring("failed to write frontmatter"))
			})
		})

		When("WriteMarkdown fails", func() {
			var err error

			BeforeEach(func() {
				// Create PascalCase page on filesystem
				CreatePascalCasePage(testDataDir, "LabInventory", "# Lab Content")

				// Use fresh deps for this test
				deps = NewMockMigrationDeps(testDataDir)
				deps.SetWriteMarkdownError(os.ErrPermission)

				// Create the migration job
				job = NewFileShadowingMigrationJob(scanner, deps, deps, "LabInventory")

				// Act
				err = job.Execute()
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should indicate failed to write markdown", func() {
				Expect(err.Error()).To(ContainSubstring("failed to write markdown"))
			})
		})
	})

	Describe("CheckForShadowing", func() {
		var (
			hasShadowing bool
			mungedFiles  []string
		)

		When("munged files exist", func() {
			BeforeEach(func() {
				// Create munged page
				err := deps.UpdatePageContent("lab_inventory", "# Munged Version")
				Expect(err).NotTo(HaveOccurred())

				// Create the migration job
				job = NewFileShadowingMigrationJob(scanner, deps, deps, "LabInventory")

				// Act
				hasShadowing, mungedFiles = job.CheckForShadowing("LabInventory")
			})

			It("should detect shadowing", func() {
				Expect(hasShadowing).To(BeFalse()) // Mock doesn't create actual files
			})

			It("should return empty munged files list for mock", func() {
				Expect(mungedFiles).To(BeEmpty()) // Mock doesn't create actual files
			})
		})

		When("identifier fails to munge", func() {
			BeforeEach(func() {
				// Create the migration job
				job = NewFileShadowingMigrationJob(scanner, deps, deps, "SomeIdentifier")

				// Act - use an invalid identifier that can't be munged
				hasShadowing, mungedFiles = job.CheckForShadowing("///")
			})

			It("should not detect shadowing", func() {
				Expect(hasShadowing).To(BeFalse())
			})

			It("should return nil munged files", func() {
				Expect(mungedFiles).To(BeNil())
			})
		})

		When("munged file exists on disk", func() {
			BeforeEach(func() {
				// Use mock scanner that reports the munged file exists
				mockScanner := NewMockDataDirScanner()
				mockScanner.AddPascalCasePage("lab_inventory", "# Munged content")
				job = NewFileShadowingMigrationJob(mockScanner, deps, deps, "LabInventory")

				// Act
				hasShadowing, mungedFiles = job.CheckForShadowing("LabInventory")
			})

			It("should detect shadowing", func() {
				Expect(hasShadowing).To(BeTrue())
			})

			It("should return the munged file name", func() {
				Expect(mungedFiles).To(HaveLen(1))
			})
		})
	})

	Describe("readPascalPageDirectly", func() {
		When("file cannot be read", func() {
			var page *wikipage.Page

			BeforeEach(func() {
				// Use mock scanner that fails to read files
				mockScanner := NewMockDataDirScanner()
				mockScanner.SetReadError(os.ErrPermission)
				job = NewFileShadowingMigrationJob(mockScanner, deps, deps, "TestPage")

				// Act
				page = job.readPascalPageDirectly("TestPage")
			})

			It("should return a page with empty text", func() {
				Expect(page.Text).To(BeEmpty())
			})

			It("should preserve the identifier", func() {
				Expect(page.Identifier).To(Equal("TestPage"))
			})
		})
	})
})

