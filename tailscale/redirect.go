package tailscale

import (
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/jcelliott/lumber"
)

// DefaultHTTPSPort is the standard HTTPS port.
const DefaultHTTPSPort = 443

const maxValidPort = 65535

// TailnetRedirector redirects HTTP requests to HTTPS on the tailnet hostname.
// - If forceRedirectToTailnet: redirect ALL HTTP requests to tailnet HTTPS
// - Otherwise: only redirect tailnet clients (detected via WhoIs)
// - Requests already HTTPS (via X-Forwarded-Proto) are served directly
type TailnetRedirector struct {
	tsHostname             string           // Tailscale hostname to redirect to
	tlsPort                int              // Port the HTTPS server is running on
	resolver               IdentityResolver // Used to detect tailnet requests via WhoIs
	fallback               http.Handler     // Handler for non-tailnet requests
	forceRedirectToTailnet bool             // If true, redirect ALL HTTP requests
	logger                 *lumber.ConsoleLogger
}

// ServeHTTP implements http.Handler.
func (h *TailnetRedirector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if request is already HTTPS (via X-Forwarded-Proto from Tailscale Serve)
	// Only trust this header from localhost (where tailscaled runs)
	// This prevents attackers from bypassing redirect by spoofing the header
	if r.Header.Get("X-Forwarded-Proto") == "https" && isFromLocalhost(r.RemoteAddr) {
		h.fallback.ServeHTTP(w, r)
		return
	}

	// Force redirect: redirect ALL HTTP requests to tailnet HTTPS
	if h.forceRedirectToTailnet {
		target := h.buildHTTPSURL(r.URL.RequestURI())
		http.Redirect(w, r, target, http.StatusMovedPermanently)
		return
	}

	// Check if request is from tailnet client via WhoIs (direct tailnet connection over HTTP)
	if h.resolver != nil {
		identity, err := h.resolver.WhoIs(r.Context(), r.RemoteAddr)
		if err != nil && h.logger != nil {
			h.logger.Debug("WhoIs lookup failed for redirect check: %v", err)
		}
		if identity != nil {
			// Tailnet client connecting directly over HTTP - redirect to HTTPS
			target := h.buildHTTPSURL(r.URL.RequestURI())
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
	}

	// Non-tailnet client - serve HTTP fallback
	h.fallback.ServeHTTP(w, r)
}

// isFromLocalhost checks if remoteAddr originates from the loopback interface.
// SECURITY: This function is fail-closed - any parse errors return false.
// This prevents attackers from bypassing localhost checks with malformed addresses.
// Only properly formatted addr:port strings (e.g., "127.0.0.1:12345" or "[::1]:12345")
// are considered; malformed input is treated as non-localhost.
func isFromLocalhost(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// If SplitHostPort fails, remoteAddr is malformed - treat as non-localhost
		return false
	}

	ip := net.ParseIP(host)
	if ip == nil {
		// Could not parse as valid IP - treat as non-localhost
		return false
	}

	return ip.IsLoopback()
}

// buildHTTPSURL constructs the HTTPS redirect URL.
func (h *TailnetRedirector) buildHTTPSURL(requestURI string) string {
	if h.tlsPort == DefaultHTTPSPort {
		return fmt.Sprintf("https://%s%s", h.tsHostname, requestURI)
	}
	return fmt.Sprintf("https://%s:%d%s", h.tsHostname, h.tlsPort, requestURI)
}

// NewTailnetRedirector creates a new tailnet redirector.
// Returns an error if tsHostname is empty, tlsPort is invalid, or fallback is nil.
func NewTailnetRedirector(tsHostname string, tlsPort int, resolver IdentityResolver, fallback http.Handler, forceRedirect bool, logger *lumber.ConsoleLogger) (*TailnetRedirector, error) {
	if tsHostname == "" {
		return nil, errors.New("tsHostname cannot be empty")
	}
	if tlsPort < 1 || tlsPort > maxValidPort {
		return nil, fmt.Errorf("tlsPort %d is invalid: must be between 1 and 65535", tlsPort)
	}
	if fallback == nil {
		return nil, errors.New("fallback handler cannot be nil")
	}

	return &TailnetRedirector{
		tsHostname:             tsHostname,
		tlsPort:                tlsPort,
		resolver:               resolver,
		fallback:               fallback,
		forceRedirectToTailnet: forceRedirect,
		logger:                 logger,
	}, nil
}
