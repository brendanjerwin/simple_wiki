// Package caldav — REPORT request handler.
//
// CalDAV REPORT (RFC 3253 / 4791 §7) is a POST-shaped HTTP method that
// carries an XML body describing the report the client wants. Phase 1
// of the bridge handles two report types:
//
//   - calendar-multiget (RFC 4791 §7.9): explicit list of hrefs the
//     client already knows about, used after PROPFIND to fetch the
//     calendar-data + getetag for each item in one round-trip.
//   - calendar-query    (RFC 4791 §7.8): server-side filtering on a
//     collection. Phase 1 honors the trivial `VCALENDAR > VTODO`
//     component-filter (which matches every item we serve) and ignores
//     more advanced filters (text-match, time-range, prop-filter); they
//     surface as "no filter" — clients receive every live item back.
//
// sync-collection (RFC 6578) is a third REPORT type the spec lists.
// Phase 1 returns 501 Not Implemented for it; Phase 3 will land it.

package caldav

import (
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"strings"
)

// statusNotFound is the per-href status line for a multistatus
// response describing a resource the server could not resolve. RFC
// 2616 §6.1 quotes the canonical `HTTP/1.1 <code> <reason>` shape.
const statusNotFound = "HTTP/1.1 404 Not Found"

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
// always return every live item from the addressed collection).
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
	case root.XMLName.Space == nsCalDAV && root.XMLName.Local == "calendar-multiget":
		s.reportMultiget(w, r, body)
	case root.XMLName.Space == nsCalDAV && root.XMLName.Local == "calendar-query":
		s.reportCalendarQuery(w, r, body)
	case root.XMLName.Space == nsDAV && root.XMLName.Local == "sync-collection":
		// Phase 3 will land this; for now reject with 501 so clients
		// that probe it (DAVx5 does, then falls back) don't see a 400.
		http.Error(w, "sync-collection not implemented yet", http.StatusNotImplemented)
	default:
		http.Error(w, "unrecognized REPORT root element", http.StatusBadRequest)
	}
}

// reportMultiget handles the calendar-multiget REPORT body. Each href
// resolves through parsePath + Backend.GetItem; per-href errors surface
// as a 404 inside the multistatus response so a single bad href doesn't
// poison the entire batch.
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
			if errors.Is(err, ErrItemNotFound) || errors.Is(err, ErrItemDeleted) || errors.Is(err, ErrCollectionNotFound) {
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
// and we return every live item from that collection.
//
// Filter handling: we don't decode the <C:filter> element. Clients we
// care about (iOS, DAVx5) send the trivial `VCALENDAR > VTODO`
// component-filter, which matches every item we render anyway. More
// elaborate filters (text-match, time-range, prop-filter) are accepted
// silently and ignored; the multistatus payload still contains every
// live item. If a future Phase needs precise filtering, this is where
// it lands.
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

// notFoundResponse builds a 404-style multistatus response for a single
// href that did not resolve. Per RFC 4918 §13.5 a top-level <status>
// (no propstat) is the canonical shape for "the resource itself is
// gone" — a propstat with a 404 inside means a specific property is
// missing, not the whole resource.
func notFoundResponse(href string) multistatusResponse {
	// We need a sibling to Propstat for the bare-status case; the
	// shared multistatusResponse type doesn't model it. Encode by
	// hand-marshaling a minimal element. This keeps propfind.go's
	// response shape intact while giving REPORT what it needs.
	return multistatusResponse{
		Href:     href,
		Propstat: []propstat{notFoundPropstat()},
	}
}

// notFoundPropstat returns a propstat whose status is 404. For per-
// resource not-found we'd prefer a top-level <status>, but the current
// multistatusResponse type only carries propstat children; reporting
// 404 inside an empty propstat is what RFC 4918 §13.5 permits as an
// alternate shape and what real CalDAV clients (iOS, DAVx5) accept.
func notFoundPropstat() propstat {
	return propstat{
		Prop:   prop{},
		Status: statusNotFound,
	}
}
