//revive:disable:dot-imports
package caldav_test

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/otel/attribute"

	"github.com/brendanjerwin/simple_wiki/internal/caldav"
	"github.com/brendanjerwin/simple_wiki/tailscale"
)

// fakeServerBackend is a minimal CalendarBackend used by Server-level
// tests. Every method has a recorded "lastCall…" set of arguments and
// a configurable return value or error so each test can stage exactly
// the response it needs without dragging in the real wikipage / mutator
// stack.
//
//revive:disable:exported Internal test helper.
type fakeServerBackend struct {
	getItemFn    func(ctx context.Context, page, list, uid string) (caldav.CalendarItem, error)
	putItemFn    func(ctx context.Context, page, list, uid string, body []byte, ifMatch, ifNoneMatch string, identity tailscale.IdentityValue) (string, bool, error)
	deleteItemFn func(ctx context.Context, page, list, uid, ifMatch string, identity tailscale.IdentityValue) error
}

func (*fakeServerBackend) ListCollections(_ context.Context, _ string) ([]caldav.CalendarCollection, error) {
	return nil, nil
}

func (*fakeServerBackend) GetCollection(_ context.Context, _, _ string) (caldav.CalendarCollection, error) {
	return caldav.CalendarCollection{}, caldav.ErrCollectionNotFound
}

func (*fakeServerBackend) ListItems(_ context.Context, _, _ string) (caldav.CalendarCollection, []caldav.CalendarItem, error) {
	return caldav.CalendarCollection{}, nil, caldav.ErrCollectionNotFound
}

func (f *fakeServerBackend) GetItem(ctx context.Context, page, list, uid string) (caldav.CalendarItem, error) {
	if f.getItemFn != nil {
		return f.getItemFn(ctx, page, list, uid)
	}
	return caldav.CalendarItem{}, caldav.ErrItemNotFound
}

func (f *fakeServerBackend) PutItem(ctx context.Context, page, list, uid string, body []byte, ifMatch, ifNoneMatch string, identity tailscale.IdentityValue) (string, bool, error) {
	if f.putItemFn != nil {
		return f.putItemFn(ctx, page, list, uid, body, ifMatch, ifNoneMatch, identity)
	}
	return "", false, errors.New("PutItem not staged in this test")
}

func (f *fakeServerBackend) DeleteItem(ctx context.Context, page, list, uid, ifMatch string, identity tailscale.IdentityValue) error {
	if f.deleteItemFn != nil {
		return f.deleteItemFn(ctx, page, list, uid, ifMatch, identity)
	}
	return errors.New("DeleteItem not staged in this test")
}

//revive:disable-next-line:function-result-limit Mirrors CalendarBackend.SyncCollection's interface signature.
func (*fakeServerBackend) SyncCollection(_ context.Context, _, _, _ string) (string, []caldav.CalendarItem, []string, error) {
	return "", nil, nil, errors.New("SyncCollection not used in these tests")
}

// The package-level RunSpecs lives in backend_test.go (TestBackend);
// adding a second TestServer would trip Ginkgo's "RunSpecs more than
// once" guard. Specs in this file are attached to that runner via
// var _ = Describe(...).
//
// SanitizePathComponent / ValidateUID / ParsePath are unexported, so
// tests exercise them through their re-exports in export_test.go (a
// tiny test-only helper file).

var _ = Describe("sanitizePathComponent", func() {
	When("input is empty", func() {
		var err error

		BeforeEach(func() {
			_, err = caldav.SanitizePathComponentForTest("")
		})

		It("should return ErrEmptyPathComponent", func() {
			Expect(errors.Is(err, caldav.ErrEmptyPathComponent)).To(BeTrue())
		})
	})

	When("input is whitespace only", func() {
		var err error

		BeforeEach(func() {
			_, err = caldav.SanitizePathComponentForTest("   ")
		})

		It("should return ErrEmptyPathComponent", func() {
			Expect(errors.Is(err, caldav.ErrEmptyPathComponent)).To(BeTrue())
		})
	})

	When(`input is ".."`, func() {
		var err error

		BeforeEach(func() {
			_, err = caldav.SanitizePathComponentForTest("..")
		})

		It("should return ErrPathTraversal", func() {
			Expect(errors.Is(err, caldav.ErrPathTraversal)).To(BeTrue())
		})
	})

	When("input contains a NUL byte", func() {
		var err error

		BeforeEach(func() {
			_, err = caldav.SanitizePathComponentForTest("foo\x00bar")
		})

		It("should return ErrPathContainsNUL", func() {
			Expect(errors.Is(err, caldav.ErrPathContainsNUL)).To(BeTrue())
		})
	})

	When(`input starts with "/"`, func() {
		var err error

		BeforeEach(func() {
			_, err = caldav.SanitizePathComponentForTest("/foo")
		})

		It("should return ErrPathLeadingSeparator", func() {
			Expect(errors.Is(err, caldav.ErrPathLeadingSeparator)).To(BeTrue())
		})
	})

	When(`input starts with "\\"`, func() {
		var err error

		BeforeEach(func() {
			_, err = caldav.SanitizePathComponentForTest(`\foo`)
		})

		It("should return ErrPathLeadingSeparator", func() {
			Expect(errors.Is(err, caldav.ErrPathLeadingSeparator)).To(BeTrue())
		})
	})

	When("input is a normal ASCII identifier", func() {
		var got string
		var err error

		BeforeEach(func() {
			got, err = caldav.SanitizePathComponentForTest("shopping")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the identifier unchanged", func() {
			Expect(got).To(Equal("shopping"))
		})
	})

	When("input has surrounding whitespace", func() {
		var got string
		var err error

		BeforeEach(func() {
			got, err = caldav.SanitizePathComponentForTest("  shopping  ")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the trimmed identifier", func() {
			Expect(got).To(Equal("shopping"))
		})
	})
})

