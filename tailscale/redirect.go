package tailscale

import (
	"net/http"
)

// RedirectHandler redirects tailnet requests to HTTPS, serves fallback for others.
// This allows non-tailnet clients (localhost, LAN) to continue using HTTP.
type RedirectHandler struct {
	TSHostname      string           // Tailscale hostname to redirect to (e.g., "my-laptop.tailnet.ts.net")
	Resolver        IResolveIdentity // Used to detect tailnet requests via WhoIs
	FallbackHandler http.Handler     // Handler for non-tailnet requests
}

// ServeHTTP implements http.Handler.
// If the request comes from a tailnet IP (WhoIs succeeds), redirect to HTTPS.
// Otherwise, serve the fallback handler (normal HTTP wiki).
func (h *RedirectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check if request is from tailnet (WhoIs succeeds for tailnet IPs)
	if h.Resolver != nil {
		identity, _ := h.Resolver.WhoIs(r.Context(), r.RemoteAddr)
		if identity != nil {
			// Tailnet request - redirect to HTTPS
			target := "https://" + h.TSHostname + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}
	}

	// Non-tailnet request - serve normal HTTP
	h.FallbackHandler.ServeHTTP(w, r)
}

// NewRedirectHandler creates a new redirect handler.
func NewRedirectHandler(tsHostname string, resolver IResolveIdentity, fallback http.Handler) *RedirectHandler {
	return &RedirectHandler{
		TSHostname:      tsHostname,
		Resolver:        resolver,
		FallbackHandler: fallback,
	}
}
