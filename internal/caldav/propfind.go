package caldav

import (
	"encoding/xml"
	"errors"
	"net/http"
)

// XML namespace constants. The CalDAV multistatus response is
// hand-rolled rather than delegated to a library so we get exact
// control over namespace prefixes — Apple Reminders is finicky about
// receiving `D:`, `C:`, and `CS:` aliases.
const (
	nsDAV          = "DAV:"
	nsCalDAV       = "urn:ietf:params:xml:ns:caldav"
	nsCalServer    = "http://calendarserver.org/ns/"
	xmlContentType = "application/xml; charset=utf-8"
)

// xmlDecl is the XML 1.0 declaration prepended to every PROPFIND
// response body. encoding/xml.Marshal does not emit one, so we write
// it manually before the multistatus element.
const xmlDecl = `<?xml version="1.0" encoding="utf-8"?>` + "\n"

// principalPath is the value used for the current-user-principal
// property on home-set responses. The wiki has no separate principal
// URL space — every authenticated request is its own principal — so
// we point it back at the home-set itself, which is what most
// CalDAV servers do when there's no separate principal collection.
const principalPath = "/"

// statusOK is the multistatus per-property status string for
// successful property fetches.
const statusOK = "HTTP/1.1 200 OK"

// depthOne is the only Depth header value that triggers child
// enumeration. RFC 4918 also defines Depth:0 (default) and
// "infinity"; CalDAV clients only send 0 or 1, and we treat any
// unrecognized value as 0 so the response stays bounded.
const depthOne = "1"

// internalErrorMessage is the body sent on backend errors that don't
// map to a more specific status. Centralized so every error path emits
// the same opaque message — clients log the HTTP status anyway.
const internalErrorMessage = "internal error"

// servePROPFIND handles WebDAV PROPFIND requests against any CalDAV
// URL. PROPFIND is the discovery primitive iOS / DAVx5 fire after the
// initial OPTIONS probe to enumerate the calendar home-set, list each
// collection's metadata (CTag, sync-token, supported components), and
// fetch per-item ETags before deciding which `.ics` resources to GET.
//
// URL shapes handled:
//
//   - /<page>                  — calendar-home-set; Depth:1 lists collections
//   - /<page>/<list>           — calendar collection; Depth:1 lists items
//   - /<page>/<list>/<uid>.ics — single item resource
//
// Depth:0 returns one <response> for the URL itself; Depth:1 adds one
// <response> per child resource. iOS sometimes omits the Depth header
// entirely (treated as Depth:0 per RFC 4918 §10.2 default).
func (s *Server) servePROPFIND(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireIdentity(w, r); !ok {
		return
	}
	page, list, uid, err := parsePath(r.URL.Path)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	depth := r.Header.Get("Depth")
	switch {
	case uid != "":
		s.propfindItem(w, r, page, list, uid)
	case list != "":
		s.propfindCollection(w, r, page, list, depth)
	case page != "":
		s.propfindHomeSet(w, r, page, depth)
	default:
		s.propfindRoot(w)
	}
}

// propfindRoot answers a PROPFIND on "/" — the discovery probe iOS,
// DAVx5, and tasks.org issue when the user enters the wiki host
// without a page path, or when the client requires creds (Basic Auth)
// before saving and pre-flights the connection. We don't have a real
// principal hierarchy; return a minimal multistatus that points
// current-user-principal at the same root so the client knows the
// connection is alive without us having to enumerate every wiki page
// as a candidate calendar-home-set.
func (*Server) propfindRoot(w http.ResponseWriter) {
	resp := multistatusResponse{
		Href: pathSep,
		Propstat: []propstat{{
			Prop: prop{
				ResourceType: &resourceType{
					Collection: &empty{},
				},
				CurrentUserPrincipal: &userPrincipal{
					Href: principalPath,
				},
			},
			Status: statusOK,
		}},
	}
	writeMultistatus(w, []multistatusResponse{resp})
}

// propfindHomeSet writes the multistatus response for a request at
// `/<page>`. Depth:0 returns just the home-set; Depth:1 enumerates
// every collection on the page.
func (s *Server) propfindHomeSet(w http.ResponseWriter, r *http.Request, page, depth string) {
	responses := []multistatusResponse{homeSetResponse(page)}
	if depth == depthOne {
		cols, err := s.Backend.ListCollections(r.Context(), page)
		if err != nil {
			http.Error(w, internalErrorMessage, http.StatusInternalServerError)
			return
		}
		for _, col := range cols {
			responses = append(responses, collectionResponse(col))
		}
	}
	writeMultistatus(w, responses)
}

