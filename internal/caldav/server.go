package caldav

import (
	"errors"
	"net/http"
	"strings"

	"github.com/brendanjerwin/simple_wiki/tailscale"
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
}

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
	page, err = sanitizePathComponent(parts[0])
	if err != nil {
		return "", "", "", err
	}
	if len(parts) == 1 {
		return page, "", "", nil
	}
	list, err = sanitizePathComponent(parts[1])
	if err != nil {
		return "", "", "", err
	}
	if len(parts) == 2 {
		return page, list, "", nil
	}
	// 3-segment path: third component must be <uid>.ics.
	leaf, err := sanitizePathComponent(parts[2])
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
