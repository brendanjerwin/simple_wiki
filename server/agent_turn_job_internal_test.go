//revive:disable:dot-imports
package server

import (
	"math"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("timeoutDurationSeconds", func() {
	When("the duration is a whole number of seconds", func() {
		It("should return that many seconds", func() {
			Expect(timeoutDurationSeconds(5 * time.Second)).To(Equal(int32(5)))
		})
	})

	When("the duration has a sub-second remainder", func() {
		It("should round up", func() {
			Expect(timeoutDurationSeconds(1500 * time.Millisecond)).To(Equal(int32(2)))
		})
	})

	When("the duration is zero", func() {
		It("should return zero", func() {
			Expect(timeoutDurationSeconds(0)).To(Equal(int32(0)))
		})
	})

	When("the duration exceeds the int32 second range (reclaimed zombie with a zero last_run)", func() {
		var seconds int32

		BeforeEach(func() {
			// time.Since(zero time) is ~2000 years — its seconds overflow int32.
			seconds = timeoutDurationSeconds(time.Since(time.Time{}))
		})

		It("should clamp to math.MaxInt32 instead of overflowing to a garbage value", func() {
			Expect(seconds).To(Equal(int32(math.MaxInt32)))
		})
	})
})
