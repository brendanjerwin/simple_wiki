package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/brendanjerwin/simple_wiki/index"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/migrations/lazy"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/utils/goldmarkrenderer"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// MockIndexOperator is a test implementation of index.IndexOperator.
type MockIndexOperator struct {
	AddPageToIndexFunc    func(identifier wikipage.PageIdentifier) error
	RemovePageFromIndexFunc func(identifier wikipage.PageIdentifier) error
	addCalled             []wikipage.PageIdentifier
	removeCalled          []wikipage.PageIdentifier
}

func (m *MockIndexOperator) AddPageToIndex(identifier wikipage.PageIdentifier) error {
	m.addCalled = append(m.addCalled, identifier)
	if m.AddPageToIndexFunc != nil {
		return m.AddPageToIndexFunc(identifier)
	}
	return nil
}

func (m *MockIndexOperator) RemovePageFromIndex(identifier wikipage.PageIdentifier) error {
	m.removeCalled = append(m.removeCalled, identifier)
	if m.RemovePageFromIndexFunc != nil {
		return m.RemovePageFromIndexFunc(identifier)
	}
	return nil
}

// LastAddPageCall returns the last identifier passed to AddPageToIndex
func (m *MockIndexOperator) LastAddPageCall() wikipage.PageIdentifier {
	if len(m.addCalled) == 0 {
		return ""
	}
	return m.addCalled[len(m.addCalled)-1]
}

// LastRemovePageCall returns the last identifier passed to RemovePageFromIndex
func (m *MockIndexOperator) LastRemovePageCall() wikipage.PageIdentifier {
	if len(m.removeCalled) == 0 {
		return ""
	}
	return m.removeCalled[len(m.removeCalled)-1]
}

