package caldav

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/brendanjerwin/simple_wiki/tailscale"
	"go.opentelemetry.io/otel/trace"
)

// Server is the CalDAV HTTP handler. It owns the CalendarBackend
// boundary; the gateway middleware (P1-C16) constructs one Server per
// process and dispatches every CalDAV-shaped request to it.
type Server struct {
	// Backend resolves (page, list, uid) tuples to wiki data. The
	// gateway middleware injects an identity into the request context
	// before dispatching; backend implementations read it via
	// tailscale.IdentityFromContext.
	Backend CalendarBackend

	// Metrics is the per-method OTel counter set fed by the
	// instrumented wrapper around every handler. Wired up by NewServer.
	// May be nil — when the OTel meter fails to register the counters
	// we drop instrumentation rather than crashing the server, so this
	// field MUST be checked for nil before use.
	Metrics *serverMetrics

	// Tracer is the OTel tracer the instrumented wrapper uses to start
	// per-method spans. Wired up by NewServer to otel.Tracer("simple_wiki/caldav").
	// When nil (zero-value Server, used by some legacy tests) the
	// instrumented wrapper falls back to otel.Tracer at call time.
	Tracer trace.Tracer

	// AuditLogger is the destination for write-path audit log lines
	// (PUT / DELETE successes and PUT precondition-failed). When nil
	// the audit path falls back to log.Default(); tests inject a
	// buffer-backed logger to capture output. Reads are never audited.
	AuditLogger *log.Logger
}

// NewServer constructs a CalDAV Server with metrics, tracing, and audit
// logging wired up. Construction never fails — when the OTel meter
// rejects a counter registration we fall back to no metrics rather
// than aborting startup; CalDAV's value to the user has nothing to do
// with whether the OTLP feed is configured.
func NewServer(backend CalendarBackend) *Server {
	metrics, _ := newServerMetrics()
	return &Server{
		Backend: backend,
		Metrics: metrics,
		Tracer:  newServerTracer(),
	}
}

