//revive:disable:dot-imports
package server

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
		When("the data directory does not exist", func() {
			It("should return an error", func() {
				s.PathToData = filepath.Join(pathToData, "nonexistent_subdir")
				_, err := s.DirectoryList()
				Expect(err).To(HaveOccurred())
			})
		})

		When("there are pages", func() {
			var listing DirectoryListing

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

				listing, err = s.DirectoryList()
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return all the pages", func() {
				Expect(listing.Entries).To(HaveLen(3))
			})

			It("should sort pages by modification time (oldest first)", func() {
				// Aider: don't change the order here. 3 should be _last_
				Expect(listing.Entries[0].Name()).To(Equal("testpage"))
				Expect(listing.Entries[1].Name()).To(Equal("testpage2"))
				Expect(listing.Entries[2].Name()).To(Equal("testpage3"))
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
				p, err = s.ReadPage(wikipage.PageIdentifier(p.Identifier))
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
					p, err = s.ReadPage(wikipage.PageIdentifier(p.Identifier))
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
				p, err = s.ReadPage(wikipage.PageIdentifier(pageIdentifier))
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

	Describe("Site.WriteFrontMatter", func() {
		When("the page does not exist on disk", func() {
			var err error

			BeforeEach(func() {
				err = s.WriteFrontMatter("brand-new-page", wikipage.FrontMatter{"title": "Created"})
			})

			It("should succeed", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			When("the page is read back", func() {
				var (
					page    *wikipage.Page
					readErr error
				)

				BeforeEach(func() {
					page, readErr = s.ReadPage("brand-new-page")
					Expect(readErr).NotTo(HaveOccurred())
				})

				It("should contain the new frontmatter", func() {
					Expect(page.Text).To(ContainSubstring("Created"))
				})

				It("should have been loaded from disk", func() {
					Expect(page.WasLoadedFromDisk).To(BeTrue())
				})
			})
		})

		When("the page has malformed TOML frontmatter", func() {
			var err error

			BeforeEach(func() {
				pageIdentifier := "malformed-fm-write"
				// Content with invalid TOML that also fails YAML parsing as a fallback.
				// 'title = [invalid' is an unclosed array — invalid for both TOML and YAML.
				malformedContent := "+++\ntitle = [invalid\n+++\n# Content"
				filePath := filepath.Join(pathToData, base32tools.EncodeToBase32(strings.ToLower(pageIdentifier))+".md")
				Expect(os.WriteFile(filePath, []byte(malformedContent), 0644)).To(Succeed())

				err = s.WriteFrontMatter(wikipage.PageIdentifier(pageIdentifier), wikipage.FrontMatter{"title": "new title"})
			})

			It("should return a parse error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to parse markdown for frontmatter write"))
			})
		})
	})

	Describe("Site.ModifyMarkdown", func() {
		When("the modifier returns an error", func() {
			var (
				modifyErr   error
				modifierErr = errors.New("modifier refused change")
			)

			BeforeEach(func() {
				modifyErr = s.ModifyMarkdown("some-modify-page", func(_ wikipage.Markdown) (wikipage.Markdown, error) {
					return "", modifierErr
				})
			})

			It("should propagate the modifier error", func() {
				Expect(modifyErr).To(MatchError(modifierErr))
			})

			It("should not create any page file", func() {
				page, readErr := s.ReadPage("some-modify-page")
				Expect(readErr).NotTo(HaveOccurred())
				Expect(page.IsNew()).To(BeTrue())
			})
		})

		When("the page has malformed TOML frontmatter", func() {
			var err error

			BeforeEach(func() {
				pageIdentifier := "malformed-fm-modify"
				malformedContent := "+++\ntitle = [invalid\n+++\n# Content"
				filePath := filepath.Join(pathToData, base32tools.EncodeToBase32(strings.ToLower(pageIdentifier))+".md")
				Expect(os.WriteFile(filePath, []byte(malformedContent), 0644)).To(Succeed())

				err = s.ModifyMarkdown(wikipage.PageIdentifier(pageIdentifier), func(md wikipage.Markdown) (wikipage.Markdown, error) {
					return md + " extra", nil
				})
			})

			It("should return a parse error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to parse markdown for modification"))
			})
		})

		When("the page does not exist on disk", func() {
			var err error

			BeforeEach(func() {
				err = s.ModifyMarkdown("nonexistent-modify-page", func(_ wikipage.Markdown) (wikipage.Markdown, error) {
					return "# New Content", nil
				})
			})

			It("should succeed and create the page", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			When("the page is read back", func() {
				var (
					page    *wikipage.Page
					readErr error
				)

				BeforeEach(func() {
					page, readErr = s.ReadPage("nonexistent-modify-page")
					Expect(readErr).NotTo(HaveOccurred())
				})

				It("should contain the written markdown", func() {
					Expect(page.Text).To(ContainSubstring("# New Content"))
				})
			})
		})
	})

	Describe("Atomic write safety", func() {
		// These tests verify that concurrent writes to different sections of a page
		// (frontmatter vs. markdown) do not lose each other's updates.
		// Without atomic read-modify-write, a concurrent WriteFrontMatter and WriteMarkdown
		// can both read the same stale state, and whichever writes last silently discards
		// the other's change.
		When("WriteFrontMatter and WriteMarkdown are called concurrently on the same page", func() {
			const iterations = 200

			BeforeEach(func() {
				// Create the page with initial state containing both frontmatter and markdown.
				initialText := "+++\ntitle = \"old title\"\n+++\n\nold content\n"
				err := s.UpdatePageContent("atomic_test_page", initialText)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should preserve both writes — neither update should be lost", func() {
				for i := 0; i < iterations; i++ {
					// Reset to known state before each iteration.
					initialText := "+++\ntitle = \"old title\"\n+++\n\nold content\n"
					Expect(s.UpdatePageContent("atomic_test_page", initialText)).To(Succeed())

					var wg sync.WaitGroup
					wg.Add(2)

					go func() {
						defer wg.Done()
						_ = s.WriteFrontMatter("atomic_test_page", wikipage.FrontMatter{"title": "new title"})
					}()

					go func() {
						defer wg.Done()
						_ = s.WriteMarkdown("atomic_test_page", "\nnew content\n")
					}()

					wg.Wait()

					_, fm, fmErr := s.ReadFrontMatter("atomic_test_page")
					Expect(fmErr).NotTo(HaveOccurred())

					_, md, mdErr := s.ReadMarkdown("atomic_test_page")
					Expect(mdErr).NotTo(HaveOccurred())

					// Both updates must be visible — neither should overwrite the other.
					Expect(fm["title"]).To(Equal("new title"),
						"iteration %d: frontmatter title update was lost", i)
					Expect(string(md)).To(ContainSubstring("new content"),
						"iteration %d: markdown update was lost", i)
				}
			})
		})
	})
})
