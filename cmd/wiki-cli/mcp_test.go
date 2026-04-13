package main

import (
	"net/http"
	"time"

	mcpserver "github.com/mark3labs/mcp-go/server"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	cli "gopkg.in/urfave/cli.v1"
)

var _ = Describe("buildMCPCommand", func() {
	var cmd cli.Command

	BeforeEach(func() {
		urlFlag := cli.StringFlag{
			Name:  "url, u",
			Usage: "wiki base URL",
			Value: "http://localhost:8050",
		}
		cmd = buildMCPCommand(urlFlag)
	})

	It("should have name mcp", func() {
		Expect(cmd.Name).To(Equal("mcp"))
	})

	It("should have a non-empty usage", func() {
		Expect(cmd.Usage).NotTo(BeEmpty())
	})

	It("should include the url flag", func() {
		Expect(cmd.Flags).To(HaveLen(1))
	})

	It("should have a non-nil action", func() {
		Expect(cmd.Action).NotTo(BeNil())
	})

	When("the action is invoked with an unsupported URL scheme", func() {
		var actionErr error

		BeforeEach(func() {
			app := cli.NewApp()
			app.Commands = []cli.Command{cmd}
			actionErr = app.Run([]string{"app", "mcp", "--url", "ftp://wiki.example.com"})
		})

		It("should return an error about the unsupported scheme", func() {
			Expect(actionErr).To(MatchError(ContainSubstring("unsupported URL scheme")))
		})
	})
})

var _ = Describe("setupMCPServer", func() {
	When("given a valid http URL", func() {
		var s *mcpserver.MCPServer
		var httpClient *http.Client
		var err error

		BeforeEach(func() {
			s, httpClient, err = setupMCPServer("http://localhost:1")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a non-nil MCP server", func() {
			Expect(s).NotTo(BeNil())
		})

		It("should return a non-nil HTTP client", func() {
			Expect(httpClient).NotTo(BeNil())
		})
	})
})

var _ = Describe("normalizeBaseURL", func() {
	When("given an https URL", func() {
		var normalized string
		var err error

		BeforeEach(func() {
			normalized, err = normalizeBaseURL("https://wiki.example.com")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the URL unchanged", func() {
			Expect(normalized).To(Equal("https://wiki.example.com"))
		})
	})

	When("given an http URL", func() {
		var normalized string
		var err error

		BeforeEach(func() {
			normalized, err = normalizeBaseURL("http://wiki.example.com")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the URL unchanged", func() {
			Expect(normalized).To(Equal("http://wiki.example.com"))
		})
	})

	When("given a URL with an unsupported scheme", func() {
		var err error

		BeforeEach(func() {
			_, err = normalizeBaseURL("ftp://wiki.example.com")
		})

		It("should return an error mentioning the scheme", func() {
			Expect(err).To(MatchError(ContainSubstring(`unsupported URL scheme "ftp"`)))
		})
	})

	When("given an unparseable URL", func() {
		var err error

		BeforeEach(func() {
			// "://invalid" has no scheme before "://" which causes url.Parse to fail
			_, err = normalizeBaseURL("://invalid")
		})

		It("should return a parse error", func() {
			Expect(err).To(MatchError(ContainSubstring("invalid base URL")))
		})
	})

	When("given a URL with a trailing slash", func() {
		var normalized string
		var err error

		BeforeEach(func() {
			normalized, err = normalizeBaseURL("https://wiki.example.com/")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the URL without a trailing slash", func() {
			Expect(normalized).To(Equal("https://wiki.example.com"))
		})
	})

	When("given a URL with an empty host", func() {
		var err error

		BeforeEach(func() {
			_, err = normalizeBaseURL("https://")
		})

		It("should return an error mentioning the missing host", func() {
			Expect(err).To(MatchError(ContainSubstring("missing host")))
		})
	})
})

