package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// HTTPMetrics provides metrics for HTTP request handling.
type HTTPMetrics struct {
	requestDuration metric.Float64Histogram
	requestTotal    metric.Int64Counter
	errorTotal      metric.Int64Counter
	activeRequests  metric.Int64UpDownCounter
}

// NewHTTPMetrics creates a new HTTPMetrics instance.
func NewHTTPMetrics() (*HTTPMetrics, error) {
	meter := otel.Meter("simple_wiki/http")

	requestDuration, err := meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("Histogram of HTTP request durations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(httpHistogramBucketBoundariesSeconds...),
	)
	if err != nil {
		return nil, err
	}

	requestTotal, err := meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	errorTotal, err := meter.Int64Counter(
		"http_errors_total",
		metric.WithDescription("Total number of HTTP errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	activeRequests, err := meter.Int64UpDownCounter(
		"http_active_requests",
		metric.WithDescription("Number of active HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	return &HTTPMetrics{
		requestDuration: requestDuration,
		requestTotal:    requestTotal,
		errorTotal:      errorTotal,
		activeRequests:  activeRequests,
	}, nil
}

// RequestStarted records the start of an HTTP request.
func (m *HTTPMetrics) RequestStarted(ctx context.Context, method, path string) {
	attrs := []attribute.KeyValue{
		attribute.String(attrHTTPMethod, method),
		attribute.String(attrHTTPRoute, path),
	}
	m.activeRequests.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RequestFinished records the completion of an HTTP request.
func (m *HTTPMetrics) RequestFinished(ctx context.Context, method, path string, statusCode int, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String(attrHTTPMethod, method),
		attribute.String(attrHTTPRoute, path),
		attribute.Int(attrHTTPStatusCode, statusCode),
	}

	m.activeRequests.Add(ctx, -1, metric.WithAttributes(
		attribute.String(attrHTTPMethod, method),
		attribute.String(attrHTTPRoute, path),
	))
	m.requestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	m.requestTotal.Add(ctx, 1, metric.WithAttributes(attrs...))

	// Record errors (4xx and 5xx status codes)
	if statusCode >= httpErrorStatusThreshold {
		errorAttrs := []attribute.KeyValue{
			attribute.String(attrHTTPMethod, method),
			attribute.String(attrHTTPRoute, path),
			attribute.Int(attrHTTPStatusCode, statusCode),
		}
		m.errorTotal.Add(ctx, 1, metric.WithAttributes(errorAttrs...))
	}
}

// RecordDuration records the duration of an HTTP request with method, path, and status code.
func (m *HTTPMetrics) RecordDuration(ctx context.Context, method, path string, statusCode int, durationSeconds float64) {
	attrs := []attribute.KeyValue{
		attribute.String(attrHTTPMethod, method),
		attribute.String(attrHTTPRoute, path),
		attribute.Int(attrHTTPStatusCode, statusCode),
	}

	m.requestDuration.Record(ctx, durationSeconds, metric.WithAttributes(attrs...))
	m.requestTotal.Add(ctx, 1, metric.WithAttributes(attrs...))

	if statusCode >= httpErrorStatusThreshold {
		m.errorTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}