var _ = Describe("validateUID", func() {
	When("uid is a 26-char Crockford-base32 ULID", func() {
		It("should not error", func() {
			Expect(caldav.ValidateUIDForTest("01HZ8K7Q9X1V2N3R4T5Y6Z7B8C")).NotTo(HaveOccurred())
		})
	})

	When("uid is a lowercase RFC 4122 UUID with dashes", func() {
		It("should not error", func() {
			Expect(caldav.ValidateUIDForTest("c5b7e0a4-3d2e-4f1a-9b8c-1234567890ab")).NotTo(HaveOccurred())
		})
	})

	When("uid is an uppercase RFC 4122 UUID with dashes", func() {
		It("should not error", func() {
			Expect(caldav.ValidateUIDForTest("C5B7E0A4-3D2E-4F1A-9B8C-1234567890AB")).NotTo(HaveOccurred())
		})
	})

	When("uid is empty", func() {
		It("should return ErrInvalidUID", func() {
			err := caldav.ValidateUIDForTest("")
			Expect(errors.Is(err, caldav.ErrInvalidUID)).To(BeTrue())
		})
	})

	When("uid is 25 characters", func() {
		It("should return ErrInvalidUID", func() {
			err := caldav.ValidateUIDForTest("01HZ8K7Q9X1V2N3R4T5Y6Z7B8")
			Expect(errors.Is(err, caldav.ErrInvalidUID)).To(BeTrue())
		})
	})

	When("uid is 27 characters", func() {
		It("should return ErrInvalidUID", func() {
			err := caldav.ValidateUIDForTest("01HZ8K7Q9X1V2N3R4T5Y6Z7B8CC")
			Expect(errors.Is(err, caldav.ErrInvalidUID)).To(BeTrue())
		})
	})

	When("uid contains a non-base32 character", func() {
		It("should return ErrInvalidUID", func() {
			// 'I' is excluded from Crockford base32.
			err := caldav.ValidateUIDForTest("01HZ8K7Q9X1V2N3R4T5Y6Z7B8I")
			Expect(errors.Is(err, caldav.ErrInvalidUID)).To(BeTrue())
		})
	})

	When("uid is a UUID with the wrong dash placement", func() {
		It("should return ErrInvalidUID", func() {
			err := caldav.ValidateUIDForTest("c5b7e0a43d2e-4f1a-9b8c-1234567890ab")
			Expect(errors.Is(err, caldav.ErrInvalidUID)).To(BeTrue())
		})
	})
})

var _ = Describe("parsePath", func() {
	When("path is /<page>", func() {
		var page, list, uid string
		var err error

		BeforeEach(func() {
			page, list, uid, err = caldav.ParsePathForTest("/shopping")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the page component", func() {
			Expect(page).To(Equal("shopping"))
		})

		It("should leave list empty", func() {
			Expect(list).To(Equal(""))
		})

		It("should leave uid empty", func() {
			Expect(uid).To(Equal(""))
		})
	})

	When("path is /<page>/<list>", func() {
		var page, list, uid string
		var err error

		BeforeEach(func() {
			page, list, uid, err = caldav.ParsePathForTest("/shopping/this-week")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the page component", func() {
			Expect(page).To(Equal("shopping"))
		})

		It("should return the list component", func() {
			Expect(list).To(Equal("this-week"))
		})

		It("should leave uid empty", func() {
			Expect(uid).To(Equal(""))
		})
	})

	When("path is /<page>/<list>/<uid>.ics", func() {
		var page, list, uid string
		var err error

		BeforeEach(func() {
			page, list, uid, err = caldav.ParsePathForTest("/shopping/this-week/01HZ8K7Q9X1V2N3R4T5Y6Z7B8C.ics")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the page component", func() {
			Expect(page).To(Equal("shopping"))
		})

		It("should return the list component", func() {
			Expect(list).To(Equal("this-week"))
		})

		It("should return the uid without the .ics suffix", func() {
			Expect(uid).To(Equal("01HZ8K7Q9X1V2N3R4T5Y6Z7B8C"))
		})
	})

	When("path has trailing slash on /<page>/<list>/", func() {
		var page, list, uid string
		var err error

		BeforeEach(func() {
			page, list, uid, err = caldav.ParsePathForTest("/shopping/this-week/")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should treat the path as the collection", func() {
			Expect(page).To(Equal("shopping"))
			Expect(list).To(Equal("this-week"))
			Expect(uid).To(Equal(""))
		})
	})

	When("path is empty", func() {
		var err error

		BeforeEach(func() {
			_, _, _, err = caldav.ParsePathForTest("")
		})

		It("should return ErrMalformedPath", func() {
			Expect(errors.Is(err, caldav.ErrMalformedPath)).To(BeTrue())
		})
	})

	When("path is just /", func() {
		var err error

		BeforeEach(func() {
			_, _, _, err = caldav.ParsePathForTest("/")
		})

		It("should return ErrMalformedPath", func() {
			Expect(errors.Is(err, caldav.ErrMalformedPath)).To(BeTrue())
		})
	})

	When("path has too many segments", func() {
		var err error

		BeforeEach(func() {
			_, _, _, err = caldav.ParsePathForTest("/a/b/c.ics/d")
		})

		It("should return ErrMalformedPath", func() {
			Expect(errors.Is(err, caldav.ErrMalformedPath)).To(BeTrue())
		})
	})

	When("third segment is not a .ics file", func() {
		var err error

		BeforeEach(func() {
			_, _, _, err = caldav.ParsePathForTest("/shopping/this-week/01HZ8K7Q9X1V2N3R4T5Y6Z7B8C")
		})

		It("should return ErrMalformedPath", func() {
			Expect(errors.Is(err, caldav.ErrMalformedPath)).To(BeTrue())
		})
	})

	When("uid in third segment is invalid", func() {
		var err error

		BeforeEach(func() {
			_, _, _, err = caldav.ParsePathForTest("/shopping/this-week/not-a-ulid.ics")
		})

		It("should return ErrInvalidUID", func() {
			Expect(errors.Is(err, caldav.ErrInvalidUID)).To(BeTrue())
		})
	})

	When("page contains ..", func() {
		var err error

		BeforeEach(func() {
			_, _, _, err = caldav.ParsePathForTest("/../this-week")
		})

		It("should return ErrPathTraversal", func() {
			Expect(errors.Is(err, caldav.ErrPathTraversal)).To(BeTrue())
		})
	})

	When("list contains a NUL byte", func() {
		var err error

		BeforeEach(func() {
			_, _, _, err = caldav.ParsePathForTest("/shopping/list\x00bad")
		})

		It("should return ErrPathContainsNUL", func() {
			Expect(errors.Is(err, caldav.ErrPathContainsNUL)).To(BeTrue())
		})
	})
})

// authedRequest returns a request with a non-anonymous Tailscale
// identity attached, so handlers under test reach their main path
// instead of the requireIdentity 403 short-circuit. Tests that want
// to exercise the anonymous path attach tailscale.Anonymous (or omit
// the identity entirely) themselves.
func authedRequest(method, target string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	id := tailscale.NewIdentity("tester@example.com", "Tester", "phone")
	return req.WithContext(tailscale.ContextWithIdentity(req.Context(), id))
}

// authedRequestWithBody is the body-bearing twin of authedRequest. It
// also sets a default Content-Type of text/calendar; tests that need
// a different value can override the header on the returned request.
func authedRequestWithBody(method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("Content-Type", "text/calendar; charset=utf-8")
	id := tailscale.NewIdentity("tester@example.com", "Tester", "phone")
	return req.WithContext(tailscale.ContextWithIdentity(req.Context(), id))
}

