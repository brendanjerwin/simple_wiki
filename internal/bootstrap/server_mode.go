// Package bootstrap provides server initialization and configuration.
package bootstrap

// ServerMode represents the mode of operation for the server.
type ServerMode int

const (
	// ModePlainHTTP serves only HTTP without TLS.
	ModePlainHTTP ServerMode = iota

	// ModeTailscaleServe uses Tailscale Serve for HTTPS termination.
	// The application listens on HTTP and Tailscale Serve proxies HTTPS to it.
	ModeTailscaleServe

	// ModeFullTLS runs its own TLS listener using Tailscale certificates.
	// Also runs an HTTP server that redirects tailnet clients to HTTPS.
	ModeFullTLS
)

// String returns a human-readable description of the server mode.
func (m ServerMode) String() string {
	switch m {
	case ModePlainHTTP:
		return "PlainHTTP"
	case ModeTailscaleServe:
		return "TailscaleServe"
	case ModeFullTLS:
		return "FullTLS"
	default:
		return "Unknown"
	}
}
