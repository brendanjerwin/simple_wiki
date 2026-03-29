package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	grpccodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GRPCInstrumentation provides gRPC interceptors for tracing and metrics collection.
type GRPCInstrumentation struct {
	tracer   trace.Tracer
	metrics  *GRPCMetrics
	counters RequestCounter
}

// NewGRPCInstrumentation creates a new GRPCInstrumentation instance.
// The counters parameter accepts any RequestCounter implementation for aggregate counting.
func NewGRPCInstrumentation(metrics *GRPCMetrics, counters RequestCounter) *GRPCInstrumentation {
	return &GRPCInstrumentation{
		tracer:   otel.Tracer("simple_wiki/grpc"),
		metrics:  metrics,
		counters: counters,
	}
}

// UnaryServerInterceptor returns a gRPC unary server interceptor for tracing and metrics.
func (g *GRPCInstrumentation) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// Start the span
		ctx, span := g.tracer.Start(ctx, info.FullMethod,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String(attrRPCSystem, "grpc"),
				attribute.String(attrRPCMethod, info.FullMethod),
			),
		)
		defer span.End()

		// Record request start
		start := time.Now()
		if g.metrics != nil {
			g.metrics.RequestStarted(ctx, info.FullMethod)
		}

		// Call the handler
		resp, err := handler(ctx, req)

		// Record request completion
		duration := time.Since(start)
		statusCode := grpccodes.OK
		if err != nil {
			if st, ok := status.FromError(err); ok {
				statusCode = st.Code()
			}
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
		} else {
			span.SetStatus(codes.Ok, "")
		}

		span.SetAttributes(attribute.String(attrRPCStatusCode, statusCode.String()))

		if g.metrics != nil {
			g.metrics.RequestFinished(ctx, info.FullMethod, statusCode.String(), duration)
		}

		// Record to aggregate counters (wiki metrics, etc.)
		if g.counters != nil {
			g.counters.RecordGRPCRequest()
			if err != nil {
				g.counters.RecordGRPCError()
			}
		}

		return resp, err
	}
}

// StreamServerInterceptor returns a gRPC stream server interceptor for tracing.
func (g *GRPCInstrumentation) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()

		// Start the span
		ctx, span := g.tracer.Start(ctx, info.FullMethod,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String(attrRPCSystem, "grpc"),
				attribute.String(attrRPCMethod, info.FullMethod),
				attribute.Bool("rpc.grpc.is_streaming", true),
			),
		)
		defer span.End()

		// Record request start
		start := time.Now()
		if g.metrics != nil {
			g.metrics.RequestStarted(ctx, info.FullMethod)
		}

		// Create a wrapped stream with the new context
		wrappedStream := &serverStreamWithContext{
			ServerStream: ss,
			ctx:          ctx,
		}

		// Call the handler
		err := handler(srv, wrappedStream)

		// Record request completion
		duration := time.Since(start)
		statusCode := grpccodes.OK
		if err != nil {
			if st, ok := status.FromError(err); ok {
				statusCode = st.Code()
			}
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
		} else {
			span.SetStatus(codes.Ok, "")
		}

		span.SetAttributes(attribute.String(attrRPCStatusCode, statusCode.String()))

		if g.metrics != nil {
			g.metrics.RequestFinished(ctx, info.FullMethod, statusCode.String(), duration)
		}

		// Record to aggregate counters (wiki metrics, etc.)
		if g.counters != nil {
			g.counters.RecordGRPCRequest()
			if err != nil {
				g.counters.RecordGRPCError()
			}
		}

		return err
	}
}

// serverStreamWithContext wraps a grpc.ServerStream to provide a custom context.
type serverStreamWithContext struct {
	grpc.ServerStream
	ctx context.Context
}

// Context returns the wrapped context.
func (s *serverStreamWithContext) Context() context.Context {
	return s.ctx
}
