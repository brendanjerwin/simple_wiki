package caldav

import (
	"encoding/xml"
	"io"
	"net/http"
	"strings"
)

// servePROPPATCH answers WebDAV PROPPATCH (RFC 4918 §9.2) on a CalDAV
// collection. iOS Reminders fires PROPPATCH during account setup to
// write displayname / calendar-color / calendar-order on each calendar
// collection it discovers. The wiki has no stable place to store those
// per-device cosmetic props (page name owns displayname), but if we
// answer 405 Method Not Allowed iOS treats the calendar as broken and
// silently stops issuing PUTs for the rest of the account's lifetime.
//
// To stay compatible we return a 207 Multi-Status whose body lists one
// `<propstat>` per requested property:
//
//   - displayname / calendar-color / calendar-order / calendar-timezone
//     — well-known cosmetic props the wiki simply ignores; reported as
//     200 OK (silently dropped) so the iOS account-setup state machine
//     sees no error.
//   - everything else — reported as 403 Forbidden, the per-prop status
//     RFC 4918 §9.2 specifies for "the server refuses to allow the
//     property to be set." iOS tolerates 403-per-prop without abandoning
//     the account.
//
// Request body shape (we only read the prop names; values are
// discarded):
//
//	<propertyupdate xmlns="DAV:">
//	  <set>
//	    <prop>
//	      <displayname>Grocery</displayname>
//	      <C:calendar-color xmlns:C="...">#ff0000</C:calendar-color>
//	    </prop>
//	  </set>
//	</propertyupdate>
//
// Anonymous callers are 403'd by requireIdentity before this method
// runs. Path-shape errors (non-collection target) return 400 — the
// only PROPPATCH iOS issues is on the collection URL.
func (s *Server) servePROPPATCH(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireIdentity(w, r); !ok {
		return
	}
	page, list, uid, err := parsePath(r.URL.Path)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if uid != "" || list == "" || page == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, proppatchBodyMaxBytes))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	propNames := parseProppatchPropNames(body)
	href := buildHref(page, list, "")
	writeProppatchResponse(w, href, propNames)
}

// proppatchBodyMaxBytes caps the PROPPATCH body. iOS Reminders sends a
// few hundred bytes; 16 KiB is generous enough for any reasonable
// client and small enough to bound a malicious payload.
const proppatchBodyMaxBytes = 16 * 1024

// proppatchProp is one element inside a <prop> child of <set> or
// <remove>. We only care about its tag (Local) — the value is ignored.
type proppatchProp struct {
	XMLName xml.Name
}

// proppatchPropContainer is the inner <prop> element. Each child is
// captured as a proppatchProp so we can see the local name (and
// optionally namespace) of every property the client wants to write.
type proppatchPropContainer struct {
	Props []proppatchProp `xml:",any"`
}

// proppatchSet is one <set> or <remove> instruction. The PROPPATCH XML
// model lets a single body carry several of these; we treat all the
// inner <prop> children identically (no semantics differ between set
// and remove for our acknowledge-everything stub).
type proppatchSet struct {
	Prop proppatchPropContainer `xml:"prop"`
}

// proppatchUpdate is the <propertyupdate> root.
type proppatchUpdate struct {
	XMLName xml.Name       `xml:"propertyupdate"`
	Set     []proppatchSet `xml:"set"`
	Remove  []proppatchSet `xml:"remove"`
}

// parseProppatchPropNames extracts the list of requested property names
// from a PROPPATCH body. Returns the names in document order with
// duplicates preserved — clients that send the same prop twice get two
// matching propstats back, which is RFC-legal and mirrors how the
// canonical CalDAV servers (Calendar Server, Radicale) reply. Any
// parse error returns nil; the caller emits an empty multistatus,
// which iOS treats as "all good" and proceeds.
func parseProppatchPropNames(body []byte) []proppatchProp {
	if len(body) == 0 {
		return nil
	}
	var update proppatchUpdate
	if err := xml.Unmarshal(body, &update); err != nil {
		return nil
	}
	var names []proppatchProp
	for _, set := range update.Set {
		names = append(names, set.Prop.Props...)
	}
	for _, rm := range update.Remove {
		names = append(names, rm.Prop.Props...)
	}
	return names
}

