# ADR-0007: Observability Architecture

## Status

Accepted

## Context

The application needs instrumentation for monitoring, debugging, and performance analysis. Key requirements:

- Distributed tracing for request flow visibility
- Metrics for performance and usage tracking
- Lightweight fallback when full observability infrastructure is unavailable
- Zero-configuration defaults with optional production configuration

## Decision

We will implement a **dual-layer observability architecture**:

### 1. OpenTelemetry Integration

Standard OpenTelemetry SDK for metrics and distributed tracing with automatic exporter selection:

- **OTLP HTTP exporter**: Used when `OTEL_EXPORTER_OTLP_ENDPOINT` is configured
- **Stdout exporter**: Fallback for development and debugging

This follows the OpenTelemetry standard environment variable conventions, allowing zero-configuration deployments with observability infrastructure.

### 2. Wiki-Based Metrics Persistence

Lightweight metrics persistence directly to a wiki page (`observability_metrics`):

- Works independently of OpenTelemetry configuration
- Uses direct frontmatter manipulation (not APIs) to avoid artificially inflating statistics
- Provides an audit trail visible within the wiki itself
- Persists via async job queue to avoid blocking request processing
- Creates a markdown template (using frontmatter references) when the page is first created
- Preserves user-customized markdown while updating frontmatter data

### 3. Environment-Based Configuration

All configuration via standard environment variables:

| Variable                       | Purpose                                               |
| ------------------------------ | ----------------------------------------------------- |
| `OTEL_ENABLED`                 | Enable OpenTelemetry (set to "true")                  |
| `OTEL_SERVICE_NAME`            | Service name for telemetry (default: "simple_wiki")   |
| `OTEL_EXPORTER_OTLP_ENDPOINT`  | OTLP collector endpoint (auto-selects OTLP exporter)  |

### 4. Instrumentation Scope

Pre-built instrumentation for:

- **HTTP**: Request counts, error rates, latency histograms
- **gRPC**: Server interceptors for tracing and metrics
- **Tailscale Identity**: Lookup latency, success/failure rates, header extractions

### 5. Relationship Between OTEL and Wiki Metrics

The two systems are complementary and can run simultaneously:

| Aspect              | OpenTelemetry Metrics                      | Wiki-Based Metrics                        |
| ------------------- | ------------------------------------------ | ----------------------------------------- |
| **Data model**      | Time-series with rich dimensions/labels    | Simple counters, no time-series           |
| **Storage**         | External collector (Prometheus, etc.)      | Wiki page frontmatter                     |
| **Query**           | Full query language (PromQL, etc.)         | Human-readable markdown table             |
| **Infrastructure**  | Requires collector/backend                 | Zero infrastructure, uses existing wiki   |
| **Use case**        | Production monitoring dashboards           | Lightweight auditing and visibility       |
| **Granularity**     | Per-request with method/path/status labels | Aggregate totals only                     |

**When to use each:**

- Use **OTEL metrics** for production monitoring, alerting, and dashboards
- Use **Wiki metrics** for simple visibility without infrastructure, auditing, and when you want metrics visible directly in the wiki

## Implementation

### Core Components

1. **TelemetryProvider**: Initializes and manages OpenTelemetry providers with automatic exporter selection
2. **WikiMetricsRecorder**: Thread-safe atomic counters with async wiki persistence
3. **RequestCounter**: Opaque interface for aggregate metric recording, allowing multiple backends
4. **CompositeRequestCounter**: Aggregates multiple RequestCounter implementations (e.g., WikiMetricsRecorder + future backends)
5. **GRPCInstrumentation**: Server interceptors wiring tracing and metrics
6. **Domain-specific metrics**: HTTPMetrics, GRPCMetrics, TailscaleMetrics

### Exporter Selection Flow

```text
OTEL_ENABLED=true?
├─ No → Return disabled provider (no-op)
└─ Yes → OTEL_EXPORTER_OTLP_ENDPOINT set?
         ├─ Yes → Use OTLP HTTP exporters
         └─ No → Use stdout exporters
```

### Usage Examples

**Basic Tracing:**

```go
tracer := observability.Tracer("simple_wiki/component")
ctx, span := tracer.Start(ctx, "operation-name")
defer span.End()
```

**Recording Metrics:**

```go
metrics, _ := observability.NewHTTPMetrics()
metrics.RequestStarted(ctx, "GET", "/api/pages")
metrics.RequestFinished(ctx, "GET", "/api/pages", 200, duration)
```

**Wiki-Based Fallback:**

```go
recorder, _ := observability.NewWikiMetricsRecorder(site, site, jobQueue, logger)
recorder.RecordHTTPRequest()
recorder.PersistAsync() // Enqueues persistence job to job queue
```

### Wiki Metrics Persistence Flow

All wiki writes go through the job queue:

```text
Cron Scheduler (every minute)
└─► Triggers wiki_metrics_persist_trigger job
    └─► Calls WikiMetricsRecorder.PersistAsync()
        └─► Enqueues observability_metrics_persist job to JobQueueCoordinator
            └─► Job queue processes job
                └─► Calls WikiMetricsRecorder.Persist()
                    ├─► Writes markdown template (if page content is empty)
                    └─► Updates frontmatter with current metrics
```

This ensures:
- Non-blocking request processing (cron triggers async job)
- Proper job queue serialization for wiki writes
- Markdown template created only once, then preserved

## Benefits

1. **Zero Configuration**: Works out of the box with stdout output
2. **Production Ready**: Standard OTLP integration for observability platforms
3. **Graceful Degradation**: Wiki-based metrics work without OTEL infrastructure
4. **Audit Trail**: Metrics persisted within wiki for historical visibility
5. **Non-Blocking**: Async persistence via job queue

## Consequences

### Positive

- Standard OpenTelemetry patterns portable to any observability platform
- Lightweight wiki metrics provide visibility without infrastructure
- Environment-based configuration follows 12-factor app principles
- No source code changes required to switch between exporters

### Negative

- Dual-layer approach adds some complexity
- Wiki-based metrics have limited query capabilities compared to time-series databases
- Stdout exporter output can be verbose in development

### Trade-offs

- Chose OTLP HTTP over gRPC for simpler firewall/proxy configuration
- Wiki metrics use atomic counters (no histograms) for simplicity

## Related Decisions

- ADR-0001: gRPC/gRPC-Web APIs (instrumented by GRPCInstrumentation)
- Job queue system for async wiki persistence
