//revive:disable:dot-imports
package bootstrap

import (
	"context"

	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var _ = Describe("buildGRPCInterceptors", func() {
	When("test identity metadata is trusted without a Tailscale resolver", func() {
		var (
			capturedIdentity tailscale.IdentityValue
			err              error
		)

		BeforeEach(func() {
			loggingInterceptor := func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
				return handler(ctx, req)
			}
			unaryInterceptors, _, buildErr := buildGRPCInterceptors(
				nil, identityMetadataOptions{TrustMetadata: true}, loggingInterceptor, nil, lumber.NewConsoleLogger(lumber.WARN),
			)
			Expect(buildErr).NotTo(HaveOccurred())
			Expect(unaryInterceptors).To(HaveLen(3))

			ctx := metadata.NewIncomingContext(context.Background(), metadata.New(map[string]string{
				"tailscale-user-login": "e2e@example.com",
				"tailscale-user-name":  "E2E User",
			}))
			_, err = unaryInterceptors[1](ctx, nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, _ any) (any, error) {
				capturedIdentity = tailscale.IdentityFromContext(ctx)
				return nil, nil
			})
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should extract identity from metadata", func() {
			Expect(capturedIdentity.IsAnonymous()).To(BeFalse())
			Expect(capturedIdentity.LoginName()).To(Equal("e2e@example.com"))
		})
	})

	When("test identity metadata is not trusted and there is no Tailscale resolver", func() {
		var unaryInterceptors []grpc.UnaryServerInterceptor

		BeforeEach(func() {
			loggingInterceptor := func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
				return handler(ctx, req)
			}
			var err error
			unaryInterceptors, _, err = buildGRPCInterceptors(
				nil, identityMetadataOptions{}, loggingInterceptor, nil, lumber.NewConsoleLogger(lumber.WARN),
			)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should only install instrumentation and logging", func() {
			Expect(unaryInterceptors).To(HaveLen(2))
		})
	})
})
