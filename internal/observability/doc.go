// Package observability provides instrumentation for metrics and distributed tracing.
//
// This package provides both OpenTelemetry-based instrumentation and lightweight
// wiki-based metrics persistence. Both systems are wired in automatically and work
// independently - wiki metrics are always available, and OTEL metrics are enabled
// via environment variable.
//
// # Enabling OpenTelemetry
//
// Set the OTEL_ENABLED environment variable to "true" to enable OpenTelemetry:
//
//	OTEL_ENABLED=true OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 ./simple_wiki
//
// When enabled, traces and metrics are sent via OTLP HTTP to the configured endpoint.
//
// # Environment Variables
//
//   - OTEL_ENABLED: Set to "true" to enable OpenTelemetry (default: disabled)
//   - OTEL_SERVICE_NAME: The service name for telemetry (defaults to "simple_wiki")
//   - OTEL_EXPORTER_OTLP_ENDPOINT: OTLP collector endpoint
//
// # Wiki-Based Metrics
//
// The WikiMetricsRecorder provides lightweight metrics persistence to a wiki page,
// independent of OpenTelemetry. This allows tracking basic statistics even when
// OTEL is unavailable or disabled. Wiki metrics are always enabled and provide
// visibility without requiring any external infrastructure.
package observability
