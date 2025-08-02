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

	"github.com/brendanjerwin/simple_wiki/rollingmigrations"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/utils/goldmarkrenderer"
)

var _ = Describe("Site.OpenOrInit with URL parameters", func() {
	var (
		s         *Site
		req       *http.Request
		p         *Page
		err       error
		tmpDir    string
		reqURL    *url.URL
		mockIndex *MockIndexMaintainer
	)

	BeforeEach(func() {
		tmpDir, err = os.MkdirTemp("", "site-openorinit-test")
		Expect(err).NotTo(HaveOccurred())

		mockIndex = &MockIndexMaintainer{}

		// Set up empty migration applicator for unit testing
		applicator := rollingmigrations.NewEmptyApplicator()

		s = &Site{
			PathToData:              tmpDir,
			Logger:                  lumber.NewConsoleLogger(lumber.INFO),
			MarkdownRenderer:        &goldmarkrenderer.GoldmarkRenderer{},
			IndexMaintainer:         mockIndex,
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
		BeforeEach(func() {
			// Set up URL with dotted parameters
			params := url.Values{}
			params.Set("title", "Kinect To Windows Adapter")
			params.Set("inventory.container", "LabTub_61c0030e-00e3-47b5-a797-1ac01f8d05b1")
			params.Set("inventory.location", "Lab A")
			params.Set("tmpl", "inv_item")
			reqURL.RawQuery = params.Encode()

			p, err = s.OpenOrInit("kinect_to_windows_adapter", req)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create valid TOML frontmatter", func() {
			content := p.Text.GetCurrent()
			Expect(content).To(ContainSubstring("+++\n"))
			
			// Extract frontmatter between +++ delimiters
			startIdx := 4 // After first "+++\n"
			endIdx := len(content)
			for i := startIdx; i < len(content)-3; i++ {
				if content[i:i+3] == "+++" {
					endIdx = i
					break
				}
			}
			
			frontmatterTOML := content[startIdx:endIdx]
			
			// Parse the TOML to verify it's valid
			var parsed map[string]any
			err := toml.Unmarshal([]byte(frontmatterTOML), &parsed)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify the structure
			Expect(parsed).To(HaveKeyWithValue("identifier", "kinect_to_windows_adapter"))
			Expect(parsed).To(HaveKeyWithValue("title", "Kinect To Windows Adapter"))
			
			// Verify nested inventory structure
			Expect(parsed).To(HaveKey("inventory"))
			inventory, ok := parsed["inventory"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(inventory).To(HaveKeyWithValue("container", "LabTub_61c0030e-00e3-47b5-a797-1ac01f8d05b1"))
			Expect(inventory).To(HaveKeyWithValue("location", "Lab A"))
			Expect(inventory).To(HaveKey("items"))
		})

		It("should not include tmpl parameter in frontmatter", func() {
			content := p.Text.GetCurrent()
			Expect(content).NotTo(ContainSubstring("tmpl"))
		})

		It("should save the page successfully", func() {
			// The page should be saved to disk
			identifier := p.Identifier
			expectedPath := path.Join(tmpDir, base32tools.EncodeToBase32(strings.ToLower(identifier))+".md")
			_, err := os.Stat(expectedPath)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("when creating a page with array parameters", func() {
		BeforeEach(func() {
			params := url.Values{}
			params.Set("title", "Test Page")
			params.Add("tags", "electronics")
			params.Add("tags", "hardware")
			params.Add("tags", "adapter")
			reqURL.RawQuery = params.Encode()

			p, err = s.OpenOrInit("test_page_arrays", req)
		})

		It("should handle array values correctly", func() {
			content := p.Text.GetCurrent()
			
			// Extract and parse frontmatter
			startIdx := 4
			endIdx := len(content)
			for i := startIdx; i < len(content)-3; i++ {
				if content[i:i+3] == "+++" {
					endIdx = i
					break
				}
			}
			
			frontmatterTOML := content[startIdx:endIdx]
			var parsed map[string]any
			err := toml.Unmarshal([]byte(frontmatterTOML), &parsed)
			Expect(err).NotTo(HaveOccurred())
			
			// Verify tags is an array
			Expect(parsed).To(HaveKey("tags"))
			tags, ok := parsed["tags"].([]any)
			Expect(ok).To(BeTrue())
			Expect(tags).To(HaveLen(3))
		})
	})
})