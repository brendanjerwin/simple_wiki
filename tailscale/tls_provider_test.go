package tailscale_test

import (
	"crypto/tls"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/tailscale"
)

var _ = Describe("TLSProvider", func() {
	Describe("NewTLSProvider", func() {
		When("creating a new TLS provider", func() {
			var provider *tailscale.TLSProvider

			BeforeEach(func() {
				provider = tailscale.NewTLSProvider()
			})

			It("should not be nil", func() {
				Expect(provider).NotTo(BeNil())
			})
		})
	})

	Describe("GetTLSConfig", func() {
		When("getting TLS config", func() {
			var (
				provider  *tailscale.TLSProvider
				tlsConfig *tls.Config
			)

			BeforeEach(func() {
				provider = tailscale.NewTLSProvider()
				tlsConfig = provider.GetTLSConfig()
			})

			It("should not return nil", func() {
				Expect(tlsConfig).NotTo(BeNil())
			})

			It("should have GetCertificate configured", func() {
				Expect(tlsConfig.GetCertificate).NotTo(BeNil())
			})

			It("should require TLS 1.2 or higher", func() {
				Expect(tlsConfig.MinVersion).To(Equal(uint16(tls.VersionTLS12)))
			})
		})
	})

	Describe("interface compliance", func() {
		It("should implement IProvideTLS", func() {
			var _ tailscale.IProvideTLS = (*tailscale.TLSProvider)(nil)
		})
	})
})
