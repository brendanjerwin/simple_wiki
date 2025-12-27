package bootstrap_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/bootstrap"
)

var _ = Describe("Server", func() {
	Describe("ServerResult", func() {
		When("creating server result", func() {
			var result bootstrap.ServerResult

			BeforeEach(func() {
				result = bootstrap.ServerResult{
					MainServer:     nil,
					MainListener:   nil,
					RedirectServer: nil,
				}
			})

			It("should allow nil main server", func() {
				Expect(result.MainServer).To(BeNil())
			})

			It("should allow nil main listener", func() {
				Expect(result.MainListener).To(BeNil())
			})

			It("should allow nil redirect server", func() {
				Expect(result.RedirectServer).To(BeNil())
			})
		})
	})
})
