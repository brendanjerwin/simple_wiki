//revive:disable:dot-imports
package observability_test

import (
	"context"
	"testing"
	"time"

	"github.com/brendanjerwin/simple_wiki/internal/observability"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestObservability(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Observability Suite")
}

var _ = Describe("TelemetryProvider", func() {
	Describe("Initialize", func() {
		When("OTEL_ENABLED is not set", func() {
			var (
				provider *observability.TelemetryProvider
				initErr  error
			)

			BeforeEach(func() {
				// Ensure the environment variable is not set
				provider, initErr = observability.Initialize(context.Background(), "test-version")
			})

			It("should not return an error", func() {
				Expect(initErr).ToNot(HaveOccurred())
			})

			It("should return a disabled provider", func() {
				Expect(provider.IsEnabled()).To(BeFalse())
			})

			It("should allow shutdown without error", func() {
				err := provider.Shutdown(context.Background())
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})

var _ = Describe("TailscaleMetrics", func() {
	Describe("NewTailscaleMetrics", func() {
		var (
			metrics   *observability.TailscaleMetrics
			createErr error
		)

		BeforeEach(func() {
			metrics, createErr = observability.NewTailscaleMetrics()
		})

		It("should not return an error", func() {
			Expect(createErr).ToNot(HaveOccurred())
		})

		It("should return a valid metrics instance", func() {
			Expect(metrics).ToNot(BeNil())
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
				metrics.RecordLookup(context.Background(), 0.05, observability.ResultSuccess)
			})

			It("should not panic", func() {
				// If we got here, the recording did not panic
				Expect(true).To(BeTrue())
			})
		})

		When("recording a failed lookup", func() {
			BeforeEach(func() {
				metrics.RecordLookup(context.Background(), 0.1, observability.ResultFailure)
			})

			It("should not panic", func() {
				Expect(true).To(BeTrue())
			})
		})

		When("recording a not_tailnet lookup", func() {
			BeforeEach(func() {
				metrics.RecordLookup(context.Background(), 0.001, observability.ResultNotTailnet)
			})

			It("should not panic", func() {
				Expect(true).To(BeTrue())
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
				metrics.RecordLookupDuration(context.Background(), 50*time.Millisecond, observability.ResultSuccess)
			})

			It("should not panic", func() {
				Expect(true).To(BeTrue())
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
				Expect(true).To(BeTrue())
			})
		})
	})
})

var _ = Describe("HTTPMetrics", func() {
	Describe("NewHTTPMetrics", func() {
		var (
			metrics   *observability.HTTPMetrics
			createErr error
		)

		BeforeEach(func() {
			metrics, createErr = observability.NewHTTPMetrics()
		})

		It("should not return an error", func() {
			Expect(createErr).ToNot(HaveOccurred())
		})

		It("should return a valid metrics instance", func() {
			Expect(metrics).ToNot(BeNil())
		})
	})

	Describe("RequestStarted", func() {
		var metrics *observability.HTTPMetrics

		BeforeEach(func() {
			var err error
			metrics, err = observability.NewHTTPMetrics()
			Expect(err).ToNot(HaveOccurred())
		})

		When("starting a request", func() {
			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "GET", "/test")
			})

			It("should not panic", func() {
				Expect(true).To(BeTrue())
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

		When("finishing a successful request", func() {
			BeforeEach(func() {
				metrics.RequestFinished(context.Background(), "GET", "/test", 200, 100*time.Millisecond)
			})

			It("should not panic", func() {
				Expect(true).To(BeTrue())
			})
		})

		When("finishing an error request", func() {
			BeforeEach(func() {
				metrics.RequestFinished(context.Background(), "POST", "/api/data", 500, 50*time.Millisecond)
			})

			It("should not panic", func() {
				Expect(true).To(BeTrue())
			})
		})
	})
})

var _ = Describe("GRPCMetrics", func() {
	Describe("NewGRPCMetrics", func() {
		var (
			metrics   *observability.GRPCMetrics
			createErr error
		)

		BeforeEach(func() {
			metrics, createErr = observability.NewGRPCMetrics()
		})

		It("should not return an error", func() {
			Expect(createErr).ToNot(HaveOccurred())
		})

		It("should return a valid metrics instance", func() {
			Expect(metrics).ToNot(BeNil())
		})
	})

	Describe("RequestStarted", func() {
		var metrics *observability.GRPCMetrics

		BeforeEach(func() {
			var err error
			metrics, err = observability.NewGRPCMetrics()
			Expect(err).ToNot(HaveOccurred())
		})

		When("starting a request", func() {
			BeforeEach(func() {
				metrics.RequestStarted(context.Background(), "/api.v1.TestService/TestMethod")
			})

			It("should not panic", func() {
				Expect(true).To(BeTrue())
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

		When("finishing a successful request", func() {
			BeforeEach(func() {
				metrics.RequestFinished(context.Background(), "/api.v1.TestService/TestMethod", "OK", 100*time.Millisecond)
			})

			It("should not panic", func() {
				Expect(true).To(BeTrue())
			})
		})

		When("finishing an error request", func() {
			BeforeEach(func() {
				metrics.RequestFinished(context.Background(), "/api.v1.TestService/TestMethod", "INTERNAL", 50*time.Millisecond)
			})

			It("should not panic", func() {
				Expect(true).To(BeTrue())
			})
		})
	})
})

var _ = Describe("GRPCInstrumentation", func() {
	Describe("NewGRPCInstrumentation", func() {
		var (
			metrics *observability.GRPCMetrics
			instr   *observability.GRPCInstrumentation
		)

		BeforeEach(func() {
			var err error
			metrics, err = observability.NewGRPCMetrics()
			Expect(err).ToNot(HaveOccurred())
			instr = observability.NewGRPCInstrumentation(metrics)
		})

		It("should return a valid instrumentation instance", func() {
			Expect(instr).ToNot(BeNil())
		})
	})

	Describe("UnaryServerInterceptor", func() {
		var (
			metrics *observability.GRPCMetrics
			instr   *observability.GRPCInstrumentation
		)

		BeforeEach(func() {
			var err error
			metrics, err = observability.NewGRPCMetrics()
			Expect(err).ToNot(HaveOccurred())
			instr = observability.NewGRPCInstrumentation(metrics)
		})

		It("should return a non-nil interceptor", func() {
			interceptor := instr.UnaryServerInterceptor()
			Expect(interceptor).ToNot(BeNil())
		})
	})

	Describe("StreamServerInterceptor", func() {
		var (
			metrics *observability.GRPCMetrics
			instr   *observability.GRPCInstrumentation
		)

		BeforeEach(func() {
			var err error
			metrics, err = observability.NewGRPCMetrics()
			Expect(err).ToNot(HaveOccurred())
			instr = observability.NewGRPCInstrumentation(metrics)
		})

		It("should return a non-nil interceptor", func() {
			interceptor := instr.StreamServerInterceptor()
			Expect(interceptor).ToNot(BeNil())
		})
	})
})

