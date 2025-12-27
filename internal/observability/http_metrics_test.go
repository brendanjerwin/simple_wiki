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
			var collected metricdata.ResourceMetrics

			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "GET", "/api/test")
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())
			})

			It("should increment active requests counter", func() {
				metric := findMetricByName(collected, "http_active_requests")
				Expect(metric).ToNot(BeNil())
				sum, ok := metric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				Expect(sum.DataPoints).To(HaveLen(1))
				Expect(sum.DataPoints[0].Value).To(Equal(int64(1)))
			})
		})

		When("recording start with various HTTP methods", func() {
			var collected metricdata.ResourceMetrics

			BeforeEach(func() {
				methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
				for _, method := range methods {
					metrics.RequestStarted(context.Background(), method, "/api/test")
				}
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())
			})

			It("should track all requests with different method attributes", func() {
				metric := findMetricByName(collected, "http_active_requests")
				Expect(metric).ToNot(BeNil())
				sum, ok := metric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				// Each method/path combo creates a separate data point
				Expect(len(sum.DataPoints)).To(BeNumerically(">=", 1))
				// Total value should be 5 (one for each method)
				var total int64
				for _, dp := range sum.DataPoints {
					total += dp.Value
				}
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
			var collected metricdata.ResourceMetrics

			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "GET", "/api/test")
				metrics.RequestFinished(context.Background(), "GET", "/api/test", 200, 100*time.Millisecond)
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())
			})

			It("should record request duration in histogram", func() {
				metric := findMetricByName(collected, "http_request_duration_seconds")
				Expect(metric).ToNot(BeNil())
				histogram, ok := metric.Data.(metricdata.Histogram[float64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				Expect(histogram.DataPoints).To(HaveLen(1))
				Expect(histogram.DataPoints[0].Count).To(Equal(uint64(1)))
			})

			It("should increment request counter", func() {
				metric := findMetricByName(collected, "http_requests_total")
				Expect(metric).ToNot(BeNil())
				sum, ok := metric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				Expect(sum.DataPoints).To(HaveLen(1))
				Expect(sum.DataPoints[0].Value).To(Equal(int64(1)))
			})

			It("should not record any errors for 2xx status", func() {
				metric := findMetricByName(collected, "http_errors_total")
				// Error counter should not have data for 2xx status
				if metric == nil {
					Succeed() // No error metric at all - explicitly acceptable
				return
				}
				sum, ok := metric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "expected Sum[int64] but got different type")
				// Verify no data points exist for 2xx status
				Expect(sum.DataPoints).To(BeEmpty())
			})

			It("should decrement active requests back to zero", func() {
				metric := findMetricByName(collected, "http_active_requests")
				Expect(metric).ToNot(BeNil())
				sum, ok := metric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				Expect(sum.DataPoints).To(HaveLen(1))
				Expect(sum.DataPoints[0].Value).To(Equal(int64(0)))
			})
		})

		When("recording a client error (4xx)", func() {
			var collected metricdata.ResourceMetrics

			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "GET", "/not-found")
				metrics.RequestFinished(context.Background(), "GET", "/not-found", 404, 5*time.Millisecond)
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())
			})

			It("should not record any errors for 4xx status", func() {
				metric := findMetricByName(collected, "http_errors_total")
				// 4xx errors should NOT be counted (only 5xx are server errors)
				if metric == nil {
					Succeed() // No error metric at all - explicitly acceptable
				return
				}
				sum, ok := metric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "expected Sum[int64] but got different type")
				// Verify no data points exist for 4xx status
				Expect(sum.DataPoints).To(BeEmpty())
			})

			It("should still increment request counter", func() {
				metric := findMetricByName(collected, "http_requests_total")
				Expect(metric).ToNot(BeNil())
				sum, ok := metric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				Expect(sum.DataPoints).To(HaveLen(1))
				Expect(sum.DataPoints[0].Value).To(Equal(int64(1)))
			})
		})

		When("recording a server error (5xx)", func() {
			var collected metricdata.ResourceMetrics

			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "POST", "/api/error")
				metrics.RequestFinished(context.Background(), "POST", "/api/error", 500, 50*time.Millisecond)
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())
			})

			It("should increment error counter for 5xx status", func() {
				metric := findMetricByName(collected, "http_errors_total")
				Expect(metric).ToNot(BeNil())
				sum, ok := metric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				Expect(sum.DataPoints).To(HaveLen(1))
				Expect(sum.DataPoints[0].Value).To(Equal(int64(1)))
			})

			It("should still increment request counter", func() {
				metric := findMetricByName(collected, "http_requests_total")
				Expect(metric).ToNot(BeNil())
				sum, ok := metric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				Expect(sum.DataPoints).To(HaveLen(1))
				Expect(sum.DataPoints[0].Value).To(Equal(int64(1)))
			})
		})

		When("recording boundary status code (499 vs 500)", func() {
			var collected metricdata.ResourceMetrics

			BeforeEach(func() {
				// 499 is a client error (should not count as server error)
				metrics.RequestFinished(context.Background(), "GET", "/test", 499, 10*time.Millisecond)
				// 500 is a server error (should count)
				metrics.RequestFinished(context.Background(), "GET", "/test", 500, 10*time.Millisecond)
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())
			})

			It("should only count 500 as error (not 499)", func() {
				metric := findMetricByName(collected, "http_errors_total")
				Expect(metric).ToNot(BeNil())
				sum, ok := metric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				// Should have exactly 1 error (the 500, not the 499)
				var totalErrors int64
				for _, dp := range sum.DataPoints {
					totalErrors += dp.Value
				}
				Expect(totalErrors).To(Equal(int64(1)))
			})
		})

		When("recording multiple requests with various status codes", func() {
			var collected metricdata.ResourceMetrics

			BeforeEach(func() {
				// Mix of successful and error requests
				metrics.RequestFinished(context.Background(), "GET", "/test", 200, 10*time.Millisecond)
				metrics.RequestFinished(context.Background(), "GET", "/test", 201, 10*time.Millisecond)
				metrics.RequestFinished(context.Background(), "GET", "/test", 404, 10*time.Millisecond)
				metrics.RequestFinished(context.Background(), "GET", "/test", 500, 10*time.Millisecond)
				metrics.RequestFinished(context.Background(), "GET", "/test", 503, 10*time.Millisecond)
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())
			})

			It("should count all requests in histogram", func() {
				metric := findMetricByName(collected, "http_request_duration_seconds")
				Expect(metric).ToNot(BeNil())
				histogram, ok := metric.Data.(metricdata.Histogram[float64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				var totalCount uint64
				for _, dp := range histogram.DataPoints {
					totalCount += dp.Count
				}
				Expect(totalCount).To(Equal(uint64(5)))
			})

			It("should count only 5xx as errors", func() {
				metric := findMetricByName(collected, "http_errors_total")
				Expect(metric).ToNot(BeNil())
				sum, ok := metric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				var totalErrors int64
				for _, dp := range sum.DataPoints {
					totalErrors += dp.Value
				}
				// Only 500 and 503 should count as errors
				Expect(totalErrors).To(Equal(int64(2)))
			})
		})
	})
})
