package tailscale_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/brendanjerwin/simple_wiki/tailscale"
)

// mockServerStream implements grpc.ServerStream for testing.
type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	return m.ctx
}

var _ = Describe("IdentityInterceptor", func() {
	var (
		interceptor grpc.UnaryServerInterceptor
	)

	Describe("constructor validation", func() {
		When("logger is nil", func() {
			var err error

			BeforeEach(func() {
				_, err = tailscale.IdentityInterceptor(nil, nil)
			})

			It("should return an error", func() {
				Expect(err).To(MatchError("logger is required"))
			})
		})
	})

	Describe("extracting identity from metadata", func() {
		When("Tailscale-User-Login metadata is present", func() {
			var (
				capturedIdentity tailscale.IdentityValue
				handlerCalled    bool
			)

			BeforeEach(func() {
				var err error
				interceptor, err = tailscale.IdentityInterceptor(nil, testLogger())
				Expect(err).NotTo(HaveOccurred())

				md := metadata.New(map[string]string{
					"tailscale-user-login": "user@example.com",
					"tailscale-user-name":  "Test User",
				})
				ctx := metadata.NewIncomingContext(context.Background(), md)

				handler := func(ctx context.Context, req any) (any, error) {
					handlerCalled = true
					capturedIdentity = tailscale.IdentityFromContext(ctx)
					return "response", nil
				}

				_, _ = interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
			})

			It("should call the handler", func() {
				Expect(handlerCalled).To(BeTrue())
			})

			It("should not be anonymous", func() {
				Expect(capturedIdentity.IsAnonymous()).To(BeFalse())
			})

			It("should have the correct login name", func() {
				Expect(capturedIdentity.LoginName()).To(Equal("user@example.com"))
			})

			It("should have the correct display name", func() {
				Expect(capturedIdentity.DisplayName()).To(Equal("Test User"))
			})
		})

		When("only login name metadata is present", func() {
			var (
				capturedIdentity tailscale.IdentityValue
			)

			BeforeEach(func() {
				var err error
				interceptor, err = tailscale.IdentityInterceptor(nil, testLogger())
				Expect(err).NotTo(HaveOccurred())

				md := metadata.New(map[string]string{
					"tailscale-user-login": "user@example.com",
				})
				ctx := metadata.NewIncomingContext(context.Background(), md)

				handler := func(ctx context.Context, req any) (any, error) {
					capturedIdentity = tailscale.IdentityFromContext(ctx)
					return "response", nil
				}

				_, _ = interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
			})

			It("should not be anonymous", func() {
				Expect(capturedIdentity.IsAnonymous()).To(BeFalse())
			})

			It("should have the correct login name", func() {
				Expect(capturedIdentity.LoginName()).To(Equal("user@example.com"))
			})

			It("should have empty display name", func() {
				Expect(capturedIdentity.DisplayName()).To(BeEmpty())
			})
		})
	})

	Describe("without metadata", func() {
		When("no metadata and no resolver", func() {
			var (
				capturedIdentity tailscale.IdentityValue
				handlerCalled    bool
			)

			BeforeEach(func() {
				var err error
				interceptor, err = tailscale.IdentityInterceptor(nil, testLogger())
				Expect(err).NotTo(HaveOccurred())

				handler := func(ctx context.Context, req any) (any, error) {
					handlerCalled = true
					capturedIdentity = tailscale.IdentityFromContext(ctx)
					return "response", nil
				}

				_, _ = interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
			})

			It("should still call the handler", func() {
				Expect(handlerCalled).To(BeTrue())
			})

			It("should have anonymous identity in context", func() {
				Expect(capturedIdentity.IsAnonymous()).To(BeTrue())
			})

			It("should equal Anonymous singleton", func() {
				Expect(capturedIdentity).To(Equal(tailscale.Anonymous))
			})
		})
	})

	Describe("metadata priority over WhoIs", func() {
		When("metadata is present and resolver would return different identity", func() {
			var (
				capturedIdentity tailscale.IdentityValue
			)

			BeforeEach(func() {
				resolver := &mockIdentityResolver{
					identity: tailscale.NewIdentity("whois@example.com", "", "my-laptop"),
					err:      nil,
				}

				var err error
				interceptor, err = tailscale.IdentityInterceptor(resolver, testLogger())
				Expect(err).NotTo(HaveOccurred())

				md := metadata.New(map[string]string{
					"tailscale-user-login": "header@example.com",
				})
				ctx := metadata.NewIncomingContext(context.Background(), md)

				handler := func(ctx context.Context, req any) (any, error) {
					capturedIdentity = tailscale.IdentityFromContext(ctx)
					return "response", nil
				}

				_, _ = interceptor(ctx, nil, &grpc.UnaryServerInfo{}, handler)
			})

			It("should prefer metadata identity over WhoIs", func() {
				Expect(capturedIdentity.LoginName()).To(Equal("header@example.com"))
			})
		})
	})

	Describe("handler return values", func() {
		When("handler returns a response and no error", func() {
			var (
				resp any
				err  error
			)

			BeforeEach(func() {
				var createErr error
				interceptor, createErr = tailscale.IdentityInterceptor(nil, testLogger())
				Expect(createErr).NotTo(HaveOccurred())

				handler := func(ctx context.Context, req any) (any, error) {
					return "test-response", nil
				}

				resp, err = interceptor(context.Background(), nil, &grpc.UnaryServerInfo{}, handler)
			})

			It("should return the handler response", func() {
				Expect(resp).To(Equal("test-response"))
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})

var _ = Describe("IdentityStreamInterceptor", func() {
	var (
		interceptor grpc.StreamServerInterceptor
	)

	Describe("constructor validation", func() {
		When("logger is nil", func() {
			var err error

			BeforeEach(func() {
				_, err = tailscale.IdentityStreamInterceptor(nil, nil)
			})

			It("should return an error", func() {
				Expect(err).To(MatchError("logger is required"))
			})
		})
	})

	Describe("extracting identity from metadata", func() {
		When("Tailscale-User-Login metadata is present", func() {
			var (
				capturedIdentity tailscale.IdentityValue
				handlerCalled    bool
			)

			BeforeEach(func() {
				var err error
				interceptor, err = tailscale.IdentityStreamInterceptor(nil, testLogger())
				Expect(err).NotTo(HaveOccurred())

				md := metadata.New(map[string]string{
					"tailscale-user-login": "stream-user@example.com",
					"tailscale-user-name":  "Stream User",
				})
				ctx := metadata.NewIncomingContext(context.Background(), md)
				stream := &mockServerStream{ctx: ctx}

				handler := func(srv any, ss grpc.ServerStream) error {
					handlerCalled = true
					capturedIdentity = tailscale.IdentityFromContext(ss.Context())
					return nil
				}

				_ = interceptor(nil, stream, &grpc.StreamServerInfo{}, handler)
			})

			It("should call the handler", func() {
				Expect(handlerCalled).To(BeTrue())
			})

			It("should not be anonymous", func() {
				Expect(capturedIdentity.IsAnonymous()).To(BeFalse())
			})

			It("should have the correct login name", func() {
				Expect(capturedIdentity.LoginName()).To(Equal("stream-user@example.com"))
			})

			It("should have the correct display name", func() {
				Expect(capturedIdentity.DisplayName()).To(Equal("Stream User"))
			})
		})
	})

	Describe("without metadata", func() {
		When("no metadata and no resolver", func() {
			var (
				capturedIdentity tailscale.IdentityValue
			)

			BeforeEach(func() {
				var err error
				interceptor, err = tailscale.IdentityStreamInterceptor(nil, testLogger())
				Expect(err).NotTo(HaveOccurred())

				stream := &mockServerStream{ctx: context.Background()}

				handler := func(srv any, ss grpc.ServerStream) error {
					capturedIdentity = tailscale.IdentityFromContext(ss.Context())
					return nil
				}

				_ = interceptor(nil, stream, &grpc.StreamServerInfo{}, handler)
			})

			It("should have anonymous identity in context", func() {
				Expect(capturedIdentity.IsAnonymous()).To(BeTrue())
			})
		})
	})

	Describe("handler return values", func() {
		When("handler returns no error", func() {
			var err error

			BeforeEach(func() {
				var createErr error
				interceptor, createErr = tailscale.IdentityStreamInterceptor(nil, testLogger())
				Expect(createErr).NotTo(HaveOccurred())

				stream := &mockServerStream{ctx: context.Background()}

				handler := func(srv any, ss grpc.ServerStream) error {
					return nil
				}

				err = interceptor(nil, stream, &grpc.StreamServerInfo{}, handler)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})

		When("handler returns an error", func() {
			var err error

			BeforeEach(func() {
				var createErr error
				interceptor, createErr = tailscale.IdentityStreamInterceptor(nil, testLogger())
				Expect(createErr).NotTo(HaveOccurred())

				stream := &mockServerStream{ctx: context.Background()}

				handler := func(srv any, ss grpc.ServerStream) error {
					return errors.New("stream error")
				}

				err = interceptor(nil, stream, &grpc.StreamServerInfo{}, handler)
			})

			It("should return the handler error", func() {
				Expect(err).To(MatchError("stream error"))
			})
		})
	})
})