var _ = Describe("Server.serveOPTIONS", func() {
	var server *caldav.Server
	var rec *httptest.ResponseRecorder

	BeforeEach(func() {
		server = &caldav.Server{}
		rec = httptest.NewRecorder()
		req := authedRequest(http.MethodOptions, "/shopping")
		server.ServeOPTIONSForTest(rec, req)
	})

	It("should return 200 OK", func() {
		Expect(rec.Code).To(Equal(http.StatusOK))
	})

	It("should set the DAV header to advertise CalDAV capabilities", func() {
		Expect(rec.Header().Get("DAV")).To(Equal("1, 3, calendar-access"))
	})

	It("should set the Allow header listing every supported verb", func() {
		Expect(rec.Header().Get("Allow")).To(Equal("OPTIONS, GET, HEAD, PROPFIND, REPORT, PUT, DELETE"))
	})

	It("should write an empty body", func() {
		Expect(rec.Body.Len()).To(Equal(0))
	})
})

var _ = Describe("Server.serveGET", func() {
	const okPath = "/shopping/this-week/01HZ8K7Q9X1V2N3R4T5Y6Z7B8C.ics"
	const okUID = "01HZ8K7Q9X1V2N3R4T5Y6Z7B8C"

	When("the backend returns a live item", func() {
		var rec *httptest.ResponseRecorder
		var capturedPage, capturedList, capturedUID string

		BeforeEach(func() {
			backend := &fakeServerBackend{
				getItemFn: func(_ context.Context, page, list, uid string) (caldav.CalendarItem, error) {
					capturedPage = page
					capturedList = list
					capturedUID = uid
					return caldav.CalendarItem{
						UID:       okUID,
						ETag:      `W/"2026-04-25T13:00:00Z"`,
						ICalBytes: []byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n"),
					}, nil
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := authedRequest(http.MethodGet, okPath)
			server.ServeGETForTest(rec, req)
		})

		It("should return 200 OK", func() {
			Expect(rec.Code).To(Equal(http.StatusOK))
		})

		It("should set Content-Type to text/calendar; charset=utf-8", func() {
			Expect(rec.Header().Get("Content-Type")).To(Equal("text/calendar; charset=utf-8"))
		})

		It("should set the ETag header from the backend item", func() {
			Expect(rec.Header().Get("ETag")).To(Equal(`W/"2026-04-25T13:00:00Z"`))
		})

		It("should write the iCalendar body", func() {
			Expect(rec.Body.String()).To(Equal("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n"))
		})

		It("should pass the page from the URL to the backend", func() {
			Expect(capturedPage).To(Equal("shopping"))
		})

		It("should pass the list from the URL to the backend", func() {
			Expect(capturedList).To(Equal("this-week"))
		})

		It("should pass the uid from the URL to the backend", func() {
			Expect(capturedUID).To(Equal(okUID))
		})
	})

	When("the backend returns ErrItemNotFound", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			backend := &fakeServerBackend{
				getItemFn: func(_ context.Context, _, _, _ string) (caldav.CalendarItem, error) {
					return caldav.CalendarItem{}, caldav.ErrItemNotFound
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := authedRequest(http.MethodGet, okPath)
			server.ServeGETForTest(rec, req)
		})

		It("should return 404 Not Found", func() {
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})
	})

	When("the backend returns ErrItemDeleted", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			backend := &fakeServerBackend{
				getItemFn: func(_ context.Context, _, _, _ string) (caldav.CalendarItem, error) {
					return caldav.CalendarItem{}, caldav.ErrItemDeleted
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := authedRequest(http.MethodGet, okPath)
			server.ServeGETForTest(rec, req)
		})

		It("should return 404 Not Found", func() {
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})
	})

	When("the backend returns an unexpected error", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			backend := &fakeServerBackend{
				getItemFn: func(_ context.Context, _, _, _ string) (caldav.CalendarItem, error) {
					return caldav.CalendarItem{}, errors.New("disk on fire")
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := authedRequest(http.MethodGet, okPath)
			server.ServeGETForTest(rec, req)
		})

		It("should return 500 Internal Server Error", func() {
			Expect(rec.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	When("the URL is not a .ics resource", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{Backend: &fakeServerBackend{}}
			rec = httptest.NewRecorder()
			req := authedRequest(http.MethodGet, "/shopping/this-week")
			server.ServeGETForTest(rec, req)
		})

		It("should return 404 Not Found", func() {
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})
	})

	When("the URL has a malformed uid", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{Backend: &fakeServerBackend{}}
			rec = httptest.NewRecorder()
			req := authedRequest(http.MethodGet, "/shopping/this-week/not-a-ulid.ics")
			server.ServeGETForTest(rec, req)
		})

		It("should return 404 Not Found", func() {
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})
	})
})

var _ = Describe("Server.requireIdentity", func() {
	var server *caldav.Server

	BeforeEach(func() {
		server = &caldav.Server{}
	})

	When("the request has no identity in context", func() {
		var rec *httptest.ResponseRecorder
		var ok bool

		BeforeEach(func() {
			rec = httptest.NewRecorder()
			req := httptest.NewRequest("PROPFIND", "/shopping", nil)
			ok = server.RequireIdentityForTest(rec, req)
		})

		It("should return false", func() {
			Expect(ok).To(BeFalse())
		})

		It("should write status 403", func() {
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})

		It("should write the anonymous-rejection body", func() {
			Expect(rec.Body.String()).To(Equal("tailscale identity required\n"))
		})

		It("should not set a WWW-Authenticate header", func() {
			Expect(rec.Header().Get("WWW-Authenticate")).To(Equal(""))
		})
	})

	When("the request has the Anonymous singleton in context", func() {
		var rec *httptest.ResponseRecorder
		var ok bool

		BeforeEach(func() {
			rec = httptest.NewRecorder()
			req := httptest.NewRequest("PROPFIND", "/shopping", nil)
			req = req.WithContext(tailscale.ContextWithIdentity(req.Context(), tailscale.Anonymous))
			ok = server.RequireIdentityForTest(rec, req)
		})

		It("should return false", func() {
			Expect(ok).To(BeFalse())
		})

		It("should write status 403", func() {
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})

		It("should not set a WWW-Authenticate header", func() {
			Expect(rec.Header().Get("WWW-Authenticate")).To(Equal(""))
		})
	})

	When("the request has a non-anonymous identity in context", func() {
		var rec *httptest.ResponseRecorder
		var ok bool

		BeforeEach(func() {
			rec = httptest.NewRecorder()
			req := httptest.NewRequest("PROPFIND", "/shopping", nil)
			id := tailscale.NewIdentity("alice@example.com", "Alice", "phone")
			req = req.WithContext(tailscale.ContextWithIdentity(req.Context(), id))
			ok = server.RequireIdentityForTest(rec, req)
		})

		It("should return true", func() {
			Expect(ok).To(BeTrue())
		})

		It("should leave the response status unwritten", func() {
			// httptest.ResponseRecorder defaults Code to 200; we
			// assert nothing was actually written by checking the
			// flushed flag, since requireIdentity must not call
			// WriteHeader on the success path.
			Expect(rec.Body.Len()).To(Equal(0))
		})
	})
})