// propfindCollection writes the multistatus response for a request at
// `/<page>/<list>`. Depth:0 returns just the collection; Depth:1
// enumerates every live item in the collection. Returns 404 when the
// collection does not exist.
func (s *Server) propfindCollection(w http.ResponseWriter, r *http.Request, page, list, depth string) {
	if depth == depthOne {
		col, items, err := s.Backend.ListItems(r.Context(), page, list)
		if err != nil {
			if errors.Is(err, ErrCollectionNotFound) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, internalErrorMessage, http.StatusInternalServerError)
			return
		}
		responses := []multistatusResponse{collectionResponse(col)}
		for _, item := range items {
			responses = append(responses, itemResponse(page, list, item))
		}
		writeMultistatus(w, responses)
		return
	}
	col, err := s.Backend.GetCollection(r.Context(), page, list)
	if err != nil {
		if errors.Is(err, ErrCollectionNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, internalErrorMessage, http.StatusInternalServerError)
		return
	}
	writeMultistatus(w, []multistatusResponse{collectionResponse(col)})
}

// propfindItem writes the multistatus response for a request at
// `/<page>/<list>/<uid>.ics`. There are no children below an item, so
// Depth is effectively ignored — both 0 and 1 return a single
// <response>. Returns 404 for unknown or tombstoned uids.
func (s *Server) propfindItem(w http.ResponseWriter, r *http.Request, page, list, uid string) {
	item, err := s.Backend.GetItem(r.Context(), page, list, uid)
	if err != nil {
		if errors.Is(err, ErrItemNotFound) || errors.Is(err, ErrItemDeleted) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, internalErrorMessage, http.StatusInternalServerError)
		return
	}
	writeMultistatus(w, []multistatusResponse{itemResponse(page, list, item)})
}

// homeSetResponse builds the <response> element advertising a CalDAV
// calendar-home-set. iOS reads calendar-home-set from this response
// to find the URL it should issue a Depth:1 PROPFIND against to
// enumerate collections.
func homeSetResponse(page string) multistatusResponse {
	href := buildHref(page, "", "")
	return multistatusResponse{
		Href: href,
		Propstat: []propstat{{
			Prop: prop{
				ResourceType: &resourceType{
					Collection: &empty{},
				},
				DisplayName: page,
				// Point current-user-principal at the page itself.
				// In our model the page is the account boundary, so
				// when a client follows current-user-principal back
				// here it sees displayname=<page> and labels the
				// account with the page name instead of falling
				// back to the URL's hostname.
				CurrentUserPrincipal: &userPrincipal{
					Href: href,
				},
				CalendarHomeSet: &calendarHomeSet{
					Href: href,
				},
			},
			Status: statusOK,
		}},
	}
}

// collectionResponse builds the <response> element for a single
// CalDAV calendar collection. It advertises the collection-level
// metadata (CTag, sync-token, displayname, supported components) so
// iOS / DAVx5 can decide whether to skip a sync (CTag unchanged) or
// run an incremental sync-collection REPORT (sync-token).
func collectionResponse(col CalendarCollection) multistatusResponse {
	href := buildHref(col.Page, col.ListName, "")
	return multistatusResponse{
		Href: href,
		Propstat: []propstat{{
			Prop: prop{
				ResourceType: &resourceType{
					Collection: &empty{},
					Calendar:   &empty{},
				},
				DisplayName: col.DisplayName,
				CTag:        col.CTag,
				SyncToken:   col.SyncToken,
				SupportedCalendarComponentSet: &supportedComponents{
					Comps: []comp{{Name: "VTODO"}},
				},
			},
			Status: statusOK,
		}},
	}
}

// itemResponse builds the <response> element for a single VTODO
// resource. iOS / DAVx5 use the embedded calendar-data on Depth:1
// PROPFINDs to populate task lists without firing a follow-up GET per
// item; the ETag lets them skip re-fetching unchanged items.
func itemResponse(page, list string, item CalendarItem) multistatusResponse {
	href := buildHref(page, list, item.UID)
	return multistatusResponse{
		Href: href,
		Propstat: []propstat{{
			Prop: prop{
				ResourceType:    &resourceType{},
				GetETag:         item.ETag,
				GetContentType:  iCalendarContentType,
				CalendarDataRaw: string(item.ICalBytes),
			},
			Status: statusOK,
		}},
	}
}

// writeMultistatus serializes a multistatus body to the response
// writer with the canonical CalDAV headers: 207 Multi-Status status
// and `application/xml; charset=utf-8` content type.
func writeMultistatus(w http.ResponseWriter, responses []multistatusResponse) {
	writeMultistatusWithSyncToken(w, responses, "")
}