// writeProppatchResponse emits the 207 Multi-Status body acknowledging
// every requested prop. Cosmetic props get 200 OK; everything else
// gets 403 Forbidden. If the body is empty / unparseable the response
// is a single propstat carrying an empty <prop>, which iOS reads as
// "the verb succeeded with nothing to say" and accepts.
func writeProppatchResponse(w http.ResponseWriter, href string, props []proppatchProp) {
	if len(props) == 0 {
		body := proppatchMultistatus{
			Responses: []proppatchResponse{{
				Href: href,
				Propstat: []proppatchPropstat{{
					Status: statusOK,
				}},
			}},
		}
		writeProppatchMultistatus(w, body)
		return
	}

	var okProps, forbiddenProps []proppatchProp
	for _, p := range props {
		if isAcceptedCosmeticProp(p) {
			okProps = append(okProps, p)
		} else {
			forbiddenProps = append(forbiddenProps, p)
		}
	}

	var stats []proppatchPropstat
	if len(okProps) > 0 {
		stats = append(stats, proppatchPropstat{
			Prop:   proppatchPropEcho{Props: okProps},
			Status: statusOK,
		})
	}
	if len(forbiddenProps) > 0 {
		stats = append(stats, proppatchPropstat{
			Prop:   proppatchPropEcho{Props: forbiddenProps},
			Status: statusForbidden,
		})
	}
	body := proppatchMultistatus{
		Responses: []proppatchResponse{{
			Href:     href,
			Propstat: stats,
		}},
	}
	writeProppatchMultistatus(w, body)
}

// proppatchMultistatus is the `<multistatus>` root we emit on PROPPATCH
// responses. The shape is identical to the PROPFIND multistatus but
// the per-propstat `<prop>` element echoes back arbitrary client-named
// props, so we use a dedicated set of types rather than reusing the
// PROPFIND prop struct (which only knows our hand-coded fields).
type proppatchMultistatus struct {
	Responses []proppatchResponse
}

// proppatchResponse is one `<response>` entry in the PROPPATCH body.
type proppatchResponse struct {
	Href     string              `xml:"href"`
	Propstat []proppatchPropstat `xml:"propstat"`
}

// proppatchPropstat groups one echoed `<prop>` element with the HTTP
// status that applies to every property inside it.
type proppatchPropstat struct {
	Prop   proppatchPropEcho `xml:"prop"`
	Status string            `xml:"status"`
}

// proppatchPropEcho is the body of a `<prop>` element on a PROPPATCH
// propstat. It echoes the client's prop names back with empty values
// — RFC 4918 §9.2.1 requires the names to appear inside `<prop>` but
// permits the values to be omitted, which is the canonical shape for
// an acknowledge-only response.
type proppatchPropEcho struct {
	Props []proppatchProp
}

// MarshalXML emits each echoed prop name as a self-closing element
// inside the wrapping `<prop>`. encoding/xml writes proppatchProp's
// XMLName as the element name, so the wire form is e.g.
// `<prop><displayname/><C:calendar-color xmlns:C="..."/></prop>`.
func (p proppatchPropEcho) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, child := range p.Props {
		childStart := xml.StartElement{Name: child.XMLName}
		if err := e.EncodeToken(childStart); err != nil {
			return err
		}
		if err := e.EncodeToken(childStart.End()); err != nil {
			return err
		}
	}
	return e.EncodeToken(start.End())
}

// MarshalXML on proppatchMultistatus emits the root `<multistatus>`
// element with the same xmlns declarations as the PROPFIND multistatus
// (DAV / CalDAV / Calendar Server) so iOS parses both responses with
// the same XML namespace context.
func (m proppatchMultistatus) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
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
	return e.EncodeToken(start.End())
}

// writeProppatchMultistatus is the PROPPATCH analogue of
// writeMultistatus: serializes the body, sets headers, writes 207.
func writeProppatchMultistatus(w http.ResponseWriter, body proppatchMultistatus) {
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

// statusForbidden is the per-prop status string emitted on every
// PROPPATCH property the wiki refuses to store. Mirrors statusOK's
// `HTTP/1.1 …` shape so propstat consumers parse them with the same
// regex.
const statusForbidden = "HTTP/1.1 403 Forbidden"

// acceptedCosmeticProps is the set of property local-names the wiki
// silently accepts (200 OK) on PROPPATCH. These are the per-device
// cosmetic props iOS Reminders / DAVx5 / tasks.org write during
// account setup; pretending to accept them keeps those clients from
// flagging the calendar as read-only without forcing the wiki to
// store anything.
var acceptedCosmeticProps = map[string]struct{}{
	"displayname":           {},
	"calendar-color":        {},
	"calendar-order":        {},
	"calendar-timezone":     {},
	"calendar-description":  {},
	"calendar-free-busy-set": {},
}

// isAcceptedCosmeticProp reports whether a property local-name is on
// the silently-accepted list. Namespace is ignored — iOS uses the DAV
// namespace for displayname and the CalDAV namespace for
// calendar-color, but the local-name alone is enough to identify the
// prop unambiguously inside the PROPPATCH model.
func isAcceptedCosmeticProp(p proppatchProp) bool {
	_, ok := acceptedCosmeticProps[strings.ToLower(p.XMLName.Local)]
	return ok
}

