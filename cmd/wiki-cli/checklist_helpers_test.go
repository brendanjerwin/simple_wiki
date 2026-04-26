//revive:disable:dot-imports
package main

import (
	"bytes"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	cli "gopkg.in/urfave/cli.v1"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/tailscale"
)

// newTestContext creates a minimal *cli.Context with the given positional args.
func newTestContext(args ...string) *cli.Context {
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	_ = set.Parse(args)
	return cli.NewContext(cli.NewApp(), set, nil)
}

var _ = Describe("requireArgs", func() {
	When("sufficient args are provided", func() {
		var (
			page, listName string
			err            error
		)

		BeforeEach(func() {
			ctx := newTestContext("my-page", "my-list")
			page, listName, err = requireArgs(ctx, 2, "list", "<page> <list>")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the page arg", func() {
			Expect(page).To(Equal("my-page"))
		})

		It("should return the list arg", func() {
			Expect(listName).To(Equal("my-list"))
		})
	})

	When("insufficient args are provided", func() {
		var err error

		BeforeEach(func() {
			ctx := newTestContext("only-one")
			_, _, err = requireArgs(ctx, 2, "list", "<page> <list>")
		})

		It("should return a usage error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("checklist list"))
		})
	})
})

var _ = Describe("requireThreeArgs", func() {
	When("three args are provided", func() {
		var (
			result pageListUIDArgs
			err    error
		)

		BeforeEach(func() {
			ctx := newTestContext("my-page", "my-list", "uid-123")
			result, err = requireThreeArgs(ctx, "toggle", "<page> <list> <uid>")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the correct page", func() {
			Expect(result.page).To(Equal("my-page"))
		})

		It("should return the correct list name", func() {
			Expect(result.listName).To(Equal("my-list"))
		})

		It("should return the correct uid", func() {
			Expect(result.uid).To(Equal("uid-123"))
		})
	})

	When("fewer than three args are provided", func() {
		var err error

		BeforeEach(func() {
			ctx := newTestContext("only-one", "only-two")
			_, err = requireThreeArgs(ctx, "delete", "<page> <list> <uid>")
		})

		It("should return a usage error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("checklist delete"))
		})
	})
})

var _ = Describe("printJSON", func() {
	var (
		savedStdout *os.File
		pipeReader  *os.File
		pipeWriter  *os.File
	)

	BeforeEach(func() {
		var pipeErr error
		pipeReader, pipeWriter, pipeErr = os.Pipe()
		Expect(pipeErr).NotTo(HaveOccurred())
		savedStdout = os.Stdout
		os.Stdout = pipeWriter
	})

	AfterEach(func() {
		os.Stdout = savedStdout
		_ = pipeWriter.Close()
		_ = pipeReader.Close()
	})

	captureAndClose := func() string {
		_ = pipeWriter.Close()
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, pipeReader)
		return buf.String()
	}

	When("given a proto message", func() {
		var (
			returnErr error
			output    string
		)

		BeforeEach(func() {
			msg := &apiv1.ChecklistItem{
				Uid:  "01HXAAAAAAAAAAAAAAAAAAAAAA",
				Text: "buy milk",
			}
			returnErr = printJSON(msg)
			output = captureAndClose()
		})

		It("should not error", func() {
			Expect(returnErr).NotTo(HaveOccurred())
		})

		It("should output JSON containing the uid", func() {
			Expect(output).To(ContainSubstring("01HXAAAAAAAAAAAAAAAAAAAAAA"))
		})

		It("should output JSON containing the text", func() {
			Expect(output).To(ContainSubstring("buy milk"))
		})
	})
})

var _ = Describe("newAgentAwareHTTPClient with non-nil base", func() {
	var (
		captured    http.Header
		server      *httptest.Server
		customCalls int
	)

	BeforeEach(func() {
		captured = nil
		customCalls = 0
		server = httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			captured = r.Header.Clone()
		}))
	})

	AfterEach(func() {
		server.Close()
	})

	When("base is a non-nil *http.Client with nil Transport", func() {
		BeforeEach(func() {
			GinkgoT().Setenv("WIKI_CLI_HUMAN", "")
			base := &http.Client{} // Transport is nil → should fall back to DefaultTransport
			client := newAgentAwareHTTPClient(base)
			req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
			_, err := client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should inject the agent header", func() {
			Expect(captured.Get(tailscale.WikiIsAgentHeader)).To(Equal("true"))
		})
	})

	When("base is a non-nil *http.Client with a custom Transport", func() {
		BeforeEach(func() {
			GinkgoT().Setenv("WIKI_CLI_HUMAN", "")
			// Custom transport that counts calls
			custom := &countingTransport{inner: http.DefaultTransport, calls: &customCalls}
			base := &http.Client{Transport: custom}
			client := newAgentAwareHTTPClient(base)
			req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
			_, err := client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should inject the agent header", func() {
			Expect(captured.Get(tailscale.WikiIsAgentHeader)).To(Equal("true"))
		})

		It("should route through the custom transport", func() {
			Expect(customCalls).To(Equal(1))
		})
	})
})

// countingTransport counts RoundTrip calls and delegates to inner.
type countingTransport struct {
	inner http.RoundTripper
	calls *int
}

func (t *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	*t.calls++
	return t.inner.RoundTrip(req)
}
