package main

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
)

var _ = Describe("parseGRPCHost", func() {
	When("given an https URL without an explicit port", func() {
		var host, scheme string
		var err error

		BeforeEach(func() {
			host, scheme, err = parseGRPCHost("https://wiki.example.com")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should append the default HTTPS port", func() {
			Expect(host).To(Equal("wiki.example.com:443"))
		})

		It("should return scheme https", func() {
			Expect(scheme).To(Equal("https"))
		})
	})

	When("given an https URL with an explicit port", func() {
		var host, scheme string
		var err error

		BeforeEach(func() {
			host, scheme, err = parseGRPCHost("https://wiki.example.com:8443")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should preserve the explicit port", func() {
			Expect(host).To(Equal("wiki.example.com:8443"))
		})

		It("should return scheme https", func() {
			Expect(scheme).To(Equal("https"))
		})
	})

	When("given an http URL without an explicit port", func() {
		var host, scheme string
		var err error

		BeforeEach(func() {
			host, scheme, err = parseGRPCHost("http://wiki.example.com")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should append the default HTTP port", func() {
			Expect(host).To(Equal("wiki.example.com:80"))
		})

		It("should return scheme http", func() {
			Expect(scheme).To(Equal("http"))
		})
	})

	When("given an http URL with an explicit port", func() {
		var host, scheme string
		var err error

		BeforeEach(func() {
			host, scheme, err = parseGRPCHost("http://wiki.example.com:8080")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should preserve the explicit port", func() {
			Expect(host).To(Equal("wiki.example.com:8080"))
		})

		It("should return scheme http", func() {
			Expect(scheme).To(Equal("http"))
		})
	})

	When("given a URL with an unsupported scheme", func() {
		var err error

		BeforeEach(func() {
			_, _, err = parseGRPCHost("ftp://wiki.example.com")
		})

		It("should return an error mentioning the scheme", func() {
			Expect(err).To(MatchError(ContainSubstring(`unsupported URL scheme "ftp"`)))
		})
	})

	When("given an empty scheme", func() {
		var err error

		BeforeEach(func() {
			_, _, err = parseGRPCHost("wiki.example.com")
		})

		It("should return an unsupported scheme error", func() {
			Expect(err).To(MatchError(ContainSubstring("unsupported URL scheme")))
		})
	})
})

var _ = Describe("createGRPCConn", func() {
	When("given a valid https URL", func() {
		var err error
		var conn *grpc.ClientConn

		BeforeEach(func() {
			conn, err = createGRPCConn("https://wiki.example.com")
		})

		AfterEach(func() {
			if conn != nil {
				_ = conn.Close()
			}
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a non-nil connection", func() {
			Expect(conn).NotTo(BeNil())
		})
	})

	When("given a valid http URL", func() {
		var err error
		var conn *grpc.ClientConn

		BeforeEach(func() {
			conn, err = createGRPCConn("http://wiki.example.com")
		})

		AfterEach(func() {
			if conn != nil {
				_ = conn.Close()
			}
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a non-nil connection", func() {
			Expect(conn).NotTo(BeNil())
		})
	})

	When("given an unsupported scheme", func() {
		var err error

		BeforeEach(func() {
			_, err = createGRPCConn("ftp://wiki.example.com")
		})

		It("should return an error", func() {
			Expect(err).To(MatchError(ContainSubstring(`unsupported URL scheme "ftp"`)))
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
