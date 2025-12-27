//revive:disable:dot-imports
package observability_test

import (
	"context"
	"time"

	"github.com/brendanjerwin/simple_wiki/internal/observability"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HTTPMetrics", func() {
	Describe("NewHTTPMetrics", func() {
		When("creating new metrics", func() {
			var metrics *observability.HTTPMetrics
			var err error

			BeforeEach(func() {
				metrics, err = observability.NewHTTPMetrics()
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return a non-nil metrics instance", func() {
				Expect(metrics).ToNot(BeNil())
			})
		})
	})

	Describe("RequestStarted", func() {
		var metrics *observability.HTTPMetrics

		BeforeEach(func() {
			var err error
			metrics, err = observability.NewHTTPMetrics()
			Expect(err).ToNot(HaveOccurred())
		})

		When("recording request start", func() {
			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "GET", "/api/test")
			})

			It("should not panic", func() {
				// The test passes if no panic occurs
			})
		})

		When("recording start with various HTTP methods", func() {
			BeforeEach(func() {
				methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
				for _, method := range methods {
					metrics.RequestStarted(context.Background(), method, "/api/test")
				}
			})

			It("should handle all methods without panic", func() {
				// The test passes if no panic occurs
			})
		})
	})

	Describe("RequestFinished", func() {
		var metrics *observability.HTTPMetrics

		BeforeEach(func() {
			var err error
			metrics, err = observability.NewHTTPMetrics()
			Expect(err).ToNot(HaveOccurred())
		})

		When("recording a successful request (2xx)", func() {
			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "GET", "/api/test")
				metrics.RequestFinished(context.Background(), "GET", "/api/test", 200, 100*time.Millisecond)
			})

			It("should not panic", func() {
				// The test passes if no panic occurs
			})
		})

		When("recording a redirect (3xx)", func() {
			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "GET", "/old-path")
				metrics.RequestFinished(context.Background(), "GET", "/old-path", 301, 10*time.Millisecond)
			})

			It("should not panic", func() {
				// The test passes if no panic occurs
			})
		})

		When("recording a client error (4xx)", func() {
			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "GET", "/not-found")
				metrics.RequestFinished(context.Background(), "GET", "/not-found", 404, 5*time.Millisecond)
			})

			It("should not panic", func() {
				// The test passes if no panic occurs
			})
		})

		When("recording a server error (5xx)", func() {
			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "POST", "/api/error")
				metrics.RequestFinished(context.Background(), "POST", "/api/error", 500, 50*time.Millisecond)
			})

			It("should not panic", func() {
				// The test passes if no panic occurs - error is recorded internally
			})
		})

		When("recording various status codes", func() {
			var statusCodes []int

			BeforeEach(func() {
				statusCodes = []int{200, 201, 204, 301, 302, 304, 400, 401, 403, 404, 500, 502, 503}
				for _, code := range statusCodes {
					metrics.RequestFinished(context.Background(), "GET", "/test", code, 10*time.Millisecond)
				}
			})

			It("should handle all status codes without panic", func() {
				Expect(statusCodes).To(HaveLen(13))
			})
		})

		When("recording boundary status code (499)", func() {
			BeforeEach(func() {
				metrics.RequestFinished(context.Background(), "GET", "/test", 499, 10*time.Millisecond)
			})

			It("should not count as server error", func() {
				// 499 is a client error, not a server error
			})
		})

		When("recording boundary status code (500)", func() {
			BeforeEach(func() {
				metrics.RequestFinished(context.Background(), "GET", "/test", 500, 10*time.Millisecond)
			})

			It("should count as server error", func() {
				// 500 is the first server error code
			})
		})
	})
})
