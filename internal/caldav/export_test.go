package caldav

import "net/http"

// Test-only re-exports for unexported helpers. Keeping them in
// export_test.go (compiled only with `go test`) means the public API
// surface stays minimal.

// SanitizePathComponentForTest is the test-only re-export of
// sanitizePathComponent.
func SanitizePathComponentForTest(s string) (string, error) {
	return sanitizePathComponent(s)
}

// ValidateUIDForTest is the test-only re-export of validateUID.
func ValidateUIDForTest(uid string) error {
	return validateUID(uid)
}

// ParsePathForTest is the test-only re-export of parsePath.
//
//revive:disable-next-line:function-result-limit Mirrors parsePath's shape; bundling into a struct adds noise in tests.
func ParsePathForTest(reqURL string) (page, list, uid string, err error) {
	return parsePath(reqURL)
}

// ServeOPTIONSForTest is the test-only re-export of (*Server).serveOPTIONS.
func (s *Server) ServeOPTIONSForTest(w http.ResponseWriter, r *http.Request) {
	s.serveOPTIONS(w, r)
}

// ServeGETForTest is the test-only re-export of (*Server).serveGET.
func (s *Server) ServeGETForTest(w http.ResponseWriter, r *http.Request) {
	s.serveGET(w, r)
}

// ServePROPFINDForTest is the test-only re-export of (*Server).servePROPFIND.
func (s *Server) ServePROPFINDForTest(w http.ResponseWriter, r *http.Request) {
	s.servePROPFIND(w, r)
}

// ServeREPORTForTest is the test-only re-export of (*Server).serveREPORT.
func (s *Server) ServeREPORTForTest(w http.ResponseWriter, r *http.Request) {
	s.serveREPORT(w, r)
}

// RequireIdentityForTest is the test-only re-export of
// (*Server).requireIdentity. The second return is the "ok" flag —
// true means the caller may proceed.
func (s *Server) RequireIdentityForTest(w http.ResponseWriter, r *http.Request) bool {
	_, ok := s.requireIdentity(w, r)
	return ok
}

// ServePUTForTest is the test-only re-export of (*Server).servePUT.
func (s *Server) ServePUTForTest(w http.ResponseWriter, r *http.Request) {
	s.servePUT(w, r)
}

// ServeDELETEForTest is the test-only re-export of (*Server).serveDELETE.
func (s *Server) ServeDELETEForTest(w http.ResponseWriter, r *http.Request) {
	s.serveDELETE(w, r)
}
