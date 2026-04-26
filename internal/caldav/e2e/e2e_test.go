// Package e2e_test exercises the full CalDAV server stack against a
// real *checklistmutator.Mutator backed by an in-memory page store. The
// goal is to catch integration regressions that the unit tests in
// internal/caldav and server/checklistmutator each miss in isolation —
// the HTTP method dispatch, XML codec, backend, mutator funnel, and
// frontmatter codec all run end-to-end inside one test process.
//
// These specs simulate the wire-level behavior of two real clients —
// Apple's iOS CalDAV client and DAVx5 — and a concurrent-write race.
// They drive *caldav.Server.ServeHTTP directly via httptest, so every
// request goes through the same method-routing switch real traffic
// hits.
//
//revive:disable:dot-imports
package e2e_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/caldav"
	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/server/checklistmutator"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "caldav e2e")
}

// stubStore is an in-memory wikipage.PageReaderMutator backing the
// real Mutator. Mirrors checklistmutator_test.fakeStore in shape, but
// declared here because that one isn't exported.
type stubStore struct {
	mu    sync.Mutex
	pages map[string]wikipage.FrontMatter
}

func newStubStore() *stubStore {
	return &stubStore{pages: make(map[string]wikipage.FrontMatter)}
}

func (s *stubStore) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fm, ok := s.pages[string(id)]
	if !ok {
		fm = wikipage.FrontMatter{}
	}
	return id, deepCopyFM(fm), nil
}

func (s *stubStore) WriteFrontMatter(id wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pages[string(id)] = deepCopyFM(fm)
	return nil
}

func (*stubStore) ReadMarkdown(_ wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	return "", "", nil
}

func (*stubStore) WriteMarkdown(_ wikipage.PageIdentifier, _ wikipage.Markdown) error {
	return nil
}

func (*stubStore) DeletePage(_ wikipage.PageIdentifier) error { return nil }

func (*stubStore) ModifyMarkdown(_ wikipage.PageIdentifier, _ func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	return nil
}

// deepCopyFM clones a FrontMatter map so callers can't accidentally
// mutate the store's persisted state. TOML-shaped maps with strings,
// numbers, booleans, and nested maps/slices is enough for these specs.
func deepCopyFM(in wikipage.FrontMatter) wikipage.FrontMatter {
	if in == nil {
		return nil
	}
	out := make(wikipage.FrontMatter, len(in))
	for k, v := range in {
		out[k] = deepCopyValue(v)
	}
	return out
}

func deepCopyValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, v2 := range x {
			out[k] = deepCopyValue(v2)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, v2 := range x {
			out[i] = deepCopyValue(v2)
		}
		return out
	default:
		return v
	}
}

// stubClock returns a fixed time advanced manually between steps.
type stubClock struct {
	mu  sync.Mutex
	now time.Time
}

func newStubClock(t time.Time) *stubClock { return &stubClock{now: t} }

