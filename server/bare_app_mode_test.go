//revive:disable:dot-imports
package server_test

import (
	"bytes"
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

// bufferWriteCloser adapts a bytes.Buffer to io.WriteCloser for the lumber logger.
type bufferWriteCloser struct {
	*bytes.Buffer
}

func (*bufferWriteCloser) Close() error { return nil }

var _ = Describe("BareAppMode", func() {
	var (
		site   *server.Site
		router *gin.Engine
		w      *httptest.ResponseRecorder
		tmpDir string
	)

	createSite := func(pages map[string]string) {
		GinkgoHelper()
		var err error
		logger := lumber.NewBasicLogger(&bufferWriteCloser{Buffer: bytes.NewBuffer(nil)}, lumber.ERROR)
		site, err = server.NewSite(tmpDir, "testpage", 0, "secret", logger)
		Expect(err).NotTo(HaveOccurred())
		router = site.GinRouter()
		for identifier, body := range pages {
			Expect(site.UpdatePageContent(wikipage.PageIdentifier(identifier), body)).NotTo(HaveOccurred())
		}
	}

	act := func(url string) {
		GinkgoHelper()
		req := httptest.NewRequest(http.MethodGet, url, nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "bare_app_mode_test")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	When("the requested page carries a wiki-page-app fence", func() {
		const pageWithApp = "has-app"
		const fenceBody = "Visible prose here.\n\n```wiki-page-app\n{\"hello\": true}\n```\n\nTrailing prose.\n"

		BeforeEach(func() {
			createSite(map[string]string{pageWithApp: fenceBody})
			act("/" + pageWithApp + "/bare")
		})

		It("should respond 200 OK", func() {
			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should disable caching", func() {
			Expect(w.Header().Get("Cache-Control")).To(Equal("no-store"))
		})

		It("should mount the page-app host", func() {
			Expect(w.Body.String()).To(ContainSubstring(`<wiki-page-app-host page="` + pageWithApp + `"`))
		})

		It("should mount the PWA install affordance with the page identifier", func() {
			Expect(w.Body.String()).To(ContainSubstring(`<wiki-pwa-install bare page="` + pageWithApp + `"`))
		})

		It("should not render the page prose", func() {
			Expect(w.Body.String()).NotTo(ContainSubstring("Visible prose here."))
		})

		It("should not expose wiki chrome", func() {
			Expect(w.Body.String()).NotTo(ContainSubstring(`id="menu"`))
			Expect(w.Body.String()).NotTo(ContainSubstring("wiki-search"))
			Expect(w.Body.String()).NotTo(ContainSubstring("page-chat-panel"))
			Expect(w.Body.String()).NotTo(ContainSubstring("<system-info"))
		})

		It("should link the per-page manifest", func() {
			Expect(w.Body.String()).To(ContainSubstring(`<link rel="manifest" href="/manifest.json?page=` + pageWithApp + `">`))
		})
	})

	When("the requested page has no page-app fence", func() {
		BeforeEach(func() {
			createSite(map[string]string{"no-app": "Just prose, nothing else.\n"})
			act("/no-app/bare")
		})

		It("should respond 404 Not Found", func() {
			Expect(w.Code).To(Equal(http.StatusNotFound))
		})

		It("should explain the missing app", func() {
			Expect(w.Body.String()).To(ContainSubstring("no page app"))
		})
	})

	When("the requested page does not exist", func() {
		BeforeEach(func() {
			createSite(map[string]string{})
			act("/ghost-page-xyz/bare")
		})

		It("should respond 404 Not Found", func() {
			Expect(w.Code).To(Equal(http.StatusNotFound))
		})
	})
})
