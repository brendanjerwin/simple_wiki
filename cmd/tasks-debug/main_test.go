//revive:disable:dot-imports
package main

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTasksDebug(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "cmd/tasks-debug")
}

var _ = Describe("parseDueFlag", func() {
	When("the input is empty", func() {
		var (
			result time.Time
			err    error
		)

		BeforeEach(func() {
			result, err = parseDueFlag("")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the zero time", func() {
			Expect(result.IsZero()).To(BeTrue())
		})
	})

	When("the input is a valid YYYY-MM-DD", func() {
		var (
			result time.Time
			err    error
		)

		BeforeEach(func() {
			result, err = parseDueFlag("2026-05-01")
		})

		It("should not error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the parsed date", func() {
			Expect(result.Year()).To(Equal(2026))
			Expect(result.Month()).To(Equal(time.May))
			Expect(result.Day()).To(Equal(1))
		})
	})

	When("the input is malformed", func() {
		var err error

		BeforeEach(func() {
			_, err = parseDueFlag("not a date")
		})

		It("should error", func() {
			Expect(err).To(HaveOccurred())
		})
	})
})

var _ = Describe("stringsEqual", func() {
	When("the slices match element-wise", func() {
		It("should return true", func() {
			Expect(stringsEqual([]string{"a", "b"}, []string{"a", "b"})).To(BeTrue())
		})
	})

	When("the slices are different lengths", func() {
		It("should return false", func() {
			Expect(stringsEqual([]string{"a"}, []string{"a", "b"})).To(BeFalse())
		})
	})

	When("the slices differ at one position", func() {
		It("should return false", func() {
			Expect(stringsEqual([]string{"a", "b"}, []string{"a", "c"})).To(BeFalse())
		})
	})
})

var _ = Describe("stringsEqualReversed", func() {
	When("the slices are reverses", func() {
		It("should return true", func() {
			Expect(stringsEqualReversed([]string{"a", "b", "c"}, []string{"c", "b", "a"})).To(BeTrue())
		})
	})

	When("the slices are equal but not reversed mirrors of each other", func() {
		// "a","a" reversed is "a","a" — palindromic case is true.
		It("should treat palindromes as reversed equal", func() {
			Expect(stringsEqualReversed([]string{"a", "a"}, []string{"a", "a"})).To(BeTrue())
		})
	})

	When("the slices have different lengths", func() {
		It("should return false", func() {
			Expect(stringsEqualReversed([]string{"a"}, []string{"a", "b"})).To(BeFalse())
		})
	})
})