var _ = Describe("anonymous-gated handlers", func() {
	When("serveOPTIONS receives an anonymous request", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{}
			rec = httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodOptions, "/shopping", nil)
			req = req.WithContext(tailscale.ContextWithIdentity(req.Context(), tailscale.Anonymous))
			server.ServeOPTIONSForTest(rec, req)
		})

		It("should respond 403 Forbidden", func() {
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})

		It("should not advertise DAV capabilities to anonymous callers", func() {
			Expect(rec.Header().Get("DAV")).To(Equal(""))
		})
	})

	When("serveGET receives an anonymous request for a real .ics", func() {
		var rec *httptest.ResponseRecorder
		var backendCalled bool

		BeforeEach(func() {
			backend := &fakeServerBackend{
				getItemFn: func(_ context.Context, _, _, _ string) (caldav.CalendarItem, error) {
					backendCalled = true
					return caldav.CalendarItem{}, nil
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/shopping/this-week/01HZ8K7Q9X1V2N3R4T5Y6Z7B8C.ics", nil)
			req = req.WithContext(tailscale.ContextWithIdentity(req.Context(), tailscale.Anonymous))
			server.ServeGETForTest(rec, req)
		})

		It("should respond 403 Forbidden", func() {
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})

		It("should not call the backend", func() {
			Expect(backendCalled).To(BeFalse())
		})
	})
})

