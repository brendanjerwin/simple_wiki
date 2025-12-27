package observability

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// HTTPInstrumentation provides Gin middleware for tracing and metrics collection.
type HTTPInstrumentation struct {
	tracer   trace.Tracer
	metrics  *HTTPMetrics
	counters RequestCounter
}

// NewHTTPInstrumentation creates a new HTTPInstrumentation instance.
// The counters parameter accepts any RequestCounter implementation for aggregate counting.
func NewHTTPInstrumentation(metrics *HTTPMetrics, counters RequestCounter) *HTTPInstrumentation {
	return &HTTPInstrumentation{
		tracer:   otel.Tracer("simple_wiki/http"),
		metrics:  metrics,
		counters: counters,
	}
}

// GinMiddleware returns a Gin middleware handler for tracing and metrics.
func (h *HTTPInstrumentation) GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		// Start the span
		ctx, span := h.tracer.Start(c.Request.Context(), method+" "+path,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String(attrHTTPMethod, method),
				attribute.String(attrHTTPRoute, path),
			),
		)
		defer span.End()

		// Update request context with span
		c.Request = c.Request.WithContext(ctx)

		// Record request start
		start := time.Now()
		if h.metrics != nil {
			h.metrics.RequestStarted(ctx, method, path)
		}

		// Process request
		c.Next()

		// Record request completion
		duration := time.Since(start)
		statusCode := c.Writer.Status()

		span.SetAttributes(attribute.Int(attrHTTPStatusCode, statusCode))

		isError := statusCode >= httpErrorStatusThreshold
		if isError {
			span.SetStatus(codes.Error, "HTTP error")
		} else {
			span.SetStatus(codes.Ok, "")
		}

		if h.metrics != nil {
			h.metrics.RequestFinished(ctx, method, path, statusCode, duration)
		}

		// Record to aggregate counters (wiki metrics, etc.)
		if h.counters != nil {
			h.counters.RecordHTTPRequest()
			if isError {
				h.counters.RecordHTTPError()
			}
		}
	}
}