func (c *stubClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *stubClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// e2eFixture bundles the test plumbing used across the iOS, DAVx5, and
// concurrent-write specs. Construction is centralized so each Describe
// block sees the same identity, baseURL, and starting clock.
type e2eFixture struct {
	store   *stubStore
	clock   *stubClock
	mutator *checklistmutator.Mutator
	server  *caldav.Server
	alice   tailscale.IdentityValue
	bob     tailscale.IdentityValue
	ctx     context.Context //revive:disable-line:context-as-argument Field on test fixture; not a function arg.
}

// fixtureStartTime is the deterministic wall-clock start point for
// every e2e fixture. Pulled out to a named value so the per-call
// digits don't trip the add-constant lint.
var fixtureStartTime = time.Date(2026, 4, 25, 13, 0, 0, 0, time.UTC) //nolint:revive // anchored test instant; not a magic value

func newFixture() *e2eFixture {
	store := newStubStore()
	clock := newStubClock(fixtureStartTime)
	ulids := ulid.NewSequenceGenerator(
		"01HXAAAAAAAAAAAAAAAAAAAAAA",
		"01HXBBBBBBBBBBBBBBBBBBBBBB",
		"01HXCCCCCCCCCCCCCCCCCCCCCC",
		"01HXDDDDDDDDDDDDDDDDDDDDDD",
		"01HXEEEEEEEEEEEEEEEEEEEEEE",
	)
	mutator := checklistmutator.New(store, clock, ulids)
	backend := caldav.NewBackend(mutator, "https://wiki.example.com", clock.Now)
	server := caldav.NewServer(backend)
	alice := tailscale.NewIdentity("alice@example.com", "Alice", "alice-laptop")
	bob := tailscale.NewIdentity("bob@example.com", "Bob", "bob-laptop")
	return &e2eFixture{
		store:   store,
		clock:   clock,
		mutator: mutator,
		server:  server,
		alice:   alice,
		bob:     bob,
		ctx:     context.Background(),
	}
}

// do builds an httptest request with the given method/URL/body, attaches
// alice's identity, and runs it through server.ServeHTTP. Returns the
// recorder so callers can assert on status/headers/body.
func (f *e2eFixture) do(method, target, contentType, body string) *httptest.ResponseRecorder {
	return f.doWithIdentity(method, target, contentType, body, f.alice, nil)
}

// doWithIdentity is the full-featured request driver. headers may be nil.
func (f *e2eFixture) doWithIdentity(method, target, contentType, body string, identity tailscale.IdentityValue, headers map[string]string) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body != "" {
		reader = bytes.NewReader([]byte(body))
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, target, reader)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req = req.WithContext(tailscale.ContextWithIdentity(f.ctx, identity))
	rec := httptest.NewRecorder()
	f.server.ServeHTTP(rec, req)
	return rec
}

// propfind issues a PROPFIND with the given Depth and body as alice.
// Centralizes the application/xml Content-Type and the Depth header
// shape every CalDAV PROPFIND in these specs uses.
func (f *e2eFixture) propfind(target, depth, body string) *httptest.ResponseRecorder {
	return f.doWithIdentity("PROPFIND", target, xmlContentType, body, f.alice, map[string]string{"Depth": depth})
}

// report issues a REPORT (Depth:1, application/xml) with the given body
// as alice. Used by every calendar-query / calendar-multiget /
// sync-collection driver in these specs.
func (f *e2eFixture) report(target, body string) *httptest.ResponseRecorder {
	return f.doWithIdentity("REPORT", target, xmlContentType, body, f.alice, map[string]string{"Depth": "1"})
}

// vtodoBody renders a minimal VCALENDAR/VTODO body for an iOS-style
// PUT. summary is the SUMMARY text; status is one of NEEDS-ACTION /
// COMPLETED. uid is the VTODO/UID property which must match the URL.
func vtodoBody(uid, summary, status string) string {
	return strings.Join([]string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//e2e-test//iOS//EN",
		"BEGIN:VTODO",
		"UID:" + uid,
		"SUMMARY:" + summary,
		"STATUS:" + status,
		"X-APPLE-SORT-ORDER:1000",
		"END:VTODO",
		"END:VCALENDAR",
	}, "\r\n") + "\r\n"
}

// itemPath is the canonical CalDAV item URL on the test page/list.
func itemPath(uid string) string {
	return fmt.Sprintf("/shopping/this-week/%s.ics", uid)
}

// quotedETag wraps a raw ETag value (which is already weak-quoted, e.g.
// `W/"2026-04-25T13:00:00Z"`) for use in If-Match headers. The server
// strips the W/ prefix before comparison, so passing the value verbatim
// is correct.
func quotedETag(raw string) string { return raw }

// iCalContentType is the wire Content-Type every CalDAV PUT body must
// carry. The PUT handler's media-type check is case-insensitive on the
// type portion and tolerates an optional `; charset=...` parameter; we
// pick the exact form iOS sends so the test mirrors production traffic.
const iCalContentType = "text/calendar; charset=utf-8"

// xmlContentType is the wire Content-Type for PROPFIND / REPORT bodies.
const xmlContentType = "application/xml; charset=utf-8"

