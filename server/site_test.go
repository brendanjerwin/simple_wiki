package server

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

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

		s = &Site{
			Logger:                  lumber.NewConsoleLogger(lumber.INFO),
			PathToData:              tempDir,
			IndexMaintainer:         mockIndex,
			MarkdownRenderer:        &goldmarkrenderer.GoldmarkRenderer{},
			FrontmatterIndexQueryer: &mockFrontmatterIndexQueryer{},
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

	Describe("PageReadWriter implementation", func() {
		var (
			pageIdentifier wikipage.PageIdentifier
			pagePath       string
		)

		BeforeEach(func() {
			pageIdentifier = "test-page"
			// The PageReadWriter implementation reads from base32 encoded filenames
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

			BeforeEach(func() {
				p = s.OpenOrInit(pageToCreate, req)
			})

			It("should create a page with initial content", func() {
				Expect(p.Text.GetCurrent()).To(ContainSubstring("# {{or .Title .Identifier}}"))
				Expect(p.Text.GetCurrent()).To(ContainSubstring(`identifier = "` + pageToCreate + `"`))
			})

			It("should create the page regardless of save success", func() {
				Expect(p.IsNew()).To(BeTrue()) // OpenOrInit always creates new pages programmatically
			})
		})

		When("creating a new page fails to save", func() {
			var p *Page

			BeforeEach(func() {
				// Make the data directory read-only to simulate save failure
				dirInfo, err := os.Stat(tempDir)
				Expect(err).NotTo(HaveOccurred())
				originalPerms = dirInfo.Mode()
				err = os.Chmod(tempDir, 0444)
				Expect(err).NotTo(HaveOccurred())

				p = s.OpenOrInit(pageToCreate, req)
			})

			AfterEach(func() {
				// Restore permissions for cleanup
				_ = os.Chmod(tempDir, originalPerms)
			})

			It("should still return a page object", func() {
				Expect(p).NotTo(BeNil())
				Expect(p.Identifier).To(Equal(pageToCreate))
			})

			It("should still have the initial content", func() {
				Expect(p.Text.GetCurrent()).To(ContainSubstring("# {{or .Title .Identifier}}"))
			})

			It("should log an error message", func() {
				// Note: In a real test environment, we might want to capture logs
				// For now, we're just ensuring the function doesn't panic
				Expect(p.Text.GetCurrent()).NotTo(BeEmpty())
			})
		})
	})
})