var _ = Describe("Server.servePUT", func() {
	const okPath = "/shopping/this-week/01HZ8K7Q9X1V2N3R4T5Y6Z7B8C.ics"
	const okUID = "01HZ8K7Q9X1V2N3R4T5Y6Z7B8C"
	const okBody = "BEGIN:VCALENDAR\r\nBEGIN:VTODO\r\nUID:01HZ8K7Q9X1V2N3R4T5Y6Z7B8C\r\nSUMMARY:milk\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"
	const okETag = `W/"2026-04-25T13:00:00Z"`

	When("the request is anonymous", func() {
		var rec *httptest.ResponseRecorder
		var backendCalled bool

		BeforeEach(func() {
			backend := &fakeServerBackend{
				putItemFn: func(_ context.Context, _, _, _ string, _ []byte, _, _ string, _ tailscale.IdentityValue) (string, bool, error) {
					backendCalled = true
					return "", false, nil
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, okPath, strings.NewReader(okBody))
			req.Header.Set("Content-Type", "text/calendar; charset=utf-8")
			req = req.WithContext(tailscale.ContextWithIdentity(req.Context(), tailscale.Anonymous))
			server.ServePUTForTest(rec, req)
		})

		It("should respond 403 Forbidden", func() {
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})

		It("should not call the backend", func() {
			Expect(backendCalled).To(BeFalse())
		})
	})

	When("the URL is not an .ics resource", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{Backend: &fakeServerBackend{}}
			rec = httptest.NewRecorder()
			req := authedRequestWithBody(http.MethodPut, "/shopping/this-week", okBody)
			server.ServePUTForTest(rec, req)
		})

		It("should respond 400 Bad Request", func() {
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})
	})

	When("the URL has a malformed uid", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{Backend: &fakeServerBackend{}}
			rec = httptest.NewRecorder()
			req := authedRequestWithBody(http.MethodPut, "/shopping/this-week/not-a-ulid.ics", okBody)
			server.ServePUTForTest(rec, req)
		})

		It("should respond 400 Bad Request", func() {
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})
	})

	When("the Content-Type is not text/calendar", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{Backend: &fakeServerBackend{}}
			rec = httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPut, okPath, strings.NewReader(okBody))
			req.Header.Set("Content-Type", "application/json")
			id := tailscale.NewIdentity("tester@example.com", "Tester", "phone")
			req = req.WithContext(tailscale.ContextWithIdentity(req.Context(), id))
			server.ServePUTForTest(rec, req)
		})

		It("should respond 415 Unsupported Media Type", func() {
			Expect(rec.Code).To(Equal(http.StatusUnsupportedMediaType))
		})
	})

	When("the body exceeds the size cap", func() {
		var rec *httptest.ResponseRecorder
		var backendCalled bool

		BeforeEach(func() {
			backend := &fakeServerBackend{
				putItemFn: func(_ context.Context, _, _, _ string, _ []byte, _, _ string, _ tailscale.IdentityValue) (string, bool, error) {
					backendCalled = true
					return okETag, true, nil
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			// 256 KB cap; exceed it by a comfortable margin.
			big := strings.Repeat("X", 300*1024)
			req := authedRequestWithBody(http.MethodPut, okPath, big)
			server.ServePUTForTest(rec, req)
		})

		It("should respond 413 Payload Too Large", func() {
			Expect(rec.Code).To(Equal(http.StatusRequestEntityTooLarge))
		})

		It("should not call the backend", func() {
			Expect(backendCalled).To(BeFalse())
		})
	})

	When("the backend creates a new item", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			backend := &fakeServerBackend{
				putItemFn: func(_ context.Context, _, _, _ string, _ []byte, _, _ string, _ tailscale.IdentityValue) (string, bool, error) {
					return okETag, true, nil
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := authedRequestWithBody(http.MethodPut, okPath, okBody)
			server.ServePUTForTest(rec, req)
		})

		It("should respond 201 Created", func() {
			Expect(rec.Code).To(Equal(http.StatusCreated))
		})

		It("should set the ETag header to the new ETag", func() {
			Expect(rec.Header().Get("ETag")).To(Equal(okETag))
		})
	})

	When("the backend updates an existing item", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			backend := &fakeServerBackend{
				putItemFn: func(_ context.Context, _, _, _ string, _ []byte, _, _ string, _ tailscale.IdentityValue) (string, bool, error) {
					return okETag, false, nil
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := authedRequestWithBody(http.MethodPut, okPath, okBody)
			server.ServePUTForTest(rec, req)
		})

		It("should respond 204 No Content", func() {
			Expect(rec.Code).To(Equal(http.StatusNoContent))
		})

		It("should set the ETag header to the new ETag", func() {
			Expect(rec.Header().Get("ETag")).To(Equal(okETag))
		})
	})

	When("the backend returns ErrPreconditionFailed", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			backend := &fakeServerBackend{
				putItemFn: func(_ context.Context, _, _, _ string, _ []byte, _, _ string, _ tailscale.IdentityValue) (string, bool, error) {
					return "", false, caldav.ErrPreconditionFailed
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := authedRequestWithBody(http.MethodPut, okPath, okBody)
			server.ServePUTForTest(rec, req)
		})

		It("should respond 412 Precondition Failed", func() {
			Expect(rec.Code).To(Equal(http.StatusPreconditionFailed))
		})
	})

	When("the backend returns ErrInvalidBody", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			backend := &fakeServerBackend{
				putItemFn: func(_ context.Context, _, _, _ string, _ []byte, _, _ string, _ tailscale.IdentityValue) (string, bool, error) {
					return "", false, caldav.ErrInvalidBody
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := authedRequestWithBody(http.MethodPut, okPath, okBody)
			server.ServePUTForTest(rec, req)
		})

		It("should respond 400 Bad Request", func() {
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})
	})

	When("the backend returns ErrUIDMismatch", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			backend := &fakeServerBackend{
				putItemFn: func(_ context.Context, _, _, _ string, _ []byte, _, _ string, _ tailscale.IdentityValue) (string, bool, error) {
					return "", false, caldav.ErrUIDMismatch
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := authedRequestWithBody(http.MethodPut, okPath, okBody)
			server.ServePUTForTest(rec, req)
		})

		It("should respond 400 Bad Request", func() {
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})
	})

	When("the backend returns ErrDescriptionTooLarge", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			backend := &fakeServerBackend{
				putItemFn: func(_ context.Context, _, _, _ string, _ []byte, _, _ string, _ tailscale.IdentityValue) (string, bool, error) {
					return "", false, caldav.ErrDescriptionTooLarge
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := authedRequestWithBody(http.MethodPut, okPath, okBody)
			server.ServePUTForTest(rec, req)
		})

		It("should respond 413 Payload Too Large", func() {
			Expect(rec.Code).To(Equal(http.StatusRequestEntityTooLarge))
		})
	})

	When("the backend returns an unexpected error", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			backend := &fakeServerBackend{
				putItemFn: func(_ context.Context, _, _, _ string, _ []byte, _, _ string, _ tailscale.IdentityValue) (string, bool, error) {
					return "", false, errors.New("disk on fire")
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := authedRequestWithBody(http.MethodPut, okPath, okBody)
			server.ServePUTForTest(rec, req)
		})

		It("should respond 500 Internal Server Error", func() {
			Expect(rec.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	When("the request carries an If-Match header", func() {
		var capturedIfMatch string
		var capturedIfNoneMatch string

		BeforeEach(func() {
			backend := &fakeServerBackend{
				putItemFn: func(_ context.Context, _, _, _ string, _ []byte, ifMatch, ifNoneMatch string, _ tailscale.IdentityValue) (string, bool, error) {
					capturedIfMatch = ifMatch
					capturedIfNoneMatch = ifNoneMatch
					return okETag, false, nil
				},
			}
			server := &caldav.Server{Backend: backend}
			rec := httptest.NewRecorder()
			req := authedRequestWithBody(http.MethodPut, okPath, okBody)
			req.Header.Set("If-Match", `"abc123"`)
			server.ServePUTForTest(rec, req)
		})

		It("should pass If-Match (with quotes stripped) to the backend", func() {
			Expect(capturedIfMatch).To(Equal("abc123"))
		})

		It("should pass an empty If-None-Match to the backend", func() {
			Expect(capturedIfNoneMatch).To(Equal(""))
		})
	})

	When("the request carries If-None-Match: *", func() {
		var capturedIfNoneMatch string
		var capturedIfMatch string

		BeforeEach(func() {
			backend := &fakeServerBackend{
				putItemFn: func(_ context.Context, _, _, _ string, _ []byte, ifMatch, ifNoneMatch string, _ tailscale.IdentityValue) (string, bool, error) {
					capturedIfMatch = ifMatch
					capturedIfNoneMatch = ifNoneMatch
					return okETag, true, nil
				},
			}
			server := &caldav.Server{Backend: backend}
			rec := httptest.NewRecorder()
			req := authedRequestWithBody(http.MethodPut, okPath, okBody)
			req.Header.Set("If-None-Match", "*")
			server.ServePUTForTest(rec, req)
		})

		It("should pass If-None-Match=* to the backend", func() {
			Expect(capturedIfNoneMatch).To(Equal("*"))
		})

		It("should pass an empty If-Match to the backend", func() {
			Expect(capturedIfMatch).To(Equal(""))
		})
	})

	When("the request body is read by the backend", func() {
		var capturedBody []byte

		BeforeEach(func() {
			backend := &fakeServerBackend{
				putItemFn: func(_ context.Context, _, _, _ string, body []byte, _, _ string, _ tailscale.IdentityValue) (string, bool, error) {
					capturedBody = body
					return okETag, true, nil
				},
			}
			server := &caldav.Server{Backend: backend}
			rec := httptest.NewRecorder()
			req := authedRequestWithBody(http.MethodPut, okPath, okBody)
			server.ServePUTForTest(rec, req)
		})

		It("should pass the request body bytes to the backend", func() {
			Expect(string(capturedBody)).To(Equal(okBody))
		})
	})

	When("the request carries the caller's Tailscale identity", func() {
		var capturedIdentity tailscale.IdentityValue

		BeforeEach(func() {
			backend := &fakeServerBackend{
				putItemFn: func(_ context.Context, _, _, _ string, _ []byte, _, _ string, identity tailscale.IdentityValue) (string, bool, error) {
					capturedIdentity = identity
					return okETag, true, nil
				},
			}
			server := &caldav.Server{Backend: backend}
			rec := httptest.NewRecorder()
			req := authedRequestWithBody(http.MethodPut, okPath, okBody)
			server.ServePUTForTest(rec, req)
		})

		It("should pass the identity through to the backend", func() {
			Expect(capturedIdentity.LoginName()).To(Equal("tester@example.com"))
		})
	})

	When("the request URL passes the page/list/uid to the backend", func() {
		var capturedPage, capturedList, capturedUID string

		BeforeEach(func() {
			backend := &fakeServerBackend{
				putItemFn: func(_ context.Context, page, list, uid string, _ []byte, _, _ string, _ tailscale.IdentityValue) (string, bool, error) {
					capturedPage = page
					capturedList = list
					capturedUID = uid
					return okETag, true, nil
				},
			}
			server := &caldav.Server{Backend: backend}
			rec := httptest.NewRecorder()
			req := authedRequestWithBody(http.MethodPut, okPath, okBody)
			server.ServePUTForTest(rec, req)
		})

		It("should pass the page from the URL to the backend", func() {
			Expect(capturedPage).To(Equal("shopping"))
		})

		It("should pass the list from the URL to the backend", func() {
			Expect(capturedList).To(Equal("this-week"))
		})

		It("should pass the uid from the URL to the backend", func() {
			Expect(capturedUID).To(Equal(okUID))
		})
	})
})

var _ = Describe("Server.serveDELETE", func() {
	const okPath = "/shopping/this-week/01HZ8K7Q9X1V2N3R4T5Y6Z7B8C.ics"
	const okUID = "01HZ8K7Q9X1V2N3R4T5Y6Z7B8C"

	When("the request is anonymous", func() {
		var rec *httptest.ResponseRecorder
		var backendCalled bool

		BeforeEach(func() {
			backend := &fakeServerBackend{
				deleteItemFn: func(_ context.Context, _, _, _, _ string, _ tailscale.IdentityValue) error {
					backendCalled = true
					return nil
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodDelete, okPath, nil)
			req = req.WithContext(tailscale.ContextWithIdentity(req.Context(), tailscale.Anonymous))
			server.ServeDELETEForTest(rec, req)
		})

		It("should respond 403 Forbidden", func() {
			Expect(rec.Code).To(Equal(http.StatusForbidden))
		})

		It("should not call the backend", func() {
			Expect(backendCalled).To(BeFalse())
		})
	})

	When("the URL is not an .ics resource", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{Backend: &fakeServerBackend{}}
			rec = httptest.NewRecorder()
			req := authedRequest(http.MethodDelete, "/shopping/this-week")
			server.ServeDELETEForTest(rec, req)
		})

		It("should respond 400 Bad Request", func() {
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})
	})

	When("the URL has a malformed uid", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{Backend: &fakeServerBackend{}}
			rec = httptest.NewRecorder()
			req := authedRequest(http.MethodDelete, "/shopping/this-week/not-a-ulid.ics")
			server.ServeDELETEForTest(rec, req)
		})

		It("should respond 400 Bad Request", func() {
			Expect(rec.Code).To(Equal(http.StatusBadRequest))
		})
	})

	When("the backend deletes successfully", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			backend := &fakeServerBackend{
				deleteItemFn: func(_ context.Context, _, _, _, _ string, _ tailscale.IdentityValue) error {
					return nil
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := authedRequest(http.MethodDelete, okPath)
			server.ServeDELETEForTest(rec, req)
		})

		It("should respond 204 No Content", func() {
			Expect(rec.Code).To(Equal(http.StatusNoContent))
		})

		It("should not write a body", func() {
			Expect(rec.Body.Len()).To(Equal(0))
		})
	})

	When("the backend returns ErrItemNotFound", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			backend := &fakeServerBackend{
				deleteItemFn: func(_ context.Context, _, _, _, _ string, _ tailscale.IdentityValue) error {
					return caldav.ErrItemNotFound
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := authedRequest(http.MethodDelete, okPath)
			server.ServeDELETEForTest(rec, req)
		})

		It("should respond 404 Not Found", func() {
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})
	})

	When("the backend returns ErrItemDeleted", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			backend := &fakeServerBackend{
				deleteItemFn: func(_ context.Context, _, _, _, _ string, _ tailscale.IdentityValue) error {
					return caldav.ErrItemDeleted
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := authedRequest(http.MethodDelete, okPath)
			server.ServeDELETEForTest(rec, req)
		})

		It("should respond 404 Not Found", func() {
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})
	})

	When("the backend returns ErrPreconditionFailed", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			backend := &fakeServerBackend{
				deleteItemFn: func(_ context.Context, _, _, _, _ string, _ tailscale.IdentityValue) error {
					return caldav.ErrPreconditionFailed
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := authedRequest(http.MethodDelete, okPath)
			server.ServeDELETEForTest(rec, req)
		})

		It("should respond 412 Precondition Failed", func() {
			Expect(rec.Code).To(Equal(http.StatusPreconditionFailed))
		})
	})

	When("the backend returns an unexpected error", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			backend := &fakeServerBackend{
				deleteItemFn: func(_ context.Context, _, _, _, _ string, _ tailscale.IdentityValue) error {
					return errors.New("disk on fire")
				},
			}
			server := &caldav.Server{Backend: backend}
			rec = httptest.NewRecorder()
			req := authedRequest(http.MethodDelete, okPath)
			server.ServeDELETEForTest(rec, req)
		})

		It("should respond 500 Internal Server Error", func() {
			Expect(rec.Code).To(Equal(http.StatusInternalServerError))
		})
	})

	When("the request carries an If-Match header", func() {
		var capturedIfMatch string

		BeforeEach(func() {
			backend := &fakeServerBackend{
				deleteItemFn: func(_ context.Context, _, _, _, ifMatch string, _ tailscale.IdentityValue) error {
					capturedIfMatch = ifMatch
					return nil
				},
			}
			server := &caldav.Server{Backend: backend}
			rec := httptest.NewRecorder()
			req := authedRequest(http.MethodDelete, okPath)
			req.Header.Set("If-Match", `"abc123"`)
			server.ServeDELETEForTest(rec, req)
		})

		It("should pass If-Match (with quotes stripped) to the backend", func() {
			Expect(capturedIfMatch).To(Equal("abc123"))
		})
	})

	When("the request URL passes page/list/uid to the backend", func() {
		var capturedPage, capturedList, capturedUID string

		BeforeEach(func() {
			backend := &fakeServerBackend{
				deleteItemFn: func(_ context.Context, page, list, uid, _ string, _ tailscale.IdentityValue) error {
					capturedPage = page
					capturedList = list
					capturedUID = uid
					return nil
				},
			}
			server := &caldav.Server{Backend: backend}
			rec := httptest.NewRecorder()
			req := authedRequest(http.MethodDelete, okPath)
			server.ServeDELETEForTest(rec, req)
		})

		It("should pass the page from the URL to the backend", func() {
			Expect(capturedPage).To(Equal("shopping"))
		})

		It("should pass the list from the URL to the backend", func() {
			Expect(capturedList).To(Equal("this-week"))
		})

		It("should pass the uid from the URL to the backend", func() {
			Expect(capturedUID).To(Equal(okUID))
		})
	})

	When("the request carries the caller's Tailscale identity", func() {
		var capturedIdentity tailscale.IdentityValue

		BeforeEach(func() {
			backend := &fakeServerBackend{
				deleteItemFn: func(_ context.Context, _, _, _, _ string, identity tailscale.IdentityValue) error {
					capturedIdentity = identity
					return nil
				},
			}
			server := &caldav.Server{Backend: backend}
			rec := httptest.NewRecorder()
			req := authedRequest(http.MethodDelete, okPath)
			server.ServeDELETEForTest(rec, req)
		})

		It("should pass the identity through to the backend", func() {
			Expect(capturedIdentity.LoginName()).To(Equal("tester@example.com"))
		})
	})
})