// propfindHomeSetBody is the PROPFIND request body Apple Reminders
// fires against the calendar-home-set URL. We don't currently parse the
// `<prop>` selector — every PROPFIND emits the wiki's full property set
// — but the body shape matches what iOS sends so a future filter would
// have realistic input to test against.
const propfindHomeSetBody = `<?xml version="1.0" encoding="utf-8"?>
<propfind xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:CS="http://calendarserver.org/ns/">
  <prop>
    <displayname/>
    <current-user-principal/>
    <C:calendar-home-set/>
    <resourcetype/>
  </prop>
</propfind>`

// propfindItemBody is the PROPFIND request body iOS fires against a
// single .ics resource URL.
const propfindItemBody = `<?xml version="1.0" encoding="utf-8"?>
<propfind xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <prop>
    <getetag/>
    <C:calendar-data/>
  </prop>
</propfind>`

// syncCollectionBody is the RFC 6578 sync-collection REPORT body iOS
// fires for incremental syncs. token is the URI the client last saw;
// pass "" for the initial-sync shape (`<sync-token/>`).
func syncCollectionBody(token string) string {
	if token == "" {
		return `<?xml version="1.0" encoding="utf-8"?>
<sync-collection xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <sync-token/>
  <sync-level>1</sync-level>
  <prop>
    <getetag/>
    <C:calendar-data/>
  </prop>
</sync-collection>`
	}
	return `<?xml version="1.0" encoding="utf-8"?>
<sync-collection xmlns="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <sync-token>` + token + `</sync-token>
  <sync-level>1</sync-level>
  <prop>
    <getetag/>
    <C:calendar-data/>
  </prop>
</sync-collection>`
}

// extractSyncToken pulls the trailing `<sync-token>...</sync-token>`
// element value out of a multistatus body. Returns the empty string
// when the element is absent (PROPFIND responses) or when the parse
// fails (malformed body — the caller's other assertions will fail
// loudly enough).
func extractSyncToken(body string) string {
	const open = "<sync-token>"
	const closeTag = "</sync-token>"
	i := strings.Index(body, open)
	if i < 0 {
		return ""
	}
	j := strings.Index(body[i+len(open):], closeTag)
	if j < 0 {
		return ""
	}
	return body[i+len(open) : i+len(open)+j]
}

// seedShoppingList primes the page store with a single-item checklist
// "shopping/this-week" so subsequent PROPFIND/REPORT specs have data
// to enumerate. Returns the seeded item's UID.
func seedShoppingList(f *e2eFixture, summary string) string {
	item, _, err := f.mutator.AddItem(f.ctx, "shopping", "this-week", checklistmutator.AddItemArgs{Text: summary}, f.alice)
	Expect(err).NotTo(HaveOccurred())
	return item.Uid
}