// ServeHTTP dispatches a CalDAV-shaped request to the matching method
// handler on the Server. The gateway middleware (P1-C16) runs before
// route matching and forwards every CalDAV verb (and the .ics-shaped
// GETs) here; non-CalDAV traffic never reaches this method.
//
// The implemented handlers (serveOPTIONS / servePROPFIND / serveREPORT
// / serveGET) each enforce identity via requireIdentity, so anonymous
// callers see a 403 from those branches. The unimplemented PUT / DELETE
// branches gate on identity here so the 501 isn't an information leak
// for off-tailnet probes.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodOptions:
		s.instrumented(methodLabelOptions, s.serveOPTIONS)(w, r)
	case methodPROPFIND:
		s.instrumented(methodLabelPropfind, s.servePROPFIND)(w, r)
	case methodREPORT:
		s.instrumented(methodLabelReport, s.serveREPORT)(w, r)
	case http.MethodGet, http.MethodHead:
		s.instrumented(methodLabelGet, s.serveGET)(w, r)
	case http.MethodPut:
		s.instrumented(methodLabelPut, s.servePUT)(w, r)
	case http.MethodDelete:
		s.instrumented(methodLabelDelete, s.serveDELETE)(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// methodLabel* are the lowercase verb tokens used as the {method}
// attribute on every CalDAV metric and as the suffix on every
// caldav.<method> span name. Constants because (a) revive's
// add-constant rule fires on raw string literals here and (b) any
// drift between metric and span attributes would silently break the
// OTLP-side dashboards.
const (
	methodLabelOptions  = "options"
	methodLabelPropfind = "propfind"
	methodLabelReport   = "report"
	methodLabelGet      = "get"
	methodLabelPut      = "put"
	methodLabelDelete   = "delete"
)

// methodPROPFIND is the WebDAV verb (RFC 4918 §9.1) used to retrieve
// resource properties. Declared as a constant so ServeHTTP and the
// gateway agree on the spelling.
const methodPROPFIND = "PROPFIND"

// methodREPORT is the WebDAV verb (RFC 3253) used by CalDAV for
// calendar-query, calendar-multiget, and sync-collection reports.
const methodREPORT = "REPORT"

// Path-component sanitization errors. The gateway middleware (P1-C16)
// maps these to 400 Bad Request before they ever reach a CalDAV
// handler; sanitizePathComponent and validateUID return them so the
// caller can branch on the specific failure mode for logging.
var (
	// ErrEmptyPathComponent is returned when a path component is
	// empty after trimming whitespace.
	ErrEmptyPathComponent = errors.New("caldav: path component is empty")
	// ErrPathTraversal is returned when a path component is "..".
	ErrPathTraversal = errors.New("caldav: path component is path-traversal")
	// ErrPathContainsNUL is returned when a path component contains
	// the NUL byte.
	ErrPathContainsNUL = errors.New("caldav: path component contains NUL")
	// ErrPathLeadingSeparator is returned when a path component
	// starts with a leading "/" or "\".
	ErrPathLeadingSeparator = errors.New("caldav: path component has leading separator")
	// ErrInvalidUID is returned when a uid is not a 26-character
	// Crockford-base32 ULID or an RFC 4122 UUID.
	ErrInvalidUID = errors.New("caldav: uid is not a valid ULID or UUID")
	// ErrMalformedPath is returned by parsePath when the URL path
	// does not match /<page>, /<page>/<list>, or
	// /<page>/<list>/<uid>.ics.
	ErrMalformedPath = errors.New("caldav: malformed CalDAV path")
)

// pathSep is the URL path separator. Declared as a constant to satisfy
// the add-constant lint and to centralize the value.
const pathSep = "/"

// backslash is the alternate leading-separator sanitizePathComponent
// rejects. URL paths normally use "/", but defending against "\"
// blocks Windows-style traversal payloads cheaply.
const backslash = `\`

// dotDot is the path-traversal sentinel rejected by
// sanitizePathComponent.
const dotDot = ".."

// sanitizePathComponent validates a single URL path component (page,
// list, or uid) against the CalDAV path-shape rules:
//
//   - reject any component containing the NUL byte
//   - reject leading "/" or "\" (tested before trimming so leading
//     whitespace can't smuggle a separator)
//   - reject "" after trim
//   - reject ".."
//
// Returns the trimmed component on success.
func sanitizePathComponent(s string) (string, error) {
	if strings.ContainsRune(s, 0) {
		return "", ErrPathContainsNUL
	}
	if strings.HasPrefix(s, pathSep) || strings.HasPrefix(s, backslash) {
		return "", ErrPathLeadingSeparator
	}
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return "", ErrEmptyPathComponent
	}
	if trimmed == dotDot {
		return "", ErrPathTraversal
	}
	return trimmed, nil
}

// ulidLen is the fixed length of a Crockford-base32 ULID. RFC 4122
// UUIDs (with dashes) are 36 characters, so the two formats are
// distinguishable by length alone.
const ulidLen = 26

// uuidLen is the canonical length of a hyphenated RFC 4122 UUID
// (e.g. "c5b7e0a4-3d2e-4f1a-9b8c-1234567890ab").
const uuidLen = 36

// validateUID accepts a 26-character Crockford-base32 ULID OR an RFC
// 4122 UUID (lower- or upper-case, with dashes). Anything else is
// rejected with ErrInvalidUID.
func validateUID(uid string) error {
	switch len(uid) {
	case ulidLen:
		if !isCrockfordULID(uid) {
			return ErrInvalidUID
		}
		return nil
	case uuidLen:
		if !isRFC4122UUID(uid) {
			return ErrInvalidUID
		}
		return nil
	default:
		return ErrInvalidUID
	}
}

// isCrockfordULID reports whether uid is a 26-character ULID using
// Crockford base32. Crockford base32 omits I, L, O, and U; comparison
// is case-insensitive.
func isCrockfordULID(uid string) bool {
	for i := 0; i < len(uid); i++ {
		c := uid[i]
		switch {
		case c >= '0' && c <= '9':
			// digit
		case c >= 'A' && c <= 'Z':
			if !crockfordUpper[c-'A'] {
				return false
			}
		case c >= 'a' && c <= 'z':
			if !crockfordUpper[c-'a'] {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// crockfordUpper indexes the 26 uppercase letters; true entries are
// valid Crockford base32 letters (everything except I, L, O, U).
var crockfordUpper = func() [26]bool {
	var t [26]bool
	for i := range t {
		t[i] = true
	}
	for _, excluded := range []rune{'I', 'L', 'O', 'U'} {
		t[excluded-'A'] = false
	}
	return t
}()

// uuidDashPositions are the byte offsets where dashes must appear in
// an RFC 4122 hyphenated UUID.
var uuidDashPositions = [...]int{8, 13, 18, 23}

// isRFC4122UUID reports whether uid is a 36-character UUID with
// dashes at the canonical positions and lower- or upper-case hex
// digits everywhere else.
func isRFC4122UUID(uid string) bool {
	if len(uid) != uuidLen {
		return false
	}
	for _, p := range uuidDashPositions {
		if uid[p] != '-' {
			return false
		}
	}
	for i := 0; i < len(uid); i++ {
		if isUUIDDashPosition(i) {
			continue
		}
		if !isHexDigit(uid[i]) {
			return false
		}
	}
	return true
}

// isUUIDDashPosition reports whether i is one of the four dash
// positions in a canonical hyphenated UUID.
func isUUIDDashPosition(i int) bool {
	for _, p := range uuidDashPositions {
		if p == i {
			return true
		}
	}
	return false
}

// isHexDigit reports whether c is an ASCII hexadecimal digit.
func isHexDigit(c byte) bool {
	switch {
	case c >= '0' && c <= '9':
		return true
	case c >= 'a' && c <= 'f':
		return true
	case c >= 'A' && c <= 'F':
		return true
	default:
		return false
	}
}

// icsSuffix is the file extension on item resource URLs.
const icsSuffix = ".ics"

// maxPathSegments is the largest number of "/"-separated segments a
// well-formed CalDAV path can contain (page/list/<uid>.ics).
const maxPathSegments = 3

// parsePath splits a request URL path into the wiki components a
// CalDAV handler operates on. Three accepted shapes:
//
//   - /<page>                     -> page="…", list="", uid=""
//   - /<page>/<list>              -> page="…", list="…", uid=""
//   - /<page>/<list>/<uid>.ics    -> page="…", list="…", uid="…"
//
// Each component is run through sanitizePathComponent; the uid (if
// present) is additionally checked by validateUID. A trailing slash
// on the collection URL is tolerated (RFC 4918 allows it).
//
//revive:disable-next-line:function-result-limit Returning four values keeps the call site shape natural for HTTP handlers; bundling into a struct buys nothing here.
func parsePath(reqURL string) (page, list, uid string, err error) {
	if reqURL == "" || !strings.HasPrefix(reqURL, pathSep) {
		return "", "", "", ErrMalformedPath
	}
	// Drop the leading "/" so Split doesn't yield a phantom empty
	// first segment.
	trimmed := strings.TrimPrefix(reqURL, pathSep)
	// Tolerate a single trailing slash on /<page>/<list>/.
	trimmed = strings.TrimSuffix(trimmed, pathSep)
	if trimmed == "" {
		return "", "", "", ErrMalformedPath
	}
	parts := strings.Split(trimmed, pathSep)
	if len(parts) > maxPathSegments {
		return "", "", "", ErrMalformedPath
	}
	page, err = decodePathComponent(parts[0])
	if err != nil {
		return "", "", "", err
	}
	if len(parts) == 1 {
		return page, "", "", nil
	}
	list, err = decodePathComponent(parts[1])
	if err != nil {
		return "", "", "", err
	}
	if len(parts) == 2 {
		return page, list, "", nil
	}
	// 3-segment path: third component must be <uid>.ics.
	leaf, err := decodePathComponent(parts[2])
	if err != nil {
		return "", "", "", err
	}
	if !strings.HasSuffix(leaf, icsSuffix) {
		return "", "", "", ErrMalformedPath
	}
	uid = strings.TrimSuffix(leaf, icsSuffix)
	if err := validateUID(uid); err != nil {
		return "", "", "", err
	}
	return page, list, uid, nil
}

// decodePathComponent percent-decodes one URL path segment and then
// runs it through sanitizePathComponent. Page identifiers and
// checklist names may contain characters that need escaping in URLs
// (slash, space, etc.); the gateway handler emits hrefs with those
// characters percent-encoded via buildHref, and the parser reverses
// the encoding here so the wiki data layer always sees the raw name.
func decodePathComponent(raw string) (string, error) {
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return "", ErrMalformedPath
	}
	return sanitizePathComponent(decoded)
}

// buildHref returns the URL path for a CalDAV resource, percent-
// encoding each component so characters that are reserved in URL paths
// (RFC 3986) — most importantly "/" inside a checklist name — survive
// the round trip through clients that follow the href verbatim.
//
// Pass uid="" for collection URLs (page="" is invalid; that's the
// home-set, which the caller emits as raw "/<page>/" elsewhere).
func buildHref(page, list, uid string) string {
	href := pathSep + url.PathEscape(page) + pathSep
	if list == "" {
		return href
	}
	href += url.PathEscape(list) + pathSep
	if uid == "" {
		return href
	}
	return href + url.PathEscape(uid) + icsSuffix
}

// davCapabilities is the value of the DAV response header on every
// CalDAV response. The class numbers come from RFC 4918 (1, 3) and the
// calendar-access token from RFC 4791 §5.1.
const davCapabilities = "1, 3, calendar-access"

// allowedMethods is the value of the Allow response header. Lists the
// HTTP/WebDAV/CalDAV verbs the gateway dispatches to this Server.
const allowedMethods = "OPTIONS, GET, HEAD, PROPFIND, REPORT, PUT, DELETE"

// serveOPTIONS handles OPTIONS requests against any CalDAV URL. It
// answers the WebDAV / CalDAV capability discovery probe iOS and DAVx5
// fire as the first request on a newly-configured account:
//
//   - DAV header lists the WebDAV / CalDAV class memberships we
//     support (1, 3, calendar-access).
//   - Allow header lists every method our handler will accept.
//   - 200 OK with no body.
func (s *Server) serveOPTIONS(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireIdentity(w, r); !ok {
		return
	}
	h := w.Header()
	h.Set("DAV", davCapabilities)
	h.Set("Allow", allowedMethods)
	w.WriteHeader(http.StatusOK)
}

// iCalendarContentType is the media type CalDAV clients expect on
// every per-item .ics resource (RFC 5545 §3.1, RFC 4791 §4.1).
const iCalendarContentType = "text/calendar; charset=utf-8"

// serveGET handles GET requests against /<page>/<list>/<uid>.ics. The
// CalDAV gateway only routes GETs that already match the .ics URL
// shape (the wiki's page handler owns every other GET), but we still
// re-validate via parsePath so request smuggling can't bypass the
// gateway's filter.
//
// Returns:
//   - 200 with the rendered iCalendar body and an ETag header.
//   - 404 when uid is unknown, tombstoned, or the URL doesn't name
//     an item resource.
//   - 500 on any other backend error.
func (s *Server) serveGET(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireIdentity(w, r); !ok {
		return
	}
	page, list, uid, err := parsePath(r.URL.Path)
	if err != nil || uid == "" {
		http.NotFound(w, r)
		return
	}
	item, err := s.Backend.GetItem(r.Context(), page, list, uid)
	if err != nil {
		switch {
		case errors.Is(err, ErrItemNotFound), errors.Is(err, ErrItemDeleted):
			http.NotFound(w, r)
		default:
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}
	h := w.Header()
	h.Set("Content-Type", iCalendarContentType)
	if item.ETag != "" {
		h.Set("ETag", item.ETag)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(item.ICalBytes)
}

// anonymousRejectionBody is the response body sent on every CalDAV
// request that arrives without a Tailscale identity. The plain text
// makes the failure mode explicit to anyone curling from off-tailnet;
// production clients (iOS, DAVx5) hit this only when the operator has
// misconfigured Tailscale routing.
const anonymousRejectionBody = "tailscale identity required\n"

// requireIdentity is the auth gate at the top of every CalDAV
// handler. It reads tailscale.IdentityFromContext and:
//
//   - returns (identity, true) when the identity is non-anonymous so
//     the caller can proceed.
//   - writes a 403 response with the anonymousRejectionBody and
//     returns (Anonymous, false) when the identity is anonymous.
//
// Critically, this never reads or sets the Authorization header. The
// wiki's auth model is Tailscale-only — anonymous requests fail
// closed with 403, never with a 401 challenge that would invite the
// client to retry with credentials we don't validate.
func (*Server) requireIdentity(w http.ResponseWriter, r *http.Request) (tailscale.IdentityValue, bool) {
	identity := tailscale.IdentityFromContext(r.Context())
	if identity.IsAnonymous() {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(anonymousRejectionBody))
		return tailscale.Anonymous, false
	}
	return identity, true
}

// maxPUTBodyBytes caps how many bytes servePUT will read from a
// request body before short-circuiting with 413 Payload Too Large. A
// well-formed VTODO with the wiki's 64 KB DESCRIPTION cap fits well
// under this. Pathological clients that stream gigabytes never get
// further than this read.
const maxPUTBodyBytes int64 = 256 * 1024

// iCalContentTypePrefix is the case-insensitive Content-Type prefix
// every PUT body must carry. Accepts both "text/calendar" alone and
// the canonical "text/calendar; charset=utf-8" form CalDAV clients
// send.
const iCalContentTypePrefix = "text/calendar"

// parseItemPath enforces that the request URL names an item resource
// (page, list, and uid all populated). On any failure it writes a 400
// Bad Request to w and returns ok=false; the caller should bail. On
// success it returns the parsed components and ok=true.
//
//revive:disable-next-line:function-result-limit Four returns mirror parsePath; the caller wants each component named.
func parseItemPath(w http.ResponseWriter, r *http.Request) (page, list, uid string, ok bool) {
	page, list, uid, err := parsePath(r.URL.Path)
	if err != nil || uid == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return "", "", "", false
	}
	return page, list, uid, true
}

// servePUT handles PUT against /<page>/<list>/<uid>.ics. The flow is:
//
//  1. requireIdentity — anonymous callers get 403.
//  2. parsePath — the URL must name an item resource (page, list,
//     uid all populated); anything else is 400 Bad Request.
//  3. Validate Content-Type starts with text/calendar; mismatched
//     types get 415 Unsupported Media Type.
//  4. Read the body through an io.LimitReader capped at
//     maxPUTBodyBytes. If the read produces more bytes than the cap
//     allows, return 413 Payload Too Large.
//  5. Extract If-Match / If-None-Match preconditions for the backend.
//  6. Call CalendarBackend.PutItem and map its result:
//
//     created=true  -> 201 Created  + ETag header
//     created=false -> 204 No Content + ETag header
//     ErrPreconditionFailed   -> 412
//     ErrInvalidBody, ErrUIDMismatch -> 400
//     ErrDescriptionTooLarge  -> 413
//     other                   -> 500
func (s *Server) servePUT(w http.ResponseWriter, r *http.Request) {
	identity, ok := s.requireIdentity(w, r)
	if !ok {
		return
	}
	page, list, uid, ok := parseItemPath(w, r)
	if !ok {
		return
	}
	if !isCalendarContentType(r.Header.Get("Content-Type")) {
		http.Error(w, "unsupported media type", http.StatusUnsupportedMediaType)
		return
	}
	body, tooLarge, err := readCappedBody(r.Body, maxPUTBodyBytes)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if tooLarge {
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
		return
	}
	ifMatch := stripETagQuotes(r.Header.Get("If-Match"))
	ifNoneMatch := strings.TrimSpace(r.Header.Get("If-None-Match"))

	newETag, created, err := s.Backend.PutItem(r.Context(), page, list, uid, body, ifMatch, ifNoneMatch, identity)
	audit := s.bindAuditWrite(auditActionPut, identity.Name(), page, list, uid)
	if err != nil {
		writePUTErrorStatus(w, err)
		if errors.Is(err, ErrPreconditionFailed) {
			audit(auditOutcomePreconditionFailed, "")
		}
		return
	}
	if newETag != "" {
		w.Header().Set("ETag", newETag)
	}
	if created {
		audit(auditOutcomeCreated, newETag)
		w.WriteHeader(http.StatusCreated)
		return
	}
	audit(auditOutcomeUpdated, newETag)
	w.WriteHeader(http.StatusNoContent)
}

// serveDELETE handles DELETE against /<page>/<list>/<uid>.ics. The
// flow mirrors servePUT but is much simpler:
//
//  1. requireIdentity — 403 on anonymous.
//  2. parsePath — non-item URLs return 400.
//  3. Read If-Match for precondition enforcement.
//  4. CalendarBackend.DeleteItem. Map its error:
//
//     nil                     -> 204 No Content
//     ErrItemNotFound,
//     ErrItemDeleted          -> 404
//     ErrPreconditionFailed   -> 412
//     other                   -> 500
func (s *Server) serveDELETE(w http.ResponseWriter, r *http.Request) {
	identity, ok := s.requireIdentity(w, r)
	if !ok {
		return
	}
	page, list, uid, ok := parseItemPath(w, r)
	if !ok {
		return
	}
	ifMatch := stripETagQuotes(r.Header.Get("If-Match"))

	audit := s.bindAuditWrite(auditActionDelete, identity.Name(), page, list, uid)
	if err := s.Backend.DeleteItem(r.Context(), page, list, uid, ifMatch, identity); err != nil {
		writeDELETEErrorStatus(w, err)
		if errors.Is(err, ErrPreconditionFailed) {
			audit(auditOutcomePreconditionFailed, "")
		}
		return
	}
	audit(auditOutcomeDeleted, "")
	w.WriteHeader(http.StatusNoContent)
}

// isCalendarContentType reports whether ct is a Content-Type the PUT
// handler accepts. Comparison is case-insensitive on the media type
// portion (RFC 9110 §8.3.1) and tolerates an optional `; charset=...`
// parameter.
func isCalendarContentType(ct string) bool {
	if ct == "" {
		return false
	}
	// Cut off any parameters (e.g. "; charset=utf-8") before
	// comparison so we don't reject the "text/calendar" form.
	mediaType := ct
	if i := strings.IndexByte(mediaType, ';'); i >= 0 {
		mediaType = mediaType[:i]
	}
	mediaType = strings.TrimSpace(mediaType)
	return strings.EqualFold(mediaType, iCalContentTypePrefix)
}

// readCappedBody reads up to limit+1 bytes from body. If the read
// returns more than limit bytes, tooLarge is true and the returned
// slice is empty. Otherwise body holds the request bytes (which may
// be empty). Read errors other than io.EOF are returned unchanged.
func readCappedBody(body io.Reader, limit int64) (data []byte, tooLarge bool, err error) {
	// Read one byte past the limit so we can detect overruns.
	limited := io.LimitReader(body, limit+1)
	buf, err := io.ReadAll(limited)
	if err != nil {
		return nil, false, err
	}
	if int64(len(buf)) > limit {
		return nil, true, nil
	}
	return buf, false, nil
}

// stripETagQuotes trims surrounding whitespace and the optional
// surrounding double-quotes from an HTTP If-Match header value. RFC
// 7232 §2.3 specifies the wire format `"…"` (or `W/"…"` for weak), so
// the backend wants the inner token. Returns the input unchanged
// (after trim) when no quotes are present so an explicit "*" or empty
// value passes through.
func stripETagQuotes(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, `W/`)
	v = strings.TrimSpace(v)
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return v[1 : len(v)-1]
	}
	return v
}

// writePUTErrorStatus maps a backend error to the appropriate HTTP
// status for the PUT path. Centralized so servePUT stays focused on
// the happy path.
func writePUTErrorStatus(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrPreconditionFailed):
		http.Error(w, "precondition failed", http.StatusPreconditionFailed)
	case errors.Is(err, ErrInvalidBody), errors.Is(err, ErrUIDMismatch):
		http.Error(w, "bad request", http.StatusBadRequest)
	case errors.Is(err, ErrDescriptionTooLarge):
		http.Error(w, "payload too large", http.StatusRequestEntityTooLarge)
	default:
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// writeDELETEErrorStatus maps a backend error to the appropriate HTTP
// status for the DELETE path.
func writeDELETEErrorStatus(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrItemNotFound), errors.Is(err, ErrItemDeleted):
		http.Error(w, "not found", http.StatusNotFound)
	case errors.Is(err, ErrPreconditionFailed):
		http.Error(w, "precondition failed", http.StatusPreconditionFailed)
	default:
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// Audit outcome strings emitted by auditWrite. Constants because
// (a) revive's add-constant rule fires on raw string literals, and
// (b) downstream log-aggregation rules (Loki / Grafana) hinge on the
// exact spelling — drift would silently break dashboards.
const (
	auditOutcomeCreated            = "created"
	auditOutcomeUpdated            = "updated"
	auditOutcomeDeleted            = "deleted"
	auditOutcomePreconditionFailed = "precondition_failed"
	// auditActionPut and auditActionDelete are the action= field
	// values; only PUT and DELETE are audited (reads have no useful
	// audit signal).
	auditActionPut    = "put"
	auditActionDelete = "delete"
)

// bindAuditWrite returns a closure that fills in the per-write fields
// (action / principal / page / list / uid) and lets the caller supply
// only the per-outcome ones (outcome / etag). The five "common" fields
// are captured at handler-entry time so the per-outcome call sites
// stay short and don't re-name `identity.Name()` on every line.
func (s *Server) bindAuditWrite(action, principal, page, list, uid string) func(outcome, etag string) {
	return func(outcome, etag string) {
		s.auditWrite(action, principal, page, list, uid, outcome, etag)
	}
}

// auditWrite emits a single structured log line summarising a
// CalDAV write attempt. Called from servePUT and serveDELETE on the
// outcomes the audit log cares about: PUT created / updated / 412,
// DELETE 204 / 412. Reads, 4xx-other-than-412, and 5xx are NOT
// audited — those either have no business signal (reads, 5xx leaks)
// or no consistent identity (the 403 anonymous-rejection happens
// before the handler logic, so there's no Tailscale principal to
// attribute the call to).
//
// Format is a single line keyed by `caldav: action=… principal=… …`
// so log-aggregation rules can split it deterministically. etag is
// optional (DELETE doesn't carry one); pass "" to omit the field.
//
// The destination logger is s.AuditLogger when set, falling back to
// log.Default() so production callers that haven't constructed via
// NewServer still get audit lines on stderr.
func (s *Server) auditWrite(action, principal, page, list, uid, outcome, etag string) {
	logger := s.AuditLogger
	if logger == nil {
		logger = log.Default()
	}
	line := fmt.Sprintf(
		`caldav: action=%s principal=%q page=%q list=%q uid=%q outcome=%s`,
		action, principal, page, list, uid, outcome,
	)
	if etag != "" {
		// Quote the etag so the line stays parseable even though the
		// etag itself contains the W/"…" wire form. Embedded double
		// quotes are escaped with a backslash via %q.
		line += fmt.Sprintf(` etag=%q`, etag)
	}
	logger.Println(line)
}
