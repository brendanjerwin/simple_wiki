package caldav

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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

// instrumented is a SKELETON: it returns a HandlerFunc that simply
// invokes h. The metric / span population is added in the green phase
// (P4-C1/green); shipping the wrapper as a no-op first lets ServeHTTP
// be re-routed through it without blocking on the rest of the
// instrumentation stack.
func (s *Server) instrumented(_ string, h http.HandlerFunc) http.HandlerFunc {
	return h
}

// newServerTracer is the construction-side helper that NewServer uses
// to populate the Tracer field. Resolved through otel.Tracer so tests
// (and production callers using the OTel SDK) can swap providers via
// otel.SetTracerProvider without rebuilding the Server.
func newServerTracer() trace.Tracer {
	return otel.Tracer(instrumentationScope)
}
