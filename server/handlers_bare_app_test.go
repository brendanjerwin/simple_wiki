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

var _ = Describe("Bare-app URL mode", func() {
	var (
		site   *server.Site
		router *gin.Engine
		tmpDir string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "simple_wiki_bare_app_test")
		Expect(err).NotTo(HaveOccurred())
		logger := lumber.NewConsoleLogger(lumber.WARN)
		site, err = server.NewSite(tmpDir, "testpage", 0, "secret", logger)
		Expect(err).NotTo(HaveOccurred())
		router = site.GinRouter()
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	Describe("GET /:page/app", func() {
		When("the page has a page app", func() {
			var (
				recorder *httptest.ResponseRecorder
				body     string
			)

			BeforeEach(func() {
				pageID := wikipage.PageIdentifier("my-app-page")
				Expect(site.WriteMarkdown(
					pageID,
					"# My App\n\n```wiki-page-app\n$count = 0\nroot = Card(\"Counter\", Text(\"count: \" + $count))\n```",
					wikipage.AnonymousIdentity,
				)).To(Succeed())

				recorder = httptest.NewRecorder()
				req, err := http.NewRequest(http.MethodGet, "/my-app-page/app", nil)
				Expect(err).NotTo(HaveOccurred())
				router.ServeHTTP(recorder, req)
				body = recorder.Body.String()
			})

			It("should respond with 200 OK", func() {
				Expect(recorder.Code).To(Equal(http.StatusOK))
			})

			It("should render wiki-page-app-host", func() {
				Expect(body).To(ContainSubstring("wiki-page-app-host"))
			})

			It("should include the page attribute on the host element", func() {
				Expect(body).To(ContainSubstring(`page="my-app-page"`))
			})

			It("should set the bare attribute on the host element", func() {
				Expect(body).To(ContainSubstring(`bare`))
			})

			It("should not render the wiki navigation menu", func() {
				Expect(body).NotTo(ContainSubstring(`id="menu"`))
			})

			It("should not render the chat panel", func() {
				Expect(body).NotTo(ContainSubstring("page-chat-panel"))
			})

			It("should not render the system-info widget", func() {
				Expect(body).NotTo(ContainSubstring("system-info"))
			})

			It("should include viewport-fit=cover for mobile", func() {
				Expect(body).To(ContainSubstring("viewport-fit=cover"))
			})

			It("should include the web-components script", func() {
				Expect(body).To(ContainSubstring("/static/js/dist/web-components.js"))
			})

			It("should set window.simple_wiki with the page name", func() {
				Expect(body).To(ContainSubstring(`pageName: "my-app-page"`))
			})
		})

		When("the page does not have a page app", func() {
			var recorder *httptest.ResponseRecorder

			BeforeEach(func() {
				pageID := wikipage.PageIdentifier("no-app-page")
				Expect(site.WriteMarkdown(pageID, "# Regular Page\n\nJust prose.", wikipage.AnonymousIdentity)).To(Succeed())

				recorder = httptest.NewRecorder()
				req, err := http.NewRequest(http.MethodGet, "/no-app-page/app", nil)
				Expect(err).NotTo(HaveOccurred())
				router.ServeHTTP(recorder, req)
			})

			It("should redirect to the page view", func() {
				Expect(recorder.Code).To(Equal(http.StatusFound))
			})

			It("should redirect to the view URL", func() {
				Expect(recorder.Header().Get("Location")).To(Equal("/no-app-page/view"))
			})
		})
	})
})