var _ = Describe("CalDAV e2e: iPhone subscribes and syncs", func() {
	var (
		f         *e2eFixture
		seededUID string
	)

	BeforeEach(func() {
		f = newFixture()
		seededUID = seedShoppingList(f, "Bread")
	})

	When("iOS issues the initial PROPFIND on the home-set Depth:0", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			rec = f.propfind("/shopping/", "0", propfindHomeSetBody)
		})

		It("should return 207 Multi-Status", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should advertise calendar-home-set pointing back at /shopping/", func() {
			body := rec.Body.String()
			Expect(body).To(ContainSubstring("calendar-home-set"))
			Expect(body).To(ContainSubstring("/shopping/"))
		})

		It("should advertise current-user-principal", func() {
			Expect(rec.Body.String()).To(ContainSubstring("current-user-principal"))
		})
	})

	When("iOS enumerates collections via PROPFIND home-set Depth:1", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			rec = f.propfind("/shopping/", "1", propfindHomeSetBody)
		})

		It("should return 207 Multi-Status", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should include a response for the seeded collection", func() {
			Expect(rec.Body.String()).To(ContainSubstring("/shopping/this-week/"))
		})

		It("should advertise getctag on the collection", func() {
			Expect(rec.Body.String()).To(ContainSubstring("getctag"))
		})

		It("should advertise sync-token on the collection", func() {
			Expect(rec.Body.String()).To(ContainSubstring("sync-token"))
		})

		It("should advertise VTODO via supported-calendar-component-set", func() {
			body := rec.Body.String()
			Expect(body).To(ContainSubstring("supported-calendar-component-set"))
			Expect(body).To(ContainSubstring(`name="VTODO"`))
		})
	})

	When("iOS issues an initial sync-collection REPORT", func() {
		var rec *httptest.ResponseRecorder
		var initialSyncToken string

		BeforeEach(func() {
			rec = f.report("/shopping/this-week/", syncCollectionBody(""))
			initialSyncToken = extractSyncToken(rec.Body.String())
		})

		It("should return 207 Multi-Status", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should include a response for the seeded item", func() {
			Expect(rec.Body.String()).To(ContainSubstring(seededUID + ".ics"))
		})

		It("should emit a sync-token element", func() {
			Expect(initialSyncToken).NotTo(BeEmpty())
		})
	})

	When("iOS PUTs a new VTODO", func() {
		const newUID = "01HXNEW1NEW1NEW1NEW1NEW1NE"
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			f.clock.advance(time.Minute)
			rec = f.do(http.MethodPut, itemPath(newUID), iCalContentType, vtodoBody(newUID, "Eggs", "NEEDS-ACTION"))
		})

		It("should return 201 Created", func() {
			Expect(rec.Code).To(Equal(http.StatusCreated))
		})

		It("should set an ETag header", func() {
			Expect(rec.Header().Get("ETag")).NotTo(BeEmpty())
		})

		It("should make the new item visible to ListItems", func() {
			cl, err := f.mutator.ListItems(f.ctx, "shopping", "this-week")
			Expect(err).NotTo(HaveOccurred())
			uids := []string{}
			for _, it := range cl.Items {
				uids = append(uids, it.Uid)
			}
			Expect(uids).To(ContainElement(newUID))
		})
	})

	When("iOS PROPFINDs the just-PUT item", func() {
		const newUID = "01HXNEW2NEW2NEW2NEW2NEW2NE"
		var putETag string
		var propfindRec *httptest.ResponseRecorder

		BeforeEach(func() {
			f.clock.advance(time.Minute)
			putRec := f.do(http.MethodPut, itemPath(newUID), iCalContentType, vtodoBody(newUID, "Eggs", "NEEDS-ACTION"))
			Expect(putRec.Code).To(Equal(http.StatusCreated))
			putETag = putRec.Header().Get("ETag")
			f.clock.advance(time.Second)
			propfindRec = f.propfind(itemPath(newUID), "0", propfindItemBody)
		})

		It("should return 207 Multi-Status", func() {
			Expect(propfindRec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should return getetag matching the PUT ETag", func() {
			body := propfindRec.Body.String()
			// The wire-form ETag is the W/"..." string; PROPFIND emits
			// it inside <getetag>, but encoding/xml escapes the quotes
			// to `&#34;`. Pull off the W/ prefix AND the surrounding
			// quotes to get the bare RFC3339Nano timestamp suitable for
			// substring matching against the escaped XML.
			trimmed := strings.TrimPrefix(putETag, "W/")
			trimmed = strings.Trim(trimmed, `"`)
			Expect(body).To(ContainSubstring(trimmed))
		})

		It("should embed calendar-data containing the SUMMARY", func() {
			body := propfindRec.Body.String()
			Expect(body).To(ContainSubstring("calendar-data"))
			Expect(body).To(ContainSubstring("Eggs"))
		})
	})

	When("iOS toggles the item to COMPLETED via If-Match PUT", func() {
		const newUID = "01HXNEW3NEW3NEW3NEW3NEW3NE"
		var firstETag string
		var toggleRec *httptest.ResponseRecorder

		BeforeEach(func() {
			f.clock.advance(time.Minute)
			putRec := f.do(http.MethodPut, itemPath(newUID), iCalContentType, vtodoBody(newUID, "Eggs", "NEEDS-ACTION"))
			Expect(putRec.Code).To(Equal(http.StatusCreated))
			firstETag = putRec.Header().Get("ETag")
			f.clock.advance(time.Second)
			toggleRec = f.doWithIdentity(
				http.MethodPut,
				itemPath(newUID),
				iCalContentType,
				vtodoBody(newUID, "Eggs", "COMPLETED"),
				f.alice,
				map[string]string{"If-Match": quotedETag(firstETag)},
			)
		})

		It("should return 204 No Content", func() {
			Expect(toggleRec.Code).To(Equal(http.StatusNoContent))
		})

		It("should set a new ETag", func() {
			newETag := toggleRec.Header().Get("ETag")
			Expect(newETag).NotTo(BeEmpty())
			Expect(newETag).NotTo(Equal(firstETag))
		})
	})

	When("iOS issues sync-collection REPORT with the previous token", func() {
		const newUID = "01HXNEW4NEW4NEW4NEW4NEW4NE"
		var prevToken string
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			// Capture the sync token after seeding (one item exists).
			initRec := f.report("/shopping/this-week/", syncCollectionBody(""))
			prevToken = extractSyncToken(initRec.Body.String())
			Expect(prevToken).NotTo(BeEmpty())

			// Mutate the collection: PUT a new item.
			f.clock.advance(time.Minute)
			putRec := f.do(http.MethodPut, itemPath(newUID), iCalContentType, vtodoBody(newUID, "Milk", "NEEDS-ACTION"))
			Expect(putRec.Code).To(Equal(http.StatusCreated))

			// Now sync-collection REPORT with the pre-mutation token.
			rec = f.report("/shopping/this-week/", syncCollectionBody(prevToken))
		})

		It("should return 207 Multi-Status", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should include the new item in the response", func() {
			Expect(rec.Body.String()).To(ContainSubstring(newUID + ".ics"))
		})

		It("should advance the sync-token past the previous one", func() {
			newToken := extractSyncToken(rec.Body.String())
			Expect(newToken).NotTo(BeEmpty())
			Expect(newToken).NotTo(Equal(prevToken))
		})
	})

	When("iOS DELETEs an item", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			f.clock.advance(time.Minute)
			rec = f.do(http.MethodDelete, itemPath(seededUID), "", "")
		})

		It("should return 204 No Content", func() {
			Expect(rec.Code).To(Equal(http.StatusNoContent))
		})

		It("should remove the item from ListItems", func() {
			cl, err := f.mutator.ListItems(f.ctx, "shopping", "this-week")
			Expect(err).NotTo(HaveOccurred())
			for _, it := range cl.Items {
				Expect(it.Uid).NotTo(Equal(seededUID))
			}
		})

		It("should record a tombstone for the deleted uid", func() {
			cl, err := f.mutator.ListItems(f.ctx, "shopping", "this-week")
			Expect(err).NotTo(HaveOccurred())
			tombUIDs := []string{}
			for _, t := range cl.Tombstones {
				tombUIDs = append(tombUIDs, t.Uid)
			}
			Expect(tombUIDs).To(ContainElement(seededUID))
		})
	})

	When("iOS issues sync-collection REPORT after a DELETE", func() {
		var rec *httptest.ResponseRecorder
		var prevToken string

		BeforeEach(func() {
			// Initial sync to capture the pre-delete token.
			initRec := f.report("/shopping/this-week/", syncCollectionBody(""))
			prevToken = extractSyncToken(initRec.Body.String())

			// DELETE the seeded item.
			f.clock.advance(time.Minute)
			delRec := f.do(http.MethodDelete, itemPath(seededUID), "", "")
			Expect(delRec.Code).To(Equal(http.StatusNoContent))

			// Sync with the pre-delete token.
			rec = f.report("/shopping/this-week/", syncCollectionBody(prevToken))
		})

		It("should return 207 Multi-Status", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should include a 404 response for the deleted uid", func() {
			body := rec.Body.String()
			Expect(body).To(ContainSubstring(seededUID + ".ics"))
			Expect(body).To(ContainSubstring("404"))
		})
	})
})

