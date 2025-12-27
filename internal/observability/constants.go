package observability

// Histogram bucket boundaries for local operation latency measurements in seconds.
// Includes sub-millisecond buckets for fast local operations like cache lookups.
var localOperationHistogramBucketBoundariesSeconds = []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0}

// Histogram bucket boundaries for network request latency measurements in seconds.
// Starts at 5ms since network requests are rarely faster than that.
var networkRequestHistogramBucketBoundariesSeconds = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0}

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

	// httpErrorStatusThreshold defines the boundary for HTTP errors.
	// Status codes >= 400 (4xx client errors and 5xx server errors) are counted as errors.
	httpErrorStatusThreshold = 400

	// gRPC success status.
	grpcStatusOK = "OK"
)
