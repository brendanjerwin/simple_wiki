// Package observability provides instrumentation for metrics and distributed tracing.
//
// This package provides both OpenTelemetry-based instrumentation and lightweight
// wiki-based metrics persistence. Instrumentation works independently when OTEL
// is unavailable.
//
// # Enabling OpenTelemetry
//
// Set the OTEL_ENABLED environment variable to "true" to enable OpenTelemetry:
//
//	OTEL_ENABLED=true ./simple_wiki
//
// # OTLP Exporter Configuration
//
// The exporter is automatically selected based on environment variables:
//
//   - If OTEL_EXPORTER_OTLP_ENDPOINT is set, traces and metrics are sent via OTLP HTTP
//   - If OTEL_EXPORTER_OTLP_ENDPOINT is not set, traces and metrics are written to stdout
//
// Example with OTLP collector:
//
//	OTEL_ENABLED=true OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 ./simple_wiki
//
// # Environment Variables
//
//   - OTEL_ENABLED: Set to "true" to enable OpenTelemetry (default: disabled)
//   - OTEL_SERVICE_NAME: The service name for telemetry (defaults to "simple_wiki")
//   - OTEL_EXPORTER_OTLP_ENDPOINT: OTLP collector endpoint (if set, uses OTLP HTTP exporter)
//
// # Wiki-Based Metrics
//
// The WikiMetricsRecorder provides lightweight metrics persistence to a wiki page,
// independent of OpenTelemetry. This allows tracking basic statistics even when
// OTEL is unavailable or disabled.
package observability
