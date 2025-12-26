package tailscale

import (
	"crypto/tls"

	"github.com/tailscale/tscert"
)

// IProvideTLS provides TLS configuration for Tailscale HTTPS.
type IProvideTLS interface {
	GetTLSConfig() *tls.Config
}

// TLSProvider creates TLS configs using Tailscale certificates.
type TLSProvider struct{}

// NewTLSProvider creates a new TLS provider.
func NewTLSProvider() *TLSProvider {
	return &TLSProvider{}
}

// GetTLSConfig returns a TLS config that uses Tailscale certificates.
// Uses tscert.GetCertificate for automatic cert provisioning.
func (*TLSProvider) GetTLSConfig() *tls.Config {
	return &tls.Config{
		GetCertificate: tscert.GetCertificate,
		MinVersion:     tls.VersionTLS12,
	}
}

// Ensure TLSProvider implements IProvideTLS
var _ IProvideTLS = (*TLSProvider)(nil)
