package observability

// Histogram bucket boundaries for network request latency measurements in seconds.
// Starts at 5ms since network requests are rarely faster than that.
var networkRequestHistogramBucketBoundariesSeconds = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0}

// Histogram bucket boundaries for local IPC operations (e.g., Tailscale WhoIs lookups).
// Starts at 100µs since local operations can be very fast.
var localOperationHistogramBucketBoundariesSeconds = []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5}

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
	// Only 5xx server errors are counted as errors. 4xx client errors (404 not found,
	// 400 bad request, etc.) are expected behavior and not counted as server errors.
	httpErrorStatusThreshold = 500

	// gRPC success status.
	grpcStatusOK = "OK"
)