// writeMultistatusWithSyncToken serializes a multistatus body that
// carries an RFC 6578 `<sync-token>` element after the per-resource
// responses. Used by the sync-collection REPORT handler; PROPFIND /
// calendar-multiget / calendar-query callers should use writeMultistatus
// (which omits the element entirely).
func writeMultistatusWithSyncToken(w http.ResponseWriter, responses []multistatusResponse, syncToken string) {
	body := multistatus{Responses: responses, SyncToken: syncToken}
	out, err := xml.Marshal(body)
	if err != nil {
		http.Error(w, internalErrorMessage, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", xmlContentType)
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = w.Write([]byte(xmlDecl))
	_, _ = w.Write(out)
}

// multistatus is the WebDAV `<multistatus>` root element. The custom
// MarshalXML below emits the start-element with the three namespace
// declarations CalDAV clients expect (DAV, C:CalDAV, CS:Calendar
// Server) and then defers to encoding/xml for each child response.
//
// SyncToken, when non-empty, is emitted as a trailing
// `<sync-token>URI</sync-token>` element after every response — the
// shape RFC 6578 §3.5 requires on sync-collection REPORT responses.
// For PROPFIND / calendar-multiget / calendar-query responses the
// field is left empty and the element is omitted entirely.
type multistatus struct {
	Responses []multistatusResponse
	SyncToken string
}

// MarshalXML overrides the default encoding/xml output so the root
// `<multistatus>` element carries the CalDAV and Calendar Server
// namespace declarations alongside the default DAV namespace. Without
// this hook the property elements that live in those namespaces would
// be emitted with auto-generated prefixes that some clients (notably
// older Apple Reminders builds) don't normalize correctly.
func (m multistatus) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
	start := xml.StartElement{
		Name: xml.Name{Local: "multistatus"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "xmlns"}, Value: nsDAV},
			{Name: xml.Name{Local: "xmlns:C"}, Value: nsCalDAV},
			{Name: xml.Name{Local: "xmlns:CS"}, Value: nsCalServer},
		},
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, resp := range m.Responses {
		if err := e.EncodeElement(resp, xml.StartElement{Name: xml.Name{Local: "response"}}); err != nil {
			return err
		}
	}
	if m.SyncToken != "" {
		tokenStart := xml.StartElement{Name: xml.Name{Local: "sync-token"}}
		if err := e.EncodeElement(m.SyncToken, tokenStart); err != nil {
			return err
		}
	}
	return e.EncodeToken(start.End())
}

// multistatusResponse is one `<response>` entry inside a `<multistatus>`.
// Each describes a single resource (the URL itself or a child).
type multistatusResponse struct {
	Href     string     `xml:"href"`
	Propstat []propstat `xml:"propstat"`
}

// propstat groups one <prop> with the HTTP status that applies to
// every property inside it. The wiki only emits the 200 OK group; we
// don't bother synthesizing 404 entries for properties the client
// requested but we don't have, because RFC 4918 §9.1.1 lets servers
// either omit them or report 404 — most CalDAV clients tolerate the
// omission and are stricter about the 404 case.
type propstat struct {
	Prop   prop   `xml:"prop"`
	Status string `xml:"status"`
}

// prop is the union of every property the wiki advertises. Pointer
// fields let us omit ones that don't apply to a given resource type
// (e.g. calendar-home-set on a per-item response).
type prop struct {
	ResourceType                  *resourceType        `xml:"resourcetype,omitempty"`
	DisplayName                   string               `xml:"displayname,omitempty"`
	GetETag                       string               `xml:"getetag,omitempty"`
	GetContentType                string               `xml:"getcontenttype,omitempty"`
	CTag                          string               `xml:"http://calendarserver.org/ns/ getctag,omitempty"`
	SyncToken                     string               `xml:"sync-token,omitempty"`
	CurrentUserPrincipal          *userPrincipal       `xml:"current-user-principal,omitempty"`
	CalendarHomeSet               *calendarHomeSet     `xml:"urn:ietf:params:xml:ns:caldav calendar-home-set,omitempty"`
	SupportedCalendarComponentSet *supportedComponents `xml:"urn:ietf:params:xml:ns:caldav supported-calendar-component-set,omitempty"`
	CalendarDataRaw               string               `xml:"urn:ietf:params:xml:ns:caldav calendar-data,omitempty"`
}

// resourceType is the WebDAV `<resourcetype>` value. Empty children
// are valid — `<collection/>` and `<calendar/>` are flag elements,
// not containers.
type resourceType struct {
	Collection *empty `xml:"collection,omitempty"`
	Calendar   *empty `xml:"urn:ietf:params:xml:ns:caldav calendar,omitempty"`
}

// empty is a placeholder for self-closing XML elements. encoding/xml
// emits `<foo></foo>` for a zero-value struct; we use this when the
// element should be empty regardless.
type empty struct{}

// userPrincipal is the value of `<current-user-principal>`. The
// referenced URL identifies the principal collection for the
// requester; CalDAV clients use it to discover principal-level
// resources like calendar-home-set if they aren't advertised inline.
type userPrincipal struct {
	Href string `xml:"href"`
}

// calendarHomeSet is the value of `<C:calendar-home-set>`, the CalDAV
// extension that points clients at the URL where they can issue a
// Depth:1 PROPFIND to enumerate every calendar collection accessible
// to the current principal.
type calendarHomeSet struct {
	Href string `xml:"DAV: href"`
}

// supportedComponents is the value of `<C:supported-calendar-component-set>`.
// Clients use this to filter which iCalendar component types they
// expect to receive from a calendar — the wiki only stores VTODOs.
type supportedComponents struct {
	Comps []comp `xml:"urn:ietf:params:xml:ns:caldav comp"`
}

// comp is one entry inside `<C:supported-calendar-component-set>`,
// identifying a single iCalendar component type (e.g. VTODO).
type comp struct {
	Name string `xml:"name,attr"`
}

