//revive:disable:dot-imports
package bootstrap

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
)

var _ = Describe("buildGRPCServerOptions", func() {
	When("maxUploadSizeMB is zero", func() {
		var opts []grpc.ServerOption

		BeforeEach(func() {
			opts = buildGRPCServerOptions(0, nil, nil)
		})

		It("should return two options (unary and stream interceptors only)", func() {
			Expect(opts).To(HaveLen(2))
		})
	})

	When("maxUploadSizeMB is greater than zero", func() {
		var opts []grpc.ServerOption

		BeforeEach(func() {
			opts = buildGRPCServerOptions(10, nil, nil)
		})

		It("should return three options (interceptors plus MaxRecvMsgSize)", func() {
			Expect(opts).To(HaveLen(3))
		})
	})
})
