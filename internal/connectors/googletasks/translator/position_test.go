//revive:disable:dot-imports
package translator_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/internal/connectors/googletasks/translator"
)

var _ = Describe("PositionToSortOrder", func() {
	When("position is empty", func() {
		It("should return 0", func() {
			Expect(translator.PositionToSortOrder("")).To(Equal(int64(0)))
		})
	})

	When("position is all zeros", func() {
		It("should return 0", func() {
			Expect(translator.PositionToSortOrder("00000000000000000000")).To(Equal(int64(0)))
		})
	})

	When("position is a typical 20-digit zero-padded value", func() {
		It("should return the numeric value", func() {
			Expect(translator.PositionToSortOrder("00000000000000001000")).To(Equal(int64(1000)))
		})
	})

	When("position has non-zero leading digit", func() {
		It("should return the numeric value", func() {
			Expect(translator.PositionToSortOrder("12345")).To(Equal(int64(12345)))
		})
	})

	When("position contains non-decimal characters", func() {
		It("should clamp to int64 max rather than silently zero", func() {
			Expect(translator.PositionToSortOrder("not-a-number")).To(Equal(int64(1<<63 - 1)))
		})
	})

	When("two positions are compared", func() {
		It("should preserve relative order across the conversion", func() {
			a := translator.PositionToSortOrder("00000000000000000500")
			b := translator.PositionToSortOrder("00000000000000001000")
			Expect(a < b).To(BeTrue())
		})
	})
})

var _ = Describe("SortOrderToPosition", func() {
	When("sort_order is zero", func() {
		It("should return a 20-character all-zero string", func() {
			Expect(translator.SortOrderToPosition(0)).To(Equal("00000000000000000000"))
		})
	})

	When("sort_order is a typical wiki value (1000)", func() {
		It("should zero-pad to 20 characters", func() {
			Expect(translator.SortOrderToPosition(1000)).To(Equal("00000000000000001000"))
		})
	})

	When("sort_order is negative", func() {
		It("should clamp to zero", func() {
			Expect(translator.SortOrderToPosition(-5)).To(Equal("00000000000000000000"))
		})
	})

	When("sort_order is large", func() {
		It("should still produce a 20-character string", func() {
			result := translator.SortOrderToPosition(1<<60 - 1)
			Expect(len(result)).To(Equal(20))
		})
	})
})

var _ = Describe("PositionToSortOrder ↔ SortOrderToPosition order preservation", func() {
	When("positions are produced from sort_orders", func() {
		It("should preserve relative ordering", func() {
			sortOrders := []int64{500, 1000, 2000, 12345}
			var prev string
			for i, so := range sortOrders {
				p := translator.SortOrderToPosition(so)
				if i > 0 {
					Expect(prev < p).To(BeTrue(),
						"position %q should sort before %q for ascending sort_orders", prev, p)
				}
				prev = p
			}
		})
	})

	When("a sort_order is round-tripped through SortOrderToPosition→PositionToSortOrder", func() {
		It("should return the original sort_order", func() {
			Expect(translator.PositionToSortOrder(translator.SortOrderToPosition(1000))).To(Equal(int64(1000)))
		})
	})
})
