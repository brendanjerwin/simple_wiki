// Package observability provides OpenTelemetry instrumentation for the simple_wiki application.
package observability

// Histogram bucket boundaries for latency measurements.
// These follow OpenTelemetry semantic conventions for request durations.
var defaultLatencyBuckets = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0}

// HTTP-specific histogram buckets (slightly different range for web requests).
var httpLatencyBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0}

// Constants for attribute keys.
const (
	// HTTP attribute keys.
	attrHTTPMethod     = "http.method"
	attrHTTPRoute      = "http.route"
	attrHTTPStatusCode = "http.status_code"

	// gRPC attribute keys.
	attrRPCMethod     = "rpc.method"
	attrRPCStatusCode = "rpc.grpc.status_code"
	attrRPCSystem     = "rpc.system"

	// HTTP status code boundary for errors.
	httpErrorStatusThreshold = 400

	// gRPC success status.
	grpcStatusOK = "OK"
)
