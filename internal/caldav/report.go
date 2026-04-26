// Package caldav — REPORT request handler.
//
// CalDAV REPORT (RFC 3253 / 4791 §7) is a POST-shaped HTTP method that
// carries an XML body describing the report the client wants. Phase 1
// of the bridge handles two report types:
//
//   - calendar-multiget (RFC 4791 §7.9): an explicit list of hrefs the
//     client already knows about. After PROPFIND tells iOS / DAVx5
//     which items live in a collection, the client batches the per-
//     item GETs into one calendar-multiget so it gets every getetag +
//     calendar-data in a single request.
//   - calendar-query    (RFC 4791 §7.8): server-side filtering on a
//     collection. Phase 1 does not honor the <C:filter> element — every
//     calendar-query returns every live item from the addressed
//     collection. Real-world clients (iOS, DAVx5) only send the trivial
//     `VCALENDAR > VTODO` component-filter, which matches every item we
//     render anyway, so ignoring the filter is a safe approximation
//     until a future phase needs precise filtering.
//
// sync-collection (RFC 6578) is the third REPORT type the spec lists.
// Phase 3 implements it: clients pass back the sync-token they last
// saw, and the server returns the items changed and uids deleted since
// then. Apple Reminders / DAVx5 use this to do incremental sync without
// re-fetching the entire collection.

package caldav

import (
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Per-href status line used inside multistatus responses. RFC 2616 §6.1
// quotes the canonical `HTTP/1.1 <code> <reason>` shape.
const statusNotFound = "HTTP/1.1 404 Not Found"

// REPORT root element local names. Centralized so the dispatcher and
// any future per-type handler agree on the spelling.
const (
	reportLocalMultiget       = "calendar-multiget"
	reportLocalCalendarQuery  = "calendar-query"
	reportLocalSyncCollection = "sync-collection"
)

// reportRoot is the minimal XML decoded from the REPORT body just to
// learn which report type the client is asking for. Each per-type
// handler decodes the body again with the type-specific shape so its
// code only sees the elements it cares about.
type reportRoot struct {
	XMLName xml.Name
}

// hrefElement is the DAV:href wire shape used as a request element
// inside calendar-multiget. Keeping the wire decode separate from the
// response-side `multistatusResponse.Href` (a plain string) avoids
// pulling propfind.go's response types into request parsing.
type hrefElement struct {
	XMLName xml.Name `xml:"DAV: href"`
	Path    string   `xml:",chardata"`
}

// multigetRequest is the calendar-multiget request body shape.
type multigetRequest struct {
	XMLName xml.Name      `xml:"urn:ietf:params:xml:ns:caldav calendar-multiget"`
	Hrefs   []hrefElement `xml:"DAV: href"`
}

// calendarQueryRequest is the calendar-query request body shape. The
// filter element is intentionally not modeled — Phase 1 ignores it (we
// always return every live item from the addressed collection). The
// type exists only so xml.Unmarshal can validate the body parses.
type calendarQueryRequest struct {
	XMLName xml.Name `xml:"urn:ietf:params:xml:ns:caldav calendar-query"`
}

// serveREPORT handles WebDAV REPORT requests. It reads the XML body,
// dispatches on the root element name, and delegates to the per-type
// handler. The body bytes are passed through to each handler so it can
// re-decode with its own request shape without re-buffering the body.
func (s *Server) serveREPORT(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireIdentity(w, r); !ok {
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "could not read request body", http.StatusBadRequest)
		return
	}
	var root reportRoot
	if err := xml.Unmarshal(body, &root); err != nil {
		http.Error(w, "malformed REPORT body", http.StatusBadRequest)
		return
	}
	switch {
	case root.XMLName.Space == nsCalDAV && root.XMLName.Local == reportLocalMultiget:
		s.reportMultiget(w, r, body)
	case root.XMLName.Space == nsCalDAV && root.XMLName.Local == reportLocalCalendarQuery:
		s.reportCalendarQuery(w, r, body)
	case root.XMLName.Space == nsDAV && root.XMLName.Local == reportLocalSyncCollection:
		s.reportSyncCollection(w, r, body)
	default:
		http.Error(w, "unrecognized REPORT root element", http.StatusBadRequest)
	}
}