// calendarQueryVTODOBody is the calendar-query REPORT body DAVx5
// fires when enumerating a collection. It carries the trivial
// `VCALENDAR > VTODO` comp-filter that matches every item we serve;
// our server ignores filters in Phase 1 and returns every live item,
// so the shape is preserved purely to mirror what real DAVx5 traffic
// looks like.
const calendarQueryVTODOBody = `<?xml version="1.0" encoding="utf-8"?>
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

// calendarMultigetBody returns a calendar-multiget REPORT body listing
// the supplied hrefs verbatim. DAVx5 fires this after a calendar-query
// has told it which uids exist on the collection — it batches the
// per-item GETs into one round-trip.
//
// strings.Builder.WriteString always returns a nil error (it can never
// fail), but the linter is conservative: explicit `_ = b.WriteString(...)`
// silences the unhandled-error warning without obscuring the fact that
// we're discarding the return value.
func calendarMultigetBody(hrefs ...string) string {
	var b strings.Builder
	_, _ = b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	_, _ = b.WriteString(`<C:calendar-multiget xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">` + "\n")
	_, _ = b.WriteString(`  <D:prop><D:getetag/><C:calendar-data/></D:prop>` + "\n")
	for _, h := range hrefs {
		_, _ = b.WriteString("  <D:href>" + h + "</D:href>\n")
	}
	_, _ = b.WriteString(`</C:calendar-multiget>`)
	return b.String()
}

