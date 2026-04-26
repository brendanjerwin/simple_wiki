package caldav

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
)

// CounterRecord is a single Add call captured by FakeCounter. Keeping
// the type test-only (it lives in export_test.go, only compiled with
// `go test`) means production has no notion of metric "records".
type CounterRecord struct {
	Incr  int64
	Attrs []attribute.KeyValue
}

// FakeCounter is a counterAdder that records every Add call so tests
// can assert on the increment and the attribute set. Methods are
// concurrency-safe-not-required because tests drive a single
// http.ResponseWriter through the wrapper synchronously.
type FakeCounter struct {
	Records []CounterRecord
}

// Add appends the call to Records. Implements the unexported
// counterAdder interface; the method receiver is a pointer so each
// test sees its own slice.
func (f *FakeCounter) Add(_ context.Context, incr int64, attrs ...attribute.KeyValue) {
	f.Records = append(f.Records, CounterRecord{Incr: incr, Attrs: attrs})
}

// InstallFakeMetricsForTest replaces the Server's metrics with a
// fresh FakeCounter trio and returns pointers to the fakes so the
// test can read the captured records back. Production code is
// unaffected because Metrics is only consulted from inside the
// instrumented wrapper.
func (s *Server) InstallFakeMetricsForTest() (requests, bytesIn, bytesOut *FakeCounter) {
	requests = &FakeCounter{}
	bytesIn = &FakeCounter{}
	bytesOut = &FakeCounter{}
	s.Metrics = &serverMetrics{
		requests: requests,
		bytesIn:  bytesIn,
		bytesOut: bytesOut,
	}
	return requests, bytesIn, bytesOut
}

// CallInstrumentedForTest invokes the unexported instrumented wrapper
// so tests can drive the metric / span path directly without going
// through ServeHTTP and its method-dispatch switch.
func (s *Server) CallInstrumentedForTest(method string, h http.HandlerFunc, w http.ResponseWriter, r *http.Request) {
	s.instrumented(method, h)(w, r)
}

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
