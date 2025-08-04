//revive:disable:dot-imports
package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brendanjerwin/simple_wiki/rollingmigrations"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/utils/goldmarkrenderer"
	"github.com/brendanjerwin/simple_wiki/wikipage"

	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/schollz/versionedtext"
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

var _ = Describe("Page Functions", func() {
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
			MigrationApplicator: rollingmigrations.NewEmptyApplicator(),
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
				p, err := s.OpenOrInit("testpage", req)
				Expect(err).ToNot(HaveOccurred())
				err = p.Update("Some data")
				Expect(err).ToNot(HaveOccurred())
				time.Sleep(10 * time.Millisecond)

				p, err = s.OpenOrInit("testpage2", req)
				Expect(err).ToNot(HaveOccurred())
				err = p.Update("A different bunch of data")
				Expect(err).ToNot(HaveOccurred())
				time.Sleep(10 * time.Millisecond)

				p, err = s.OpenOrInit("testpage3", req)
				Expect(err).ToNot(HaveOccurred())
				err = p.Update("Not much else")
				Expect(err).ToNot(HaveOccurred())

				// Wait for any background indexing operations to complete
				if s.IndexingService != nil {
					completed, _ := s.IndexingService.WaitForCompletionWithTimeout(context.Background(), 1*time.Second)
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
		var p *Page

		BeforeEach(func() {
			req, _ := http.NewRequest("GET", "/", nil)
			var err error
			p, err = s.OpenOrInit("testpage", req)
			Expect(err).ToNot(HaveOccurred())
		})

		When("A page is updated", func() {
			BeforeEach(func() {
				err := p.Update("**bold**")
				Expect(err).ToNot(HaveOccurred())
			})

			It("should render correctly", func() {
				Expect(string(p.RenderedPage)).To(ContainSubstring("<p><strong>bold</strong></p>"))
			})

			When("the page is updated again", func() {
				BeforeEach(func() {
					err := p.Update("**bold** and *italic*")
					Expect(err).ToNot(HaveOccurred())
					err = p.Save()
					Expect(err).ToNot(HaveOccurred())
				})

				It("should render the new content", func() {
					Expect(string(p.RenderedPage)).To(ContainSubstring("<p><strong>bold</strong> and <em>italic</em></p>"))
				})

				When("the page is retrieved from disk", func() {
					var (
						p2  *Page
						err error
					)

					BeforeEach(func() {
						p2, err = s.Open("testpage")
						Expect(err).NotTo(HaveOccurred())
					})

					It("should have its content preserved", func() {
						Expect(p2.Text.GetCurrent()).To(Equal("**bold** and *italic*"))
					})

					When("the retrieved page is rendered", func() {
						BeforeEach(func() {
							p2.Render()
						})

						It("should render correctly", func() {
							Expect(string(p2.RenderedPage)).To(ContainSubstring("<p><strong>bold</strong> and <em>italic</em></p>"))
						})
					})
				})
			})
		})
	})

	Describe("Page.parse", func() {
		var (
			p           *Page
			frontmatter wikipage.FrontMatter
			markdown    wikipage.Markdown
			err         error
		)

		BeforeEach(func() {
			p = &Page{
				Site:       s,
				Identifier: "testpage",
				Text:       versionedtext.NewVersionedText(""),
			}
		})

		JustBeforeEach(func() {
			frontmatter, markdown, err = p.parse()
		})

		When("the page has no frontmatter", func() {
			BeforeEach(func() {
				p.Text.Update("Just some markdown content.")
			})

			It("should return empty frontmatter", func() {
				Expect(frontmatter).To(BeEmpty())
			})

			It("should return the full text as markdown", func() {
				Expect(string(markdown)).To(Equal("Just some markdown content."))
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the page has valid frontmatter", func() {
			BeforeEach(func() {
				content := `---
title: Test Page
tags: [one, two]
---
This is the markdown content.`
				p.Text.Update(content)
			})

			It("should correctly parse the frontmatter", func() {
				Expect(frontmatter).To(HaveKeyWithValue("title", "Test Page"))
				Expect(frontmatter).To(HaveKey("tags"))
				Expect(frontmatter["tags"]).To(BeEquivalentTo([]any{"one", "two"}))
			})

			It("should return the content after the frontmatter as markdown", func() {
				Expect(string(markdown)).To(Equal("This is the markdown content."))
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the page has invalid YAML in frontmatter", func() {
			BeforeEach(func() {
				content := `---
title: Test Page
tags: [one, two
---
This is the markdown content.`
				p.Text.Update(content)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to unmarshal frontmatter for testpage"))
			})
		})

		When("the content is empty", func() {
			BeforeEach(func() {
				p.Text.Update("")
			})

			It("should return empty frontmatter and markdown", func() {
				Expect(frontmatter).To(BeEmpty())
				Expect(string(markdown)).To(BeEmpty())
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("there is only frontmatter", func() {
			BeforeEach(func() {
				content := `---
title: Only Frontmatter
---
`
				p.Text.Update(content)
			})

			It("should parse the frontmatter", func() {
				Expect(frontmatter).To(HaveKeyWithValue("title", "Only Frontmatter"))
			})

			It("should return empty markdown", func() {
				Expect(string(markdown)).To(BeEmpty())
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("the text contains '---' but not as a separator", func() {
			var content string
			BeforeEach(func() {
				content = `Here is some text.
---
And some more text. But this is not frontmatter.`
				p.Text.Update(content)
			})

			It("should return empty frontmatter", func() {
				Expect(frontmatter).To(BeEmpty())
			})

			It("should return the full text as markdown", func() {
				Expect(string(markdown)).To(Equal(content))
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
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
				p              *Page
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
				p, err = s.Open(pageIdentifier)
				Expect(err).NotTo(HaveOccurred())
				
				// Wait for any background indexing operations triggered by the save
				if s.IndexingService != nil {
					completed, _ := s.IndexingService.WaitForCompletionWithTimeout(context.Background(), 1*time.Second)
					Expect(completed).To(BeTrue())
				}
				
				// Check final content after open
				finalContent = p.Text.GetCurrent()
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