var _ = Describe("CalDAV e2e: DAVx5 subscribes and syncs", func() {
	var (
		f         *e2eFixture
		seededUID string
	)

	BeforeEach(func() {
		f = newFixture()
		seededUID = seedShoppingList(f, "Bread")
	})

	When("DAVx5 enumerates collections via PROPFIND home-set Depth:1", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			rec = f.propfind("/shopping/", "1", propfindHomeSetBody)
		})

		It("should return 207 Multi-Status", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should include a response for the seeded collection", func() {
			Expect(rec.Body.String()).To(ContainSubstring("/shopping/this-week/"))
		})
	})

	When("DAVx5 issues calendar-query REPORT with the VTODO comp-filter", func() {
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			rec = f.report("/shopping/this-week/", calendarQueryVTODOBody)
		})

		It("should return 207 Multi-Status", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should include the seeded item in the response", func() {
			Expect(rec.Body.String()).To(ContainSubstring(seededUID + ".ics"))
		})

		It("should embed calendar-data for the item", func() {
			body := rec.Body.String()
			Expect(body).To(ContainSubstring("calendar-data"))
			Expect(body).To(ContainSubstring("Bread"))
		})
	})

	When("DAVx5 PUTs a new VTODO", func() {
		const newUID = "01HXDAV1DAV1DAV1DAV1DAV1DA"
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			f.clock.advance(time.Minute)
			rec = f.do(http.MethodPut, itemPath(newUID), iCalContentType, vtodoBody(newUID, "Cheese", "NEEDS-ACTION"))
		})

		It("should return 201 Created", func() {
			Expect(rec.Code).To(Equal(http.StatusCreated))
		})

		It("should set an ETag header", func() {
			Expect(rec.Header().Get("ETag")).NotTo(BeEmpty())
		})
	})

	When("DAVx5 issues calendar-multiget REPORT with two known hrefs", func() {
		const newUID = "01HXDAV2DAV2DAV2DAV2DAV2DA"
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			f.clock.advance(time.Minute)
			putRec := f.do(http.MethodPut, itemPath(newUID), iCalContentType, vtodoBody(newUID, "Wine", "NEEDS-ACTION"))
			Expect(putRec.Code).To(Equal(http.StatusCreated))

			rec = f.report("/shopping/this-week/", calendarMultigetBody(itemPath(seededUID), itemPath(newUID)))
		})

		It("should return 207 Multi-Status", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should include the first known item", func() {
			Expect(rec.Body.String()).To(ContainSubstring(seededUID + ".ics"))
		})

		It("should include the second known item", func() {
			Expect(rec.Body.String()).To(ContainSubstring(newUID + ".ics"))
		})
	})

	When("DAVx5 issues calendar-multiget REPORT with one unknown href alongside known ones", func() {
		const unknownUID = "01HXDAVUNKNOWNUNKNOWNUNKNOW"
		var rec *httptest.ResponseRecorder

		BeforeEach(func() {
			rec = f.report("/shopping/this-week/", calendarMultigetBody(itemPath(seededUID), itemPath(unknownUID)))
		})

		It("should return 207 Multi-Status", func() {
			Expect(rec.Code).To(Equal(http.StatusMultiStatus))
		})

		It("should still include the known item", func() {
			Expect(rec.Body.String()).To(ContainSubstring(seededUID + ".ics"))
		})

		It("should emit a 404 entry for the unknown href without failing the others", func() {
			body := rec.Body.String()
			Expect(body).To(ContainSubstring(unknownUID + ".ics"))
			Expect(body).To(ContainSubstring("404"))
		})
	})
})

