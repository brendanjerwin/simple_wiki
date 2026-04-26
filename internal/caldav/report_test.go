//revive:disable:dot-imports
package caldav_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/caldav"
	"github.com/brendanjerwin/simple_wiki/tailscale"
)

// reportBackend is a CalendarBackend stub used by the REPORT tests.
// Each method dispatches to a configurable function so individual specs
// can stage exactly the data shape they need.
//
//revive:disable:exported Internal test helper.
type reportBackend struct {
	getItemFn   func(ctx context.Context, page, list, uid string) (caldav.CalendarItem, error)
	listItemsFn func(ctx context.Context, page, list string) (caldav.CalendarCollection, []caldav.CalendarItem, error)
}

func (*reportBackend) ListCollections(_ context.Context, _ string) ([]caldav.CalendarCollection, error) {
	return nil, nil
}

func (*reportBackend) GetCollection(_ context.Context, _, _ string) (caldav.CalendarCollection, error) {
	return caldav.CalendarCollection{}, caldav.ErrCollectionNotFound
}

func (r *reportBackend) ListItems(ctx context.Context, page, list string) (caldav.CalendarCollection, []caldav.CalendarItem, error) {
	if r.listItemsFn != nil {
		return r.listItemsFn(ctx, page, list)
	}
	return caldav.CalendarCollection{}, nil, caldav.ErrCollectionNotFound
}

func (r *reportBackend) GetItem(ctx context.Context, page, list, uid string) (caldav.CalendarItem, error) {
	if r.getItemFn != nil {
		return r.getItemFn(ctx, page, list, uid)
	}
	return caldav.CalendarItem{}, caldav.ErrItemNotFound
}

func (*reportBackend) PutItem(_ context.Context, _, _, _ string, _ []byte, _, _ string, _ tailscale.IdentityValue) (string, bool, error) {
	return "", false, errors.New("PutItem not used in report tests")
}

func (*reportBackend) DeleteItem(_ context.Context, _, _, _, _ string, _ tailscale.IdentityValue) error {
	return errors.New("DeleteItem not used in report tests")
}

//revive:disable-next-line:function-result-limit Mirrors CalendarBackend.SyncCollection's interface signature.
func (*reportBackend) SyncCollection(_ context.Context, _, _, _ string) (string, []caldav.CalendarItem, []string, error) {
	return "", nil, nil, errors.New("SyncCollection not used in report tests")
}

