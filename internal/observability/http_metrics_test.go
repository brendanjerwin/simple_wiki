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

var _ = Describe("HTTPMetrics", func() {
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

	Describe("NewHTTPMetrics", func() {
		When("creating new metrics", func() {
			var metrics *observability.HTTPMetrics
			var err error

			BeforeEach(func() {
				metrics, err = observability.NewHTTPMetrics()
			})

			It("should create metrics successfully", func() {
				Expect(err).ToNot(HaveOccurred())
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
			var activeSum metricdata.Sum[int64]

			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "GET", "/api/test")

				var collected metricdata.ResourceMetrics
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())

				activeMetric := findMetricByName(collected, "http_active_requests")
				Expect(activeMetric).ToNot(BeNil(), "http_active_requests metric should exist")

				var ok bool
				activeSum, ok = activeMetric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "metric should be Sum[int64]")
			})

			It("should increment active requests counter to 1", func() {
				Expect(activeSum.DataPoints).To(HaveLen(1))
				Expect(activeSum.DataPoints[0].Value).To(Equal(int64(1)))
			})
		})

		When("recording start with various HTTP methods", func() {
			var total int64

			BeforeEach(func() {
				total = 0
				methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
				for _, method := range methods {
					metrics.RequestStarted(context.Background(), method, "/api/test")
				}

				var collected metricdata.ResourceMetrics
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())

				activeMetric := findMetricByName(collected, "http_active_requests")
				Expect(activeMetric).ToNot(BeNil(), "http_active_requests metric should exist")

				activeSum, ok := activeMetric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "metric should be Sum[int64]")

				for _, dp := range activeSum.DataPoints {
					total += dp.Value
				}
			})

			It("should have total value of 5", func() {
				Expect(total).To(Equal(int64(5)))
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
			var durationHistogram metricdata.Histogram[float64]
			var requestsSum metricdata.Sum[int64]
			var activeSum metricdata.Sum[int64]
			var hasErrorsMetric bool
			var errorsSum metricdata.Sum[int64]

			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "GET", "/api/test")
				metrics.RequestFinished(context.Background(), "GET", "/api/test", 200, 100*time.Millisecond)

				var collected metricdata.ResourceMetrics
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())

				// Duration histogram
				durationMetric := findMetricByName(collected, "http_request_duration_seconds")
				Expect(durationMetric).ToNot(BeNil(), "http_request_duration_seconds should exist")
				var ok bool
				durationHistogram, ok = durationMetric.Data.(metricdata.Histogram[float64])
				Expect(ok).To(BeTrue(), "duration metric should be Histogram[float64]")

				// Requests counter
				requestsMetric := findMetricByName(collected, "http_requests_total")
				Expect(requestsMetric).ToNot(BeNil(), "http_requests_total should exist")
				requestsSum, ok = requestsMetric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "requests metric should be Sum[int64]")

				// Errors counter (may not exist if no errors recorded)
				errorsMetric := findMetricByName(collected, "http_errors_total")
				hasErrorsMetric = errorsMetric != nil
				if hasErrorsMetric {
					errorsSum, ok = errorsMetric.Data.(metricdata.Sum[int64])
					Expect(ok).To(BeTrue(), "errors metric should be Sum[int64]")
				}

				// Active requests
				activeMetric := findMetricByName(collected, "http_active_requests")
				Expect(activeMetric).ToNot(BeNil(), "http_active_requests should exist")
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

			It("should not record any errors for 2xx status", func() {
				if hasErrorsMetric {
					Expect(errorsSum.DataPoints).To(BeEmpty())
				}
			})

			It("should decrement active requests back to zero", func() {
				Expect(activeSum.DataPoints).To(HaveLen(1))
				Expect(activeSum.DataPoints[0].Value).To(Equal(int64(0)))
			})
		})

		When("recording a client error (4xx)", func() {
			var requestsSum metricdata.Sum[int64]
			var hasErrorsMetric bool
			var errorsSum metricdata.Sum[int64]

			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "GET", "/not-found")
				metrics.RequestFinished(context.Background(), "GET", "/not-found", 404, 5*time.Millisecond)

				var collected metricdata.ResourceMetrics
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())

				// Errors counter (may not exist for 4xx)
				errorsMetric := findMetricByName(collected, "http_errors_total")
				hasErrorsMetric = errorsMetric != nil
				if hasErrorsMetric {
					var ok bool
					errorsSum, ok = errorsMetric.Data.(metricdata.Sum[int64])
					Expect(ok).To(BeTrue(), "errors metric should be Sum[int64]")
				}

				// Requests counter
				requestsMetric := findMetricByName(collected, "http_requests_total")
				Expect(requestsMetric).ToNot(BeNil(), "http_requests_total should exist")
				var ok bool
				requestsSum, ok = requestsMetric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "requests metric should be Sum[int64]")
			})

			It("should not record any errors for 4xx status", func() {
				if hasErrorsMetric {
					Expect(errorsSum.DataPoints).To(BeEmpty())
				}
			})

			It("should still increment request counter", func() {
				Expect(requestsSum.DataPoints).To(HaveLen(1))
				Expect(requestsSum.DataPoints[0].Value).To(Equal(int64(1)))
			})
		})

		When("recording a server error (5xx)", func() {
			var errorsSum metricdata.Sum[int64]
			var requestsSum metricdata.Sum[int64]

			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "POST", "/api/error")
				metrics.RequestFinished(context.Background(), "POST", "/api/error", 500, 50*time.Millisecond)

				var collected metricdata.ResourceMetrics
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())

				// Errors counter
				errorsMetric := findMetricByName(collected, "http_errors_total")
				Expect(errorsMetric).ToNot(BeNil(), "http_errors_total should exist for 5xx")
				var ok bool
				errorsSum, ok = errorsMetric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "errors metric should be Sum[int64]")

				// Requests counter
				requestsMetric := findMetricByName(collected, "http_requests_total")
				Expect(requestsMetric).ToNot(BeNil(), "http_requests_total should exist")
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

		When("recording boundary status code (499 vs 500)", func() {
			var totalErrors int64

			BeforeEach(func() {
				totalErrors = 0
				// 499 is a client error (should not count as server error)
				metrics.RequestFinished(context.Background(), "GET", "/test", 499, 10*time.Millisecond)
				// 500 is a server error (should count)
				metrics.RequestFinished(context.Background(), "GET", "/test", 500, 10*time.Millisecond)

				var collected metricdata.ResourceMetrics
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())

				errorsMetric := findMetricByName(collected, "http_errors_total")
				Expect(errorsMetric).ToNot(BeNil(), "http_errors_total should exist")

				errorsSum, ok := errorsMetric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "errors metric should be Sum[int64]")

				for _, dp := range errorsSum.DataPoints {
					totalErrors += dp.Value
				}
			})

			It("should only count 500 as error (not 499)", func() {
				Expect(totalErrors).To(Equal(int64(1)))
			})
		})

		When("recording multiple requests with various status codes", func() {
			var totalCount uint64
			var totalErrors int64

			BeforeEach(func() {
				totalCount = 0
				totalErrors = 0

				// Mix of successful and error requests
				metrics.RequestFinished(context.Background(), "GET", "/test", 200, 10*time.Millisecond)
				metrics.RequestFinished(context.Background(), "GET", "/test", 201, 10*time.Millisecond)
				metrics.RequestFinished(context.Background(), "GET", "/test", 404, 10*time.Millisecond)
				metrics.RequestFinished(context.Background(), "GET", "/test", 500, 10*time.Millisecond)
				metrics.RequestFinished(context.Background(), "GET", "/test", 503, 10*time.Millisecond)

				var collected metricdata.ResourceMetrics
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())

				// Duration histogram
				durationMetric := findMetricByName(collected, "http_request_duration_seconds")
				Expect(durationMetric).ToNot(BeNil(), "http_request_duration_seconds should exist")

				durationHistogram, ok := durationMetric.Data.(metricdata.Histogram[float64])
				Expect(ok).To(BeTrue(), "duration metric should be Histogram[float64]")

				for _, dp := range durationHistogram.DataPoints {
					totalCount += dp.Count
				}

				// Errors counter
				errorsMetric := findMetricByName(collected, "http_errors_total")
				Expect(errorsMetric).ToNot(BeNil(), "http_errors_total should exist")

				errorsSum, ok := errorsMetric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "errors metric should be Sum[int64]")

				for _, dp := range errorsSum.DataPoints {
					totalErrors += dp.Value
				}
			})

			It("should count all 5 requests in histogram", func() {
				Expect(totalCount).To(Equal(uint64(5)))
			})

			It("should count only 5xx as errors (2 total)", func() {
				Expect(totalErrors).To(Equal(int64(2)))
			})
		})
	})
})
