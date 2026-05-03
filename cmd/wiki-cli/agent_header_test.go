//revive:disable:dot-imports
package main

import (
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/tailscale"
)

var _ = Describe("agent header transport", func() {
	var (
		captured http.Header
		server   *httptest.Server
	)

	BeforeEach(func() {
		captured = nil
		server = httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			captured = r.Header.Clone()
		}))
	})

	AfterEach(func() {
		server.Close()
	})

	When("WIKI_CLI_HUMAN is unset", func() {
		BeforeEach(func() {
			GinkgoT().Setenv("WIKI_CLI_HUMAN", "")
			client := newAgentAwareHTTPClient(nil)
			req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
			_, err := client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should send the x-wiki-is-agent: true header", func() {
			Expect(captured.Get(tailscale.WikiIsAgentHeader)).To(Equal("true"))
		})
	})

	When("WIKI_CLI_HUMAN=1 is set", func() {
		BeforeEach(func() {
			GinkgoT().Setenv("WIKI_CLI_HUMAN", "1")
			client := newAgentAwareHTTPClient(nil)
			req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
			_, err := client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not send the x-wiki-is-agent header", func() {
			Expect(captured.Get(tailscale.WikiIsAgentHeader)).To(BeEmpty())
		})
	})
})