// reportRequest builds an authenticated REPORT request with the given
// XML body. Depth is set to "0" by default (the value DAVx5 uses on
// calendar-multiget); callers can override via reportRequestDepth.
func reportRequest(target, body string) *http.Request {
	req := httptest.NewRequest("REPORT", target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	req.Header.Set("Depth", "0")
	id := tailscale.NewIdentity("tester@example.com", "Tester", "phone")
	return req.WithContext(tailscale.ContextWithIdentity(req.Context(), id))
}

// multigetTwoHrefsBody is a calendar-multiget body listing two valid
// item hrefs. Both ULIDs validate; whether the items exist depends on
// the backend stub the test wires up.
const multigetTwoHrefsBody = `<?xml version="1.0" encoding="utf-8"?>
<C:calendar-multiget xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:getetag/>
    <C:calendar-data/>
  </D:prop>
  <D:href>/shopping/this-week/01HZ8K7Q9X1V2N3R4T5Y6Z7B8C.ics</D:href>
  <D:href>/shopping/this-week/01HZ8K7Q9X1V2N3R4T5Y6Z7B8D.ics</D:href>
</C:calendar-multiget>`

// multigetUnknownHrefBody is a calendar-multiget body with one valid
// ULID-shaped href that the backend stub will report as not-found.
const multigetUnknownHrefBody = `<?xml version="1.0" encoding="utf-8"?>
<C:calendar-multiget xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:getetag/>
    <C:calendar-data/>
  </D:prop>
  <D:href>/shopping/this-week/01HZ8K7Q9X1V2N3R4T5Y6Z7B8E.ics</D:href>
</C:calendar-multiget>`

// queryNoFilterBody is a calendar-query body with the trivial
// `VCALENDAR > VTODO` component-filter — matches every item we serve.
const queryNoFilterBody = `<?xml version="1.0" encoding="utf-8"?>
<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:getetag/>
    <C:calendar-data/>
  </D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VTODO"/>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`

var _ = Describe("Server.serveREPORT calendar-multiget", func() {
	When("the request is anonymous", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{Backend: &reportBackend{}}
			rec = httptest.NewRecorder()
			req := httptest.NewRequest("REPORT", "/shopping/this-week/", strings.NewReader(multigetTwoHrefsBody))
			req.Header.Set("Content-Type", "application/xml; charset=utf-8")
			req = req.WithContext(tailscale.ContextWithIdentity(req.Context(), tailscale.Anonymous))
			server.ServeREPORTForTest(rec, req)
		})

		It("should return 403 Forbidden", func() {
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})
	})

	When("two known hrefs are requested", func() {
		var rec *httptest.ResponseRecorder
		var capturedUIDs []string

		BeforeEach(func() {
			capturedUIDs = nil
			backend := &reportBackend{
				getItemFn: func(_ context.Context, _, _, uid string) (caldav.CalendarItem, error) {
					capturedUIDs = append(capturedUIDs, uid)
					return caldav.CalendarItem{
						UID:       uid,
						ETag:      `W/"2026-04-25T12:00:00Z"`,
						ICalBytes: []byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n"),
					}, nil
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := reportRequest("/shopping/this-week/", multigetTwoHrefsBody)
			server.ServeREPORTForTest(rec, req)
		})

		It("should return 207 Multi-Status", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should set Content-Type to application/xml; charset=utf-8", func() {
			Expect(rec.Header().Get("Content-Type")).To(Equal("application/xml; charset=utf-8"))
		})

		It("should emit a multistatus root element", func() {
			Expect(rec.Body.String()).To(ContainSubstring("multistatus"))
		})

		It("should include the first href in the response", func() {
			Expect(rec.Body.String()).To(ContainSubstring("/shopping/this-week/01HZ8K7Q9X1V2N3R4T5Y6Z7B8C.ics"))
		})

		It("should include the second href in the response", func() {
			Expect(rec.Body.String()).To(ContainSubstring("/shopping/this-week/01HZ8K7Q9X1V2N3R4T5Y6Z7B8D.ics"))
		})

		It("should include the getetag value for each item", func() {
			Expect(rec.Body.String()).To(ContainSubstring(`W/&#34;2026-04-25T12:00:00Z&#34;`))
		})

		It("should include the calendar-data body for each item", func() {
			Expect(rec.Body.String()).To(ContainSubstring("BEGIN:VCALENDAR"))
		})

		It("should call the backend once per href", func() {
			Expect(capturedUIDs).To(HaveLen(2))
		})

		It("should pass the first uid to the backend", func() {
			Expect(capturedUIDs).To(ContainElement("01HZ8K7Q9X1V2N3R4T5Y6Z7B8C"))
		})

		It("should pass the second uid to the backend", func() {
			Expect(capturedUIDs).To(ContainElement("01HZ8K7Q9X1V2N3R4T5Y6Z7B8D"))
		})
	})

	When("an href in the multiget references an unknown item", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			backend := &reportBackend{
				getItemFn: func(_ context.Context, _, _, _ string) (caldav.CalendarItem, error) {
					return caldav.CalendarItem{}, caldav.ErrItemNotFound
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := reportRequest("/shopping/this-week/", multigetUnknownHrefBody)
			server.ServeREPORTForTest(rec, req)
		})

		It("should still return 207 Multi-Status", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should include the requested href", func() {
			Expect(rec.Body.String()).To(ContainSubstring("/shopping/this-week/01HZ8K7Q9X1V2N3R4T5Y6Z7B8E.ics"))
		})

		It("should mark that response with status 404", func() {
			Expect(rec.Body.String()).To(ContainSubstring("HTTP/1.1 404 Not Found"))
		})
	})

	When("an href in the multiget references a tombstoned item", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			backend := &reportBackend{
				getItemFn: func(_ context.Context, _, _, _ string) (caldav.CalendarItem, error) {
					return caldav.CalendarItem{}, caldav.ErrItemDeleted
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := reportRequest("/shopping/this-week/", multigetUnknownHrefBody)
			server.ServeREPORTForTest(rec, req)
		})

		It("should still return 207 Multi-Status", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should mark that response with status 404", func() {
			Expect(rec.Body.String()).To(ContainSubstring("HTTP/1.1 404 Not Found"))
		})
	})

	When("the request body is not parseable XML", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{Backend: &reportBackend{}}
			rec = httptest.NewRecorder()
			req := reportRequest("/shopping/this-week/", "not xml at all")
			server.ServeREPORTForTest(rec, req)
		})

		It("should return 400 Bad Request", func() {
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})
	})

	When("the root element is unrecognized", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{Backend: &reportBackend{}}
			rec = httptest.NewRecorder()
			req := reportRequest("/shopping/this-week/", `<?xml version="1.0"?><something xmlns="DAV:"/>`)
			server.ServeREPORTForTest(rec, req)
		})

		It("should return 400 Bad Request", func() {
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})
	})

	When("the root element is sync-collection", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			body := `<?xml version="1.0"?><sync-collection xmlns="DAV:"><sync-token/></sync-collection>`
			server := &caldav.Server{Backend: &reportBackend{}}
			rec = httptest.NewRecorder()
			req := reportRequest("/shopping/this-week/", body)
			server.ServeREPORTForTest(rec, req)
		})

		It("should return 501 Not Implemented", func() {
			Expect(rec.Code).To(Equal(http.StatusNotImplemented))
		})
	})

	When("an href references an item on a different page than another href", func() {
		var rec *httptest.ResponseRecorder
		var capturedPages []string

		BeforeEach(func() {
			capturedPages = nil
			body := `<?xml version="1.0" encoding="utf-8"?>
<C:calendar-multiget xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:getetag/>
    <C:calendar-data/>
  </D:prop>
  <D:href>/shopping/this-week/01HZ8K7Q9X1V2N3R4T5Y6Z7B8C.ics</D:href>
  <D:href>/garage/projects/01HZ8K7Q9X1V2N3R4T5Y6Z7B8D.ics</D:href>
</C:calendar-multiget>`
			backend := &reportBackend{
				getItemFn: func(_ context.Context, page, _, uid string) (caldav.CalendarItem, error) {
					capturedPages = append(capturedPages, page)
					return caldav.CalendarItem{
						UID:       uid,
						ETag:      `W/"x"`,
						ICalBytes: []byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n"),
					}, nil
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := reportRequest("/shopping/this-week/", body)
			server.ServeREPORTForTest(rec, req)
		})

		It("should still call the backend with the page from each href", func() {
			Expect(capturedPages).To(ContainElements("shopping", "garage"))
		})
	})

	When("an href in the multiget is malformed", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			body := `<?xml version="1.0" encoding="utf-8"?>
<C:calendar-multiget xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:getetag/>
    <C:calendar-data/>
  </D:prop>
  <D:href>/shopping/this-week/not-a-ulid.ics</D:href>
</C:calendar-multiget>`
			server := &caldav.Server{Backend: &reportBackend{}}
			rec = httptest.NewRecorder()
			req := reportRequest("/shopping/this-week/", body)
			server.ServeREPORTForTest(rec, req)
		})

		It("should return 207 Multi-Status", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should mark that href's response with status 404", func() {
			Expect(rec.Body.String()).To(ContainSubstring("HTTP/1.1 404 Not Found"))
		})
	})
})

