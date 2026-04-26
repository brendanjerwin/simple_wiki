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

// reportMultiget handles the calendar-multiget REPORT body. Each href
// resolves through parsePath + Backend.GetItem; per-href resolution
// errors surface as a 404 inside the multistatus response so a single
// bad href doesn't poison the entire batch. The page in each href —
// not the request URL — is what's passed to GetItem, so a multiget
// that spans pages (rare but legal) works.
func (s *Server) reportMultiget(w http.ResponseWriter, r *http.Request, body []byte) {
	var req multigetRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		http.Error(w, "malformed calendar-multiget body", http.StatusBadRequest)
		return
	}
	resps := make([]multistatusResponse, 0, len(req.Hrefs))
	for _, h := range req.Hrefs {
		hrefPath := strings.TrimSpace(h.Path)
		page, list, uid, parseErr := parsePath(hrefPath)
		if parseErr != nil || uid == "" {
			resps = append(resps, notFoundResponse(hrefPath))
			continue
		}
		item, err := s.Backend.GetItem(r.Context(), page, list, uid)
		if err != nil {
			if isResourceMissing(err) {
				resps = append(resps, notFoundResponse(hrefPath))
				continue
			}
			// Any other backend error fails the whole report — we have
			// no useful per-href status to offer the client.
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		resps = append(resps, itemResponse(page, list, item))
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
	page, list, uid, err := parsePath(r.URL.Path)
	if err != nil || page == "" || list == "" || uid != "" {
		// calendar-query only makes sense on a collection URL.
		http.NotFound(w, r)
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

// isResourceMissing reports whether err means "the resource the client
// asked about does not exist on the server" — the cluster of errors
// that turn into a per-href 404 inside a multistatus response.
func isResourceMissing(err error) bool {
	return errors.Is(err, ErrItemNotFound) ||
		errors.Is(err, ErrItemDeleted) ||
		errors.Is(err, ErrCollectionNotFound)
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

// reportSyncCollection handles the sync-collection REPORT body. The
// skeleton returns 501 Not Implemented; the green pass replaces this
// with the real handler that decodes the sync-token element, calls
// CalendarBackend.SyncCollection, and emits a multistatus with a
// trailing sync-token element per RFC 6578 §3.5.
func (*Server) reportSyncCollection(w http.ResponseWriter, _ *http.Request, _ []byte) {
	http.Error(w, "sync-collection not implemented yet", http.StatusNotImplemented)
}
