//revive:disable:dot-imports
package observability_test

import (
	"github.com/brendanjerwin/simple_wiki/internal/observability"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// mockCounter tracks calls for testing CompositeRequestCounter.
type mockCounter struct {
	httpRequests int
	httpErrors   int
	grpcRequests int
	grpcErrors   int
}

func (m *mockCounter) RecordHTTPRequest() { m.httpRequests++ }
func (m *mockCounter) RecordHTTPError()   { m.httpErrors++ }
func (m *mockCounter) RecordGRPCRequest() { m.grpcRequests++ }
func (m *mockCounter) RecordGRPCError()   { m.grpcErrors++ }

var _ = Describe("CompositeRequestCounter", func() {
	Describe("NewCompositeRequestCounter", func() {
		When("created with nil counters", func() {
			var composite *observability.CompositeRequestCounter

			BeforeEach(func() {
				composite = observability.NewCompositeRequestCounter(nil, nil)
			})

			It("should not panic when recording", func() {
				Expect(func() { composite.RecordHTTPRequest() }).ToNot(Panic())
				Expect(func() { composite.RecordHTTPError() }).ToNot(Panic())
				Expect(func() { composite.RecordGRPCRequest() }).ToNot(Panic())
				Expect(func() { composite.RecordGRPCError() }).ToNot(Panic())
			})
		})

		When("created with a mix of nil and valid counters", func() {
			var composite *observability.CompositeRequestCounter
			var counter *mockCounter

			BeforeEach(func() {
				counter = &mockCounter{}
				composite = observability.NewCompositeRequestCounter(nil, counter, nil)
			})

			It("should filter out nil counters and call valid ones", func() {
				composite.RecordHTTPRequest()
				Expect(counter.httpRequests).To(Equal(1))
			})
		})
	})

	Describe("RecordHTTPRequest", func() {
		When("recording with multiple counters", func() {
			var counter1, counter2 *mockCounter

			BeforeEach(func() {
				counter1 = &mockCounter{}
				counter2 = &mockCounter{}
				composite := observability.NewCompositeRequestCounter(counter1, counter2)
				composite.RecordHTTPRequest()
			})

			It("should call all registered counters", func() {
				Expect(counter1.httpRequests).To(Equal(1))
				Expect(counter2.httpRequests).To(Equal(1))
			})
		})
	})

	Describe("RecordHTTPError", func() {
		When("recording with multiple counters", func() {
			var counter1, counter2 *mockCounter

			BeforeEach(func() {
				counter1 = &mockCounter{}
				counter2 = &mockCounter{}
				composite := observability.NewCompositeRequestCounter(counter1, counter2)
				composite.RecordHTTPError()
			})

			It("should call all registered counters", func() {
				Expect(counter1.httpErrors).To(Equal(1))
				Expect(counter2.httpErrors).To(Equal(1))
			})
		})
	})

	Describe("RecordGRPCRequest", func() {
		When("recording with multiple counters", func() {
			var counter1, counter2 *mockCounter

			BeforeEach(func() {
				counter1 = &mockCounter{}
				counter2 = &mockCounter{}
				composite := observability.NewCompositeRequestCounter(counter1, counter2)
				composite.RecordGRPCRequest()
			})

			It("should call all registered counters", func() {
				Expect(counter1.grpcRequests).To(Equal(1))
				Expect(counter2.grpcRequests).To(Equal(1))
			})
		})
	})

	Describe("RecordGRPCError", func() {
		When("recording with multiple counters", func() {
			var counter1, counter2 *mockCounter

			BeforeEach(func() {
				counter1 = &mockCounter{}
				counter2 = &mockCounter{}
				composite := observability.NewCompositeRequestCounter(counter1, counter2)
				composite.RecordGRPCError()
			})

			It("should call all registered counters", func() {
				Expect(counter1.grpcErrors).To(Equal(1))
				Expect(counter2.grpcErrors).To(Equal(1))
			})
		})
	})
})
