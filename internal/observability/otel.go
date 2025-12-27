package observability

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	// DefaultServiceName is the default service name used when OTEL_SERVICE_NAME is not set.
	DefaultServiceName = "simple_wiki"

	// EnvOTELEnabled is the environment variable to enable OpenTelemetry.
	EnvOTELEnabled = "OTEL_ENABLED"

	// EnvServiceName is the environment variable for the service name.
	EnvServiceName = "OTEL_SERVICE_NAME"

	// shutdownTimeoutSeconds is the timeout for graceful shutdown.
	shutdownTimeoutSeconds = 5
)

// TelemetryProvider holds the OpenTelemetry providers and shutdown function.
type TelemetryProvider struct {
	tracerProvider *sdktrace.TracerProvider
	meterProvider  *sdkmetric.MeterProvider
	enabled        bool
}

// Tracer returns a named tracer from the global provider.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// Meter returns a named meter from the global provider.
func Meter(name string) metric.Meter {
	return otel.Meter(name)
}

// IsEnabled returns true if OpenTelemetry instrumentation is enabled.
func (t *TelemetryProvider) IsEnabled() bool {
	return t.enabled
}

// Shutdown gracefully shuts down the telemetry providers.
func (t *TelemetryProvider) Shutdown(ctx context.Context) error {
	if !t.enabled {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, shutdownTimeoutSeconds*time.Second)
	defer cancel()

	var errs []error

	if t.tracerProvider != nil {
		if err := t.tracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("tracer provider shutdown: %w", err))
		}
	}

	if t.meterProvider != nil {
		if err := t.meterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("meter provider shutdown: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}

	return nil
}

// Initialize sets up OpenTelemetry tracing and metrics.
// If OTEL_ENABLED is not set to "true", it returns a disabled provider (no-op).
func Initialize(ctx context.Context, version string) (*TelemetryProvider, error) {
	enabled := os.Getenv(EnvOTELEnabled)
	if enabled != "true" {
		// OTEL is not configured, return a disabled provider
		return &TelemetryProvider{enabled: false}, nil
	}

	serviceName := os.Getenv(EnvServiceName)
	if serviceName == "" {
		serviceName = DefaultServiceName
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(version),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Initialize trace exporter
	tracerProvider, err := initTracer(ctx, res)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tracer: %w", err)
	}

	// Initialize metrics exporter
	meterProvider, err := initMeter(ctx, res)
	if err != nil {
		// Clean up tracer if meter initialization fails
		_ = tracerProvider.Shutdown(ctx)
		return nil, fmt.Errorf("failed to initialize meter: %w", err)
	}

	return &TelemetryProvider{
		tracerProvider: tracerProvider,
		meterProvider:  meterProvider,
		enabled:        true,
	}, nil
}

func initTracer(ctx context.Context, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	traceExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tracerProvider)

	return tracerProvider, nil
}

func initMeter(ctx context.Context, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	metricExporter, err := otlpmetrichttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP metric exporter: %w", err)
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)

	otel.SetMeterProvider(meterProvider)

	return meterProvider, nil
}