var _ = Describe("createAPIClients", func() {
	When("given an HTTP client and base URL", func() {
		var clients *apiClients

		BeforeEach(func() {
			httpClient := &http.Client{}
			clients = createAPIClients(httpClient, "http://localhost:8050")
		})

		It("should return non-nil clients struct", func() {
			Expect(clients).NotTo(BeNil())
		})

		It("should have a non-nil chat client", func() {
			Expect(clients.chat).NotTo(BeNil())
		})

		It("should have a non-nil frontmatter client", func() {
			Expect(clients.frontmatter).NotTo(BeNil())
		})

		It("should have a non-nil inventory client", func() {
			Expect(clients.inventory).NotTo(BeNil())
		})

		It("should have a non-nil pageImport client", func() {
			Expect(clients.pageImport).NotTo(BeNil())
		})

		It("should have a non-nil pageManagement client", func() {
			Expect(clients.pageManagement).NotTo(BeNil())
		})

		It("should have a non-nil search client", func() {
			Expect(clients.search).NotTo(BeNil())
		})

		It("should have a non-nil systemInfo client", func() {
			Expect(clients.systemInfo).NotTo(BeNil())
		})
	})
})

var _ = Describe("registerToolHandlers", func() {
	When("called with a new MCPServer and empty clients", func() {
		var toolCount int

		BeforeEach(func() {
			s := mcpserver.NewMCPServer("test", "1.0")
			registerToolHandlers(s, &apiClients{})
			toolCount = len(s.ListTools())
		})

		It("should register at least one tool", func() {
			Expect(toolCount).To(BeNumerically(">", 0))
		})
	})
})

var _ = Describe("computeBackoffAfterFailure", func() {
	When("the stream ran for a short duration (rapid failure)", func() {
		var delayMs, nextMs int

		BeforeEach(func() {
			delayMs, nextMs = computeBackoffAfterFailure(initialBackoffMs, 100*time.Millisecond)
		})

		It("should use the current backoff as the delay", func() {
			Expect(delayMs).To(Equal(initialBackoffMs))
		})

		It("should double the backoff for the next iteration", func() {
			Expect(nextMs).To(Equal(initialBackoffMs * int(backoffMultiplier)))
		})
	})

	When("there are multiple rapid consecutive failures", func() {
		var delayMs2, nextMs2 int

		BeforeEach(func() {
			// Simulate accumulation from a previous failure
			_, nextMs1 := computeBackoffAfterFailure(initialBackoffMs, 100*time.Millisecond)
			delayMs2, nextMs2 = computeBackoffAfterFailure(nextMs1, 100*time.Millisecond)
		})

		It("should keep accumulating the backoff delay", func() {
			Expect(delayMs2).To(Equal(initialBackoffMs * int(backoffMultiplier)))
		})

		It("should double again for the next iteration", func() {
			Expect(nextMs2).To(Equal(initialBackoffMs * int(backoffMultiplier) * int(backoffMultiplier)))
		})
	})

	When("the stream ran long enough to be considered healthy", func() {
		var delayMs, nextMs int

		BeforeEach(func() {
			// Start from an elevated backoff (simulates previous rapid failures)
			elevatedBackoff := 16000
			healthyDuration := time.Duration(initialBackoffMs+500) * time.Millisecond
			delayMs, nextMs = computeBackoffAfterFailure(elevatedBackoff, healthyDuration)
		})

		It("should reset the delay to initialBackoffMs", func() {
			Expect(delayMs).To(Equal(initialBackoffMs))
		})

		It("should set next backoff to initial*multiplier after the reset", func() {
			Expect(nextMs).To(Equal(initialBackoffMs * int(backoffMultiplier)))
		})
	})

	When("the backoff would exceed the maximum", func() {
		var delayMs, nextMs int

		BeforeEach(func() {
			delayMs, nextMs = computeBackoffAfterFailure(maxBackoffMs, 100*time.Millisecond)
		})

		It("should use maxBackoffMs as the delay", func() {
			Expect(delayMs).To(Equal(maxBackoffMs))
		})

		It("should cap the next backoff at maxBackoffMs", func() {
			Expect(nextMs).To(Equal(maxBackoffMs))
		})
	})
})