// hrefToPath extracts the URL path from an href value. CalDAV clients
// send hrefs as either absolute URLs ("https://wiki/page/list/uid.ics")
// or absolute paths ("/page/list/uid.ics"); RFC 4791 allows both. We
// strip scheme+host when present so parsePath sees a consistent shape.
// Returns the trimmed input unchanged if it doesn't parse as an
// absolute URL — an absolute path matches that fall-through case.
func hrefToPath(href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	if !strings.Contains(href, "://") {
		return href
	}
	u, err := url.Parse(href)
	if err != nil {
		return href
	}
	return u.Path
}

// reportMultiget handles the calendar-multiget REPORT body. Each href
// resolves through parsePath; per-href resolution errors surface as a
// 404 inside the multistatus response so a single bad href doesn't
// poison the entire batch. The page in each href — not the request
// URL — is what's used, so a multiget that spans pages (rare but
// legal) works.
//
// Hrefs are grouped by (page, list) and each group's items fetched in
// a single Backend.ListItems call. This keeps a 100-item multiget from
// fanning out into 100 store reads when all the items live in the
// same collection — the common iOS/DAVx5 batch shape after a PROPFIND.
// Cross-collection multigets degrade gracefully to one ListItems per
// distinct collection.
func (s *Server) reportMultiget(w http.ResponseWriter, r *http.Request, body []byte) {
	var req multigetRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		http.Error(w, "malformed calendar-multiget body", http.StatusBadRequest)
		return
	}

	type multigetSlot struct {
		hrefPath string
		page     string
		list     string
		uid      string
		item     CalendarItem
		notFound bool
	}
	type collectionKey struct{ page, list string }

	// Pass 1: parse every href into (page, list, uid) — or mark the
	// slot as a per-href 404. Preserve request order so the response's
	// <response> children line up with the request's <href> children.
	slots := make([]multigetSlot, 0, len(req.Hrefs))
	groups := make(map[collectionKey][]int)
	for _, h := range req.Hrefs {
		hrefPath := hrefToPath(h.Path)
		page, list, uid, parseErr := parsePath(hrefPath)
		if parseErr != nil || uid == "" {
			slots = append(slots, multigetSlot{hrefPath: hrefPath, notFound: true})
			continue
		}
		idx := len(slots)
		slots = append(slots, multigetSlot{hrefPath: hrefPath, page: page, list: list, uid: uid})
		key := collectionKey{page: page, list: list}
		groups[key] = append(groups[key], idx)
	}

	// Pass 2: per (page, list) group, ListItems once and resolve each
	// uid against an in-memory map. ErrCollectionNotFound on the group
	// fans out to per-href 404s rather than failing the whole report.
	for key, indices := range groups {
		_, items, err := s.Backend.ListItems(r.Context(), key.page, key.list)
		if err != nil {
			if errors.Is(err, ErrCollectionNotFound) {
				for _, idx := range indices {
					slots[idx].notFound = true
				}
				continue
			}
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		byUID := make(map[string]CalendarItem, len(items))
		for _, item := range items {
			byUID[item.UID] = item
		}
		for _, idx := range indices {
			item, ok := byUID[slots[idx].uid]
			if !ok {
				slots[idx].notFound = true
				continue
			}
			slots[idx].item = item
		}
	}

	// Pass 3: emit responses in original request order using the
	// items already loaded in pass 2.
	resps := make([]multistatusResponse, 0, len(slots))
	for _, sl := range slots {
		if sl.notFound {
			resps = append(resps, notFoundResponse(sl.hrefPath))
			continue
		}
		resps = append(resps, itemResponse(sl.page, sl.list, sl.item))
	}
	writeMultistatus(w, resps)
}

