package observability

// RequestCounter provides an opaque interface for recording request metrics.
// Implementations can record to OTEL, wiki pages, or any other backend.
type RequestCounter interface {
	RecordHTTPRequest()
	RecordHTTPError()
	RecordGRPCRequest()
	RecordGRPCError()
}

// CompositeRequestCounter aggregates multiple RequestCounter implementations.
// All registered counters are called for each recorded event.
type CompositeRequestCounter struct {
	counters []RequestCounter
}

// NewCompositeRequestCounter creates a new CompositeRequestCounter with the given counters.
// Nil counters are filtered out.
func NewCompositeRequestCounter(counters ...RequestCounter) *CompositeRequestCounter {
	var validCounters []RequestCounter
	for _, c := range counters {
		if c != nil {
			validCounters = append(validCounters, c)
		}
	}
	return &CompositeRequestCounter{counters: validCounters}
}

// RecordHTTPRequest records an HTTP request to all registered counters.
func (c *CompositeRequestCounter) RecordHTTPRequest() {
	for _, counter := range c.counters {
		counter.RecordHTTPRequest()
	}
}

// RecordHTTPError records an HTTP error to all registered counters.
func (c *CompositeRequestCounter) RecordHTTPError() {
	for _, counter := range c.counters {
		counter.RecordHTTPError()
	}
}

// RecordGRPCRequest records a gRPC request to all registered counters.
func (c *CompositeRequestCounter) RecordGRPCRequest() {
	for _, counter := range c.counters {
		counter.RecordGRPCRequest()
	}
}

// RecordGRPCError records a gRPC error to all registered counters.
func (c *CompositeRequestCounter) RecordGRPCError() {
	for _, counter := range c.counters {
		counter.RecordGRPCError()
	}
}
