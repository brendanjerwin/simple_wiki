package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/gin-gonic/gin"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PWAEndpoints", func() {
	var (
		site   *server.Site
		router *gin.Engine
		w      *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		site = NewPwaTestSite()
		router = site.GinRouter()
		w = httptest.NewRecorder()
	})

	act := func(url string) {
		GinkgoHelper()
		req := httptest.NewRequest(http.MethodGet, url, nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	When("a per-page PWA manifest is requested", func() {
		var manifest map[string]any

		BeforeEach(func() {
			act("/manifest.json?page=has-app")
			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(w.Header().Get("Content-Type")).To(ContainSubstring("application/json"))
			Expect(json.Unmarshal(w.Body.Bytes(), &manifest)).To(Succeed())
		})

		It("should carry the page's name", func() {
			Expect(manifest["name"]).To(Equal("has-app"))
		})

		It("should start at the page's bare URL", func() {
			Expect(manifest["start_url"]).To(Equal("/has-app/bare"))
		})

		It("should declare standalone display", func() {
			Expect(manifest["display"]).To(Equal("standalone"))
		})

		It("should scope to the site root", func() {
			Expect(manifest["scope"]).To(Equal("/"))
		})

		It("should include the site icons", func() {
			icons, ok := manifest["icons"].([]any)
			Expect(ok).To(BeTrue())
			Expect(icons).NotTo(BeEmpty())
		})
	})

	When("the manifest is requested without a page parameter", func() {
		BeforeEach(func() {
			act("/manifest.json")
		})

		It("should respond 400", func() {
			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})
})

// NewPwaTestSite builds a Site with a page carrying a page-app fence for
// manifest endpoint tests.
func NewPwaTestSite() *server.Site {
	t := GinkgoT()
	tmpDir := t.TempDir()
	logger := lumber.NewConsoleLogger(lumber.ERROR)
	site, err := server.NewSite(tmpDir, "testpage", 0, "secret", logger)
	Expect(err).NotTo(HaveOccurred())
	Expect(site.UpdatePageContent(wikipage.PageIdentifier("has-app"),
		"```wiki-page-app\n{}\n```\n")).NotTo(HaveOccurred())
	return site
}
