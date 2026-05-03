package main

import (
	"net/http"
	"os"

	"connectrpc.com/connect"

	"github.com/brendanjerwin/simple_wiki/tailscale"
)

// wikiCLIHumanEnv, when set to "1", suppresses the x-wiki-is-agent header
// that wiki-cli injects on every outgoing request by default. wiki-cli is
// overwhelmingly invoked by automation (scheduled tasks, heartbeat, ad-hoc
// agent scripts), so the safer default is to mark all calls as agent-driven
// for attribution. Humans running wiki-cli interactively opt out via
// `WIKI_CLI_HUMAN=1 wiki-cli ...`.
const wikiCLIHumanEnv = "WIKI_CLI_HUMAN"

// agentHeaderTransport wraps an http.RoundTripper to inject the
// x-wiki-is-agent metadata header when the WIKI_CLI_HUMAN env var is unset
// or non-"1". The wrapped client must not be modified concurrently by
// other goroutines (RoundTripper.RoundTrip is required to be safe).
type agentHeaderTransport struct {
	wrapped http.RoundTripper
}

// RoundTrip injects the agent header when appropriate, then delegates.
func (t *agentHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if os.Getenv(wikiCLIHumanEnv) != "1" {
		// Clone the request to avoid mutating the caller's. RoundTrippers
		// are not allowed to modify Request in place.
		clone := req.Clone(req.Context())
		clone.Header.Set(tailscale.WikiIsAgentHeader, "true")
		return t.wrapped.RoundTrip(clone)
	}
	return t.wrapped.RoundTrip(req)
}

// newAgentAwareHTTPClient returns an *http.Client whose Transport injects
// the x-wiki-is-agent header by default. Callers that already have a
// configured *http.Client should pass it as base; nil uses
// http.DefaultTransport.
func newAgentAwareHTTPClient(base *http.Client) *http.Client {
	wrapped := http.DefaultTransport
	out := &http.Client{}
	if base != nil {
		*out = *base
		if base.Transport != nil {
			wrapped = base.Transport
		}
	}
	out.Transport = &agentHeaderTransport{wrapped: wrapped}
	return out
}

// Compile-time check that *http.Client satisfies connect.HTTPClient.
var _ connect.HTTPClient = (*http.Client)(nil)
