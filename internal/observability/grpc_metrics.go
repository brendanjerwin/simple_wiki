package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// GRPCMetrics provides metrics for gRPC request handling.
type GRPCMetrics struct {
	requestDuration metric.Float64Histogram
	requestTotal    metric.Int64Counter
	errorTotal      metric.Int64Counter
	activeRequests  metric.Int64UpDownCounter
}

func NewGRPCMetrics() (*GRPCMetrics, error) {
	meter := otel.Meter("simple_wiki/grpc")

	requestDuration, err := meter.Float64Histogram(
		"grpc_request_duration_seconds",
		metric.WithDescription("Histogram of gRPC request durations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(httpHistogramBucketBoundariesSeconds...),
	)
	if err != nil {
		return nil, err
	}

	requestTotal, err := meter.Int64Counter(
		"grpc_requests_total",
		metric.WithDescription("Total number of gRPC requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	errorTotal, err := meter.Int64Counter(
		"grpc_errors_total",
		metric.WithDescription("Total number of gRPC errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return nil, err
	}

	activeRequests, err := meter.Int64UpDownCounter(
		"grpc_active_requests",
		metric.WithDescription("Number of active gRPC requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	return &GRPCMetrics{
		requestDuration: requestDuration,
		requestTotal:    requestTotal,
		errorTotal:      errorTotal,
		activeRequests:  activeRequests,
	}, nil
}

// RequestStarted records the start of a gRPC request.
func (m *GRPCMetrics) RequestStarted(ctx context.Context, method string) {
	attrs := []attribute.KeyValue{
		attribute.String(attrRPCMethod, method),
	}
	m.activeRequests.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RequestFinished records the completion of a gRPC request.
func (m *GRPCMetrics) RequestFinished(ctx context.Context, method string, code string, duration time.Duration) {
	attrs := []attribute.KeyValue{
		attribute.String(attrRPCMethod, method),
		attribute.String(attrRPCStatusCode, code),
	}

	m.activeRequests.Add(ctx, -1, metric.WithAttributes(attribute.String(attrRPCMethod, method)))
	m.requestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	m.requestTotal.Add(ctx, 1, metric.WithAttributes(attrs...))

	// Record errors (anything other than OK)
	if code != grpcStatusOK {
		m.errorTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}

// RecordDuration records the duration of a gRPC request with method and status code.
func (m *GRPCMetrics) RecordDuration(ctx context.Context, method, code string, durationSeconds float64) {
	attrs := []attribute.KeyValue{
		attribute.String(attrRPCMethod, method),
		attribute.String(attrRPCStatusCode, code),
	}

	m.requestDuration.Record(ctx, durationSeconds, metric.WithAttributes(attrs...))
	m.requestTotal.Add(ctx, 1, metric.WithAttributes(attrs...))

	if code != grpcStatusOK {
		m.errorTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
}
