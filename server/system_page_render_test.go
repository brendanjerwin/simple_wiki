//revive:disable:dot-imports
package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/gin-gonic/gin"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("System page rendering", func() {
	var (
		site   *server.Site
		router *gin.Engine
		tmpDir string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "simple_wiki_syspage_render_test")
		Expect(err).NotTo(HaveOccurred())
		logger := lumber.NewConsoleLogger(lumber.WARN)
		site, err = server.NewSite(tmpDir, "testpage", 0, "secret", logger)
		Expect(err).NotTo(HaveOccurred())
		router = site.GinRouter()
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	When("the page has frontmatter wiki.system = true", func() {
		var (
			recorder *httptest.ResponseRecorder
			body     string
		)

		BeforeEach(func() {
			pageID := wikipage.PageIdentifier("help-test-page")
			Expect(site.WriteFrontMatter(pageID, wikipage.FrontMatter{
				"identifier": "help-test-page",
				"title":      "Help Test Page",
				"wiki": map[string]any{
					"system": true,
				},
			})).To(Succeed())
			Expect(site.WriteMarkdown(pageID, wikipage.Markdown("# Help body"))).To(Succeed())

			recorder = httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodGet, "/help-test-page/view", nil)
			Expect(err).NotTo(HaveOccurred())
			router.ServeHTTP(recorder, req)
			body = recorder.Body.String()
		})

		It("should respond with 200 OK", func() {
			Expect(recorder.Code).To(Equal(http.StatusOK))
		})

		It("should render the system-page banner", func() {
			Expect(body).To(ContainSubstring(`<div class="system-page-banner"`))
		})

		It("should hide the Edit menu link", func() {
			Expect(body).NotTo(ContainSubstring(`/help-test-page/edit`))
		})

		It("should hide the Edit Frontmatter menu link", func() {
			Expect(body).NotTo(ContainSubstring(`id="editFrontmatter"`))
		})

		It("should hide the Erase menu link", func() {
			Expect(body).NotTo(ContainSubstring(`id="erasePage"`))
		})
	})

	When("the page has frontmatter without system flag", func() {
		var (
			recorder *httptest.ResponseRecorder
			body     string
		)

		BeforeEach(func() {
			pageID := wikipage.PageIdentifier("regular-page")
			Expect(site.WriteFrontMatter(pageID, wikipage.FrontMatter{
				"identifier": "regular-page",
				"title":      "Regular Page",
			})).To(Succeed())
			Expect(site.WriteMarkdown(pageID, wikipage.Markdown("# Regular body"))).To(Succeed())

			recorder = httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodGet, "/regular-page/view", nil)
			Expect(err).NotTo(HaveOccurred())
			router.ServeHTTP(recorder, req)
			body = recorder.Body.String()
		})

		It("should respond with 200 OK", func() {
			Expect(recorder.Code).To(Equal(http.StatusOK))
		})

		It("should not render the system-page banner", func() {
			Expect(body).NotTo(ContainSubstring(`<div class="system-page-banner"`))
		})

		It("should show the Edit menu link", func() {
			Expect(body).To(ContainSubstring(`/regular-page/edit`))
		})
	})
})