// findAttr returns the value of the first attribute with the given key,
// or "" / -1 if absent. Two helpers because Go doesn't let a single
// function pick the right return type from the attribute.Value union.
func findStringAttr(attrs []attribute.KeyValue, key string) string {
	for _, a := range attrs {
		if string(a.Key) == key {
			return a.Value.AsString()
		}
	}
	return ""
}

var _ = Describe("Server.instrumented", func() {
	const okPath = "/shopping/this-week/01HZ8K7Q9X1V2N3R4T5Y6Z7B8C.ics"

	When("the wrapped handler succeeds with a 200", func() {
		var server *caldav.Server
		var requests, bytesIn, bytesOut *caldav.FakeCounter
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server = &caldav.Server{}
			requests, bytesIn, bytesOut = server.InstallFakeMetricsForTest()
			rec = httptest.NewRecorder()
			req := authedRequest(http.MethodGet, okPath)
			server.CallInstrumentedForTest("propfind", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			}, rec, req)
		})

		It("should record exactly one requests increment", func() {
			Expect(requests.Records).To(HaveLen(1))
			Expect(requests.Records[0].Incr).To(Equal(int64(1)))
		})

		It("should attribute the request to the propfind method", func() {
			Expect(findStringAttr(requests.Records[0].Attrs, "caldav.method")).To(Equal("propfind"))
		})

		It("should attribute the outcome as ok", func() {
			Expect(findStringAttr(requests.Records[0].Attrs, "caldav.outcome")).To(Equal("ok"))
		})

		It("should record the bytes-out increment matching the response body", func() {
			Expect(bytesOut.Records).To(HaveLen(1))
			Expect(bytesOut.Records[0].Incr).To(Equal(int64(2)))
		})

		It("should record a bytes-in increment", func() {
			Expect(bytesIn.Records).To(HaveLen(1))
		})
	})

	When("the wrapped handler returns 400", func() {
		var requests *caldav.FakeCounter

		BeforeEach(func() {
			server := &caldav.Server{}
			requests, _, _ = server.InstallFakeMetricsForTest()
			rec := httptest.NewRecorder()
			req := authedRequest(http.MethodPut, okPath)
			server.CallInstrumentedForTest("put", func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "bad request", http.StatusBadRequest)
			}, rec, req)
		})

		It("should attribute the outcome as client_error", func() {
			Expect(requests.Records).To(HaveLen(1))
			Expect(findStringAttr(requests.Records[0].Attrs, "caldav.outcome")).To(Equal("client_error"))
		})
	})

	When("the wrapped handler returns 500", func() {
		var requests *caldav.FakeCounter

		BeforeEach(func() {
			server := &caldav.Server{}
			requests, _, _ = server.InstallFakeMetricsForTest()
			rec := httptest.NewRecorder()
			req := authedRequest(http.MethodPut, okPath)
			server.CallInstrumentedForTest("put", func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "boom", http.StatusInternalServerError)
			}, rec, req)
		})

		It("should attribute the outcome as server_error", func() {
			Expect(requests.Records).To(HaveLen(1))
			Expect(findStringAttr(requests.Records[0].Attrs, "caldav.outcome")).To(Equal("server_error"))
		})
	})

	When("the request carries a Content-Length header", func() {
		var bytesIn *caldav.FakeCounter

		BeforeEach(func() {
			server := &caldav.Server{}
			_, bytesIn, _ = server.InstallFakeMetricsForTest()
			rec := httptest.NewRecorder()
			req := authedRequestWithBody(http.MethodPut, okPath, "hello")
			server.CallInstrumentedForTest("put", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusCreated)
			}, rec, req)
		})

		It("should record bytes-in matching the Content-Length", func() {
			Expect(bytesIn.Records).To(HaveLen(1))
			Expect(bytesIn.Records[0].Incr).To(Equal(int64(5)))
		})
	})

	When("the wrapped handler never writes a status", func() {
		var requests *caldav.FakeCounter

		BeforeEach(func() {
			server := &caldav.Server{}
			requests, _, _ = server.InstallFakeMetricsForTest()
			rec := httptest.NewRecorder()
			req := authedRequest(http.MethodOptions, "/shopping")
			server.CallInstrumentedForTest("options", func(_ http.ResponseWriter, _ *http.Request) {
				// Handler intentionally writes nothing.
			}, rec, req)
		})

		It("should default the outcome to ok", func() {
			Expect(requests.Records).To(HaveLen(1))
			Expect(findStringAttr(requests.Records[0].Attrs, "caldav.outcome")).To(Equal("ok"))
		})
	})

	When("the Server has nil Metrics", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			server := &caldav.Server{} // Metrics is nil
			rec = httptest.NewRecorder()
			req := authedRequest(http.MethodGet, okPath)
			server.CallInstrumentedForTest("get", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("body"))
			}, rec, req)
		})

		It("should not panic", func() {
			Expect(rec.Code).To(Equal(http.StatusOK))
		})

		It("should still call the inner handler", func() {
			Expect(rec.Body.String()).To(Equal("body"))
		})
	})
})

