//revive:disable:dot-imports
package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"

	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/gin-gonic/gin"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// recordingHandler is a test-only http.Handler that records every
// request it observes. Tests inject one as Site.caldavServer to assert
// which requests the gateway middleware forwards (vs. lets fall through
// to the regular Gin routes).
type recordingHandler struct {
	calls       atomic.Int32
	lastMethod  string
	lastURLPath string
}

// ServeHTTP increments the call counter, records the request's method
// + URL path, and writes a 204 so the gateway's c.Abort() short-circuit
// is observable through the response status.
func (h *recordingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.calls.Add(1)
	h.lastMethod = r.Method
	h.lastURLPath = r.URL.Path
	w.WriteHeader(http.StatusNoContent)
}

// caldavSentinelStatus is the response status the recordingHandler
// writes. Distinct from any status the regular page handler emits so
// tests can tell the two apart from w.Code alone.
const caldavSentinelStatus = http.StatusNoContent

var _ = Describe("caldavGateway middleware", func() {
	var site *server.Site
	var router *gin.Engine
	var caldavSrv *recordingHandler
	var w *httptest.ResponseRecorder
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "simple_wiki_caldav_gateway_test")
		Expect(err).NotTo(HaveOccurred())
		logger := lumber.NewConsoleLogger(lumber.WARN)
		site, err = server.NewSite(tmpDir, "testpage", 0, "secret", logger)
		Expect(err).NotTo(HaveOccurred())

		caldavSrv = &recordingHandler{}
		site.SetCalDAVServer(caldavSrv)

		router = site.GinRouter()
		w = httptest.NewRecorder()
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	When("a PROPFIND request hits a wiki page URL", func() {
		BeforeEach(func() {
			req := httptest.NewRequest("PROPFIND", "/shopping/", nil)
			router.ServeHTTP(w, req)
		})

		It("should forward the request to the CalDAV server", func() {
			Expect(caldavSrv.calls.Load()).To(Equal(int32(1)))
		})

		It("should record the PROPFIND method on the forwarded request", func() {
			Expect(caldavSrv.lastMethod).To(Equal("PROPFIND"))
		})

		It("should respond with the CalDAV server's status", func() {
			Expect(w.Code).To(Equal(caldavSentinelStatus))
		})
	})

	When("an OPTIONS request hits a wiki page URL", func() {
		BeforeEach(func() {
			req := httptest.NewRequest(http.MethodOptions, "/shopping/", nil)
			router.ServeHTTP(w, req)
		})

		It("should forward the request to the CalDAV server", func() {
			Expect(caldavSrv.calls.Load()).To(Equal(int32(1)))
		})
	})

	When("a REPORT request hits a wiki page URL", func() {
		BeforeEach(func() {
			req := httptest.NewRequest("REPORT", "/shopping/groceries", nil)
			router.ServeHTTP(w, req)
		})

		It("should forward the request to the CalDAV server", func() {
			Expect(caldavSrv.calls.Load()).To(Equal(int32(1)))
		})
	})

	When("a PUT request hits a wiki page URL", func() {
		BeforeEach(func() {
			req := httptest.NewRequest(http.MethodPut, "/shopping/groceries/01HXAAAAAAAAAAAAAAAAAAAAAA.ics", nil)
			router.ServeHTTP(w, req)
		})

		It("should forward the request to the CalDAV server", func() {
			Expect(caldavSrv.calls.Load()).To(Equal(int32(1)))
		})
	})

	When("a DELETE request hits a wiki page URL", func() {
		BeforeEach(func() {
			req := httptest.NewRequest(http.MethodDelete, "/shopping/groceries/01HXAAAAAAAAAAAAAAAAAAAAAA.ics", nil)
			router.ServeHTTP(w, req)
		})

		It("should forward the request to the CalDAV server", func() {
			Expect(caldavSrv.calls.Load()).To(Equal(int32(1)))
		})
	})

	When("a GET request hits an .ics resource path", func() {
		BeforeEach(func() {
			req := httptest.NewRequest(http.MethodGet, "/shopping/groceries/01HXAAAAAAAAAAAAAAAAAAAAAA.ics", nil)
			router.ServeHTTP(w, req)
		})

		It("should forward the request to the CalDAV server", func() {
			Expect(caldavSrv.calls.Load()).To(Equal(int32(1)))
		})

		It("should record the GET method on the forwarded request", func() {
			Expect(caldavSrv.lastMethod).To(Equal(http.MethodGet))
		})
	})

	When("a GET request hits the regular wiki view URL", func() {
		BeforeEach(func() {
			req := httptest.NewRequest(http.MethodGet, "/shopping/view", nil)
			router.ServeHTTP(w, req)
		})

		It("should not forward the request to the CalDAV server", func() {
			Expect(caldavSrv.calls.Load()).To(Equal(int32(0)))
		})

		It("should let the regular page handler run", func() {
			// The regular page handler renders an HTML page (status 200).
			// The CalDAV server's sentinel status (204) would only appear
			// if the gateway had short-circuited. Assert the status is
			// not the CalDAV sentinel.
			Expect(w.Code).NotTo(Equal(caldavSentinelStatus))
		})
	})

	When("a GET request hits the regular wiki edit URL", func() {
		BeforeEach(func() {
			req := httptest.NewRequest(http.MethodGet, "/shopping/edit", nil)
			router.ServeHTTP(w, req)
		})

		It("should not forward the request to the CalDAV server", func() {
			Expect(caldavSrv.calls.Load()).To(Equal(int32(0)))
		})

		It("should let the regular page handler run", func() {
			Expect(w.Code).NotTo(Equal(caldavSentinelStatus))
		})
	})

	When("a POST update request is sent", func() {
		BeforeEach(func() {
			req := httptest.NewRequest(http.MethodPost, "/update", nil)
			router.ServeHTTP(w, req)
		})

		It("should not forward the request to the CalDAV server", func() {
			Expect(caldavSrv.calls.Load()).To(Equal(int32(0)))
		})
	})

	When("a GET request hits a non-.ics two-segment path", func() {
		BeforeEach(func() {
			// /<page>/<command> — the Gin wildcard handler owns this.
			// Only 3-segment .ics paths should route to CalDAV.
			req := httptest.NewRequest(http.MethodGet, "/shopping/groceries", nil)
			router.ServeHTTP(w, req)
		})

		It("should not forward the request to the CalDAV server", func() {
			Expect(caldavSrv.calls.Load()).To(Equal(int32(0)))
		})
	})

	When("the caldavServer has not been configured", func() {
		var bareSite *server.Site
		var bareRouter *gin.Engine
		var bareW *httptest.ResponseRecorder

		BeforeEach(func() {
			var err error
			logger := lumber.NewConsoleLogger(lumber.WARN)
			bareSite, err = server.NewSite(tmpDir, "testpage", 0, "secret", logger)
			Expect(err).NotTo(HaveOccurred())
			// Note: SetCalDAVServer never called -> caldavServer == nil.
			bareRouter = bareSite.GinRouter()
			bareW = httptest.NewRecorder()
		})

		When("a PROPFIND arrives", func() {
			BeforeEach(func() {
				req := httptest.NewRequest("PROPFIND", "/shopping/", nil)
				bareRouter.ServeHTTP(bareW, req)
			})

			It("should not crash", func() {
				// nil caldavServer must be tolerated — the gateway
				// should fall through to regular routes (which return
				// 404 for PROPFIND since no Gin route matches).
				Expect(bareW.Code).NotTo(Equal(caldavSentinelStatus))
			})
		})
	})
})