var _ = Describe("CalDAV e2e: two clients race a PUT with the same If-Match", func() {
	var (
		f         *e2eFixture
		seededUID string
		// initialETag is the per-item ETag both clients see before any
		// of them mutate the item — the value both will pass to
		// If-Match to assert "I'm updating the version I last read."
		initialETag string
	)

	BeforeEach(func() {
		f = newFixture()
		seededUID = seedShoppingList(f, "Original")

		// Capture the initial ETag both clients would have read via a
		// PROPFIND. The wiki's per-item ETag is W/"<rfc3339nano of
		// updated_at>", so we ask the mutator directly rather than
		// parsing it out of XML.
		cl, err := f.mutator.ListItems(f.ctx, "shopping", "this-week")
		Expect(err).NotTo(HaveOccurred())
		Expect(cl.Items).To(HaveLen(1))
		initialETag = `W/"` + cl.Items[0].UpdatedAt.AsTime().UTC().Format(time.RFC3339Nano) + `"`
	})

	When("client A wins the race and client B's stale If-Match arrives second", func() {
		var (
			recA *httptest.ResponseRecorder
			recB *httptest.ResponseRecorder
		)

		BeforeEach(func() {
			// Client A fires first and gets through.
			f.clock.advance(time.Minute)
			recA = f.doWithIdentity(
				http.MethodPut,
				itemPath(seededUID),
				iCalContentType,
				vtodoBody(seededUID, "A wins", "NEEDS-ACTION"),
				f.alice,
				map[string]string{"If-Match": quotedETag(initialETag)},
			)

			// Client B fires second with the same now-stale ETag.
			// Bob's clock advances another second so any If-Match
			// comparison against UpdatedAt is unambiguous.
			f.clock.advance(time.Second)
			recB = f.doWithIdentity(
				http.MethodPut,
				itemPath(seededUID),
				iCalContentType,
				vtodoBody(seededUID, "B loses", "NEEDS-ACTION"),
				f.bob,
				map[string]string{"If-Match": quotedETag(initialETag)},
			)
		})

		It("should let client A win with 204 No Content", func() {
			Expect(recA.Code).To(Equal(http.StatusNoContent))
		})

		It("should set a new ETag on client A's response", func() {
			newETag := recA.Header().Get("ETag")
			Expect(newETag).NotTo(BeEmpty())
			Expect(newETag).NotTo(Equal(initialETag))
		})

		It("should reject client B with 412 Precondition Failed", func() {
			Expect(recB.Code).To(Equal(http.StatusPreconditionFailed))
		})

		It("should leave the persisted item with client A's text", func() {
			cl, err := f.mutator.ListItems(f.ctx, "shopping", "this-week")
			Expect(err).NotTo(HaveOccurred())
			Expect(cl.Items).To(HaveLen(1))
			Expect(cl.Items[0].Text).To(Equal("A wins"))
		})
	})
})
