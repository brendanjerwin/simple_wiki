package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// IdentityLookupResult represents the result of an identity lookup operation.
type IdentityLookupResult string

const (
	// ResultSuccess indicates a successful identity lookup.
	ResultSuccess IdentityLookupResult = "success"
	// ResultFailure indicates a failed identity lookup.
	ResultFailure IdentityLookupResult = "failure"
	// ResultNotTailnet indicates the request was not from a Tailnet.
	ResultNotTailnet IdentityLookupResult = "not_tailnet"
)

// TailscaleMetrics provides metrics for Tailscale identity operations.
type TailscaleMetrics struct {
	lookupDuration   metric.Float64Histogram
	lookupTotal      metric.Int64Counter
	fromHeadersTotal metric.Int64Counter
}

// NewTailscaleMetrics creates a new TailscaleMetrics instance.
func NewTailscaleMetrics() (*TailscaleMetrics, error) {
	meter := otel.Meter("simple_wiki/tailscale")

	lookupDuration, err := meter.Float64Histogram(
		"tailscale_identity_lookup_duration_seconds",
		metric.WithDescription("Histogram of WhoIs lookup times"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(localOperationHistogramBucketBoundariesSeconds...),
	)
	if err != nil {
		return nil, err
	}

	lookupTotal, err := meter.Int64Counter(
		"tailscale_identity_lookup_total",
		metric.WithDescription("Counter of identity lookups with result labels"),
		metric.WithUnit("{lookup}"),
	)
	if err != nil {
		return nil, err
	}

	fromHeadersTotal, err := meter.Int64Counter(
		"tailscale_identity_from_headers_total",
		metric.WithDescription("Counter of identity extractions via Tailscale Serve headers"),
		metric.WithUnit("{extraction}"),
	)
	if err != nil {
		return nil, err
	}

	return &TailscaleMetrics{
		lookupDuration:   lookupDuration,
		lookupTotal:      lookupTotal,
		fromHeadersTotal: fromHeadersTotal,
	}, nil
}

// RecordLookup records a Tailscale identity lookup with its duration and result.
func (m *TailscaleMetrics) RecordLookup(ctx context.Context, durationSeconds float64, result IdentityLookupResult) {
	attrs := []attribute.KeyValue{
		attribute.String("result", string(result)),
	}

	m.lookupDuration.Record(ctx, durationSeconds, metric.WithAttributes(attrs...))
	m.lookupTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// RecordLookupDuration records only the duration of a lookup operation.
// This is useful when you need to measure the time separately from recording the result.
func (m *TailscaleMetrics) RecordLookupDuration(ctx context.Context, duration time.Duration, result IdentityLookupResult) {
	m.RecordLookup(ctx, duration.Seconds(), result)
}

// RecordFromHeaders increments the counter for identity extractions from headers.
func (m *TailscaleMetrics) RecordFromHeaders(ctx context.Context) {
	m.fromHeadersTotal.Add(ctx, 1)
}
