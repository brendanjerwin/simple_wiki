//revive:disable:dot-imports
package observability_test

import (
	"context"
	"time"

	"github.com/brendanjerwin/simple_wiki/internal/observability"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

var _ = Describe("GRPCMetrics", func() {
	var (
		reader           *sdkmetric.ManualReader
		provider         *sdkmetric.MeterProvider
		originalProvider metric.MeterProvider
	)

	BeforeEach(func() {
		reader = sdkmetric.NewManualReader()
		provider = sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		originalProvider = otel.GetMeterProvider()
		otel.SetMeterProvider(provider)
	})

	AfterEach(func() {
		otel.SetMeterProvider(originalProvider)
		Expect(provider.Shutdown(context.Background())).To(Succeed())
	})

	Describe("NewGRPCMetrics", func() {
		When("creating new metrics", func() {
			var metrics *observability.GRPCMetrics
			var err error

			BeforeEach(func() {
				metrics, err = observability.NewGRPCMetrics()
			})

			It("should create metrics successfully", func() {
				Expect(err).ToNot(HaveOccurred())
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
			var activeRequestsSum metricdata.Sum[int64]

			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "/test.Service/Method")

				var collected metricdata.ResourceMetrics
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())

				activeRequestsMetric := findMetricByName(collected, "grpc_active_requests")
				Expect(activeRequestsMetric).ToNot(BeNil(), "grpc_active_requests metric should exist")

				var ok bool
				activeRequestsSum, ok = activeRequestsMetric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "metric should be Sum[int64]")
			})

			It("should increment active requests counter to 1", func() {
				Expect(activeRequestsSum.DataPoints).To(HaveLen(1))
				Expect(activeRequestsSum.DataPoints[0].Value).To(Equal(int64(1)))
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
			var durationHistogram metricdata.Histogram[float64]
			var requestsSum metricdata.Sum[int64]
			var activeSum metricdata.Sum[int64]
			var hasErrorsMetric bool
			var errorsSum metricdata.Sum[int64]

			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "/test.Service/Method")
				metrics.RequestFinished(context.Background(), "/test.Service/Method", "OK", 100*time.Millisecond)

				var collected metricdata.ResourceMetrics
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())

				// Duration histogram
				durationMetric := findMetricByName(collected, "grpc_request_duration_seconds")
				Expect(durationMetric).ToNot(BeNil(), "grpc_request_duration_seconds should exist")
				var ok bool
				durationHistogram, ok = durationMetric.Data.(metricdata.Histogram[float64])
				Expect(ok).To(BeTrue(), "duration metric should be Histogram[float64]")

				// Requests counter
				requestsMetric := findMetricByName(collected, "grpc_requests_total")
				Expect(requestsMetric).ToNot(BeNil(), "grpc_requests_total should exist")
				requestsSum, ok = requestsMetric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "requests metric should be Sum[int64]")

				// Errors counter (may not exist if no errors recorded)
				errorsMetric := findMetricByName(collected, "grpc_errors_total")
				hasErrorsMetric = errorsMetric != nil
				if hasErrorsMetric {
					errorsSum, ok = errorsMetric.Data.(metricdata.Sum[int64])
					Expect(ok).To(BeTrue(), "errors metric should be Sum[int64]")
				}

				// Active requests
				activeMetric := findMetricByName(collected, "grpc_active_requests")
				Expect(activeMetric).ToNot(BeNil(), "grpc_active_requests should exist")
				activeSum, ok = activeMetric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "active requests metric should be Sum[int64]")
			})

			It("should record one duration sample", func() {
				Expect(durationHistogram.DataPoints).To(HaveLen(1))
				Expect(durationHistogram.DataPoints[0].Count).To(Equal(uint64(1)))
			})

			It("should increment request counter to 1", func() {
				Expect(requestsSum.DataPoints).To(HaveLen(1))
				Expect(requestsSum.DataPoints[0].Value).To(Equal(int64(1)))
			})

			It("should not record any errors for OK status", func() {
				if hasErrorsMetric {
					Expect(errorsSum.DataPoints).To(BeEmpty())
				}
			})

			It("should decrement active requests back to zero", func() {
				Expect(activeSum.DataPoints).To(HaveLen(1))
				Expect(activeSum.DataPoints[0].Value).To(Equal(int64(0)))
			})
		})

		When("recording a failed request", func() {
			var errorsSum metricdata.Sum[int64]
			var requestsSum metricdata.Sum[int64]

			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "/test.Service/Method")
				metrics.RequestFinished(context.Background(), "/test.Service/Method", "INTERNAL", 50*time.Millisecond)

				var collected metricdata.ResourceMetrics
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())

				// Errors counter
				errorsMetric := findMetricByName(collected, "grpc_errors_total")
				Expect(errorsMetric).ToNot(BeNil(), "grpc_errors_total should exist for failed request")
				var ok bool
				errorsSum, ok = errorsMetric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "errors metric should be Sum[int64]")

				// Requests counter
				requestsMetric := findMetricByName(collected, "grpc_requests_total")
				Expect(requestsMetric).ToNot(BeNil(), "grpc_requests_total should exist")
				requestsSum, ok = requestsMetric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "requests metric should be Sum[int64]")
			})

			It("should increment error counter to 1", func() {
				Expect(errorsSum.DataPoints).To(HaveLen(1))
				Expect(errorsSum.DataPoints[0].Value).To(Equal(int64(1)))
			})

			It("should still increment request counter", func() {
				Expect(requestsSum.DataPoints).To(HaveLen(1))
				Expect(requestsSum.DataPoints[0].Value).To(Equal(int64(1)))
			})
		})

		When("recording multiple requests with different statuses", func() {
			var totalCount uint64

			BeforeEach(func() {
				totalCount = 0

				// Two OK requests
				metrics.RequestFinished(context.Background(), "/test.Service/Method", "OK", 10*time.Millisecond)
				metrics.RequestFinished(context.Background(), "/test.Service/Method", "OK", 20*time.Millisecond)
				// One error
				metrics.RequestFinished(context.Background(), "/test.Service/Method", "UNAVAILABLE", 30*time.Millisecond)

				var collected metricdata.ResourceMetrics
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())

				durationMetric := findMetricByName(collected, "grpc_request_duration_seconds")
				Expect(durationMetric).ToNot(BeNil(), "grpc_request_duration_seconds should exist")

				durationHistogram, ok := durationMetric.Data.(metricdata.Histogram[float64])
				Expect(ok).To(BeTrue(), "duration metric should be Histogram[float64]")

				for _, dp := range durationHistogram.DataPoints {
					totalCount += dp.Count
				}
			})

			It("should count all 3 requests in histogram", func() {
				Expect(totalCount).To(Equal(uint64(3)))
			})
		})
	})
})
