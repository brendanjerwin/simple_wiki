package slicetools_test

import (
	"testing"

	"github.com/brendanjerwin/simple_wiki/utils/slicetools"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "slicetools Suite")
}

var _ = Describe("slicetools", func() {
	Describe("ReverseSlice", func() {
		Describe("ReverseSliceInt64", func() {
			var (
				slice    []int64
				reversed []int64
			)

			BeforeEach(func() {
				slice = []int64{1, 2, 3, 4, 5}
				reversed = slicetools.ReverseSliceInt64(slice)
			})

			It("should reverse a slice of int64", func() {
				Expect(reversed).To(Equal([]int64{5, 4, 3, 2, 1}))
			})
		})

		Describe("ReverseSliceString", func() {
			var (
				slice    []string
				reversed []string
			)

			BeforeEach(func() {
				slice = []string{"apple", "banana", "cherry"}
				reversed = slicetools.ReverseSliceString(slice)
			})

			It("should reverse a slice of strings", func() {
				Expect(reversed).To(Equal([]string{"cherry", "banana", "apple"}))
			})
		})

		Describe("ReverseSliceInt", func() {
			var (
				slice    []int
				reversed []int
			)

			BeforeEach(func() {
				slice = []int{1, 2, 3, 4, 5}
				reversed = slicetools.ReverseSliceInt(slice)
			})

			It("should reverse a slice of int", func() {
				Expect(reversed).To(Equal([]int{5, 4, 3, 2, 1}))
			})
		})
	})
})
