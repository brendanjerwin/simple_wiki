package tailscale

import (
	"crypto/tls"

	"github.com/tailscale/tscert"
)

// TLSConfigurer provides TLS configuration for Tailscale HTTPS.
type TLSConfigurer interface {
	GetTLSConfig() *tls.Config
}

// TailscaleTLSConfigurer creates TLS configs using Tailscale certificates.
type TailscaleTLSConfigurer struct{}

// NewTLSProvider creates a new TLS configurer.
func NewTLSProvider() *TailscaleTLSConfigurer {
	return &TailscaleTLSConfigurer{}
}

// GetTLSConfig returns a TLS config that uses Tailscale certificates.
// Uses tscert.GetCertificate for automatic cert provisioning.
func (*TailscaleTLSConfigurer) GetTLSConfig() *tls.Config {
	return &tls.Config{
		GetCertificate: tscert.GetCertificate,
		MinVersion:     tls.VersionTLS12,
	}
}

// Ensure TailscaleTLSConfigurer implements TLSConfigurer
var _ TLSConfigurer = (*TailscaleTLSConfigurer)(nil)
