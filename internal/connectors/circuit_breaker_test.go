//revive:disable:dot-imports
package connectors_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors"
)

var _ = Describe("CircuitBreaker", func() {
	var cb *connectors.CircuitBreaker

	BeforeEach(func() {
		cb = connectors.NewCircuitBreakerForTest()
	})

	It("allows attempts while closed", func() {
		Expect(cb.Allow()).To(BeTrue())
	})

	It("opens after the default failure threshold", func() {
		for range 4 {
			cb.RecordFailure()
			Expect(cb.Allow()).To(BeTrue())
		}
		cb.RecordFailure()
		Expect(cb.Allow()).To(BeFalse(), "fifth consecutive failure should open the circuit")
	})

	It("blocks Allow when open", func() {
		for range 5 {
			cb.RecordFailure()
		}
		Expect(cb.Allow()).To(BeFalse())
	})

	It("transitions to half-open after cooldown", func() {
		cb.SetCooldown(50 * time.Millisecond)
		for range 5 {
			cb.RecordFailure()
		}
		Expect(cb.Allow()).To(BeFalse())

		time.Sleep(75 * time.Millisecond)
		Expect(cb.Allow()).To(BeTrue())
		// Second half-open probe should still be allowed because the test
		// harness exposes half-open as allowing a single probe.
		Expect(cb.Allow()).To(BeFalse())
	})

	It("closes on success after half-open", func() {
		cb.SetCooldown(50 * time.Millisecond)
		for range 5 {
			cb.RecordFailure()
		}
		time.Sleep(75 * time.Millisecond)
		Expect(cb.Allow()).To(BeTrue())

		cb.RecordSuccess()
		Expect(cb.Allow()).To(BeTrue())
		Expect(cb.Allow()).To(BeTrue())
	})

	It("resets failure count on success", func() {
		cb.RecordFailure()
		cb.RecordFailure()
		cb.RecordSuccess()
		cb.RecordFailure()
		cb.RecordFailure()
		cb.RecordFailure()
		cb.RecordFailure()
		// Only four failures since the success reset, so it should not open.
		Expect(cb.Allow()).To(BeTrue())
	})
})
