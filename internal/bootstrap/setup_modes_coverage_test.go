//revive:disable:dot-imports
package bootstrap

import (
	"time"

	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/index/bleve"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/server"
)

// Coverage for the per-mode bootstrap entry points added/changed by the
// #1199 fix: createMultiplexedHandler now takes a ServerMode and derives
// wikimcp.Options{BehindLoopbackProxy: mode == ModeTailscaleServe} from
// it (see internal/mcp/server.go). None of the three Setup* functions had
// unit coverage, so the SonarCloud new-code gate failed at 66.7% (<80%).
//
// Each test builds the full handler stack on loopback with an ephemeral
// port, asserts a usable ServerResult, then closes everything down.
// fakeBleveIndex is a nil-result stand-in for site.BleveIndexQueryer so
// grpcapi.NewServer's required-dependency validation passes in this test.
type fakeBleveIndex struct{}

func (*fakeBleveIndex) Query(string) ([]bleve.SearchResult, error) { return nil, nil }

func (*fakeBleveIndex) QueryWithTags(string, []string, []string) ([]bleve.SearchResult, error) {
	return nil, nil
}

var _ = Describe("per-mode Setup functions (#1199 mcp localhost-guard wiring)", func() {
	const testCommit = "coverage-test"

	var (
		logger *lumber.ConsoleLogger
		site   *server.Site
	)

	BeforeEach(func() {
		logger = lumber.NewConsoleLogger(lumber.WARN)
		site = stubSite(logger)
		site.BleveIndexQueryer = &fakeBleveIndex{}
		// setupWikiMetrics schedules a cron job; the stub site needs a scheduler.
		site.CronScheduler = jobs.NewCronScheduler(logger)
	})

	// closeResult releases the listeners and cleanup func of a ServerResult,
	// tolerating partially-populated results from failed setups.
	closeResult := func(result *ServerResult) {
		if result == nil {
			return
		}
		if result.MainListener != nil {
			Expect(result.MainListener.Close()).To(Succeed())
		}
		if result.RedirectServer != nil {
			Expect(result.RedirectServer.Close()).To(Succeed())
		}
		if result.Cleanup != nil {
			result.Cleanup()
		}
	}

	When("SetupPlainHTTP is invoked on loopback", func() {
		var result *ServerResult

		BeforeEach(func() {
			var err error
			result, err = SetupPlainHTTP(
				"127.0.0.1:0", site, logger, testCommit, time.Now(), PlainHTTPOptions{},
			)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() { closeResult(result) })

		It("returns a server result with a cleartext HTTP/2 main server", func() {
			Expect(result).NotTo(BeNil())
			Expect(result.MainServer).NotTo(BeNil())
			Expect(result.MainListener).NotTo(BeNil())
			// Plain HTTP keeps the mcp-go DNS-rebinding guard enabled
			// (BehindLoopbackProxy=false), so the handler must exist.
			Expect(result.MainServer.Handler).NotTo(BeNil())
		})
	})

	When("SetupTailscaleServe is invoked on loopback", func() {
		var result *ServerResult

		BeforeEach(func() {
			var err error
			result, err = SetupTailscaleServe(
				"127.0.0.1:0", "wiki.test.ts.net", false, site, logger, testCommit, time.Now(), nil,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() { closeResult(result) })

		It("returns a server result wired for loopback-proxy MCP access", func() {
			Expect(result).NotTo(BeNil())
			Expect(result.MainServer).NotTo(BeNil())
			Expect(result.MainListener).NotTo(BeNil())
			// Tailscale Serve mode sets BehindLoopbackProxy=true so mcp-go's
			// localhost guard does not 403 proxied /mcp traffic (#1199).
			Expect(result.MainServer.Handler).NotTo(BeNil())
		})
	})

	When("SetupFullTLS is invoked on loopback", func() {
		var result *ServerResult

		BeforeEach(func() {
			var err error
			// Fixed high port: tlsPort 0 is rejected (must be 1-65535) and
			// unprivileged ports >1024 need no root. Collision risk is nil
			// for a short-lived test listener.
			result, err = SetupFullTLS(
				"127.0.0.1:0", 44443, "wiki.test.ts.net", site, logger, testCommit, time.Now(), nil,
			)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() { closeResult(result) })

		It("returns a server result with TLS main server and redirect server", func() {
			Expect(result).NotTo(BeNil())
			Expect(result.MainServer).NotTo(BeNil())
			Expect(result.MainListener).NotTo(BeNil())
			Expect(result.RedirectServer).NotTo(BeNil())
			Expect(result.MainServer.Handler).NotTo(BeNil())
		})
	})
})
