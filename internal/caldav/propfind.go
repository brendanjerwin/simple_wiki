package caldav

import (
	"net/http"
)

// servePROPFIND handles WebDAV PROPFIND requests against any CalDAV
// URL. PROPFIND is the discovery primitive iOS / DAVx5 fire after the
// initial OPTIONS probe to enumerate the calendar home-set, list each
// collection's metadata (CTag, sync-token, supported components), and
// fetch per-item ETags before deciding which `.ics` resources to GET.
//
// URL shapes handled:
//
//   - /<page>            -> calendar-home-set; Depth:1 lists collections
//   - /<page>/<list>     -> calendar collection; Depth:1 lists items
//   - /<page>/<list>/<uid>.ics -> single item (Depth always 0-equivalent)
//
// This is a skeleton — the real implementation lands in P1-C11/green.
func (s *Server) servePROPFIND(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}
