//revive:disable:dot-imports
package observability_test

import (
	"context"
	"errors"

	"github.com/brendanjerwin/simple_wiki/internal/observability"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockCounter tracks calls for testing interceptors.
type mockGRPCCounter struct {
	grpcRequests int
	grpcErrors   int
}

func (*mockGRPCCounter) RecordHTTPRequest()                                          {}
func (*mockGRPCCounter) RecordHTTPError()                                            {}
func (m *mockGRPCCounter) RecordGRPCRequest()                                        { m.grpcRequests++ }
func (m *mockGRPCCounter) RecordGRPCError()                                          { m.grpcErrors++ }
func (*mockGRPCCounter) RecordTailscaleLookup(_ observability.IdentityLookupResult)  {}
func (*mockGRPCCounter) RecordHeaderExtraction()                                     {}

// mockServerStream implements grpc.ServerStream for testing.
type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	return m.ctx
}

var _ = Describe("GRPCInstrumentation", func() {
	Describe("NewGRPCInstrumentation", func() {
		When("creating with metrics", func() {
			var instrumentation *observability.GRPCInstrumentation

			BeforeEach(func() {
				metrics, err := observability.NewGRPCMetrics()
				Expect(err).ToNot(HaveOccurred())
				instrumentation = observability.NewGRPCInstrumentation(metrics, nil)
			})

			It("should return a non-nil instrumentation", func() {
				Expect(instrumentation).ToNot(BeNil())
			})
		})

		When("creating without metrics (nil)", func() {
			var instrumentation *observability.GRPCInstrumentation

			BeforeEach(func() {
				instrumentation = observability.NewGRPCInstrumentation(nil, nil)
			})

			It("should return a non-nil instrumentation", func() {
				Expect(instrumentation).ToNot(BeNil())
			})
		})
	})

	Describe("UnaryServerInterceptor", func() {
		var interceptor grpc.UnaryServerInterceptor
		var counter *mockGRPCCounter

		BeforeEach(func() {
			counter = &mockGRPCCounter{}
			instrumentation := observability.NewGRPCInstrumentation(nil, counter)
			interceptor = instrumentation.UnaryServerInterceptor()
		})

		When("handling a successful request", func() {
			var resp any
			var err error

			BeforeEach(func() {
				handler := func(ctx context.Context, req any) (any, error) {
					return "success", nil
				}
				info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
				resp, err = interceptor(context.Background(), nil, info, handler)
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return the handler response", func() {
				Expect(resp).To(Equal("success"))
			})

			It("should record a gRPC request", func() {
				Expect(counter.grpcRequests).To(Equal(1))
			})

			It("should not record a gRPC error", func() {
				Expect(counter.grpcErrors).To(Equal(0))
			})
		})

		When("handling an error request", func() {
			var resp any
			var err error

			BeforeEach(func() {
				handler := func(ctx context.Context, req any) (any, error) {
					return nil, status.Error(codes.Internal, "internal error")
				}
				info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
				resp, err = interceptor(context.Background(), nil, info, handler)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should return nil response", func() {
				Expect(resp).To(BeNil())
			})

			It("should record a gRPC request", func() {
				Expect(counter.grpcRequests).To(Equal(1))
			})

			It("should record a gRPC error", func() {
				Expect(counter.grpcErrors).To(Equal(1))
			})
		})

		When("handler returns non-gRPC error", func() {
			var err error

			BeforeEach(func() {
				handler := func(ctx context.Context, req any) (any, error) {
					return nil, errors.New("plain error")
				}
				info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}
				_, err = interceptor(context.Background(), nil, info, handler)
			})

			It("should return the error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should record a gRPC error", func() {
				Expect(counter.grpcErrors).To(Equal(1))
			})
		})
	})

	Describe("StreamServerInterceptor", func() {
		var interceptor grpc.StreamServerInterceptor
		var counter *mockGRPCCounter

		BeforeEach(func() {
			counter = &mockGRPCCounter{}
			instrumentation := observability.NewGRPCInstrumentation(nil, counter)
			interceptor = instrumentation.StreamServerInterceptor()
		})

		When("handling a successful stream", func() {
			var err error

			BeforeEach(func() {
				handler := func(srv any, stream grpc.ServerStream) error {
					return nil
				}
				info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}
				stream := &mockServerStream{ctx: context.Background()}
				err = interceptor(nil, stream, info, handler)
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should record a gRPC request", func() {
				Expect(counter.grpcRequests).To(Equal(1))
			})

			It("should not record a gRPC error", func() {
				Expect(counter.grpcErrors).To(Equal(0))
			})
		})

		When("handling an error stream", func() {
			var err error

			BeforeEach(func() {
				handler := func(srv any, stream grpc.ServerStream) error {
					return status.Error(codes.Unavailable, "service unavailable")
				}
				info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}
				stream := &mockServerStream{ctx: context.Background()}
				err = interceptor(nil, stream, info, handler)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should record a gRPC request", func() {
				Expect(counter.grpcRequests).To(Equal(1))
			})

			It("should record a gRPC error", func() {
				Expect(counter.grpcErrors).To(Equal(1))
			})
		})

		When("interceptor is created without counters", func() {
			var err error

			BeforeEach(func() {
				instrumentation := observability.NewGRPCInstrumentation(nil, nil)
				interceptor = instrumentation.StreamServerInterceptor()
				handler := func(srv any, stream grpc.ServerStream) error {
					return nil
				}
				info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}
				stream := &mockServerStream{ctx: context.Background()}
				err = interceptor(nil, stream, info, handler)
			})

			It("should still process streams successfully", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})

		When("handler accesses context from wrapped stream", func() {
			var capturedCtx context.Context
			var err error

			BeforeEach(func() {
				instrumentation := observability.NewGRPCInstrumentation(nil, nil)
				interceptor = instrumentation.StreamServerInterceptor()
				handler := func(srv any, stream grpc.ServerStream) error {
					capturedCtx = stream.Context()
					return nil
				}
				info := &grpc.StreamServerInfo{FullMethod: "/test.Service/StreamMethod"}
				originalCtx := context.WithValue(context.Background(), ctxKey("test"), "value")
				stream := &mockServerStream{ctx: originalCtx}
				err = interceptor(nil, stream, info, handler)
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should provide a context with tracing span", func() {
				// The wrapped stream should provide a context that includes the tracing span
				Expect(capturedCtx).ToNot(BeNil())
			})
		})
	})
})

// ctxKey is a type for context keys to avoid collisions.
type ctxKey string
