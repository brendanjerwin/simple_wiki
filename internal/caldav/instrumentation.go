package caldav

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// instrumentationScope is the OTel meter / tracer name shared by every
// CalDAV instrument. Centralised so meter and tracer agree on the
// scope and so a downstream consumer of the OTLP feed only needs to
// remember one string.
const instrumentationScope = "simple_wiki/caldav"

// counterAdder is the subset of metric.Int64Counter the CalDAV server
// uses. Defining a small internal interface lets tests stage a fake
// counter that records calls, without dragging an OTel SDK into the
// test binary. The real construction path wraps a metric.Int64Counter
// with otelCounter (below) so production keeps the OTel-native
// semantics.
type counterAdder interface {
	Add(ctx context.Context, incr int64, attrs ...attribute.KeyValue)
}

// otelCounter adapts a metric.Int64Counter to the local counterAdder
// interface. Construction is the only place we deal with the OTel API
// directly; the rest of the package speaks counterAdder.
type otelCounter struct {
	c metric.Int64Counter
}

// Add forwards the increment to the underlying OTel counter, wrapping
// the supplied attribute.KeyValues in a metric.WithAttributes option
// so callers don't have to know the OTel option shape.
func (a otelCounter) Add(ctx context.Context, incr int64, attrs ...attribute.KeyValue) {
	a.c.Add(ctx, incr, metric.WithAttributes(attrs...))
}

// serverMetrics owns the per-method OTel counters the CalDAV handler
// stack feeds. The meter creating these is otel.Meter(instrumentationScope);
// when the global MeterProvider is the OTel noop default these calls
// are essentially free.
//
// Fields are interface-typed so server_test.go can inject fakes.
type serverMetrics struct {
	// requests counts every dispatched CalDAV request, attributed by
	// the CalDAV verb and an outcome bucket derived from the status
	// code (ok / client_error / server_error).
	requests counterAdder
	// bytesIn counts bytes read from the request body, attributed by
	// the CalDAV verb. Read from Content-Length to avoid burning the
	// body twice; clients that omit Content-Length contribute zero.
	bytesIn counterAdder
	// bytesOut counts bytes written to the response, attributed by
	// the CalDAV verb. Captured by the statusRecorder wrapper.
	bytesOut counterAdder
}

// metric names. Constants because revive's add-constant rule fires on
// raw string literals in metric registration, and centralising them
// makes the OTLP-side dashboards trivial to grep for.
const (
	metricRequestsTotal = "simple_wiki_caldav_requests_total"
	metricBytesInTotal  = "simple_wiki_caldav_bytes_in_total"
	metricBytesOutTotal = "simple_wiki_caldav_bytes_out_total"
)

