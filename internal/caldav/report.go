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
	"net/http"
)

// serveREPORT handles WebDAV REPORT requests. This is the skeleton —
// the real dispatch + multistatus emission lands in P1-C12/green and
// P1-C13/green.
func (s *Server) serveREPORT(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}
