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
			var collected metricdata.ResourceMetrics

			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "/test.Service/Method")
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())
			})

			It("should increment active requests counter", func() {
				metric := findMetricByName(collected, "grpc_active_requests")
				Expect(metric).ToNot(BeNil())
				sum, ok := metric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				Expect(sum.DataPoints).To(HaveLen(1))
				Expect(sum.DataPoints[0].Value).To(Equal(int64(1)))
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
			var collected metricdata.ResourceMetrics

			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "/test.Service/Method")
				metrics.RequestFinished(context.Background(), "/test.Service/Method", "OK", 100*time.Millisecond)
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())
			})

			It("should record request duration in histogram", func() {
				metric := findMetricByName(collected, "grpc_request_duration_seconds")
				Expect(metric).ToNot(BeNil())
				histogram, ok := metric.Data.(metricdata.Histogram[float64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				Expect(histogram.DataPoints).To(HaveLen(1))
				Expect(histogram.DataPoints[0].Count).To(Equal(uint64(1)))
			})

			It("should increment request counter", func() {
				metric := findMetricByName(collected, "grpc_requests_total")
				Expect(metric).ToNot(BeNil())
				sum, ok := metric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				Expect(sum.DataPoints).To(HaveLen(1))
				Expect(sum.DataPoints[0].Value).To(Equal(int64(1)))
			})

			It("should not record any errors for OK status", func() {
				metric := findMetricByName(collected, "grpc_errors_total")
				// Error counter should not have data for OK status
				if metric == nil {
					Succeed() // No error metric at all - explicitly acceptable
				return
				}
				sum, ok := metric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "expected Sum[int64] but got different type")
				// Verify no data points exist for OK status
				Expect(sum.DataPoints).To(BeEmpty())
			})

			It("should decrement active requests back to zero", func() {
				metric := findMetricByName(collected, "grpc_active_requests")
				Expect(metric).ToNot(BeNil())
				sum, ok := metric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				Expect(sum.DataPoints).To(HaveLen(1))
				Expect(sum.DataPoints[0].Value).To(Equal(int64(0)))
			})
		})

		When("recording a failed request", func() {
			var collected metricdata.ResourceMetrics

			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "/test.Service/Method")
				metrics.RequestFinished(context.Background(), "/test.Service/Method", "INTERNAL", 50*time.Millisecond)
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())
			})

			It("should increment error counter for non-OK status", func() {
				metric := findMetricByName(collected, "grpc_errors_total")
				Expect(metric).ToNot(BeNil())
				sum, ok := metric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				Expect(sum.DataPoints).To(HaveLen(1))
				Expect(sum.DataPoints[0].Value).To(Equal(int64(1)))
			})

			It("should still increment request counter", func() {
				metric := findMetricByName(collected, "grpc_requests_total")
				Expect(metric).ToNot(BeNil())
				sum, ok := metric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				Expect(sum.DataPoints).To(HaveLen(1))
				Expect(sum.DataPoints[0].Value).To(Equal(int64(1)))
			})
		})

		When("recording multiple requests with different statuses", func() {
			var collected metricdata.ResourceMetrics

			BeforeEach(func() {
				// Two OK requests
				metrics.RequestFinished(context.Background(), "/test.Service/Method", "OK", 10*time.Millisecond)
				metrics.RequestFinished(context.Background(), "/test.Service/Method", "OK", 20*time.Millisecond)
				// One error
				metrics.RequestFinished(context.Background(), "/test.Service/Method", "UNAVAILABLE", 30*time.Millisecond)
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())
			})

			It("should count all requests in histogram", func() {
				metric := findMetricByName(collected, "grpc_request_duration_seconds")
				Expect(metric).ToNot(BeNil())
				histogram, ok := metric.Data.(metricdata.Histogram[float64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				// Count total across all data points
				var totalCount uint64
				for _, dp := range histogram.DataPoints {
					totalCount += dp.Count
				}
				Expect(totalCount).To(Equal(uint64(3)))
			})
		})
	})
})
