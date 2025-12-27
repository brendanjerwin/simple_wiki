//revive:disable:dot-imports
package observability_test

import (
	"context"
	"time"

	"github.com/brendanjerwin/simple_wiki/internal/observability"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GRPCMetrics", func() {
	Describe("NewGRPCMetrics", func() {
		When("creating new metrics", func() {
			var metrics *observability.GRPCMetrics
			var err error

			BeforeEach(func() {
				metrics, err = observability.NewGRPCMetrics()
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
		var metrics *observability.GRPCMetrics

		BeforeEach(func() {
			var err error
			metrics, err = observability.NewGRPCMetrics()
			Expect(err).ToNot(HaveOccurred())
		})

		When("recording request start", func() {
			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "/test.Service/Method")
			})

			It("should not panic", func() {
				// The test passes if no panic occurs
			})
		})
	})

	Describe("RequestFinished", func() {
		var metrics *observability.GRPCMetrics

		BeforeEach(func() {
			var err error
			metrics, err = observability.NewGRPCMetrics()
			Expect(err).ToNot(HaveOccurred())
		})

		When("recording a successful request", func() {
			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "/test.Service/Method")
				metrics.RequestFinished(context.Background(), "/test.Service/Method", "OK", 100*time.Millisecond)
			})

			It("should not panic", func() {
				// The test passes if no panic occurs
			})
		})

		When("recording a failed request", func() {
			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "/test.Service/Method")
				metrics.RequestFinished(context.Background(), "/test.Service/Method", "INTERNAL", 50*time.Millisecond)
			})

			It("should not panic", func() {
				// The test passes if no panic occurs
			})
		})

		When("recording various gRPC status codes", func() {
			var statusCodes []string

			BeforeEach(func() {
				statusCodes = []string{"OK", "CANCELLED", "UNKNOWN", "INVALID_ARGUMENT", "DEADLINE_EXCEEDED", "NOT_FOUND", "ALREADY_EXISTS", "PERMISSION_DENIED", "RESOURCE_EXHAUSTED", "FAILED_PRECONDITION", "ABORTED", "OUT_OF_RANGE", "UNIMPLEMENTED", "INTERNAL", "UNAVAILABLE", "DATA_LOSS", "UNAUTHENTICATED"}
				for _, code := range statusCodes {
					metrics.RequestFinished(context.Background(), "/test.Service/Method", code, 10*time.Millisecond)
				}
			})

			It("should handle all status codes without panic", func() {
				Expect(statusCodes).To(HaveLen(17))
			})
		})
	})
})
