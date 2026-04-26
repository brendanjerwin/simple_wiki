//revive:disable:dot-imports
package caldav_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/caldav"
	"github.com/brendanjerwin/simple_wiki/tailscale"
)

// propfindBackend is a CalendarBackend stub used by the PROPFIND
// tests. Each method returns a configurable canned response so each
// spec can stage exactly the data shape it cares about.
//
//revive:disable:exported Internal test helper.
type propfindBackend struct {
	listCollectionsFn func(ctx context.Context, page string) ([]caldav.CalendarCollection, error)
	getCollectionFn   func(ctx context.Context, page, list string) (caldav.CalendarCollection, error)
	listItemsFn       func(ctx context.Context, page, list string) (caldav.CalendarCollection, []caldav.CalendarItem, error)
	getItemFn         func(ctx context.Context, page, list, uid string) (caldav.CalendarItem, error)
}

func (p *propfindBackend) ListCollections(ctx context.Context, page string) ([]caldav.CalendarCollection, error) {
	if p.listCollectionsFn != nil {
		return p.listCollectionsFn(ctx, page)
	}
	return nil, nil
}

func (p *propfindBackend) GetCollection(ctx context.Context, page, list string) (caldav.CalendarCollection, error) {
	if p.getCollectionFn != nil {
		return p.getCollectionFn(ctx, page, list)
	}
	return caldav.CalendarCollection{}, caldav.ErrCollectionNotFound
}

func (p *propfindBackend) ListItems(ctx context.Context, page, list string) (caldav.CalendarCollection, []caldav.CalendarItem, error) {
	if p.listItemsFn != nil {
		return p.listItemsFn(ctx, page, list)
	}
	return caldav.CalendarCollection{}, nil, caldav.ErrCollectionNotFound
}

func (p *propfindBackend) GetItem(ctx context.Context, page, list, uid string) (caldav.CalendarItem, error) {
	if p.getItemFn != nil {
		return p.getItemFn(ctx, page, list, uid)
	}
	return caldav.CalendarItem{}, caldav.ErrItemNotFound
}

func (*propfindBackend) PutItem(_ context.Context, _, _, _ string, _ []byte, _, _ string, _ tailscale.IdentityValue) (string, bool, error) {
	return "", false, nil
}

func (*propfindBackend) DeleteItem(_ context.Context, _, _, _, _ string, _ tailscale.IdentityValue) error {
	return nil
}

//revive:disable-next-line:function-result-limit Mirrors CalendarBackend.SyncCollection's interface signature.
func (*propfindBackend) SyncCollection(_ context.Context, _, _, _ string) (string, []caldav.CalendarItem, []string, error) {
	return "", nil, nil, nil
}

// propfindRequest builds an authenticated PROPFIND request with the
// given Depth header and (optional) XML body.
func propfindRequest(target, depth, body string) *http.Request {
	var bodyReader *strings.Reader
	if body == "" {
		bodyReader = strings.NewReader("")
	} else {
		bodyReader = strings.NewReader(body)
	}
	req := httptest.NewRequest("PROPFIND", target, bodyReader)
	if depth != "" {
		req.Header.Set("Depth", depth)
	}
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	id := tailscale.NewIdentity("tester@example.com", "Tester", "phone")
	return req.WithContext(tailscale.ContextWithIdentity(req.Context(), id))
}

// allpropBody is the body Apple Reminders sends on its initial probe:
// no `<prop>` selection, just `<allprop/>`. Servers respond with every
// property they know about. Most of the PROPFIND tests use this so
// they don't need to enumerate properties one by one.
const allpropBody = `<?xml version="1.0" encoding="utf-8"?>
<propfind xmlns="DAV:">
  <allprop/>
</propfind>`

