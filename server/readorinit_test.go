//revive:disable:dot-imports
package server

import (
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/jcelliott/lumber"
	"github.com/pelletier/go-toml/v2"

	"github.com/brendanjerwin/simple_wiki/migrations/lazy"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/utils/goldmarkrenderer"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

var _ = Describe("Site.ReadOrInit with URL parameters", func() {
	var (
		s         *Site
		req       *http.Request
		p         *wikipage.Page
		err       error
		tmpDir    string
		reqURL    *url.URL
	)

	BeforeEach(func() {
		tmpDir, err = os.MkdirTemp("", "site-readorinit-test")
		Expect(err).NotTo(HaveOccurred())


		// Set up empty migration applicator for unit testing
		applicator := lazy.NewEmptyApplicator()

		s = &Site{
			PathToData:              tmpDir,
			Logger:                  lumber.NewConsoleLogger(lumber.INFO),
			MarkdownRenderer:        &goldmarkrenderer.GoldmarkRenderer{},
			FrontmatterIndexQueryer: &mockFrontmatterIndexQueryer{},
			MigrationApplicator:     applicator,
		}

		reqURL = &url.URL{
			Path: "/test_page",
		}
		req = &http.Request{
			URL: reqURL,
		}
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("when creating a new page with dotted URL parameters", func() {
		var (
			content           string
			frontmatterTOML   string
			parsed            map[string]any
			parseErr          error
			expectedPath      string
			pathStatErr       error
		)

		BeforeEach(func() {
			// Set up URL with dotted parameters
			params := url.Values{}
			params.Set("title", "Kinect To Windows Adapter")
			params.Set("inventory.container", "LabTub_61c0030e-00e3-47b5-a797-1ac01f8d05b1")
			params.Set("inventory.location", "Lab A")
			params.Set("tmpl", "inv_item")
			reqURL.RawQuery = params.Encode()

			// Act
			p, err = s.readOrInitPage("kinect_to_windows_adapter", req)
			
			// Capture test data after action
			if p != nil {
				content = p.Text.GetCurrent()
				
				// Extract frontmatter between +++ delimiters
				if strings.Contains(content, "+++") {
					startIdx := 4 // After first "+++\n"
					endIdx := len(content)
					for i := startIdx; i < len(content)-3; i++ {
						if content[i:i+3] == "+++" {
							endIdx = i
							break
						}
					}
					frontmatterTOML = content[startIdx:endIdx]
					
					// Parse the TOML
					parseErr = toml.Unmarshal([]byte(frontmatterTOML), &parsed)
				}
				
				// Check if page was saved to disk
				identifier := p.Identifier
				expectedPath = path.Join(tmpDir, base32tools.EncodeToBase32(strings.ToLower(identifier))+".md")
				_, pathStatErr = os.Stat(expectedPath)
			}
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should contain TOML frontmatter delimiters", func() {
			Expect(content).To(ContainSubstring("+++\n"))
		})

		It("should parse TOML without error", func() {
			Expect(parseErr).NotTo(HaveOccurred())
		})

		It("should include identifier in frontmatter", func() {
			Expect(parsed).To(HaveKeyWithValue("identifier", "kinect_to_windows_adapter"))
		})

		It("should include title in frontmatter", func() {
			Expect(parsed).To(HaveKeyWithValue("title", "Kinect To Windows Adapter"))
		})

		It("should include inventory structure in frontmatter", func() {
			Expect(parsed).To(HaveKey("inventory"))
		})

		It("should create inventory as map type", func() {
			_, ok := parsed["inventory"].(map[string]any)
			Expect(ok).To(BeTrue())
		})

		It("should include container in inventory", func() {
			inventory, ok := parsed["inventory"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(inventory).To(HaveKeyWithValue("container", "LabTub_61c0030e-00e3-47b5-a797-1ac01f8d05b1"))
		})

		It("should include location in inventory", func() {
			inventory, ok := parsed["inventory"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(inventory).To(HaveKeyWithValue("location", "Lab A"))
		})

		It("should include items key in inventory", func() {
			inventory, ok := parsed["inventory"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(inventory).To(HaveKey("items"))
		})

		It("should not include tmpl parameter in frontmatter", func() {
			Expect(content).NotTo(ContainSubstring("tmpl"))
		})

		It("should save the page to disk", func() {
			Expect(pathStatErr).NotTo(HaveOccurred())
		})
	})

	Describe("when creating a page with array parameters", func() {
		var (
			content         string
			frontmatterTOML string
			parsed          map[string]any
			parseErr        error
		)

		BeforeEach(func() {
			params := url.Values{}
			params.Set("title", "Test Page")
			params.Add("tags", "electronics")
			params.Add("tags", "hardware")
			params.Add("tags", "adapter")
			reqURL.RawQuery = params.Encode()

			// Act
			p, err = s.readOrInitPage("test_page_arrays", req)
			
			// Capture test data after action
			if p != nil {
				content = p.Text.GetCurrent()
				
				// Extract and parse frontmatter
				if strings.Contains(content, "+++") {
					startIdx := 4
					endIdx := len(content)
					for i := startIdx; i < len(content)-3; i++ {
						if content[i:i+3] == "+++" {
							endIdx = i
							break
						}
					}
					frontmatterTOML = content[startIdx:endIdx]
					parseErr = toml.Unmarshal([]byte(frontmatterTOML), &parsed)
				}
			}
		})

		It("should parse frontmatter without error", func() {
			Expect(parseErr).NotTo(HaveOccurred())
		})

		It("should include tags key in frontmatter", func() {
			Expect(parsed).To(HaveKey("tags"))
		})

		It("should create tags as array type", func() {
			_, ok := parsed["tags"].([]any)
			Expect(ok).To(BeTrue())
		})

		It("should contain three tag values", func() {
			tags, ok := parsed["tags"].([]any)
			Expect(ok).To(BeTrue())
			Expect(tags).To(HaveLen(3))
		})
	})
})