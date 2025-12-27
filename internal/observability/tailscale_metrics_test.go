//revive:disable:dot-imports
package observability_test

import (
	"context"
	"time"

	"github.com/brendanjerwin/simple_wiki/internal/observability"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TailscaleMetrics", func() {
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
			BeforeEach(func() {
				metrics.RecordLookup(context.Background(), 0.005, observability.ResultSuccess)
			})

			It("should not panic", func() {
				// The test passes if no panic occurs
			})
		})

		When("recording a failed lookup", func() {
			BeforeEach(func() {
				metrics.RecordLookup(context.Background(), 0.010, observability.ResultFailure)
			})

			It("should not panic", func() {
				// The test passes if no panic occurs
			})
		})

		When("recording a not-tailnet lookup", func() {
			BeforeEach(func() {
				metrics.RecordLookup(context.Background(), 0.001, observability.ResultNotTailnet)
			})

			It("should not panic", func() {
				// The test passes if no panic occurs
			})
		})

		When("recording all result types", func() {
			BeforeEach(func() {
				metrics.RecordLookup(context.Background(), 0.005, observability.ResultSuccess)
				metrics.RecordLookup(context.Background(), 0.010, observability.ResultFailure)
				metrics.RecordLookup(context.Background(), 0.001, observability.ResultNotTailnet)
			})

			It("should handle all result types without panic", func() {
				// The test passes if no panic occurs
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
			BeforeEach(func() {
				metrics.RecordLookupDuration(context.Background(), 5*time.Millisecond, observability.ResultSuccess)
			})

			It("should not panic", func() {
				// The test passes if no panic occurs
			})
		})

		When("recording various durations", func() {
			BeforeEach(func() {
				metrics.RecordLookupDuration(context.Background(), 100*time.Microsecond, observability.ResultSuccess)
				metrics.RecordLookupDuration(context.Background(), 1*time.Millisecond, observability.ResultSuccess)
				metrics.RecordLookupDuration(context.Background(), 10*time.Millisecond, observability.ResultFailure)
				metrics.RecordLookupDuration(context.Background(), 100*time.Millisecond, observability.ResultNotTailnet)
			})

			It("should handle all durations without panic", func() {
				// The test passes if no panic occurs
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
			BeforeEach(func() {
				metrics.RecordFromHeaders(context.Background())
			})

			It("should not panic", func() {
				// The test passes if no panic occurs
			})
		})

		When("recording multiple header extractions", func() {
			BeforeEach(func() {
				for i := 0; i < 10; i++ {
					metrics.RecordFromHeaders(context.Background())
				}
			})

			It("should handle multiple calls without panic", func() {
				// The test passes if no panic occurs
			})
		})
	})
})

var _ = Describe("IdentityLookupResult", func() {
	Describe("Result constants", func() {
		It("should have ResultSuccess defined", func() {
			Expect(string(observability.ResultSuccess)).To(Equal("success"))
		})

		It("should have ResultFailure defined", func() {
			Expect(string(observability.ResultFailure)).To(Equal("failure"))
		})

		It("should have ResultNotTailnet defined", func() {
			Expect(string(observability.ResultNotTailnet)).To(Equal("not_tailnet"))
		})
	})
})