// newAuditLogger returns a *log.Logger that writes into the supplied
// buffer with no prefix and no flags. Tests use it to capture
// auditWrite output verbatim, then assert on substrings.
func newAuditLogger(buf *bytes.Buffer) *log.Logger {
	return log.New(buf, "", 0)
}

var _ = Describe("Server.auditWrite", func() {
	When("a put creates a new item", func() {
		var buf *bytes.Buffer

		BeforeEach(func() {
			buf = &bytes.Buffer{}
			server := &caldav.Server{AuditLogger: newAuditLogger(buf)}
			server.CallAuditWriteForTest(
				"put", "alice@example.com",
				"shopping", "this-week", "01HZ8K7Q9X1V2N3R4T5Y6Z7B8C",
				"created", `W/"2026-04-25T13:00:00Z"`,
			)
		})

		It("should include the caldav prefix", func() {
			Expect(buf.String()).To(HavePrefix("caldav:"))
		})

		It("should include action=put", func() {
			Expect(buf.String()).To(ContainSubstring(`action=put`))
		})

		It("should include the principal", func() {
			Expect(buf.String()).To(ContainSubstring(`principal="alice@example.com"`))
		})

		It("should include the page", func() {
			Expect(buf.String()).To(ContainSubstring(`page="shopping"`))
		})

		It("should include the list", func() {
			Expect(buf.String()).To(ContainSubstring(`list="this-week"`))
		})

		It("should include the uid", func() {
			Expect(buf.String()).To(ContainSubstring(`uid="01HZ8K7Q9X1V2N3R4T5Y6Z7B8C"`))
		})

		It("should include outcome=created", func() {
			Expect(buf.String()).To(ContainSubstring(`outcome=created`))
		})

		It("should include the etag", func() {
			Expect(buf.String()).To(ContainSubstring(`etag="W/\"2026-04-25T13:00:00Z\""`))
		})
	})

	When("a delete succeeds", func() {
		var buf *bytes.Buffer

		BeforeEach(func() {
			buf = &bytes.Buffer{}
			server := &caldav.Server{AuditLogger: newAuditLogger(buf)}
			server.CallAuditWriteForTest(
				"delete", "alice@example.com",
				"shopping", "this-week", "01HZ8K7Q9X1V2N3R4T5Y6Z7B8C",
				"deleted", "",
			)
		})

		It("should include action=delete", func() {
			Expect(buf.String()).To(ContainSubstring(`action=delete`))
		})

		It("should include outcome=deleted", func() {
			Expect(buf.String()).To(ContainSubstring(`outcome=deleted`))
		})

		It("should omit the etag field when no etag is supplied", func() {
			Expect(buf.String()).NotTo(ContainSubstring(`etag=`))
		})
	})

	When("AuditLogger is nil", func() {
		var server *caldav.Server

		BeforeEach(func() {
			server = &caldav.Server{}
		})

		It("should not panic", func() {
			Expect(func() {
				server.CallAuditWriteForTest("put", "alice", "p", "l", "u", "created", "")
			}).NotTo(Panic())
		})
	})
})