var _ = Describe("Site", func() {
	var (
		s                *Site
		tempDir          string
		mockFrontmatter  *MockIndexOperator
		mockBleve        *MockIndexOperator
		coordinator      *jobs.JobQueueCoordinator
		indexCoordinator *index.IndexCoordinator
	)

	// Helper function to wait for indexing jobs to complete
	waitForIndexing := func() {
		if indexCoordinator != nil {
			completed, _ := indexCoordinator.WaitForCompletionWithTimeout(context.Background(), 1*time.Second)
			Expect(completed).To(BeTrue())
		}
	}

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "site-test")
		Expect(err).NotTo(HaveOccurred())

		mockFrontmatter = &MockIndexOperator{}
		mockBleve = &MockIndexOperator{}

		// Set up job queue coordinator and index coordinator
		logger := lumber.NewConsoleLogger(lumber.WARN) // Quiet logger for tests
		coordinator = jobs.NewJobQueueCoordinator(logger)
		indexCoordinator = index.NewIndexCoordinator(coordinator, mockFrontmatter, mockBleve)

		// Set up empty migration applicator for unit testing
		// (integration tests will configure their own mocks)
		applicator := lazy.NewEmptyApplicator()

		s = &Site{
			Logger:                  lumber.NewConsoleLogger(lumber.INFO),
			PathToData:              tempDir,
			IndexCoordinator:        indexCoordinator,
			MarkdownRenderer:        &goldmarkrenderer.GoldmarkRenderer{},
			FrontmatterIndexQueryer: &mockFrontmatterIndexQueryer{},
			MigrationApplicator:     applicator,
		}
	})

	AfterEach(func() {
		_ = os.RemoveAll(tempDir)
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
					waitForIndexing()
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
					Expect(mockFrontmatter.LastAddPageCall()).To(Equal(pageIdentifier))
					Expect(mockBleve.LastAddPageCall()).To(Equal(pageIdentifier))
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
					waitForIndexing()
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
					Expect(mockFrontmatter.LastAddPageCall()).To(Equal(pageIdentifier))
					Expect(mockBleve.LastAddPageCall()).To(Equal(pageIdentifier))
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
					waitForIndexing()
				})

				It("should successfully delete the .md file", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should remove the page from the index", func() {
					Expect(mockFrontmatter.LastRemovePageCall()).To(Equal(pageIdentifier))
					Expect(mockBleve.LastRemovePageCall()).To(Equal(pageIdentifier))
				})

				It("should remove the .md file completely", func() {
					_, statErr := os.Stat(pagePath)
					Expect(os.IsNotExist(statErr)).To(BeTrue())
				})
			})

			When("the page exists as .md file", func() {
				var err error

				BeforeEach(func() {
					// Create .md file for the page
					content := `---
title: Test Page
---
test content`
					fileErr := os.WriteFile(pagePath, []byte(content), 0644)
					Expect(fileErr).NotTo(HaveOccurred())

					err = s.DeletePage(pageIdentifier)
					waitForIndexing()
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should remove the .md file", func() {
					_, mdStatErr := os.Stat(pagePath)
					Expect(os.IsNotExist(mdStatErr)).To(BeTrue())
				})

				It("should remove the page from the index", func() {
					Expect(mockFrontmatter.LastRemovePageCall()).To(Equal(pageIdentifier))
					Expect(mockBleve.LastRemovePageCall()).To(Equal(pageIdentifier))
				})
			})

			When("the page does not exist", func() {
				var err error

				BeforeEach(func() {
					err = s.DeletePage(pageIdentifier)
					waitForIndexing()
				})

				It("should return a not found error", func() {
					Expect(os.IsNotExist(err)).To(BeTrue())
				})

				It("should still attempt to remove from index", func() {
					Expect(mockFrontmatter.LastRemovePageCall()).To(Equal(pageIdentifier))
					Expect(mockBleve.LastRemovePageCall()).To(Equal(pageIdentifier))
				})
			})

			When("the page exists and should be soft deleted", func() {
				var (
					err         error
					deletedDir  string
					currentTime int64
				)

				BeforeEach(func() {
					// Create .md file for the page
					content := `---
title: Test Page
---
test content to be soft deleted`
					fileErr := os.WriteFile(pagePath, []byte(content), 0644)
					Expect(fileErr).NotTo(HaveOccurred())

					// Capture current time before deletion
					currentTime = time.Now().Unix()

					err = s.DeletePage(pageIdentifier)
					waitForIndexing()

					// Calculate expected deleted directory path
					deletedDir = filepath.Join(s.PathToData, "__deleted__")
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				Context("file system changes", func() {
					var mdStatErr error
					var entries []os.DirEntry
					var timestampDir os.DirEntry
					var timestamp int64
					var timestampPath string
					var deletedMdPath string
					var deletedMdContent []byte

					BeforeEach(func() {
						// Check original file is gone
						_, mdStatErr = os.Stat(pagePath)

						// Read deleted directory structure
						entries, err = os.ReadDir(deletedDir)
						if len(entries) > 0 {
							timestampDir = entries[0]
							timestamp, _ = strconv.ParseInt(timestampDir.Name(), 10, 64)
							
							// Read the moved file
							timestampPath = filepath.Join(deletedDir, timestampDir.Name())
							deletedMdPath = filepath.Join(timestampPath, base32tools.EncodeToBase32(strings.ToLower(string(pageIdentifier)))+".md")
							deletedMdContent, _ = os.ReadFile(deletedMdPath)
						}
					})

					It("should remove files from original location", func() {
						Expect(os.IsNotExist(mdStatErr)).To(BeTrue())
					})

					It("should create deleted directory", func() {
						Expect(deletedDir).To(BeADirectory())
					})

					It("should create timestamped subdirectory", func() {
						Expect(entries).To(HaveLen(1))
						Expect(timestampDir.IsDir()).To(BeTrue())
					})

					It("should use reasonable timestamp within 5 seconds", func() {
						Expect(timestamp).To(BeNumerically(">=", currentTime))
						Expect(timestamp).To(BeNumerically("<=", currentTime+5))
					})

					It("should move md file to timestamped directory", func() {
						_, statErr := os.Stat(deletedMdPath)
						Expect(statErr).NotTo(HaveOccurred())
					})

					It("should preserve file contents", func() {
						Expect(string(deletedMdContent)).To(ContainSubstring("test content to be soft deleted"))
					})
				})

				It("should remove the page from the index", func() {
					Expect(mockFrontmatter.LastRemovePageCall()).To(Equal(pageIdentifier))
					Expect(mockBleve.LastRemovePageCall()).To(Equal(pageIdentifier))
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
			var p *wikipage.Page
			var err error

			BeforeEach(func() {
				p, err = s.readOrInitPage(pageToCreate, req)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create a page with initial content", func() {
				Expect(p.Text).To(ContainSubstring("# {{or .Title .Identifier}}"))
				Expect(p.Text).To(ContainSubstring(`identifier = '` + pageToCreate + `'`))
			})
		})

		PWhen("creating a new page fails to save", func() {
			var p *wikipage.Page
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

				p, err = s.readOrInitPage(pageToCreate, req)
			})

			AfterEach(func() {
				// Restore permissions for cleanup
				_ = os.Chmod(tempDir, originalPerms)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to save new page"))
			})

			It("should return nil page", func() {
				Expect(p).To(BeNil())
			})
		})
	})

	Describe("InitializeIndexing", func() {
		When("an MD file exists in the data directory", func() {
			var (
				err error
			)

			BeforeEach(func() {
				// Create a test page as an MD file since JSON files are now migrated
				encodedFilename := base32tools.EncodeToBase32(strings.ToLower("test"))
				pagePath := filepath.Join(s.PathToData, encodedFilename+".md")
				testPageContent := `+++
identifier = "test"
+++
# Test Content`
				fileErr := os.WriteFile(pagePath, []byte(testPageContent), 0644)
				Expect(fileErr).NotTo(HaveOccurred())

				err = s.InitializeIndexing()
			})

			It("should not return an error from InitializeIndexing", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should initialize IndexCoordinator", func() {
				Expect(s.IndexCoordinator).NotTo(BeNil())
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
	})

	Describe("Rolling migrations integration", func() {
		var (
			mockApplicator *lazy.DefaultApplicator
			mockMig        *lazy.MockMigration
		)

		BeforeEach(func() {
			// Set up mock migration applicator for integration testing
			mockApplicator = lazy.NewEmptyApplicator()
			mockMig = &lazy.MockMigration{
				SupportedTypesResult: []lazy.FrontmatterType{lazy.FrontmatterTOML},
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

			It("should not modify the file on disk", func() {
				// File should remain unchanged since migration failed
				Expect(readErr).NotTo(HaveOccurred())
				Expect(savedContent).To(ContainSubstring(`title = "Test Page"`))
			})
		})

		Describe("when no migration applicator is configured", func() {
			var (
				pageIdentifier wikipage.PageIdentifier
				pagePath       string
				openErr        error
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

				// With no migration applicator, Open() should return an error
				_, openErr = s.ReadPage(string(pageIdentifier))
			})

			It("should return an error", func() {
				// Open should fail when migration applicator is not configured
				Expect(openErr).To(HaveOccurred())
				Expect(openErr.Error()).To(ContainSubstring("migration applicator not configured"))
			})
		})

		Describe("migrations on page save", func() {
			var (
				page           *wikipage.Page
				pageIdentifier string
				originalContent string
				err            error
			)

			BeforeEach(func() {
				pageIdentifier = "save-migration-test"
				
				// Set up mock migration applicator
				mockApplicator = lazy.NewEmptyApplicator()
				mockMig = &lazy.MockMigration{
					SupportedTypesResult: []lazy.FrontmatterType{lazy.FrontmatterTOML},
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
				page, err = s.ReadPage(pageIdentifier)
				Expect(err).NotTo(HaveOccurred())
				err = s.UpdatePageContent(wikipage.PageIdentifier(page.Identifier), originalContent)
				
				// Re-fetch the page to get the updated content
				if err == nil {
					page, err = s.ReadPage(pageIdentifier)
					Expect(err).NotTo(HaveOccurred())
				}
			})

			It("should apply migrations to problematic content", func() {
				Expect(err).NotTo(HaveOccurred())
				
				// Verify the page now contains the migrated content
				currentContent := page.Text
				Expect(currentContent).To(ContainSubstring(`title = "Fixed Title"`))
				Expect(currentContent).NotTo(ContainSubstring(`title = "Bad Title"`))
			})

			It("should complete migration successfully", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			Describe("when no migration applies", func() {
				BeforeEach(func() {
					// Set mock to not apply
					mockMig.AppliesToResult = false
					
					page, err = s.ReadPage(pageIdentifier + "-no-migration")
					Expect(err).NotTo(HaveOccurred())
					err = s.UpdatePageContent(wikipage.PageIdentifier(page.Identifier), originalContent)
					
					// Re-fetch the page to get the updated content
					if err == nil {
						page, err = s.ReadPage(pageIdentifier + "-no-migration")
						Expect(err).NotTo(HaveOccurred())
					}
				})

				It("should save original content unchanged", func() {
					Expect(err).NotTo(HaveOccurred())
					
					currentContent := page.Text
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
					
					page, err = s.ReadPage(pageIdentifier + "-recursive")
					Expect(err).NotTo(HaveOccurred())
					err = s.UpdatePageContent(wikipage.PageIdentifier(page.Identifier), originalContent)
					
					// Re-fetch the page to get the updated content
					if err == nil {
						page, err = s.ReadPage(pageIdentifier + "-recursive")
						Expect(err).NotTo(HaveOccurred())
					}
				})

				It("should not cause infinite recursion", func() {
					Expect(err).NotTo(HaveOccurred())
					
					// Should complete successfully without hanging
					currentContent := page.Text
					Expect(currentContent).To(ContainSubstring(`title = "Migrated"`))
				})
			})
		})
	})
})

