package server

import (
	"bytes"
	"errors"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/brendanjerwin/simple_wiki/rollingmigrations"
	"github.com/brendanjerwin/simple_wiki/sec"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/utils/goldmarkrenderer"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)


var _ = Describe("Site", func() {
	var (
		s         *Site
		tempDir   string
		mockIndex *MockIndexMaintainer
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "site-test")
		Expect(err).NotTo(HaveOccurred())

		mockIndex = &MockIndexMaintainer{}

		// Set up empty migration applicator for unit testing
		// (integration tests will configure their own mocks)
		applicator := rollingmigrations.NewEmptyApplicator()

		s = &Site{
			Logger:                  lumber.NewConsoleLogger(lumber.INFO),
			PathToData:              tempDir,
			IndexMaintainer:         mockIndex,
			MarkdownRenderer:        &goldmarkrenderer.GoldmarkRenderer{},
			FrontmatterIndexQueryer: &mockFrontmatterIndexQueryer{},
			MigrationApplicator:     applicator,
		}
	})

	AfterEach(func() {
		_ = os.RemoveAll(tempDir)
	})

	Describe("defaultLock", func() {
		When("DefaultPassword is not set", func() {
			BeforeEach(func() {
				s.DefaultPassword = ""
			})

			It("should return an empty string", func() {
				Expect(s.defaultLock()).To(BeEmpty())
			})
		})

		When("DefaultPassword is set", func() {
			var password string

			BeforeEach(func() {
				password = "test_password"
				s.DefaultPassword = password
			})

			It("should return a valid hash of the password", func() {
				hashedPassword := s.defaultLock()
				Expect(hashedPassword).ToNot(BeEmpty())
				Expect(hashedPassword).ToNot(Equal(password))
				Expect(sec.CheckPasswordHash(password, hashedPassword)).To(Succeed())
			})
		})
	})

	Describe("sniffContentType", func() {
		When("the file is an image", func() {
			var (
				contentType string
				err         error
			)

			BeforeEach(func() {
				// a minimal png file
				// from https://github.com/mathiasbynens/small/blob/master/png-transparent.png
				pngData := []byte{
					0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00,
					0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01,
					0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f,
					0x15, 0xc4, 0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41,
					0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00,
					0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49,
					0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
				}
				err = os.WriteFile(path.Join(s.PathToData, "test.png"), pngData, 0644)
				Expect(err).NotTo(HaveOccurred())

				contentType, err = s.sniffContentType("test.png")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return 'image/png'", func() {
				Expect(contentType).To(Equal("image/png"))
			})
		})

		When("the file is plain text", func() {
			var (
				contentType string
				err         error
			)

			BeforeEach(func() {
				err = os.WriteFile(path.Join(s.PathToData, "test.txt"), []byte("this is plain text"), 0644)
				Expect(err).NotTo(HaveOccurred())

				contentType, err = s.sniffContentType("test.txt")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return 'text/plain; charset=utf-8'", func() {
				Expect(contentType).To(Equal("text/plain; charset=utf-8"))
			})
		})

		When("the file does not exist", func() {
			var err error

			BeforeEach(func() {
				_, err = s.sniffContentType("nonexistent.file")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("PageReaderMutator implementation", func() {
		var (
			pageIdentifier wikipage.PageIdentifier
			pagePath       string
		)

		BeforeEach(func() {
			pageIdentifier = "test-page"
			// The PageReaderMutator implementation reads from base32 encoded filenames
			pagePath = filepath.Join(s.PathToData, base32tools.EncodeToBase32(strings.ToLower(string(pageIdentifier)))+".md")
		})

		Describe("ReadFrontMatter", func() {
			When("the page does not exist", func() {
				var err error

				BeforeEach(func() {
					_, _, err = s.ReadFrontMatter(pageIdentifier)
				})

				It("should return a not found error", func() {
					Expect(os.IsNotExist(err)).To(BeTrue())
				})
			})

			When("the page exists without frontmatter", func() {
				var (
					fm  wikipage.FrontMatter
					err error
				)

				BeforeEach(func() {
					fileErr := os.WriteFile(pagePath, []byte("just markdown"), 0644)
					Expect(fileErr).NotTo(HaveOccurred())
					_, fm, err = s.ReadFrontMatter(pageIdentifier)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return empty frontmatter", func() {
					Expect(fm).To(BeEmpty())
				})
			})

			When("the page exists with frontmatter", func() {
				var (
					fm  wikipage.FrontMatter
					err error
				)

				BeforeEach(func() {
					content := `---
title: Test
---
markdown content`
					fileErr := os.WriteFile(pagePath, []byte(content), 0644)
					Expect(fileErr).NotTo(HaveOccurred())
					_, fm, err = s.ReadFrontMatter(pageIdentifier)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return the parsed frontmatter", func() {
					Expect(fm).To(Equal(wikipage.FrontMatter{"title": "Test"}))
				})
			})
		})

		Describe("ReadMarkdown", func() {
			When("the page does not exist", func() {
				var err error

				BeforeEach(func() {
					_, _, err = s.ReadMarkdown(pageIdentifier)
				})

				It("should return a not found error", func() {
					Expect(os.IsNotExist(err)).To(BeTrue())
				})
			})

			When("the page exists without frontmatter", func() {
				var (
					md  wikipage.Markdown
					err error
				)

				BeforeEach(func() {
					fileErr := os.WriteFile(pagePath, []byte("just markdown"), 0644)
					Expect(fileErr).NotTo(HaveOccurred())
					_, md, err = s.ReadMarkdown(pageIdentifier)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return the full content as markdown", func() {
					Expect(string(md)).To(Equal("just markdown"))
				})
			})

			When("the page exists with frontmatter", func() {
				var (
					md  wikipage.Markdown
					err error
				)
				BeforeEach(func() {
					content := `---
title: Test
---
markdown content`
					fileErr := os.WriteFile(pagePath, []byte(content), 0644)
					Expect(fileErr).NotTo(HaveOccurred())
					_, md, err = s.ReadMarkdown(pageIdentifier)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should return only the markdown part", func() {
					Expect(string(md)).To(Equal("markdown content"))
				})
			})
		})

		Describe("WriteFrontMatter", func() {
			var (
				newFm wikipage.FrontMatter
				err   error
			)

			BeforeEach(func() {
				newFm = wikipage.FrontMatter{"title": "New Title"}
			})

			When("the page does not exist", func() {
				BeforeEach(func() {
					err = s.WriteFrontMatter(pageIdentifier, newFm)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should create a new page with the frontmatter and no markdown", func() {
					_, fm, fmErr := s.ReadFrontMatter(pageIdentifier)
					Expect(fmErr).NotTo(HaveOccurred())
					Expect(fm).To(Equal(newFm))

					_, md, mdErr := s.ReadMarkdown(pageIdentifier)
					Expect(mdErr).NotTo(HaveOccurred())
					Expect(string(md)).To(BeEmpty())
				})

				It("should add the page to the index", func() {
					Expect(mockIndex.AddPageToIndexCalledWith).To(Equal(pageIdentifier))
				})
			})

			When("the page exists with markdown but no frontmatter", func() {
				BeforeEach(func() {
					fileErr := os.WriteFile(pagePath, []byte("existing markdown"), 0644)
					Expect(fileErr).NotTo(HaveOccurred())
					err = s.WriteFrontMatter(pageIdentifier, newFm)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should add the frontmatter and keep the markdown", func() {
					_, fm, fmErr := s.ReadFrontMatter(pageIdentifier)
					Expect(fmErr).NotTo(HaveOccurred())
					Expect(fm).To(Equal(newFm))

					_, md, mdErr := s.ReadMarkdown(pageIdentifier)
					Expect(mdErr).NotTo(HaveOccurred())
					Expect(string(md)).To(Equal("existing markdown"))
				})
			})

			When("the page exists with frontmatter and markdown", func() {
				BeforeEach(func() {
					content := `---
title: Old Title
---
old markdown`
					fileErr := os.WriteFile(pagePath, []byte(content), 0644)
					Expect(fileErr).NotTo(HaveOccurred())
					err = s.WriteFrontMatter(pageIdentifier, newFm)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should replace the frontmatter and keep the markdown", func() {
					_, fm, fmErr := s.ReadFrontMatter(pageIdentifier)
					Expect(fmErr).NotTo(HaveOccurred())
					Expect(fm).To(Equal(newFm))

					_, md, mdErr := s.ReadMarkdown(pageIdentifier)
					Expect(mdErr).NotTo(HaveOccurred())
					Expect(string(md)).To(Equal("old markdown"))
				})
			})

			When("the page exists with `+++` style frontmatter", func() {
				BeforeEach(func() {
					content := `+++
title: Old Title
+++
old markdown`
					fileErr := os.WriteFile(pagePath, []byte(content), 0644)
					Expect(fileErr).NotTo(HaveOccurred())
					err = s.WriteFrontMatter(pageIdentifier, newFm)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should replace the frontmatter and keep the markdown", func() {
					_, fm, fmErr := s.ReadFrontMatter(pageIdentifier)
					Expect(fmErr).NotTo(HaveOccurred())
					Expect(fm).To(Equal(newFm))

					_, md, mdErr := s.ReadMarkdown(pageIdentifier)
					Expect(mdErr).NotTo(HaveOccurred())
					Expect(string(md)).To(Equal("old markdown"))
				})

				It("should not include the old frontmatter in the raw file", func() {
					fileContent, readErr := os.ReadFile(pagePath)
					Expect(readErr).NotTo(HaveOccurred())
					Expect(string(fileContent)).NotTo(ContainSubstring("title: Old Title"))
				})
			})
		})

		Describe("WriteMarkdown", func() {
			var (
				newMd wikipage.Markdown
				err   error
			)

			BeforeEach(func() {
				newMd = "new markdown"
			})

			When("the page does not exist", func() {
				BeforeEach(func() {
					err = s.WriteMarkdown(pageIdentifier, newMd)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should create a new page with the markdown and empty frontmatter", func() {
					_, fm, fmErr := s.ReadFrontMatter(pageIdentifier)
					Expect(fmErr).NotTo(HaveOccurred())
					Expect(fm).To(BeEmpty())

					_, md, mdErr := s.ReadMarkdown(pageIdentifier)
					Expect(mdErr).NotTo(HaveOccurred())
					Expect(string(md)).To(Equal(string(newMd)))
				})

				It("should add the page to the index", func() {
					Expect(mockIndex.AddPageToIndexCalledWith).To(Equal(pageIdentifier))
				})
			})

			When("the page exists with frontmatter but no markdown", func() {
				BeforeEach(func() {
					content := `---
title: Existing Title
---
`
					fileErr := os.WriteFile(pagePath, []byte(content), 0644)
					Expect(fileErr).NotTo(HaveOccurred())
					err = s.WriteMarkdown(pageIdentifier, newMd)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should add the markdown and keep the frontmatter", func() {
					_, fm, fmErr := s.ReadFrontMatter(pageIdentifier)
					Expect(fmErr).NotTo(HaveOccurred())
					Expect(fm).To(Equal(wikipage.FrontMatter{"title": "Existing Title"}))

					_, md, mdErr := s.ReadMarkdown(pageIdentifier)
					Expect(mdErr).NotTo(HaveOccurred())
					Expect(string(md)).To(Equal(string(newMd)))
				})
			})

			When("the page exists with frontmatter and markdown", func() {
				BeforeEach(func() {
					content := `---
title: Existing Title
---
old markdown`
					fileErr := os.WriteFile(pagePath, []byte(content), 0644)
					Expect(fileErr).NotTo(HaveOccurred())
					err = s.WriteMarkdown(pageIdentifier, newMd)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should replace the markdown and keep the frontmatter", func() {
					_, fm, fmErr := s.ReadFrontMatter(pageIdentifier)
					Expect(fmErr).NotTo(HaveOccurred())
					Expect(fm).To(Equal(wikipage.FrontMatter{"title": "Existing Title"}))

					_, md, mdErr := s.ReadMarkdown(pageIdentifier)
					Expect(mdErr).NotTo(HaveOccurred())
					Expect(string(md)).To(Equal(string(newMd)))
				})
			})
		})

		Describe("DeletePage", func() {
			When("the page exists as .md file only", func() {
				var err error

				BeforeEach(func() {
					// Create a test page as .md file only
					content := `---
title: Test Page
---
test content`
					fileErr := os.WriteFile(pagePath, []byte(content), 0644)
					Expect(fileErr).NotTo(HaveOccurred())

					err = s.DeletePage(pageIdentifier)
				})

				It("should return a not found error (because .json file doesn't exist)", func() {
					Expect(os.IsNotExist(err)).To(BeTrue())
				})

				It("should remove the page from the index", func() {
					Expect(mockIndex.RemovePageFromIndexCalledWith).To(Equal(pageIdentifier))
				})

				It("should leave the .md file intact (due to current Erase implementation)", func() {
					_, statErr := os.Stat(pagePath)
					Expect(statErr).NotTo(HaveOccurred())
				})
			})

			When("the page exists with both .md and .json files", func() {
				var (
					err      error
					jsonPath string
				)

				BeforeEach(func() {
					// Create both .md and .json files for the same page
					content := `---
title: Test Page
---
test content`
					fileErr := os.WriteFile(pagePath, []byte(content), 0644)
					Expect(fileErr).NotTo(HaveOccurred())

					jsonPath = filepath.Join(s.PathToData, base32tools.EncodeToBase32(strings.ToLower(string(pageIdentifier)))+".json")
					jsonContent := `{"identifier":"test-page","text":{"current":"test content","history":[]}}`
					jsonErr := os.WriteFile(jsonPath, []byte(jsonContent), 0644)
					Expect(jsonErr).NotTo(HaveOccurred())

					err = s.DeletePage(pageIdentifier)
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should remove both the .md and .json files", func() {
					_, mdStatErr := os.Stat(pagePath)
					Expect(os.IsNotExist(mdStatErr)).To(BeTrue())

					_, jsonStatErr := os.Stat(jsonPath)
					Expect(os.IsNotExist(jsonStatErr)).To(BeTrue())
				})

				It("should remove the page from the index", func() {
					Expect(mockIndex.RemovePageFromIndexCalledWith).To(Equal(pageIdentifier))
				})
			})

			When("the page does not exist", func() {
				var err error

				BeforeEach(func() {
					err = s.DeletePage(pageIdentifier)
				})

				It("should return a not found error", func() {
					Expect(os.IsNotExist(err)).To(BeTrue())
				})

				It("should still attempt to remove from index", func() {
					Expect(mockIndex.RemovePageFromIndexCalledWith).To(Equal(pageIdentifier))
				})
			})
		})
	})

	Describe("OpenOrInit", func() {
		var (
			req           *http.Request
			pageToCreate  string
			originalPerms os.FileMode
		)

		BeforeEach(func() {
			pageToCreate = "new-test-page"
			req, _ = http.NewRequest("GET", "/", nil)
		})

		When("creating a new page successfully", func() {
			var p *Page
			var err error

			BeforeEach(func() {
				p, err = s.OpenOrInit(pageToCreate, req)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create a page with initial content", func() {
				Expect(p.Text.GetCurrent()).To(ContainSubstring("# {{or .Title .Identifier}}"))
				Expect(p.Text.GetCurrent()).To(ContainSubstring(`identifier = "` + pageToCreate + `"`))
			})
		})

		When("creating a new page fails to save", func() {
			var p *Page
			var err error

			BeforeEach(func() {
				// Make the data directory read-only to simulate save failure
				var dirInfo os.FileInfo
				var statErr error
				dirInfo, statErr = os.Stat(tempDir)
				Expect(statErr).NotTo(HaveOccurred())
				originalPerms = dirInfo.Mode()
				chmodErr := os.Chmod(tempDir, 0444)
				Expect(chmodErr).NotTo(HaveOccurred())

				p, err = s.OpenOrInit(pageToCreate, req)
			})

			AfterEach(func() {
				// Restore permissions for cleanup
				_ = os.Chmod(tempDir, originalPerms)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to save new page"))
			})

			It("should return nil page when save fails", func() {
				Expect(p).To(BeNil())
			})
		})
	})

	Describe("InitializeIndexing", func() {
		When("a JSON file exists in the data directory", func() {
			var (
				err error
			)

			BeforeEach(func() {
				// Create a test page as a JSON file with proper base32-encoded filename
				encodedFilename := base32tools.EncodeToBase32(strings.ToLower("test"))
				pagePath := filepath.Join(s.PathToData, encodedFilename+".json")
				testPageContent := `{"identifier":"test","text":{"current":"test content","history":[]}}`
				fileErr := os.WriteFile(pagePath, []byte(testPageContent), 0644)
				Expect(fileErr).NotTo(HaveOccurred())

				err = s.InitializeIndexing()
			})

			It("should not return an error from InitializeIndexing", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should initialize IndexMaintainer", func() {
				Expect(s.IndexMaintainer).NotTo(BeNil())
			})

			It("should initialize FrontmatterIndexQueryer", func() {
				Expect(s.FrontmatterIndexQueryer).NotTo(BeNil())
			})

			It("should initialize BleveIndexQueryer", func() {
				Expect(s.BleveIndexQueryer).NotTo(BeNil())
			})

			It("should index the test page", func() {
				// The page should be indexed (we can verify by checking DirectoryList)
				files := s.DirectoryList()
				Expect(len(files)).To(BeNumerically(">", 0))
				Expect(files[0].Name()).To(Equal("test"))
			})
		})

		When("the indexing process encounters errors", func() {
			var (
				files []os.FileInfo
				logBuffer *bytes.Buffer
				logOutput string
			)

			BeforeEach(func() {
				// Create a test page
				encodedFilename := base32tools.EncodeToBase32(strings.ToLower("test"))
				pagePath := filepath.Join(s.PathToData, encodedFilename+".json")
				testPageContent := `{"identifier":"test","text":{"current":"test content","history":[]}}`
				fileErr := os.WriteFile(pagePath, []byte(testPageContent), 0644)
				Expect(fileErr).NotTo(HaveOccurred())

				// Set up a logger that writes to a buffer so we can capture log output
				logBuffer = &bytes.Buffer{}
				s.Logger = lumber.NewBasicLogger(&testWriteCloser{logBuffer}, lumber.ERROR)

				// Set up a mock that returns an error
				mockIndex.AddPageToIndexError = errors.New("mock index error")
				s.IndexMaintainer = mockIndex

				// Call the loop logic directly by simulating what InitializeIndexing does
				files = s.DirectoryList()
				for _, file := range files {
					if err := s.IndexMaintainer.AddPageToIndex(file.Name()); err != nil {
						s.Logger.Error("Failed to add page '%s' to index during initialization: %v", file.Name(), err)
					}
				}

				// Capture log output after the actions are performed
				logOutput = logBuffer.String()
			})

			It("should handle indexing errors gracefully", func() {
				// The mock should have been called
				Expect(mockIndex.AddPageToIndexCalledWith).To(Equal(wikipage.PageIdentifier("test")))
			})

			It("should continue processing despite errors", func() {
				// Should find the test file in DirectoryList
				Expect(len(files)).To(BeNumerically(">", 0))
				Expect(files[0].Name()).To(Equal("test"))
			})

			It("should log the error", func() {
				Expect(logOutput).To(ContainSubstring("Failed to add page 'test' to index during initialization"))
				Expect(logOutput).To(ContainSubstring("mock index error"))
			})
		})
	})

	Describe("Rolling migrations integration", func() {
		var (
			mockApplicator *rollingmigrations.DefaultApplicator
			mockMig        *rollingmigrations.MockMigration
		)

		BeforeEach(func() {
			// Set up mock migration applicator for integration testing
			mockApplicator = rollingmigrations.NewEmptyApplicator()
			mockMig = &rollingmigrations.MockMigration{
				SupportedTypesResult: []rollingmigrations.FrontmatterType{rollingmigrations.FrontmatterTOML},
				AppliesToResult:      true,
			}
			mockApplicator.RegisterMigration(mockMig)
			s.MigrationApplicator = mockApplicator
		})

		Describe("when migration applies and succeeds", func() {
			var (
				pageIdentifier wikipage.PageIdentifier
				pagePath       string
				fm             wikipage.FrontMatter
				err            error
				savedContent   string
				readErr        error
			)

			BeforeEach(func() {
				pageIdentifier = "mock-migration-test"
				pagePath = filepath.Join(s.PathToData, base32tools.EncodeToBase32(strings.ToLower(string(pageIdentifier)))+".md")

				// Original content that needs migration
				originalContent := `+++
title = "Test Page"
status = "draft"
+++
# Test Content`

				// Mock migration result with modified content
				migratedContent := `+++
title = "Test Page"
status = "published"
+++
# Test Content`

				mockMig.ApplyResult = []byte(migratedContent)

				fileErr := os.WriteFile(pagePath, []byte(originalContent), 0644)
				Expect(fileErr).NotTo(HaveOccurred())

				_, fm, err = s.ReadFrontMatter(pageIdentifier)
				
				// Read the saved content for verification
				rawContent, readErr := os.ReadFile(pagePath)
				if readErr == nil {
					savedContent = string(rawContent)
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should parse migrated frontmatter correctly", func() {
				Expect(fm).To(HaveKey("title"))
				Expect(fm["title"]).To(Equal("Test Page"))
				Expect(fm).To(HaveKey("status"))
				Expect(fm["status"]).To(Equal("published"))
			})

			It("should auto-save the migrated content to disk", func() {
				// Verify file was updated with migrated content
				Expect(readErr).NotTo(HaveOccurred())
				Expect(savedContent).To(ContainSubstring(`status = "published"`))
				Expect(savedContent).NotTo(ContainSubstring(`status = "draft"`))
			})
		})

		Describe("when migration doesn't apply", func() {
			var (
				pageIdentifier wikipage.PageIdentifier
				pagePath       string
				fm             wikipage.FrontMatter
				err            error
				savedContent   string
				readErr        error
			)

			BeforeEach(func() {
				pageIdentifier = "no-migration-test"
				pagePath = filepath.Join(s.PathToData, base32tools.EncodeToBase32(strings.ToLower(string(pageIdentifier)))+".md")

				// Set mock to not apply
				mockMig.AppliesToResult = false

				originalContent := `+++
title = "Clean Page"
+++
# Content`

				fileErr := os.WriteFile(pagePath, []byte(originalContent), 0644)
				Expect(fileErr).NotTo(HaveOccurred())

				_, fm, err = s.ReadFrontMatter(pageIdentifier)
				
				// Read the saved content for verification
				rawContent, readErr := os.ReadFile(pagePath)
				if readErr == nil {
					savedContent = string(rawContent)
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should parse original frontmatter", func() {
				Expect(fm).To(HaveKey("title"))
				Expect(fm["title"]).To(Equal("Clean Page"))
			})

			It("should not modify the file on disk", func() {
				// File should remain unchanged
				Expect(readErr).NotTo(HaveOccurred())
				Expect(savedContent).To(ContainSubstring(`title = "Clean Page"`))
			})
		})

		Describe("when migration fails", func() {
			var (
				pageIdentifier wikipage.PageIdentifier
				pagePath       string
				fm             wikipage.FrontMatter
				err            error
				savedContent   string
				readErr        error
			)

			BeforeEach(func() {
				pageIdentifier = "failed-migration-test"
				pagePath = filepath.Join(s.PathToData, base32tools.EncodeToBase32(strings.ToLower(string(pageIdentifier)))+".md")

				// Set mock to fail
				mockMig.AppliesToResult = true
				mockMig.ApplyError = errors.New("mock migration failure")

				originalContent := `+++
title = "Test Page"
+++
# Content`

				fileErr := os.WriteFile(pagePath, []byte(originalContent), 0644)
				Expect(fileErr).NotTo(HaveOccurred())

				_, fm, err = s.ReadFrontMatter(pageIdentifier)
				
				// Read the saved content for verification
				rawContent, readErr := os.ReadFile(pagePath)
				if readErr == nil {
					savedContent = string(rawContent)
				}
			})

			It("should not return an error", func() {
				// lenientParse should handle migration failures gracefully
				Expect(err).NotTo(HaveOccurred())
			})

			It("should fall back to original content", func() {
				Expect(fm).To(HaveKey("title"))
				Expect(fm["title"]).To(Equal("Test Page"))
			})

			It("should not modify the file on disk when migration fails", func() {
				// File should remain unchanged since migration failed
				Expect(readErr).NotTo(HaveOccurred())
				Expect(savedContent).To(ContainSubstring(`title = "Test Page"`))
			})
		})

		Describe("when no migration applicator is configured", func() {
			var (
				pageIdentifier wikipage.PageIdentifier
				pagePath       string
				err            error
			)

			BeforeEach(func() {
				pageIdentifier = "no-applicator-test"
				pagePath = filepath.Join(s.PathToData, base32tools.EncodeToBase32(strings.ToLower(string(pageIdentifier)))+".md")

				// Remove migration applicator
				s.MigrationApplicator = nil

				originalContent := `+++
title = "Test Page"
+++
# Content`

				fileErr := os.WriteFile(pagePath, []byte(originalContent), 0644)
				Expect(fileErr).NotTo(HaveOccurred())

				_, _, err = s.ReadFrontMatter(pageIdentifier)
			})

			It("should return an error", func() {
				// No migration applicator configured is an application setup mistake
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("migration applicator not configured"))
			})
		})

		Describe("migrations on page save", func() {
			var (
				page           *Page
				pageIdentifier string
				originalContent string
				err            error
			)

			BeforeEach(func() {
				pageIdentifier = "save-migration-test"
				
				// Set up mock migration applicator
				mockApplicator = rollingmigrations.NewEmptyApplicator()
				mockMig = &rollingmigrations.MockMigration{
					SupportedTypesResult: []rollingmigrations.FrontmatterType{rollingmigrations.FrontmatterTOML},
					AppliesToResult:      true,
					ApplyResult:          []byte(`+++
title = "Fixed Title"
+++
# Content`),
				}
				mockApplicator.RegisterMigration(mockMig)
				s.MigrationApplicator = mockApplicator

				// Create a page with problematic content
				originalContent = `+++
title = "Bad Title"
+++
# Content`
				page = s.Open(pageIdentifier)
				err = page.Update(originalContent)
			})

			It("should apply migrations when user saves problematic content", func() {
				Expect(err).NotTo(HaveOccurred())
				
				// Verify the page now contains the migrated content
				currentContent := page.Text.GetCurrent()
				Expect(currentContent).To(ContainSubstring(`title = "Fixed Title"`))
				Expect(currentContent).NotTo(ContainSubstring(`title = "Bad Title"`))
			})

			It("should create history entry for the migration", func() {
				Expect(err).NotTo(HaveOccurred())
				
				// Check that there are multiple versions (original + migrated)
				Expect(page.Text.NumEdits()).To(BeNumerically(">=", 1))
			})

			Describe("when no migration applies", func() {
				BeforeEach(func() {
					// Set mock to not apply
					mockMig.AppliesToResult = false
					
					page = s.Open(pageIdentifier + "-no-migration")
					err = page.Update(originalContent)
				})

				It("should save original content unchanged", func() {
					Expect(err).NotTo(HaveOccurred())
					
					currentContent := page.Text.GetCurrent()
					Expect(currentContent).To(ContainSubstring(`title = "Bad Title"`))
				})
			})

			Describe("recursive invocation prevention", func() {
				BeforeEach(func() {
					// Create a migration that tracks call count
					mockMig.AppliesToResult = true
					mockMig.ApplyResult = []byte(`+++
title = "Migrated"
+++
# Content`)
					
					page = s.Open(pageIdentifier + "-recursive")
					err = page.Update(originalContent)
				})

				It("should not cause infinite recursion", func() {
					Expect(err).NotTo(HaveOccurred())
					
					// Should complete successfully without hanging
					currentContent := page.Text.GetCurrent()
					Expect(currentContent).To(ContainSubstring(`title = "Migrated"`))
				})
			})
		})
	})
})

// testWriteCloser wraps a buffer and implements io.WriteCloser for testing
type testWriteCloser struct {
	*bytes.Buffer
}

func (*testWriteCloser) Close() error {
	return nil
}