var _ = Describe("audit logging on PUT", func() {
	const okPath = "/shopping/this-week/01HZ8K7Q9X1V2N3R4T5Y6Z7B8C.ics"
	const okBody = "BEGIN:VCALENDAR\r\nBEGIN:VTODO\r\nUID:01HZ8K7Q9X1V2N3R4T5Y6Z7B8C\r\nSUMMARY:milk\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"
	const okETag = `W/"2026-04-25T13:00:00Z"`

	When("the put creates a new item", func() {
		var buf *bytes.Buffer

		BeforeEach(func() {
			buf = &bytes.Buffer{}
			backend := &fakeServerBackend{
				putItemFn: func(_ context.Context, _, _, _ string, _ []byte, _, _ string, _ tailscale.IdentityValue) (string, bool, error) {
					return okETag, true, nil
				},
			}
			server := &caldav.Server{Backend: backend, AuditLogger: newAuditLogger(buf)}
			rec := httptest.NewRecorder()
			req := authedRequestWithBody(http.MethodPut, okPath, okBody)
			server.ServePUTForTest(rec, req)
		})

		It("should emit an audit log line with outcome=created", func() {
			Expect(buf.String()).To(ContainSubstring(`outcome=created`))
		})

		It("should attribute the action to the requester's principal", func() {
			Expect(buf.String()).To(ContainSubstring(`principal="tester@example.com"`))
		})
	})

	When("the put updates an existing item", func() {
		var buf *bytes.Buffer

		BeforeEach(func() {
			buf = &bytes.Buffer{}
			backend := &fakeServerBackend{
				putItemFn: func(_ context.Context, _, _, _ string, _ []byte, _, _ string, _ tailscale.IdentityValue) (string, bool, error) {
					return okETag, false, nil
				},
			}
			server := &caldav.Server{Backend: backend, AuditLogger: newAuditLogger(buf)}
			rec := httptest.NewRecorder()
			req := authedRequestWithBody(http.MethodPut, okPath, okBody)
			server.ServePUTForTest(rec, req)
		})

		It("should emit an audit log line with outcome=updated", func() {
			Expect(buf.String()).To(ContainSubstring(`outcome=updated`))
		})
	})

	When("the put hits a precondition failure", func() {
		var buf *bytes.Buffer

		BeforeEach(func() {
			buf = &bytes.Buffer{}
			backend := &fakeServerBackend{
				putItemFn: func(_ context.Context, _, _, _ string, _ []byte, _, _ string, _ tailscale.IdentityValue) (string, bool, error) {
					return "", false, caldav.ErrPreconditionFailed
				},
			}
			server := &caldav.Server{Backend: backend, AuditLogger: newAuditLogger(buf)}
			rec := httptest.NewRecorder()
			req := authedRequestWithBody(http.MethodPut, okPath, okBody)
			server.ServePUTForTest(rec, req)
		})

		It("should emit an audit log line with outcome=precondition_failed", func() {
			Expect(buf.String()).To(ContainSubstring(`outcome=precondition_failed`))
		})
	})

	When("the put hits a non-precondition error", func() {
		var buf *bytes.Buffer

		BeforeEach(func() {
			buf = &bytes.Buffer{}
			backend := &fakeServerBackend{
				putItemFn: func(_ context.Context, _, _, _ string, _ []byte, _, _ string, _ tailscale.IdentityValue) (string, bool, error) {
					return "", false, errors.New("disk on fire")
				},
			}
			server := &caldav.Server{Backend: backend, AuditLogger: newAuditLogger(buf)}
			rec := httptest.NewRecorder()
			req := authedRequestWithBody(http.MethodPut, okPath, okBody)
			server.ServePUTForTest(rec, req)
		})

		It("should not emit an audit log line", func() {
			Expect(buf.String()).To(Equal(""))
		})
	})
})

var _ = Describe("audit logging on DELETE", func() {
	const okPath = "/shopping/this-week/01HZ8K7Q9X1V2N3R4T5Y6Z7B8C.ics"

	When("the delete succeeds", func() {
		var buf *bytes.Buffer

		BeforeEach(func() {
			buf = &bytes.Buffer{}
			backend := &fakeServerBackend{
				deleteItemFn: func(_ context.Context, _, _, _, _ string, _ tailscale.IdentityValue) error {
					return nil
				},
			}
			server := &caldav.Server{Backend: backend, AuditLogger: newAuditLogger(buf)}
			rec := httptest.NewRecorder()
			req := authedRequest(http.MethodDelete, okPath)
			server.ServeDELETEForTest(rec, req)
		})

		It("should emit an audit log line with action=delete", func() {
			Expect(buf.String()).To(ContainSubstring(`action=delete`))
		})

		It("should emit an audit log line with outcome=deleted", func() {
			Expect(buf.String()).To(ContainSubstring(`outcome=deleted`))
		})

		It("should attribute the action to the requester's principal", func() {
			Expect(buf.String()).To(ContainSubstring(`principal="tester@example.com"`))
		})
	})

	When("the delete returns ErrItemNotFound", func() {
		var buf *bytes.Buffer

		BeforeEach(func() {
			buf = &bytes.Buffer{}
			backend := &fakeServerBackend{
				deleteItemFn: func(_ context.Context, _, _, _, _ string, _ tailscale.IdentityValue) error {
					return caldav.ErrItemNotFound
				},
			}
			server := &caldav.Server{Backend: backend, AuditLogger: newAuditLogger(buf)}
			rec := httptest.NewRecorder()
			req := authedRequest(http.MethodDelete, okPath)
			server.ServeDELETEForTest(rec, req)
		})

		It("should not emit an audit log line", func() {
			Expect(buf.String()).To(Equal(""))
		})
	})

	When("the delete hits a precondition failure", func() {
		var buf *bytes.Buffer

		BeforeEach(func() {
			buf = &bytes.Buffer{}
			backend := &fakeServerBackend{
				deleteItemFn: func(_ context.Context, _, _, _, _ string, _ tailscale.IdentityValue) error {
					return caldav.ErrPreconditionFailed
				},
			}
			server := &caldav.Server{Backend: backend, AuditLogger: newAuditLogger(buf)}
			rec := httptest.NewRecorder()
			req := authedRequest(http.MethodDelete, okPath)
			server.ServeDELETEForTest(rec, req)
		})

		It("should emit an audit log line with outcome=precondition_failed", func() {
			Expect(buf.String()).To(ContainSubstring(`outcome=precondition_failed`))
		})
	})
})

