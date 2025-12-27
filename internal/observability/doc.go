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
// By default, traces and metrics are exported to stdout. For production use with
// an OTLP-compatible collector (like Jaeger, Zipkin, or Grafana Agent), replace
// the stdout exporters with OTLP HTTP/gRPC exporters:
//
//	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp
//	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp
//
// # Environment Variables
//
//   - OTEL_ENABLED: Set to "true" to enable OpenTelemetry (default: disabled)
//   - OTEL_SERVICE_NAME: The service name for telemetry (defaults to "simple_wiki")
//   - OTEL_RESOURCE_ATTRIBUTES: Additional resource attributes in key=value,key2=value2 format
//
// # Wiki-Based Metrics
//
// The WikiMetricsRecorder provides lightweight metrics persistence to a wiki page,
// independent of OpenTelemetry. This allows tracking basic statistics even when
// OTEL is unavailable or disabled.
package observability
