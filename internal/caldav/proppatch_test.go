//revive:disable:dot-imports
package caldav_test

import (
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/caldav"
	"github.com/brendanjerwin/simple_wiki/tailscale"
)

// proppatchSetDisplayname is the body iOS Reminders POSTs during
// account setup to write displayname. Captured from real iOS traffic
// to keep the test pinned to the wire shape we have to support.
const proppatchSetDisplayname = `<?xml version="1.0" encoding="UTF-8"?>
<A:propertyupdate xmlns:A="DAV:">
  <A:set>
    <A:prop>
      <A:displayname>Grocery</A:displayname>
    </A:prop>
  </A:set>
</A:propertyupdate>`

// proppatchSetCalendarColor sets calendar-color in the CalDAV
// namespace. iOS sends this immediately after displayname.
const proppatchSetCalendarColor = `<?xml version="1.0" encoding="UTF-8"?>
<A:propertyupdate xmlns:A="DAV:" xmlns:B="urn:ietf:params:xml:ns:caldav">
  <A:set>
    <A:prop>
      <B:calendar-color>#FF0000</B:calendar-color>
    </A:prop>
  </A:set>
</A:propertyupdate>`

// proppatchSetUnknown sets a property the wiki has no business storing.
// We expect a per-prop 403 Forbidden in the response — never a
// top-level 405 or 5xx, which would scare iOS off.
const proppatchSetUnknown = `<?xml version="1.0" encoding="UTF-8"?>
<A:propertyupdate xmlns:A="DAV:">
  <A:set>
    <A:prop>
      <A:owner>nobody</A:owner>
    </A:prop>
  </A:set>
</A:propertyupdate>`

// proppatchSetMixed sets one cosmetic prop (displayname → 200) and one
// unknown prop (owner → 403). Expect both propstats in the body.
const proppatchSetMixed = `<?xml version="1.0" encoding="UTF-8"?>
<A:propertyupdate xmlns:A="DAV:">
  <A:set>
    <A:prop>
      <A:displayname>Grocery</A:displayname>
      <A:owner>nobody</A:owner>
    </A:prop>
  </A:set>
</A:propertyupdate>`

// authedPROPPATCH builds an authenticated PROPPATCH request with the
// given body. PROPPATCH bodies are application/xml.
func authedPROPPATCH(target, body string) *http.Request {
	req := httptest.NewRequest("PROPPATCH", target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	id := tailscale.NewIdentity("tester@example.com", "Tester", "phone")
	return req.WithContext(tailscale.ContextWithIdentity(req.Context(), id))
}

var _ = Describe("Server.servePROPPATCH", func() {
	var server *caldav.Server

	BeforeEach(func() {
		server = &caldav.Server{Backend: &fakeServerBackend{}}
	})

	When("an iOS-style PROPPATCH targets a collection", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			rec = httptest.NewRecorder()
			req := authedPROPPATCH("/shopping/grocery/", proppatchSetDisplayname)
			server.ServePROPPATCHForTest(rec, req)
		})

		It("should return 207 Multi-Status, never 405", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should return application/xml", func() {
			Expect(rec.Header().Get("Content-Type")).To(ContainSubstring("xml"))
		})

		It("should echo the displayname element back inside <prop>", func() {
			Expect(rec.Body.String()).To(ContainSubstring("<displayname"))
		})

		It("should mark displayname with HTTP 200 OK (silently accepted)", func() {
			Expect(rec.Body.String()).To(ContainSubstring("HTTP/1.1 200 OK"))
		})
	})

	When("the PROPPATCH sets calendar-color", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			rec = httptest.NewRecorder()
			req := authedPROPPATCH("/shopping/grocery/", proppatchSetCalendarColor)
			server.ServePROPPATCHForTest(rec, req)
		})

		It("should return 207 Multi-Status", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should mark calendar-color with HTTP 200 OK", func() {
			Expect(rec.Body.String()).To(ContainSubstring("HTTP/1.1 200 OK"))
		})
	})

	When("the PROPPATCH sets a property the wiki refuses to store", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			rec = httptest.NewRecorder()
			req := authedPROPPATCH("/shopping/grocery/", proppatchSetUnknown)
			server.ServePROPPATCHForTest(rec, req)
		})

		It("should still return 207 Multi-Status (never 405 / 500)", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should mark the unknown prop with HTTP 403 Forbidden", func() {
			Expect(rec.Body.String()).To(ContainSubstring("HTTP/1.1 403 Forbidden"))
		})

		It("should not emit a HTTP 200 OK propstat (no cosmetic props in this body)", func() {
			Expect(rec.Body.String()).NotTo(ContainSubstring("HTTP/1.1 200 OK"))
		})
	})

	When("the PROPPATCH sets one cosmetic prop and one unknown prop", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			rec = httptest.NewRecorder()
			req := authedPROPPATCH("/shopping/grocery/", proppatchSetMixed)
			server.ServePROPPATCHForTest(rec, req)
		})

		It("should return 207 Multi-Status", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should mark displayname with HTTP 200 OK", func() {
			Expect(rec.Body.String()).To(ContainSubstring("HTTP/1.1 200 OK"))
		})

		It("should mark the unknown prop with HTTP 403 Forbidden", func() {
			Expect(rec.Body.String()).To(ContainSubstring("HTTP/1.1 403 Forbidden"))
		})
	})

	When("the request is anonymous", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			rec = httptest.NewRecorder()
			req := httptest.NewRequest("PROPPATCH", "/shopping/grocery/", strings.NewReader(proppatchSetDisplayname))
			req.Header.Set("Content-Type", "application/xml; charset=utf-8")
			server.ServePROPPATCHForTest(rec, req)
		})

		It("should return 403 Forbidden", func() {
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})
	})

	When("the URL targets a single item .ics resource", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			rec = httptest.NewRecorder()
			req := authedPROPPATCH("/shopping/grocery/01HZ8K7Q9X1V2N3R4T5Y6Z7B8C.ics", proppatchSetDisplayname)
			server.ServePROPPATCHForTest(rec, req)
		})

		It("should return 400 Bad Request — PROPPATCH is collection-only", func() {
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})
	})

	When("the body is empty", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			rec = httptest.NewRecorder()
			req := authedPROPPATCH("/shopping/grocery/", "")
			server.ServePROPPATCHForTest(rec, req)
		})

		It("should still return 207 Multi-Status (don't fail iOS account setup)", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})
	})
})
