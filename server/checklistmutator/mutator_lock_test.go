//revive:disable:dot-imports
package checklistmutator

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("lockPage", func() {
	When("page identifiers resolve to the same stored page", func() {
		var lockCount int

		BeforeEach(func() {
			m := &Mutator{}
			m.lockPage("ai-assistant")()
			m.lockPage("ai_assistant")()
			m.pageMu.Range(func(_, _ any) bool {
				lockCount++
				return true
			})
		})

		It("uses the same lock", func() {
			Expect(lockCount).To(Equal(1))
		})
	})
})
