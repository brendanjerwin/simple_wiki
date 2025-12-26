package tailscale

import (
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/jcelliott/lumber"
)

const (
	defaultHTTPSPort = 443
	maxValidPort     = 65535
)

// TailnetRedirector redirects HTTP requests to HTTPS on the tailnet hostname.
// - If ForceRedirectToTailnet: redirect ALL HTTP requests to tailnet HTTPS
// - Otherwise: only redirect tailnet clients (detected via WhoIs)
// - Requests already HTTPS (via X-Forwarded-Proto) are served directly
type TailnetRedirector struct {
	TSHostname             string           // Tailscale hostname to redirect to (e.g., "my-laptop.tailnet.ts.net")
	TLSPort                int              // Port the HTTPS server is running on
	Resolver               IResolveIdentity // Used to detect tailnet requests via WhoIs
	FallbackHandler        http.Handler     // Handler for non-tailnet requests
	ForceRedirectToTailnet bool             // If true, redirect ALL HTTP requests to tailnet HTTPS
	Logger                 *lumber.ConsoleLogger
}

// ServeHTTP implements http.Handler.
func (h *TailnetRedirector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if request is already HTTPS (via X-Forwarded-Proto from Tailscale Serve)
	// Only trust this header from localhost (where tailscaled runs)
	// This prevents attackers from bypassing redirect by spoofing the header
	if r.Header.Get("X-Forwarded-Proto") == "https" && isFromLocalhost(r.RemoteAddr) {
		h.FallbackHandler.ServeHTTP(w, r)
		return
	}

	// Force redirect: redirect ALL HTTP requests to tailnet HTTPS
	if h.ForceRedirectToTailnet {
		target := h.buildHTTPSURL(r.URL.RequestURI())
		http.Redirect(w, r, target, http.StatusMovedPermanently)
		return
	}

	// Check if request is from tailnet client via WhoIs (direct tailnet connection over HTTP)
	if h.Resolver != nil {
		identity, err := h.Resolver.WhoIs(r.Context(), r.RemoteAddr)
		if err != nil && h.Logger != nil {
			h.Logger.Debug("WhoIs lookup failed for redirect check: %v", err)
		}
		if identity != nil {
			// Tailnet client connecting directly over HTTP - redirect to HTTPS
			target := h.buildHTTPSURL(r.URL.RequestURI())
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
	}

	// Non-tailnet client - serve HTTP fallback
	h.FallbackHandler.ServeHTTP(w, r)
}

// isFromLocalhost checks if the remote address is from localhost.
// This is used to validate that X-Forwarded-Proto header came from a trusted proxy
// (like tailscaled) rather than being spoofed by an external client.
func isFromLocalhost(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		// Try parsing as just an IP (no port)
		host = remoteAddr
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	return ip.IsLoopback()
}

// buildHTTPSURL constructs the HTTPS redirect URL.
func (h *TailnetRedirector) buildHTTPSURL(requestURI string) string {
	if h.TLSPort == defaultHTTPSPort {
		return fmt.Sprintf("https://%s%s", h.TSHostname, requestURI)
	}
	return fmt.Sprintf("https://%s:%d%s", h.TSHostname, h.TLSPort, requestURI)
}

// ErrEmptyHostname indicates the hostname was not provided.
var ErrEmptyHostname = errors.New("tsHostname cannot be empty")

// ErrInvalidPort indicates the port is outside valid range.
var ErrInvalidPort = errors.New("tlsPort must be between 1 and 65535")

// ErrNilFallback indicates the fallback handler was not provided.
var ErrNilFallback = errors.New("fallback handler cannot be nil")

// NewTailnetRedirector creates a new tailnet redirector.
// Returns an error if tsHostname is empty, tlsPort is invalid, or fallback is nil.
func NewTailnetRedirector(tsHostname string, tlsPort int, resolver IResolveIdentity, fallback http.Handler, forceRedirectToTailnet bool, logger *lumber.ConsoleLogger) (*TailnetRedirector, error) {
	if tsHostname == "" {
		return nil, ErrEmptyHostname
	}
	if tlsPort <= 0 || tlsPort > maxValidPort {
		return nil, ErrInvalidPort
	}
	if fallback == nil {
		return nil, ErrNilFallback
	}

	return &TailnetRedirector{
		TSHostname:             tsHostname,
		TLSPort:                tlsPort,
		Resolver:               resolver,
		FallbackHandler:        fallback,
		ForceRedirectToTailnet: forceRedirectToTailnet,
		Logger:                 logger,
	}, nil
}
