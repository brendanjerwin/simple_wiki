//revive:disable:dot-imports
package main

import (
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// stubMCPServer answers the two /mcp calls FetchSurface makes: the JSON-RPC
// initialize handshake and the tools/list request.
func stubMCPServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, 4096)
		n, _ := r.Body.Read(body)
		payload := string(body[:n])
		switch {
		case strings.Contains(payload, "initialize"):
			w.Header().Set("Mcp-Session-Id", "test-session")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
		case strings.Contains(payload, "tools/list"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"tool_a","description":"Does A"}]}}`))
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
}

var _ = Describe("main", func() {
	When("run with --dry-run against a live tool surface", func() {
		It("prints the run config and returns without calling the LLM", func() {
			srv := stubMCPServer()
			defer srv.Close()

			// main() registers its flags on flag.CommandLine and calls
			// flag.Parse. Swap in a fresh FlagSet and controlled os.Args so
			// the parse succeeds with --dry-run and never os.Exits, then
			// restore both so the test harness keeps its own state.
			oldArgs := os.Args
			oldFlags := flag.CommandLine
			flag.CommandLine = flag.NewFlagSet("eval-main-test", flag.ContinueOnError)
			os.Args = []string{"eval", "-dry-run", "-wiki-url", srv.URL}
			defer func() {
				os.Args = oldArgs
				flag.CommandLine = oldFlags
			}()

			Expect(func() { main() }).NotTo(Panic())
		})
	})
})
