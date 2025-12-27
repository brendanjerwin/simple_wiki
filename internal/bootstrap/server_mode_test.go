package bootstrap_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/bootstrap"
	"github.com/brendanjerwin/simple_wiki/tailscale"
)

var _ = Describe("ServerMode", func() {
	Describe("String", func() {
		When("mode is ModePlainHTTP", func() {
			var result string

			BeforeEach(func() {
				result = bootstrap.ModePlainHTTP.String()
			})

			It("should return PlainHTTP", func() {
				Expect(result).To(Equal("PlainHTTP"))
			})
		})

		When("mode is ModeTailscaleServe", func() {
			var result string

			BeforeEach(func() {
				result = bootstrap.ModeTailscaleServe.String()
			})

			It("should return TailscaleServe", func() {
				Expect(result).To(Equal("TailscaleServe"))
			})
		})

		When("mode is ModeFullTLS", func() {
			var result string

			BeforeEach(func() {
				result = bootstrap.ModeFullTLS.String()
			})

			It("should return FullTLS", func() {
				Expect(result).To(Equal("FullTLS"))
			})
		})

		When("mode is unknown", func() {
			var result string

			BeforeEach(func() {
				result = bootstrap.ServerMode(99).String()
			})

			It("should return Unknown", func() {
				Expect(result).To(Equal("Unknown"))
			})
		})
	})

	Describe("DetermineServerMode", func() {
		When("tsStatus is nil", func() {
			var mode bootstrap.ServerMode

			BeforeEach(func() {
				mode = bootstrap.DetermineServerMode(nil, false)
			})

			It("should return ModePlainHTTP", func() {
				Expect(mode).To(Equal(bootstrap.ModePlainHTTP))
			})
		})

		When("Tailscale is not available", func() {
			var mode bootstrap.ServerMode

			BeforeEach(func() {
				tsStatus := &tailscale.Status{Available: false, DNSName: ""}
				mode = bootstrap.DetermineServerMode(tsStatus, false)
			})

			It("should return ModePlainHTTP", func() {
				Expect(mode).To(Equal(bootstrap.ModePlainHTTP))
			})
		})

		When("Tailscale is available but DNS name is empty", func() {
			var mode bootstrap.ServerMode

			BeforeEach(func() {
				tsStatus := &tailscale.Status{Available: true, DNSName: ""}
				mode = bootstrap.DetermineServerMode(tsStatus, false)
			})

			It("should return ModePlainHTTP", func() {
				Expect(mode).To(Equal(bootstrap.ModePlainHTTP))
			})
		})

		When("Tailscale is available with DNS name and Tailscale Serve is enabled", func() {
			var mode bootstrap.ServerMode

			BeforeEach(func() {
				tsStatus := &tailscale.Status{Available: true, DNSName: "my-laptop.tailnet.ts.net"}
				mode = bootstrap.DetermineServerMode(tsStatus, true)
			})

			It("should return ModeTailscaleServe", func() {
				Expect(mode).To(Equal(bootstrap.ModeTailscaleServe))
			})
		})

		When("Tailscale is available with DNS name and Tailscale Serve is disabled", func() {
			var mode bootstrap.ServerMode

			BeforeEach(func() {
				tsStatus := &tailscale.Status{Available: true, DNSName: "my-laptop.tailnet.ts.net"}
				mode = bootstrap.DetermineServerMode(tsStatus, false)
			})

			It("should return ModeFullTLS", func() {
				Expect(mode).To(Equal(bootstrap.ModeFullTLS))
			})
		})
	})
})