var _ = Describe("Server.servePROPFIND", func() {
	When("the request is anonymous", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{Backend: &propfindBackend{}}
			rec = httptest.NewRecorder()
			req := httptest.NewRequest("PROPFIND", "/shopping", strings.NewReader(allpropBody))
			req.Header.Set("Depth", "0")
			req = req.WithContext(tailscale.ContextWithIdentity(req.Context(), tailscale.Anonymous))
			server.ServePROPFINDForTest(rec, req)
		})

		It("should return 403 Forbidden", func() {
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})
	})

	When("the URL is the home-set with Depth: 0", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{Backend: &propfindBackend{}}
			rec = httptest.NewRecorder()
			req := propfindRequest("/shopping", "0", allpropBody)
			server.ServePROPFINDForTest(rec, req)
		})

		It("should return 207 Multi-Status", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should set Content-Type to application/xml; charset=utf-8", func() {
			Expect(rec.Header().Get("Content-Type")).To(Equal("application/xml; charset=utf-8"))
		})

		It("should return a multistatus root element", func() {
			Expect(rec.Body.String()).To(ContainSubstring("multistatus"))
		})

		It("should include a response href for the home-set", func() {
			Expect(rec.Body.String()).To(ContainSubstring("<href>/shopping/</href>"))
		})

		It("should advertise calendar-home-set pointing back to the page URL", func() {
			Expect(rec.Body.String()).To(ContainSubstring("calendar-home-set"))
			Expect(rec.Body.String()).To(ContainSubstring("/shopping/"))
		})

		It("should advertise current-user-principal", func() {
			Expect(rec.Body.String()).To(ContainSubstring("current-user-principal"))
		})

		It("should set displayname to the page name", func() {
			Expect(rec.Body.String()).To(ContainSubstring("<displayname>shopping</displayname>"))
		})

		It("should not enumerate child collections at Depth:0", func() {
			Expect(rec.Body.String()).NotTo(ContainSubstring("<href>/shopping/this-week/</href>"))
		})
	})

	When("the URL is the home-set with Depth: 1", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{Backend: &propfindBackend{
				listCollectionsFn: func(_ context.Context, page string) ([]caldav.CalendarCollection, error) {
					return []caldav.CalendarCollection{
						{
							Page:        page,
							ListName:    "this-week",
							DisplayName: "this-week",
							SyncToken:   "http://simple-wiki.local/ns/sync/7",
							CTag:        `"2026-04-25T13:00:00Z"`,
						},
						{
							Page:        page,
							ListName:    "next-week",
							DisplayName: "next-week",
							SyncToken:   "http://simple-wiki.local/ns/sync/3",
							CTag:        `"2026-04-25T12:00:00Z"`,
						},
					}, nil
				},
			}}
			rec = httptest.NewRecorder()
			req := propfindRequest("/shopping", "1", allpropBody)
			server.ServePROPFINDForTest(rec, req)
		})

		It("should return 207 Multi-Status", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should include a response for the home-set itself", func() {
			Expect(rec.Body.String()).To(ContainSubstring("<href>/shopping/</href>"))
		})

		It("should include a response for the first child collection", func() {
			Expect(rec.Body.String()).To(ContainSubstring("<href>/shopping/this-week/</href>"))
		})

		It("should include a response for the second child collection", func() {
			Expect(rec.Body.String()).To(ContainSubstring("<href>/shopping/next-week/</href>"))
		})

		It("should advertise getctag on each child collection", func() {
			Expect(rec.Body.String()).To(ContainSubstring("getctag"))
			Expect(rec.Body.String()).To(ContainSubstring(`"2026-04-25T13:00:00Z"`))
		})

		It("should advertise sync-token on each child collection", func() {
			Expect(rec.Body.String()).To(ContainSubstring("sync-token"))
			Expect(rec.Body.String()).To(ContainSubstring("http://simple-wiki.local/ns/sync/7"))
		})

		It("should advertise supported-calendar-component-set with VTODO", func() {
			Expect(rec.Body.String()).To(ContainSubstring("supported-calendar-component-set"))
			Expect(rec.Body.String()).To(ContainSubstring(`name="VTODO"`))
		})

		It("should advertise resourcetype as collection + calendar on children", func() {
			body := rec.Body.String()
			Expect(body).To(ContainSubstring("<collection"))
			Expect(body).To(ContainSubstring("<calendar"))
		})
	})

	When("the URL is a collection with Depth: 0", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{Backend: &propfindBackend{
				getCollectionFn: func(_ context.Context, page, list string) (caldav.CalendarCollection, error) {
					return caldav.CalendarCollection{
						Page:        page,
						ListName:    list,
						DisplayName: list,
						SyncToken:   "http://simple-wiki.local/ns/sync/11",
						CTag:        `"2026-04-25T13:00:00Z"`,
					}, nil
				},
			}}
			rec = httptest.NewRecorder()
			req := propfindRequest("/shopping/this-week/", "0", allpropBody)
			server.ServePROPFINDForTest(rec, req)
		})

		It("should return 207 Multi-Status", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should include a response href for the collection", func() {
			Expect(rec.Body.String()).To(ContainSubstring("<href>/shopping/this-week/</href>"))
		})

		It("should advertise getctag on the collection", func() {
			Expect(rec.Body.String()).To(ContainSubstring("getctag"))
			Expect(rec.Body.String()).To(ContainSubstring(`"2026-04-25T13:00:00Z"`))
		})

		It("should advertise sync-token on the collection", func() {
			Expect(rec.Body.String()).To(ContainSubstring("http://simple-wiki.local/ns/sync/11"))
		})

		It("should advertise resourcetype as collection + calendar", func() {
			body := rec.Body.String()
			Expect(body).To(ContainSubstring("<collection"))
			Expect(body).To(ContainSubstring("<calendar"))
		})

		It("should advertise supported-calendar-component-set with VTODO", func() {
			Expect(rec.Body.String()).To(ContainSubstring(`name="VTODO"`))
		})

		It("should not enumerate items at Depth:0", func() {
			Expect(rec.Body.String()).NotTo(ContainSubstring(".ics"))
		})
	})

	When("the URL is a collection with Depth: 1", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{Backend: &propfindBackend{
				listItemsFn: func(_ context.Context, page, list string) (caldav.CalendarCollection, []caldav.CalendarItem, error) {
					col := caldav.CalendarCollection{
						Page:        page,
						ListName:    list,
						DisplayName: list,
						SyncToken:   "http://simple-wiki.local/ns/sync/11",
						CTag:        `"2026-04-25T13:00:00Z"`,
					}
					items := []caldav.CalendarItem{
						{
							UID:       "01HXAAAAAAAAAAAAAAAAAAAAAA",
							ETag:      `W/"2026-04-25T12:30:00Z"`,
							ICalBytes: []byte("BEGIN:VCALENDAR\r\nBEGIN:VTODO\r\nUID:01HXAAAAAAAAAAAAAAAAAAAAAA\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"),
						},
						{
							UID:       "01HXBBBBBBBBBBBBBBBBBBBBBB",
							ETag:      `W/"2026-04-25T12:00:00Z"`,
							ICalBytes: []byte("BEGIN:VCALENDAR\r\nBEGIN:VTODO\r\nUID:01HXBBBBBBBBBBBBBBBBBBBBBB\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"),
						},
					}
					return col, items, nil
				},
			}}
			rec = httptest.NewRecorder()
			req := propfindRequest("/shopping/this-week/", "1", allpropBody)
			server.ServePROPFINDForTest(rec, req)
		})

		It("should return 207 Multi-Status", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should include a response for the collection itself", func() {
			Expect(rec.Body.String()).To(ContainSubstring("<href>/shopping/this-week/</href>"))
		})

		It("should include a response for the first item", func() {
			Expect(rec.Body.String()).To(ContainSubstring("/shopping/this-week/01HXAAAAAAAAAAAAAAAAAAAAAA.ics"))
		})

		It("should include a response for the second item", func() {
			Expect(rec.Body.String()).To(ContainSubstring("/shopping/this-week/01HXBBBBBBBBBBBBBBBBBBBBBB.ics"))
		})

		It("should advertise getetag on each item", func() {
			body := rec.Body.String()
			Expect(body).To(ContainSubstring(`W/&#34;2026-04-25T12:30:00Z&#34;`))
			Expect(body).To(ContainSubstring(`W/&#34;2026-04-25T12:00:00Z&#34;`))
		})
	})

	When("the URL is an item with Depth: 0", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{Backend: &propfindBackend{
				getItemFn: func(_ context.Context, _, _, uid string) (caldav.CalendarItem, error) {
					return caldav.CalendarItem{
						UID:       uid,
						ETag:      `W/"2026-04-25T12:30:00Z"`,
						ICalBytes: []byte("BEGIN:VCALENDAR\r\nBEGIN:VTODO\r\nUID:" + uid + "\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"),
					}, nil
				},
			}}
			rec = httptest.NewRecorder()
			req := propfindRequest("/shopping/this-week/01HXAAAAAAAAAAAAAAAAAAAAAA.ics", "0", allpropBody)
			server.ServePROPFINDForTest(rec, req)
		})

		It("should return 207 Multi-Status", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should include a response href for the item", func() {
			Expect(rec.Body.String()).To(ContainSubstring("/shopping/this-week/01HXAAAAAAAAAAAAAAAAAAAAAA.ics"))
		})

		It("should advertise getetag on the item", func() {
			Expect(rec.Body.String()).To(ContainSubstring(`W/&#34;2026-04-25T12:30:00Z&#34;`))
		})

		It("should embed calendar-data when requested", func() {
			Expect(rec.Body.String()).To(ContainSubstring("BEGIN:VTODO"))
		})
	})

	When("the URL names an unknown collection", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{Backend: &propfindBackend{
				getCollectionFn: func(_ context.Context, _, _ string) (caldav.CalendarCollection, error) {
					return caldav.CalendarCollection{}, caldav.ErrCollectionNotFound
				},
			}}
			rec = httptest.NewRecorder()
			req := propfindRequest("/shopping/missing/", "0", allpropBody)
			server.ServePROPFINDForTest(rec, req)
		})

		It("should return 404 Not Found", func() {
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})
	})

	When("the URL names an unknown item", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{Backend: &propfindBackend{
				getItemFn: func(_ context.Context, _, _, _ string) (caldav.CalendarItem, error) {
					return caldav.CalendarItem{}, caldav.ErrItemNotFound
				},
			}}
			rec = httptest.NewRecorder()
			req := propfindRequest("/shopping/this-week/01HXAAAAAAAAAAAAAAAAAAAAAA.ics", "0", allpropBody)
			server.ServePROPFINDForTest(rec, req)
		})

		It("should return 404 Not Found", func() {
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})
	})

	When("the URL is malformed", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{Backend: &propfindBackend{}}
			rec = httptest.NewRecorder()
			req := propfindRequest("/", "0", allpropBody)
			server.ServePROPFINDForTest(rec, req)
		})

		It("should return 400 Bad Request", func() {
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})
	})
})
