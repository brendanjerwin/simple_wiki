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

var _ = Describe("TailscaleMetrics", func() {
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

	Describe("NewTailscaleMetrics", func() {
		When("creating new metrics", func() {
			var metrics *observability.TailscaleMetrics
			var err error

			BeforeEach(func() {
				metrics, err = observability.NewTailscaleMetrics()
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return a non-nil metrics instance", func() {
				Expect(metrics).ToNot(BeNil())
			})
		})
	})

	Describe("RecordLookup", func() {
		var metrics *observability.TailscaleMetrics

		BeforeEach(func() {
			var err error
			metrics, err = observability.NewTailscaleMetrics()
			Expect(err).ToNot(HaveOccurred())
		})

		When("recording a successful lookup", func() {
			var collected metricdata.ResourceMetrics

			BeforeEach(func() {
				metrics.RecordLookup(context.Background(), 0.005, observability.ResultSuccess)
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())
			})

			It("should record duration in histogram", func() {
				metric := findMetricByName(collected, "tailscale_identity_lookup_duration_seconds")
				Expect(metric).ToNot(BeNil())
				histogram, ok := metric.Data.(metricdata.Histogram[float64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				Expect(histogram.DataPoints).To(HaveLen(1))
				Expect(histogram.DataPoints[0].Count).To(Equal(uint64(1)))
			})

			It("should increment lookup counter with success result", func() {
				metric := findMetricByName(collected, "tailscale_identity_lookup_total")
				Expect(metric).ToNot(BeNil())
				sum, ok := metric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				Expect(sum.DataPoints).To(HaveLen(1))
				Expect(sum.DataPoints[0].Value).To(Equal(int64(1)))
			})
		})

		When("recording a failed lookup", func() {
			var collected metricdata.ResourceMetrics

			BeforeEach(func() {
				metrics.RecordLookup(context.Background(), 0.010, observability.ResultFailure)
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())
			})

			It("should record duration in histogram", func() {
				metric := findMetricByName(collected, "tailscale_identity_lookup_duration_seconds")
				Expect(metric).ToNot(BeNil())
				histogram, ok := metric.Data.(metricdata.Histogram[float64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				Expect(histogram.DataPoints).To(HaveLen(1))
				Expect(histogram.DataPoints[0].Count).To(Equal(uint64(1)))
			})

			It("should increment lookup counter with failure result", func() {
				metric := findMetricByName(collected, "tailscale_identity_lookup_total")
				Expect(metric).ToNot(BeNil())
				sum, ok := metric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				Expect(sum.DataPoints).To(HaveLen(1))
				Expect(sum.DataPoints[0].Value).To(Equal(int64(1)))
			})
		})

		When("recording a not-tailnet lookup", func() {
			var collected metricdata.ResourceMetrics

			BeforeEach(func() {
				metrics.RecordLookup(context.Background(), 0.001, observability.ResultNotTailnet)
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())
			})

			It("should increment lookup counter with not_tailnet result", func() {
				metric := findMetricByName(collected, "tailscale_identity_lookup_total")
				Expect(metric).ToNot(BeNil())
				sum, ok := metric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				Expect(sum.DataPoints).To(HaveLen(1))
				Expect(sum.DataPoints[0].Value).To(Equal(int64(1)))
			})
		})

		When("recording all result types", func() {
			var collected metricdata.ResourceMetrics

			BeforeEach(func() {
				metrics.RecordLookup(context.Background(), 0.005, observability.ResultSuccess)
				metrics.RecordLookup(context.Background(), 0.010, observability.ResultFailure)
				metrics.RecordLookup(context.Background(), 0.001, observability.ResultNotTailnet)
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())
			})

			It("should record all lookups in histogram", func() {
				metric := findMetricByName(collected, "tailscale_identity_lookup_duration_seconds")
				Expect(metric).ToNot(BeNil())
				histogram, ok := metric.Data.(metricdata.Histogram[float64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				var totalCount uint64
				for _, dp := range histogram.DataPoints {
					totalCount += dp.Count
				}
				Expect(totalCount).To(Equal(uint64(3)))
			})

			It("should have separate counter data points for each result type", func() {
				metric := findMetricByName(collected, "tailscale_identity_lookup_total")
				Expect(metric).ToNot(BeNil())
				sum, ok := metric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				// Each result type creates a separate data point due to different attributes
				Expect(sum.DataPoints).To(HaveLen(3))
				// Total count should be 3
				var total int64
				for _, dp := range sum.DataPoints {
					total += dp.Value
				}
				Expect(total).To(Equal(int64(3)))
			})
		})
	})

	Describe("RecordLookupDuration", func() {
		var metrics *observability.TailscaleMetrics

		BeforeEach(func() {
			var err error
			metrics, err = observability.NewTailscaleMetrics()
			Expect(err).ToNot(HaveOccurred())
		})

		When("recording with time.Duration", func() {
			var collected metricdata.ResourceMetrics

			BeforeEach(func() {
				metrics.RecordLookupDuration(context.Background(), 5*time.Millisecond, observability.ResultSuccess)
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())
			})

			It("should convert duration to seconds and record in histogram", func() {
				metric := findMetricByName(collected, "tailscale_identity_lookup_duration_seconds")
				Expect(metric).ToNot(BeNil())
				histogram, ok := metric.Data.(metricdata.Histogram[float64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				Expect(histogram.DataPoints).To(HaveLen(1))
				// 5ms = 0.005s
				Expect(histogram.DataPoints[0].Sum).To(BeNumerically("~", 0.005, 0.0001))
			})
		})
	})

	Describe("RecordFromHeaders", func() {
		var metrics *observability.TailscaleMetrics

		BeforeEach(func() {
			var err error
			metrics, err = observability.NewTailscaleMetrics()
			Expect(err).ToNot(HaveOccurred())
		})

		When("recording header extraction", func() {
			var collected metricdata.ResourceMetrics

			BeforeEach(func() {
				metrics.RecordFromHeaders(context.Background())
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())
			})

			It("should increment header extraction counter", func() {
				metric := findMetricByName(collected, "tailscale_identity_from_headers_total")
				Expect(metric).ToNot(BeNil())
				sum, ok := metric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				Expect(sum.DataPoints).To(HaveLen(1))
				Expect(sum.DataPoints[0].Value).To(Equal(int64(1)))
			})
		})

		When("recording multiple header extractions", func() {
			var collected metricdata.ResourceMetrics

			BeforeEach(func() {
				for i := 0; i < 10; i++ {
					metrics.RecordFromHeaders(context.Background())
				}
				Expect(reader.Collect(context.Background(), &collected)).To(Succeed())
			})

			It("should count all header extractions", func() {
				metric := findMetricByName(collected, "tailscale_identity_from_headers_total")
				Expect(metric).ToNot(BeNil())
				sum, ok := metric.Data.(metricdata.Sum[int64])
				Expect(ok).To(BeTrue(), "expected correct metric data type")
				Expect(sum.DataPoints).To(HaveLen(1))
				Expect(sum.DataPoints[0].Value).To(Equal(int64(10)))
			})
		})
	})
})