// newServerMetrics constructs the per-method counter set against the
// process-wide OTel meter. Returns (nil, err) on any registration
// failure; callers (NewServer) treat a nil result as "instrumentation
// disabled" and proceed without metrics so a missing OTel SDK can
// never crash the server.
func newServerMetrics() (*serverMetrics, error) {
	meter := otel.Meter(instrumentationScope)
	requests, err := meter.Int64Counter(
		metricRequestsTotal,
		metric.WithDescription("Total number of CalDAV requests, attributed by method and outcome"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}
	bytesIn, err := meter.Int64Counter(
		metricBytesInTotal,
		metric.WithDescription("Total bytes read from CalDAV request bodies, attributed by method"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, err
	}
	bytesOut, err := meter.Int64Counter(
		metricBytesOutTotal,
		metric.WithDescription("Total bytes written to CalDAV response bodies, attributed by method"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, err
	}
	return &serverMetrics{
		requests: otelCounter{c: requests},
		bytesIn:  otelCounter{c: bytesIn},
		bytesOut: otelCounter{c: bytesOut},
	}, nil
}

// outcome buckets. Bucketing on the integer division of the status
// code keeps the cardinality of the attribute set tiny while still
// surfacing the only distinction operators care about: did the call
// succeed, did the client send something we rejected, or did we crash.
const (
	outcomeOK          = "ok"
	outcomeClientError = "client_error"
	outcomeServerError = "server_error"
)

// httpStatusOK and the *Error thresholds are the boundaries between
// the outcome buckets. Constants because revive's add-constant rule
// fires on the raw thresholds; named with the http* prefix to avoid
// colliding with the multistatus statusOK string in propfind.go.
const (
	httpStatusOK          = 200
	httpStatusClientError = 400
	httpStatusServerError = 500
)

// outcomeFor maps an HTTP status code to one of the three outcome
// strings. A status of 0 (handler never called WriteHeader) is
// treated as 200 OK — net/http's default.
func outcomeFor(status int) string {
	if status == 0 {
		status = httpStatusOK
	}
	switch {
	case status >= httpStatusServerError:
		return outcomeServerError
	case status >= httpStatusClientError:
		return outcomeClientError
	default:
		return outcomeOK
	}
}

// statusRecorder wraps an http.ResponseWriter to capture the status
// code and the total number of bytes written. The wrapper is only
// installed by Server.instrumented; handlers see a normal
// http.ResponseWriter.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

// WriteHeader captures the status code and forwards it to the
// underlying writer. Multiple calls record only the first status —
// matching net/http's "first WriteHeader wins" semantics.
func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

// Write tallies bytes written so the bytesOut counter sees the full
// payload, including writes that happen before an explicit
// WriteHeader (which net/http handles by stamping a 200 OK).
func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = httpStatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += int64(n)
	return n, err
}

// requestBytesIn extracts a byte count for the bytesIn counter. We
// honor Content-Length when present (the CalDAV clients we care about
// — iOS, DAVx5 — always set it on PUT) and report zero otherwise so a
// chunked-encoded request doesn't have to be drained twice. The
// parsed value is clamped at zero so a malformed negative
// Content-Length never under-counts the running total.
func requestBytesIn(r *http.Request) int64 {
	if r == nil {
		return 0
	}
	if r.ContentLength > 0 {
		return r.ContentLength
	}
	cl := strings.TrimSpace(r.Header.Get("Content-Length"))
	if cl == "" {
		return 0
	}
	n, err := strconv.ParseInt(cl, 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// caldav span attribute keys. Defined as constants both because
// revive's add-constant rule complains otherwise and because the OTLP
// consumer (Grafana / Tempo) needs stable attribute names.
const (
	attrCalDAVMethod  = "caldav.method"
	attrCalDAVPage    = "caldav.page"
	attrCalDAVList    = "caldav.list"
	attrCalDAVUID     = "caldav.uid"
	attrCalDAVOutcome = "caldav.outcome"
	attrHTTPStatus    = "http.status_code"
)

// instrumented wraps a CalDAV handler with metric and tracing
// instrumentation. The returned http.HandlerFunc:
//
//  1. Starts a server-kind span named "caldav.<method>".
//  2. Best-effort populates page/list/uid span attributes by running
//     the URL through parsePath. parsePath errors are swallowed —
//     handlers re-validate and produce a 400, which gets attributed
//     into the outcome counter as client_error.
//  3. Wraps w in a statusRecorder so we can read the status and
//     byte count after the handler returns.
//  4. Calls the inner handler.
//  5. Increments requests / bytesIn / bytesOut and attaches the
//     status to the span before ending it.
//
// When s.Metrics is nil (newServerMetrics failed at construction
// time, or a zero-value Server is used in legacy tests) the metric
// path is skipped; tracing still happens because the global tracer
// is the OTel noop tracer in that case, and noop spans are
// essentially free.
func (s *Server) instrumented(method string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := s.tracer().Start(
			r.Context(),
			"caldav."+method,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(attribute.String(attrCalDAVMethod, method)),
		)
		defer span.End()

		// Best-effort path attribution. Handlers re-validate so we
		// can ignore errors here; the worst outcome is a missing
		// span attribute on a malformed-URL trace.
		if page, list, uid, err := parsePath(r.URL.Path); err == nil {
			if page != "" {
				span.SetAttributes(attribute.String(attrCalDAVPage, page))
			}
			if list != "" {
				span.SetAttributes(attribute.String(attrCalDAVList, list))
			}
			if uid != "" {
				span.SetAttributes(attribute.String(attrCalDAVUID, uid))
			}
		}

		recorder := &statusRecorder{ResponseWriter: w}
		h(recorder, r.WithContext(ctx))

		status := recorder.status
		if status == 0 {
			status = httpStatusOK
		}
		outcome := outcomeFor(status)

		span.SetAttributes(
			attribute.Int(attrHTTPStatus, status),
			attribute.String(attrCalDAVOutcome, outcome),
		)
		if status >= httpStatusServerError {
			span.SetStatus(codes.Error, "caldav handler 5xx")
		}

		if s.Metrics != nil {
			methodAttr := attribute.String(attrCalDAVMethod, method)
			s.Metrics.requests.Add(
				ctx, 1,
				methodAttr,
				attribute.String(attrCalDAVOutcome, outcome),
			)
			s.Metrics.bytesIn.Add(ctx, requestBytesIn(r), methodAttr)
			s.Metrics.bytesOut.Add(ctx, recorder.bytes, methodAttr)
		}
	}
}

// tracer returns the Server's tracer, falling back to the global OTel
// tracer when one isn't explicitly configured. This keeps the
// zero-value Server (used by older tests that pre-date P4-C1) working
// without requiring every test to wire up tracing.
func (s *Server) tracer() trace.Tracer {
	if s.Tracer != nil {
		return s.Tracer
	}
	return otel.Tracer(instrumentationScope)
}

// newServerTracer is the construction-side helper that NewServer uses
// to populate the Tracer field. Resolved through otel.Tracer so tests
// (and production callers using the OTel SDK) can swap providers via
// otel.SetTracerProvider without rebuilding the Server.
func newServerTracer() trace.Tracer {
	return otel.Tracer(instrumentationScope)
}