// reportCalendarQuery handles the calendar-query REPORT body. Phase 1
// does no real filtering — the request URL must point at a collection,
// and we return every live item from that collection. See the package
// doc comment for the rationale.
func (s *Server) reportCalendarQuery(w http.ResponseWriter, r *http.Request, body []byte) {
	var req calendarQueryRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		http.Error(w, "malformed calendar-query body", http.StatusBadRequest)
		return
	}
	page, list, ok := parseCollectionPath(w, r)
	if !ok {
		return
	}
	_, items, listErr := s.Backend.ListItems(r.Context(), page, list)
	if listErr != nil {
		if errors.Is(listErr, ErrCollectionNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	resps := make([]multistatusResponse, 0, len(items))
	for _, item := range items {
		resps = append(resps, itemResponse(page, list, item))
	}
	writeMultistatus(w, resps)
}

// parseCollectionPath enforces that the request URL names a calendar
// collection (page and list both populated, no uid). On any failure
// it writes a 404 to w and returns ok=false so the caller can bail.
// On success it returns the parsed (page, list) pair and ok=true.
//
// Used by both reportCalendarQuery and reportSyncCollection — neither
// REPORT type makes sense at the home-set level or against a single
// item.
func parseCollectionPath(w http.ResponseWriter, r *http.Request) (page, list string, ok bool) {
	page, list, uid, err := parsePath(r.URL.Path)
	if err != nil || page == "" || list == "" || uid != "" {
		http.NotFound(w, r)
		return "", "", false
	}
	return page, list, true
}

// notFoundResponse builds a 404-style multistatus response for a single
// href that did not resolve. RFC 4918 §13.5 prefers a top-level <status>
// (no propstat) for "the resource itself is gone", but the
// multistatusResponse type shared with PROPFIND only carries propstat
// children; emitting the 404 inside an empty propstat is the alternate
// shape that real CalDAV clients (iOS, DAVx5) accept.
func notFoundResponse(href string) multistatusResponse {
	return multistatusResponse{
		Href: href,
		Propstat: []propstat{{
			Prop:   prop{},
			Status: statusNotFound,
		}},
	}
}

// syncCollectionRequest is the RFC 6578 sync-collection request body.
// We model only `<sync-token>`; sync-level is ignored (VTODO
// collections are flat) and the `<prop>` element is informational —
// we always emit getetag + calendar-data for the changed set.
type syncCollectionRequest struct {
	XMLName   xml.Name `xml:"DAV: sync-collection"`
	SyncToken string   `xml:"DAV: sync-token"`
}

// validSyncTokenErrorBody is the response body emitted on
// ErrInvalidSyncToken — the RFC 6578 §3.2 precondition element that
// tells the client to drop its state and replay an initial full sync.
const validSyncTokenErrorBody = xmlDecl +
	`<error xmlns="DAV:"><valid-sync-token/></error>`

// reportSyncCollection handles the sync-collection REPORT body. The
// flow:
//
//  1. Decode the request body into syncCollectionRequest. A parse
//     failure is 400 Bad Request.
//  2. Re-validate the URL — sync-collection only makes sense on a
//     collection URL (`/<page>/<list>/`). Anything else is 404.
//  3. Call CalendarBackend.SyncCollection, mapping its sentinel
//     errors:
//       - ErrCollectionNotFound -> 404
//       - ErrInvalidSyncToken   -> 403 + DAV:valid-sync-token body
//       - other                 -> 500
//  4. On success, emit one multistatus response per changed item plus
//     one 404-response per deleted uid, and append the new sync-token
//     element after the responses.
func (s *Server) reportSyncCollection(w http.ResponseWriter, r *http.Request, body []byte) {
	var req syncCollectionRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		http.Error(w, "malformed sync-collection body", http.StatusBadRequest)
		return
	}
	page, list, ok := parseCollectionPath(w, r)
	if !ok {
		return
	}

	clientToken := strings.TrimSpace(req.SyncToken)
	newToken, changed, deletedUIDs, syncErr := s.Backend.SyncCollection(r.Context(), page, list, clientToken)
	if syncErr != nil {
		writeSyncCollectionError(w, syncErr)
		return
	}

	resps := make([]multistatusResponse, 0, len(changed)+len(deletedUIDs))
	for _, item := range changed {
		resps = append(resps, itemResponse(page, list, item))
	}
	for _, deletedUID := range deletedUIDs {
		resps = append(resps, notFoundResponse(deletedHref(page, list, deletedUID)))
	}
	writeMultistatusWithSyncToken(w, resps, newToken)
}

// deletedHref builds the href for a deleted item's per-uid path. The
// sync-collection multistatus response embeds this so clients can
// recognize the uid they should drop from local state.
func deletedHref(page, list, uid string) string {
	return pathSep + page + pathSep + list + pathSep + uid + icsSuffix
}

// writeSyncCollectionError maps a backend error from SyncCollection
// onto the appropriate HTTP status. ErrInvalidSyncToken gets the
// RFC 6578 precondition element so iOS / DAVx5 know to wipe their
// local state and replay an initial sync; ErrCollectionNotFound is a
// plain 404; everything else is a generic 500.
func writeSyncCollectionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidSyncToken):
		w.Header().Set("Content-Type", xmlContentType)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(validSyncTokenErrorBody))
	case errors.Is(err, ErrCollectionNotFound):
		http.Error(w, "not found", http.StatusNotFound)
	default:
		http.Error(w, internalErrorMessage, http.StatusInternalServerError)
	}
}