var _ = Describe("Server.serveREPORT calendar-query", func() {
	When("the request is anonymous", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{Backend: &reportBackend{}}
			rec = httptest.NewRecorder()
			req := httptest.NewRequest("REPORT", "/shopping/this-week/", strings.NewReader(queryNoFilterBody))
			req.Header.Set("Content-Type", "application/xml; charset=utf-8")
			req = req.WithContext(tailscale.ContextWithIdentity(req.Context(), tailscale.Anonymous))
			server.ServeREPORTForTest(rec, req)
		})

		It("should return 403 Forbidden", func() {
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})
	})

	When("the collection has two items", func() {
		var rec *httptest.ResponseRecorder
		var capturedPage, capturedList string

		BeforeEach(func() {
			backend := &reportBackend{
				listItemsFn: func(_ context.Context, page, list string) (caldav.CalendarCollection, []caldav.CalendarItem, error) {
					capturedPage = page
					capturedList = list
					return caldav.CalendarCollection{
							Page: page, ListName: list, DisplayName: list,
						}, []caldav.CalendarItem{
							{
								UID:       "01HZ8K7Q9X1V2N3R4T5Y6Z7B8C",
								ETag:      `W/"2026-04-25T12:00:00Z"`,
								ICalBytes: []byte("BEGIN:VCALENDAR\r\nUID:item-1\r\nEND:VCALENDAR\r\n"),
							},
							{
								UID:       "01HZ8K7Q9X1V2N3R4T5Y6Z7B8D",
								ETag:      `W/"2026-04-25T13:00:00Z"`,
								ICalBytes: []byte("BEGIN:VCALENDAR\r\nUID:item-2\r\nEND:VCALENDAR\r\n"),
							},
						}, nil
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := reportRequest("/shopping/this-week/", queryNoFilterBody)
			server.ServeREPORTForTest(rec, req)
		})

		It("should return 207 Multi-Status", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should set Content-Type to application/xml; charset=utf-8", func() {
			Expect(rec.Header().Get("Content-Type")).To(Equal("application/xml; charset=utf-8"))
		})

		It("should pass the page from the URL to the backend", func() {
			Expect(capturedPage).To(Equal("shopping"))
		})

		It("should pass the list from the URL to the backend", func() {
			Expect(capturedList).To(Equal("this-week"))
		})

		It("should include an href for the first item", func() {
			Expect(rec.Body.String()).To(ContainSubstring("/shopping/this-week/01HZ8K7Q9X1V2N3R4T5Y6Z7B8C.ics"))
		})

		It("should include an href for the second item", func() {
			Expect(rec.Body.String()).To(ContainSubstring("/shopping/this-week/01HZ8K7Q9X1V2N3R4T5Y6Z7B8D.ics"))
		})

		It("should include the getetag for each item", func() {
			Expect(rec.Body.String()).To(ContainSubstring(`W/&#34;2026-04-25T12:00:00Z&#34;`))
			Expect(rec.Body.String()).To(ContainSubstring(`W/&#34;2026-04-25T13:00:00Z&#34;`))
		})

		It("should include the calendar-data body for each item", func() {
			Expect(rec.Body.String()).To(ContainSubstring("UID:item-1"))
			Expect(rec.Body.String()).To(ContainSubstring("UID:item-2"))
		})
	})

	When("the collection is empty", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			backend := &reportBackend{
				listItemsFn: func(_ context.Context, page, list string) (caldav.CalendarCollection, []caldav.CalendarItem, error) {
					return caldav.CalendarCollection{Page: page, ListName: list, DisplayName: list}, nil, nil
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := reportRequest("/shopping/this-week/", queryNoFilterBody)
			server.ServeREPORTForTest(rec, req)
		})

		It("should return 207 Multi-Status", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should not include any item hrefs", func() {
			Expect(rec.Body.String()).NotTo(ContainSubstring(".ics"))
		})
	})

	When("the URL points at a collection that does not exist", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			backend := &reportBackend{
				listItemsFn: func(_ context.Context, _, _ string) (caldav.CalendarCollection, []caldav.CalendarItem, error) {
					return caldav.CalendarCollection{}, nil, caldav.ErrCollectionNotFound
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := reportRequest("/shopping/this-week/", queryNoFilterBody)
			server.ServeREPORTForTest(rec, req)
		})

		It("should return 404 Not Found", func() {
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})
	})

	When("the URL is a page rather than a collection", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{Backend: &reportBackend{}}
			rec = httptest.NewRecorder()
			req := reportRequest("/shopping", queryNoFilterBody)
			server.ServeREPORTForTest(rec, req)
		})

		It("should return 404 Not Found", func() {
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})
	})

	When("the URL is malformed", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{Backend: &reportBackend{}}
			rec = httptest.NewRecorder()
			req := reportRequest("/", queryNoFilterBody)
			server.ServeREPORTForTest(rec, req)
		})

		It("should return 404 Not Found", func() {
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})
	})

	When("the calendar-query body is malformed", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{Backend: &reportBackend{}}
			rec = httptest.NewRecorder()
			req := reportRequest("/shopping/this-week/", "<not-xml")
			server.ServeREPORTForTest(rec, req)
		})

		It("should return 400 Bad Request", func() {
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})
	})

	When("the calendar-query has no filter element", func() {
		var rec *httptest.ResponseRecorder
		var listCalled bool

		BeforeEach(func() {
			body := `<?xml version="1.0" encoding="utf-8"?>
<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:getetag/>
    <C:calendar-data/>
  </D:prop>
</C:calendar-query>`
			backend := &reportBackend{
				listItemsFn: func(_ context.Context, page, list string) (caldav.CalendarCollection, []caldav.CalendarItem, error) {
					listCalled = true
					return caldav.CalendarCollection{Page: page, ListName: list, DisplayName: list}, []caldav.CalendarItem{
						{
							UID:       "01HZ8K7Q9X1V2N3R4T5Y6Z7B8C",
							ETag:      `W/"x"`,
							ICalBytes: []byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n"),
						},
					}, nil
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := reportRequest("/shopping/this-week/", body)
			server.ServeREPORTForTest(rec, req)
		})

		It("should return 207 Multi-Status", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should call the backend ListItems", func() {
			Expect(listCalled).To(BeTrue())
		})

		It("should include the item href", func() {
			Expect(rec.Body.String()).To(ContainSubstring("/shopping/this-week/01HZ8K7Q9X1V2N3R4T5Y6Z7B8C.ics"))
		})
	})
})
