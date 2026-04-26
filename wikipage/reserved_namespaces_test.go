package wikipage_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/wikipage"
)

var _ = Describe("IsReservedTopLevelKey", func() {
	When("the key names the wiki namespace", func() {
		var result bool

		BeforeEach(func() {
			result = wikipage.IsReservedTopLevelKey("wiki")
		})

		It("should be reserved", func() {
			Expect(result).To(BeTrue())
		})
	})

	When("the key names the agent namespace", func() {
		var result bool

		BeforeEach(func() {
			result = wikipage.IsReservedTopLevelKey("agent")
		})

		It("should be reserved", func() {
			Expect(result).To(BeTrue())
		})
	})

	When("the key is an unrelated namespace", func() {
		var result bool

		BeforeEach(func() {
			result = wikipage.IsReservedTopLevelKey("foo")
		})

		It("should not be reserved", func() {
			Expect(result).To(BeFalse())
		})
	})

	When("the key is empty", func() {
		var result bool

		BeforeEach(func() {
			result = wikipage.IsReservedTopLevelKey("")
		})

		It("should not be reserved", func() {
			Expect(result).To(BeFalse())
		})
	})

	When("the key differs only by case", func() {
		var result bool

		BeforeEach(func() {
			result = wikipage.IsReservedTopLevelKey("WIKI")
		})

		It("should not be reserved (comparison is case-sensitive)", func() {
			Expect(result).To(BeFalse())
		})
	})
})
