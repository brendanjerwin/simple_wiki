package tailscale

import (
	"fmt"
	"net/http"
)

// RedirectHandler redirects HTTP requests to HTTPS on the tailnet hostname.
// - If ForceRedirectToTailnet: redirect ALL HTTP requests to tailnet HTTPS
// - Otherwise: only redirect tailnet clients (detected via WhoIs)
// - Requests already HTTPS (via X-Forwarded-Proto) are served directly
type RedirectHandler struct {
	TSHostname              string           // Tailscale hostname to redirect to (e.g., "my-laptop.tailnet.ts.net")
	TLSPort                 int              // Port the HTTPS server is running on
	Resolver                IResolveIdentity // Used to detect tailnet requests via WhoIs
	FallbackHandler         http.Handler     // Handler for non-tailnet requests
	ForceRedirectToTailnet  bool             // If true, redirect ALL HTTP requests to tailnet HTTPS
}

// ServeHTTP implements http.Handler.
func (h *RedirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if request is already HTTPS (via X-Forwarded-Proto from Tailscale Serve)
	// If already HTTPS, don't redirect - just serve
	if r.Header.Get("X-Forwarded-Proto") == "https" {
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
		identity, _ := h.Resolver.WhoIs(r.Context(), r.RemoteAddr)
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

// buildHTTPSURL constructs the HTTPS redirect URL.
func (h *RedirectHandler) buildHTTPSURL(requestURI string) string {
	if h.TLSPort == 443 {
		return fmt.Sprintf("https://%s%s", h.TSHostname, requestURI)
	}
	return fmt.Sprintf("https://%s:%d%s", h.TSHostname, h.TLSPort, requestURI)
}

// NewRedirectHandler creates a new redirect handler.
func NewRedirectHandler(tsHostname string, tlsPort int, resolver IResolveIdentity, fallback http.Handler, forceRedirectToTailnet bool) *RedirectHandler {
	return &RedirectHandler{
		TSHostname:             tsHostname,
		TLSPort:                tlsPort,
		Resolver:               resolver,
		FallbackHandler:        fallback,
		ForceRedirectToTailnet: forceRedirectToTailnet,
	}
}
