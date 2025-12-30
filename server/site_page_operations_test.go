//revive:disable:dot-imports
package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brendanjerwin/simple_wiki/migrations/lazy"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/utils/goldmarkrenderer"
	"github.com/brendanjerwin/simple_wiki/wikipage"

	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// mockMigrationApplicatorForCircularTest simulates a migration that modifies content
type mockMigrationApplicatorForCircularTest struct {
	shouldModifyContent bool
}

func (m *mockMigrationApplicatorForCircularTest) ApplyMigrations(content []byte) ([]byte, error) {
	if m.shouldModifyContent {
		// Simulate a migration that would modify content, requiring a save
		modifiedContent := append(content, []byte("\n# Migration applied")...)
		return modifiedContent, nil
	}
	return content, nil
}

var _ = Describe("Site Page Operations", func() {
	var (
		pathToData string
		s          *Site
	)

	BeforeEach(func() {
		pathToData = "testdata_page"
		err := os.MkdirAll(pathToData, 0755)
		Expect(err).NotTo(HaveOccurred())
		s = &Site{
			PathToData:          pathToData,
			MarkdownRenderer:    &goldmarkrenderer.GoldmarkRenderer{},
			Logger:              lumber.NewConsoleLogger(lumber.INFO),
			MigrationApplicator: lazy.NewEmptyApplicator(),
		}
		err = s.InitializeIndexing()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_ = os.RemoveAll(pathToData)
	})

	Describe("Site.DirectoryList", func() {
		When("there are pages", func() {
			var pages []os.FileInfo

			BeforeEach(func() {
				req, _ := http.NewRequest("GET", "/", nil)
				p, err := s.readOrInitPage("testpage", req)
				Expect(err).ToNot(HaveOccurred())
				err = s.UpdatePageContent(wikipage.PageIdentifier(p.Identifier), "Some data")
				Expect(err).ToNot(HaveOccurred())
				time.Sleep(10 * time.Millisecond)

				p, err = s.readOrInitPage("testpage2", req)
				Expect(err).ToNot(HaveOccurred())
				err = s.UpdatePageContent(wikipage.PageIdentifier(p.Identifier), "A different bunch of data")
				Expect(err).ToNot(HaveOccurred())
				time.Sleep(10 * time.Millisecond)

				p, err = s.readOrInitPage("testpage3", req)
				Expect(err).ToNot(HaveOccurred())
				err = s.UpdatePageContent(wikipage.PageIdentifier(p.Identifier), "Not much else")
				Expect(err).ToNot(HaveOccurred())

				// Wait for any background indexing operations to complete
				if s.IndexCoordinator != nil {
					completed, _ := s.IndexCoordinator.WaitForCompletionWithTimeout(context.Background(), 1*time.Second)
					Expect(completed).To(BeTrue())
				}

				pages = s.DirectoryList()
			})

			It("should return all the pages", func() {
				Expect(pages).To(HaveLen(3))
			})

			It("should sort pages by modification time (oldest first)", func() {
				// Aider: don't change the order here. 3 should be _last_
				Expect(pages[0].Name()).To(Equal("testpage"))
				Expect(pages[1].Name()).To(Equal("testpage2"))
				Expect(pages[2].Name()).To(Equal("testpage3"))
			})
		})
	})

	Describe("Page update and render", func() {
		var p *wikipage.Page

		BeforeEach(func() {
			req, _ := http.NewRequest("GET", "/", nil)
			var err error
			p, err = s.readOrInitPage("testpage", req)
			Expect(err).ToNot(HaveOccurred())
		})

		When("A page is updated", func() {
			BeforeEach(func() {
				err := s.UpdatePageContent(wikipage.PageIdentifier(p.Identifier), "**bold**")
				Expect(err).ToNot(HaveOccurred())
				
				// Re-fetch the page to get the updated content
				p, err = s.ReadPage(p.Identifier)
				Expect(err).ToNot(HaveOccurred())
				Expect(p.Render(s, s.MarkdownRenderer, TemplateExecutor{}, s.FrontmatterIndexQueryer)).To(Succeed())
			})

			It("should render correctly", func() {
				Expect(string(p.RenderedPage)).To(ContainSubstring("<p><strong>bold</strong></p>"))
			})

			When("the page is updated again", func() {
				BeforeEach(func() {
					err := s.UpdatePageContent(wikipage.PageIdentifier(p.Identifier), "**bold** and *italic*")
					Expect(err).ToNot(HaveOccurred())
					
					// Re-fetch the page to get the updated content
					p, err = s.ReadPage(p.Identifier)
					Expect(err).ToNot(HaveOccurred())
					Expect(p.Render(s, s.MarkdownRenderer, TemplateExecutor{}, s.FrontmatterIndexQueryer)).To(Succeed())
				})

				It("should render the new content", func() {
					Expect(string(p.RenderedPage)).To(ContainSubstring("<p><strong>bold</strong> and <em>italic</em></p>"))
				})

				When("the page is retrieved from disk", func() {
					var (
						p2  *wikipage.Page
						err error
					)

					BeforeEach(func() {
						p2, err = s.ReadPage("testpage")
						Expect(err).NotTo(HaveOccurred())
					})

					It("should have its content preserved", func() {
						Expect(p2.Text).To(Equal("**bold** and *italic*"))
					})

					When("the retrieved page is rendered", func() {
						BeforeEach(func() {
							Expect(p2.Render(s, s.MarkdownRenderer, TemplateExecutor{}, s.FrontmatterIndexQueryer)).To(Succeed())
						})

						It("should render correctly", func() {
							Expect(string(p2.RenderedPage)).To(ContainSubstring("<p><strong>bold</strong> and <em>italic</em></p>"))
						})
					})
				})
			})
		})
	})


	Describe("Site.Open migration integration", func() {
		var (
			pageIdentifier string
			pagePath       string
		)

		BeforeEach(func() {
			pageIdentifier = "test_migration_page"
			pagePath = filepath.Join(s.PathToData, base32tools.EncodeToBase32(strings.ToLower(pageIdentifier))+".md")
			
			// Create initial page content on disk
			initialContent := "+++\nidentifier = \"test_migration_page\"\n+++\n# Test Page"
			writeErr := os.WriteFile(pagePath, []byte(initialContent), 0644)
			Expect(writeErr).NotTo(HaveOccurred())
		})

		When("Open is called with migrations that modify content", func() {
			var (
				p              *wikipage.Page
				err            error
				originalDiskContent string
				finalContent   string
			)

			BeforeEach(func() {
				// Record original disk content
				diskBytes, _ := os.ReadFile(pagePath)
				originalDiskContent = string(diskBytes)
				
				// Set up a mock migration that modifies content
				mockApplicator := &mockMigrationApplicatorForCircularTest{
					shouldModifyContent: true,
				}
				s.MigrationApplicator = mockApplicator
				
				// This call should complete without hanging and apply migrations
				p, err = s.ReadPage(pageIdentifier)
				Expect(err).NotTo(HaveOccurred())
				
				// Wait for any background indexing operations triggered by the save
				if s.IndexCoordinator != nil {
					completed, _ := s.IndexCoordinator.WaitForCompletionWithTimeout(context.Background(), 1*time.Second)
					Expect(completed).To(BeTrue())
				}
				
				// Check final content after open
				finalContent = p.Text
			})

			It("should complete without hanging", func() {
				// If we get here, the operation completed successfully without infinite recursion
				Expect(p).NotTo(BeNil())
				Expect(p.WasLoadedFromDisk).To(BeTrue())
			})

			It("should load page successfully", func() {
				Expect(p.Identifier).To(Equal(pageIdentifier))
			})

			It("should persist migrated content", func() {
				// The migration should have been applied and saved during Open()
				Expect(finalContent).To(ContainSubstring("# Migration applied"))
				Expect(finalContent).NotTo(Equal(originalDiskContent))
			})
		})
	})
})
